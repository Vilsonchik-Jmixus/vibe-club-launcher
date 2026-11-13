package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const LauncherVersion = "1.0.14"

// DefaultManifestURL is where the pack list lives. It is replaced once the GitHub repo exists;
// settings.json "manifestUrl" or env VIBECLUB_MANIFEST override it.
const DefaultManifestURL = "https://raw.githubusercontent.com/Vilsonchik-Jmixus/vibe-club-launcher/main/pack/manifest.json"

//go:embed web
var webFS embed.FS

var (
	lastPing   time.Time
	lastPingMu sync.Mutex
	quitCh     = make(chan struct{})
	quitOnce   sync.Once
)

func quit() { quitOnce.Do(func() { close(quitCh) }) }

var uiURL string

func main() {
	loadSettings()
	appendLog("Запуск лаунчера " + LauncherVersion)
	cleanupOldBinary()
	applyPendingSkin()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		messageBox("Vibe Club", "Не удалось запустить лаунчер: "+err.Error())
		return
	}
	tok := make([]byte, 16)
	_, _ = rand.Read(tok)
	token := hex.EncodeToString(tok)

	mux := http.NewServeMux()
	sub, _ := fs.Sub(webFS, "web")
	static := http.StripPrefix("/"+token+"/", http.FileServer(http.FS(sub)))
	mux.Handle("/"+token+"/", noCache(static))
	registerAPI(mux, "/"+token+"/api/")
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)

	url := fmt.Sprintf("http://%s/%s/index.html", ln.Addr().String(), token)
	uiURL = url
	if os.Getenv("VC_HEADLESS") != "" {
		fmt.Println(url)
	}
	cmd, _ := openAppWindow(url)
	touchPing()
	if cmd != nil && cmd.Process != nil {
		go func() {
			_ = cmd.Wait()
			// Edge may hand the window to an already-running instance and exit at once;
			// only treat it as "window closed" if it ran for a while.
			waitIdleThenQuit()
		}()
	}
	go installDesktopEntry()
	go watchdog()
	<-quitCh
	if j := currentJob(); j != nil && j.status().Running {
		j.doCancel()
		time.Sleep(time.Second)
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	appendLog("Выход")
}

func touchPing() {
	lastPingMu.Lock()
	lastPing = time.Now()
	lastPingMu.Unlock()
}

func sincePing() time.Duration {
	lastPingMu.Lock()
	defer lastPingMu.Unlock()
	return time.Since(lastPing)
}

// watchdog exits when the UI stopped pinging (window closed) and nothing is running.
func watchdog() {
	for {
		time.Sleep(3 * time.Second)
		if os.Getenv("VC_HEADLESS") != "" {
			continue
		}
		busy := false
		if j := currentJob(); j != nil && j.status().Running {
			busy = true
		}
		if !busy && sincePing() > 25*time.Second {
			quit()
			return
		}
	}
}

func waitIdleThenQuit() {
	for {
		time.Sleep(2 * time.Second)
		if sincePing() > 25*time.Second {
			quit()
			return
		}
	}
}

func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------- API

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(400)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

type StateResp struct {
	LauncherVersion string      `json:"launcherVersion"`
	Settings        Settings    `json:"settings"`
	Installed       bool        `json:"installed"`
	LocalVersion    string      `json:"localVersion"`
	Remote          *RemoteInfo `json:"remote"`
	RemoteError     string      `json:"remoteError,omitempty"`
	UpdateAvailable bool        `json:"updateAvailable"`
	LauncherUpdate  *Release    `json:"launcherUpdate,omitempty"`
	FreeBytes       int64       `json:"freeBytes"`
	ProfileExists   bool        `json:"profileExists"`
	Job             *JobStatus  `json:"job,omitempty"`
	HasSkin         bool        `json:"hasSkin"`
	Profile         ProfileInfo `json:"profile"`
	SkinSlim        bool        `json:"skinSlim"`
}

type RemoteInfo struct {
	Pack      string     `json:"pack"`
	Version   string     `json:"version"`
	Minecraft string     `json:"minecraft"`
	Loader    Loader     `json:"loader"`
	Server    ServerInfo `json:"server"`
	News      []News     `json:"news"`
	Files     int        `json:"files"`
	Mods      int        `json:"mods"`
	Size      int64      `json:"size"`
}

func buildState(force bool) StateResp {
	s := getSettings()
	r := StateResp{LauncherVersion: LauncherVersion, Settings: s}
	r.Settings.PendingSkinHash = ""
	r.Installed = isInstalled(s.InstallDir)
	if r.Installed {
		r.LocalVersion = loadState(s.InstallDir).Version
	}
	r.FreeBytes = freeSpace(s.InstallDir)
	r.ProfileExists = profileExists()
	_, r.SkinSlim = currentSkin()
	_, err := os.Stat(filepath.Join(appDataDir(), "skin.png"))
	r.HasSkin = err == nil
	m, err := fetchManifest(force)
	if err != nil {
		r.RemoteError = err.Error()
	} else {
		ri := &RemoteInfo{Pack: m.Pack, Version: m.Version, Minecraft: m.Minecraft, Loader: m.Loader, Server: m.Server, News: m.News, Files: len(m.Files), Mods: m.ModCount}
		for _, f := range m.Files {
			ri.Size += f.Size
			if m.ModCount == 0 && strings.HasPrefix(f.Path, "mods/") && strings.HasSuffix(f.Path, ".jar") {
				ri.Mods++
			}
		}
		r.Remote = ri
		r.Profile = detectProfile(s.InstallDir, m)
		r.UpdateAvailable = r.Installed && r.LocalVersion != m.Version
		if lr := m.launcherRelease(); lr != nil && versionLess(LauncherVersion, lr.Version) {
			r.LauncherUpdate = lr
		}
	}
	if r.Profile.Kind == "" {
		r.Profile = detectProfile(s.InstallDir, nil)
	}
	if j := currentJob(); j != nil {
		st := j.status()
		r.Job = &st
	}
	return r
}

func versionLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			fmt.Sscan(pa[i], &x)
		}
		if i < len(pb) {
			fmt.Sscan(pb[i], &y)
		}
		if x != y {
			return x < y
		}
	}
	return false
}

func registerAPI(mux *http.ServeMux, p string) {
	h := func(name string, fn func(w http.ResponseWriter, r *http.Request)) {
		mux.HandleFunc(p+name, func(w http.ResponseWriter, r *http.Request) {
			touchPing()
			if r.Method != "GET" && r.Header.Get("X-VC") != "1" {
				w.WriteHeader(403)
				return
			}
			fn(w, r)
		})
	}
	h("ping", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, map[string]bool{"ok": true}) })
	h("state", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, buildState(r.URL.Query().Get("refresh") == "1"))
	})
	h("job", func(w http.ResponseWriter, r *http.Request) {
		if j := currentJob(); j != nil {
			writeJSON(w, j.status())
			return
		}
		writeJSON(w, JobStatus{})
	})
	h("server", func(w http.ResponseWriter, r *http.Request) {
		m, _ := fetchManifest(false)
		writeJSON(w, serverStatus(m))
	})
	h("start", func(w http.ResponseWriter, r *http.Request) {
		kind := JobKind(r.URL.Query().Get("kind"))
		switch kind {
		case JobInstall, JobUpdate, JobRepair, JobReinstall:
		default:
			writeErr(w, errors.New("неизвестная операция"))
			return
		}
		if _, err := startJob(kind); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	})
	h("pause", func(w http.ResponseWriter, r *http.Request) {
		if j := currentJob(); j != nil {
			j.set(func(s *JobStatus) { s.Paused = !s.Paused })
		}
		writeJSON(w, map[string]bool{"ok": true})
	})
	h("cancel", func(w http.ResponseWriter, r *http.Request) {
		if j := currentJob(); j != nil && j.status().Running {
			j.doCancel()
		}
		writeJSON(w, map[string]bool{"ok": true})
	})
	h("pick-folder", func(w http.ResponseWriter, r *http.Request) {
		cur := getSettings().InstallDir
		p, err := pickFolder(cur)
		if err != nil {
			writeErr(w, err)
			return
		}
		if p == "" {
			writeJSON(w, map[string]string{"path": ""})
			return
		}
		writeJSON(w, describeFolder(p))
	})
	h("browsers", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"current": browserSetting(), "options": browserList()})
	})
	// open-in-browser opens the launcher page again in the chosen browser right away.
	h("open-in-browser", func(w http.ResponseWriter, r *http.Request) {
		cmd, ok := openChosenWindow(uiURL)
		if !ok {
			cmd, _ = openAppWindow(uiURL)
		}
		_ = cmd
		writeJSON(w, map[string]bool{"ok": true})
	})
	h("apply-options", func(w http.ResponseWriter, r *http.Request) {
		dir := getSettings().InstallDir
		if !isInstalled(dir) {
			writeErr(w, errors.New("сначала установи сборку"))
			return
		}
		n, err := applyPackOptions(dir, true)
		if err != nil {
			writeErr(w, err)
			return
		}
		appendLog(fmt.Sprintf("Настройки сборки применены вручную: %d параметров", n))
		writeJSON(w, map[string]int{"changed": n})
	})
	// check-folder takes a path typed or pasted by the player and answers like pick-folder.
	h("check-folder", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&in); err != nil {
			writeErr(w, err)
			return
		}
		p, err := cleanUserPath(in.Path)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, describeFolder(p))
	})
	h("settings", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			InstallDir       *string `json:"installDir"`
			MemoryGB         *int    `json:"memoryGB"`
			CheckUpdates     *bool   `json:"checkUpdates"`
			CloseAfterLaunch *bool   `json:"closeAfterLaunch"`
			Browser          *string `json:"browser"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&in); err != nil {
			writeErr(w, err)
			return
		}
		if j := currentJob(); j != nil && j.status().Running && in.InstallDir != nil {
			writeErr(w, errors.New("дождись окончания загрузки"))
			return
		}
		s := updateSettings(func(s *Settings) {
			if in.InstallDir != nil && *in.InstallDir != "" {
				d := *in.InstallDir
				if strings.HasPrefix(d, "~") {
					if home, err := os.UserHomeDir(); err == nil {
						d = home + d[1:]
					}
				}
				s.InstallDir = filepath.Clean(d)
			}
			if in.MemoryGB != nil && *in.MemoryGB >= 2 && *in.MemoryGB <= 32 {
				s.MemoryGB = *in.MemoryGB
			}
			if in.CheckUpdates != nil {
				s.CheckUpdates = *in.CheckUpdates
			}
			if in.Browser != nil {
				s.Browser = *in.Browser
			}
			if in.CloseAfterLaunch != nil {
				s.CloseAfterLaunch = *in.CloseAfterLaunch
			}
		})
		if (in.MemoryGB != nil || in.InstallDir != nil) && profileExists() {
			if m, err := fetchManifest(false); err == nil && isInstalled(s.InstallDir) {
				_ = writeProfile(m)
			}
		}
		writeJSON(w, buildState(false))
	})
	h("open-folder", func(w http.ResponseWriter, r *http.Request) {
		_ = openFolder(getSettings().InstallDir)
		writeJSON(w, map[string]bool{"ok": true})
	})
	h("open-log", func(w http.ResponseWriter, r *http.Request) {
		_ = openFolder(appDataDir())
		writeJSON(w, map[string]bool{"ok": true})
	})
	h("open-url", func(w http.ResponseWriter, r *http.Request) {
		u := r.URL.Query().Get("u")
		if strings.HasPrefix(u, "https://") {
			_ = openLink(u)
		}
		writeJSON(w, map[string]bool{"ok": true})
	})
	h("open-downloads", func(w http.ResponseWriter, r *http.Request) {
		if d := downloadDirs(); len(d) > 0 {
			_ = openFolder(d[0])
		}
		writeJSON(w, map[string]bool{"ok": true})
	})
	h("open-manual", func(w http.ResponseWriter, r *http.Request) {
		if j := currentJob(); j != nil {
			one := r.URL.Query().Get("file")
			for _, it := range j.status().Manual {
				if one != "" && it.File != one {
					continue
				}
				if strings.HasPrefix(it.URL, "https://") {
					_ = openLink(it.URL)
					time.Sleep(700 * time.Millisecond)
				}
			}
		}
		writeJSON(w, map[string]bool{"ok": true})
	})
	h("play", func(w http.ResponseWriter, r *http.Request) {
		s := getSettings()
		if !isInstalled(s.InstallDir) {
			writeErr(w, errors.New("сборка ещё не установлена"))
			return
		}
		m, merr := fetchManifest(false)
		if merr != nil {
			m = nil
		}
		pi := detectProfile(s.InstallDir, m)
		if pi.Kind == KindOfficial && m != nil {
			if !loaderInstalled(m) || !profileExists() {
				appendLog("Играть: нет NeoForge или профиля")
				writeErr(w, errors.New("не хватает NeoForge или профиля — нажми «Проверить и починить файлы»"))
				return
			}
			_ = writeProfile(m) // refresh lastUsed so the launcher preselects Vibe Club
		}
		applyPendingSkin()
		msg, err := launchFor(pi, s.InstallDir)
		if err != nil {
			appendLog("Играть: " + err.Error())
			writeErr(w, err)
			return
		}
		if s.CloseAfterLaunch {
			go func() { time.Sleep(1500 * time.Millisecond); quit() }()
		}
		writeJSON(w, map[string]any{"ok": true, "message": msg, "loaderOk": pi.LoaderOK, "loader": pi.Loader})
	})
	h("skin", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			b, slim, src := currentSkinWithSource()
			if b == nil {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("X-Skin-Source", src)
			if slim {
				w.Header().Set("X-Skin-Slim", "1")
			}
			w.Header().Set("Content-Type", "image/png")
			w.Write(b)
		case "POST":
			data, err := io.ReadAll(io.LimitReader(r.Body, 3<<20))
			if err != nil {
				writeErr(w, err)
				return
			}
			if err := saveSkin(data, r.URL.Query().Get("slim") == "1"); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, map[string]bool{"ok": true})
		case "DELETE":
			resetSkin()
			writeJSON(w, map[string]bool{"ok": true})
		}
	})
	h("skin-model", func(w http.ResponseWriter, r *http.Request) {
		slim := r.URL.Query().Get("slim") == "1"
		updateSettings(func(s *Settings) {
			if s.PendingSkinHash != "" {
				if slim {
					s.PendingSkinModel = "slim"
				} else {
					s.PendingSkinModel = "classic"
				}
				s.PendingSkinActive = true
			}
		})
		applyPendingSkin()
		writeJSON(w, map[string]bool{"ok": true})
	})
	h("update-launcher", func(w http.ResponseWriter, r *http.Request) {
		if err := selfUpdate(); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	})
	h("quit", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]bool{"ok": true})
		go func() { time.Sleep(300 * time.Millisecond); quit() }()
	})
}

// suggestSubfolder returns p itself when it is empty, already a pack folder or an existing
// Minecraft profile (CurseForge instance, official launcher game folder); otherwise p\VibeClub.
// describeFolder picks the real install folder for a chosen path and says what is in it.
func describeFolder(p string) map[string]string {
	// A folder that already has other content gets a VibeClub subfolder, so we never mix
	// the pack into e.g. the Desktop or Documents.
	p = suggestSubfolder(p)
	mode := "new"
	if isInstalled(p) {
		mode = "installed"
	} else if isGameFolder(p) {
		mode = "profile"
	}
	pi := detectProfile(p, nil)
	return map[string]string{"path": p, "mode": mode, "launcher": pi.Name, "kind": pi.Kind}
}

// cleanUserPath turns a pasted path into a clean absolute one: strips quotes and spaces
// (Explorer's "Copy as path" adds quotes), expands ~ and %VARS%.
func cleanUserPath(s string) (string, error) {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'«»")
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("вставь путь к папке")
	}
	if strings.HasPrefix(s, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			s = home + s[1:]
		}
	}
	s = os.Expand(s, os.Getenv)
	if isWindows() {
		s = expandPercentVars(s)
	}
	if strings.HasPrefix(strings.ToLower(s), "file://") {
		s = s[len("file://"):]
		if u, err := url.PathUnescape(s); err == nil {
			s = u
		}
		if len(s) > 2 && s[0] == '/' && s[2] == ':' { // file:///C:/Games
			s = s[1:]
		}
	}
	if !filepath.IsAbs(s) {
		return "", errors.New("нужен полный путь, например C:\\Games\\VibeClub")
	}
	s = filepath.Clean(s)
	if fi, err := os.Stat(s); err == nil && !fi.IsDir() {
		s = filepath.Dir(s) // a file was pasted — use its folder
	}
	if existingParent(s) == "" {
		return "", errors.New("такого диска нет: " + s)
	}
	return s, nil
}

func expandPercentVars(s string) string {
	for {
		i := strings.Index(s, "%")
		if i < 0 {
			return s
		}
		j := strings.Index(s[i+1:], "%")
		if j < 0 {
			return s
		}
		name := s[i+1 : i+1+j]
		v, ok := os.LookupEnv(name)
		if !ok {
			return s
		}
		s = s[:i] + v + s[i+2+j:]
	}
}

// existingParent returns the path itself or its nearest existing parent folder.
func existingParent(p string) string {
	if p == "" {
		return ""
	}
	d := filepath.Clean(p)
	for {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

func suggestSubfolder(p string) string {
	if exists(filepath.Join(p, "instance.cfg")) { // Prism/MultiMC instance root
		for _, sub := range []string{"minecraft", ".minecraft"} {
			if fi, err := os.Stat(filepath.Join(p, sub)); err == nil && fi.IsDir() {
				return filepath.Join(p, sub)
			}
		}
		return filepath.Join(p, "minecraft")
	}
	if isInstalled(p) || isGameFolder(p) {
		return p
	}
	entries, err := os.ReadDir(p)
	if err != nil || len(entries) == 0 {
		return p
	}
	if strings.EqualFold(filepath.Base(p), "VibeClub") {
		return p
	}
	return filepath.Join(p, "VibeClub")
}

// isGameFolder reports whether p already looks like a Minecraft profile folder.
func isGameFolder(p string) bool {
	for _, n := range []string{"mods", "config", "saves", "options.txt", "minecraftinstance.json", "resourcepacks"} {
		if _, err := os.Stat(filepath.Join(p, n)); err == nil {
			return true
		}
	}
	return false
}

// selfUpdate downloads the new launcher next to the current exe and swaps it after exit.
func selfUpdate() error {
	m, err := fetchManifest(true)
	if err != nil {
		return err
	}
	rel := m.launcherRelease()
	if rel == nil || !versionLess(LauncherVersion, rel.Version) {
		return errors.New("обновлений лаунчера нет")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	newPath := exe + ".new"
	j := &Job{cancel: make(chan struct{})}
	if err := simpleDownload(j, rel.URL, newPath); err != nil {
		return err
	}
	if rel.SHA1 != "" {
		if h, _ := sha1File(newPath); !strings.EqualFold(h, rel.SHA1) {
			_ = os.Remove(newPath)
			return errors.New("файл обновления повреждён")
		}
	}
	return replaceAndRestart(exe, newPath)
}

// replaceAndRestart swaps in the downloaded launcher and starts it. A running exe cannot be
// overwritten on Windows, but it can be renamed, so the old one becomes <exe>.old and is
// removed on the next start.
func replaceAndRestart(exe, newPath string) error {
	_ = os.Chmod(newPath, 0o755)
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("не удалось заменить лаунчер: %w", err)
	}
	if err := os.Rename(newPath, exe); err != nil {
		_ = os.Rename(old, exe)
		return fmt.Errorf("не удалось заменить лаунчер: %w", err)
	}
	appendLog("Лаунчер обновлён, перезапуск")
	if err := exec.Command(exe).Start(); err != nil {
		return err
	}
	go func() { time.Sleep(500 * time.Millisecond); quit() }()
	return nil
}

func cleanupOldBinary() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	go func() {
		for i := 0; i < 10; i++ {
			if err := os.Remove(exe + ".old"); err == nil || os.IsNotExist(err) {
				break
			}
			time.Sleep(time.Second)
		}
		_ = os.Remove(exe + ".new")
	}()
}
