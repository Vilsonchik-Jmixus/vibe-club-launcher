package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manifest describes one version of the pack. It is published as JSON next to the files.
type Manifest struct {
	Pack       string     `json:"pack"`
	Version    string     `json:"version"`
	Minecraft  string     `json:"minecraft"`
	Loader     Loader     `json:"loader"`
	Server     ServerInfo `json:"server"`
	Launcher   *Release   `json:"launcher,omitempty"`      // Windows exe
	LauncherLx *Release   `json:"launcherLinux,omitempty"` // Linux x64 binary
	News       []News     `json:"news,omitempty"`
	JavaArgs   string     `json:"javaArgs,omitempty"`
	ModCount   int        `json:"modCount,omitempty"`
	Files      []PackFile `json:"files"`
	BaseURL    string     `json:"baseUrl,omitempty"` // for files without url: baseUrl + path
	fetchedURL string
}

type Loader struct {
	Type    string `json:"type"`    // "neoforge"
	Version string `json:"version"` // "21.1.265"
}

type ServerInfo struct {
	Name    string `json:"name"`
	Address string `json:"address"` // shown to players, may use SRV
	Host    string `json:"host"`    // fallback host for status ping
	Port    int    `json:"port"`
}

type Release struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA1    string `json:"sha1"`
	Size    int64  `json:"size"`
}

type News struct {
	Date  string `json:"date"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

// PackFile is one file of the pack. Policy "managed" keeps it identical to the manifest,
// "once" only creates it when missing (player settings such as options.txt).
type PackFile struct {
	Path   string   `json:"path"`
	SHA1   string   `json:"sha1"`
	Size   int64    `json:"size"`
	URL    string   `json:"url,omitempty"`
	Mirror []string `json:"mirrors,omitempty"`
	Policy string   `json:"policy,omitempty"`
	// Manual: the author does not allow third-party downloads. The player downloads the
	// file in the browser from this page; we pick it up from the Downloads folder.
	Manual string `json:"manual,omitempty"`
	Name   string `json:"name,omitempty"`
}

type ManualItem struct {
	Name string `json:"name"`
	File string `json:"file"`
	URL  string `json:"url"`
}

func (f PackFile) urls(m *Manifest) []string {
	var out []string
	if f.URL != "" {
		out = append(out, f.URL)
	} else if m.BaseURL != "" {
		out = append(out, strings.TrimRight(m.BaseURL, "/")+"/"+urlPathEscape(f.Path))
	}
	return append(out, f.Mirror...)
}

func urlPathEscape(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = strings.ReplaceAll(urlQueryEscape(s), "+", "%20")
	}
	return strings.Join(parts, "/")
}

// LocalState is stored in <install>/.vibeclub/state.json and remembers what we installed.
type LocalState struct {
	Version string            `json:"version"`
	Files   map[string]string `json:"files"` // path -> sha1 of files we manage
	Updated time.Time         `json:"updated"`
}

var httpClient = &http.Client{Timeout: 0}

var (
	manifestMu    sync.Mutex
	cachedManifst *Manifest
	cachedAt      time.Time
)

func fetchManifest(force bool) (*Manifest, error) {
	manifestMu.Lock()
	defer manifestMu.Unlock()
	u := manifestURL()
	if !force && cachedManifst != nil && cachedManifst.fetchedURL == u && time.Since(cachedAt) < 2*time.Minute {
		return cachedManifst, nil
	}
	var data []byte
	var err error
	if strings.HasPrefix(u, "file://") || !strings.Contains(u, "://") {
		data, err = os.ReadFile(strings.TrimPrefix(u, "file://"))
	} else {
		// raw.githubusercontent.com caches files for ~5 minutes; a changing query string
		// makes a freshly pushed update visible right away.
		fu := u
		if !strings.Contains(fu, "?") {
			fu += fmt.Sprintf("?t=%d", time.Now().Unix()/30)
		}
		req, _ := http.NewRequest("GET", fu, nil)
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("User-Agent", "VibeClubLauncher/"+LauncherVersion)
		c := &http.Client{Timeout: 20 * time.Second}
		var resp *http.Response
		resp, err = c.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				err = fmt.Errorf("сервер обновлений ответил %d", resp.StatusCode)
			} else {
				data, err = io.ReadAll(io.LimitReader(resp.Body, 64<<20))
			}
		}
	}
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("список сборки повреждён: %w", err)
	}
	for _, f := range m.Files {
		if err := checkRelPath(f.Path); err != nil {
			return nil, err
		}
	}
	m.fetchedURL = u
	cachedManifst, cachedAt = &m, time.Now()
	return &m, nil
}

// checkRelPath refuses absolute paths and ".." so a manifest can never write outside the pack folder.
func checkRelPath(p string) error {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, `\`) || strings.Contains(p, ":") {
		return fmt.Errorf("недопустимый путь в списке сборки: %q", p)
	}
	for _, s := range strings.Split(p, "/") {
		if s == ".." || s == "." || s == "" {
			return fmt.Errorf("недопустимый путь в списке сборки: %q", p)
		}
	}
	if strings.HasPrefix(strings.ToLower(p), ".vibeclub/") {
		return fmt.Errorf("недопустимый путь в списке сборки: %q", p)
	}
	return nil
}

func statePath(dir string) string { return filepath.Join(dir, ".vibeclub", "state.json") }

func loadState(dir string) *LocalState {
	st := &LocalState{Files: map[string]string{}}
	b, err := os.ReadFile(statePath(dir))
	if err == nil {
		_ = json.Unmarshal(b, st)
	}
	if st.Files == nil {
		st.Files = map[string]string{}
	}
	return st
}

func saveState(dir string, st *LocalState) error {
	st.Updated = time.Now()
	b, _ := json.MarshalIndent(st, "", " ")
	return writeFileAtomic(statePath(dir), b)
}

func isInstalled(dir string) bool {
	_, err := os.Stat(statePath(dir))
	return err == nil
}

func sha1File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ---------------------------------------------------------------- jobs

type JobKind string

const (
	JobInstall   JobKind = "install"
	JobUpdate    JobKind = "update"
	JobRepair    JobKind = "repair"
	JobReinstall JobKind = "reinstall"
)

type JobStatus struct {
	Running    bool         `json:"running"`
	Kind       JobKind      `json:"kind"`
	Phase      string       `json:"phase"`
	PhaseText  string       `json:"phaseText"`
	Total      int          `json:"total"`
	Done       int          `json:"done"`
	BytesTotal int64        `json:"bytesTotal"`
	BytesDone  int64        `json:"bytesDone"`
	Speed      float64      `json:"speed"`
	Current    string       `json:"current"`
	Paused     bool         `json:"paused"`
	Error      string       `json:"error"`
	Finished   bool         `json:"finished"`
	Log        []string     `json:"log"`
	Manual     []ManualItem `json:"manual,omitempty"`
}

type Job struct {
	mu       sync.Mutex
	st       JobStatus
	cancel   chan struct{}
	pauseCh  chan struct{}
	canceled bool
}

var (
	jobMu  sync.Mutex
	curJob *Job
)

func currentJob() *Job {
	jobMu.Lock()
	defer jobMu.Unlock()
	return curJob
}

func (j *Job) status() JobStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	s := j.st
	s.Log = append([]string(nil), j.st.Log...)
	return s
}

func (j *Job) set(fn func(s *JobStatus)) {
	j.mu.Lock()
	fn(&j.st)
	j.mu.Unlock()
}

func (j *Job) logf(format string, a ...any) {
	line := time.Now().Format("15:04:05 ") + fmt.Sprintf(format, a...)
	j.mu.Lock()
	j.st.Log = append(j.st.Log, line)
	if len(j.st.Log) > 200 {
		j.st.Log = j.st.Log[len(j.st.Log)-200:]
	}
	j.mu.Unlock()
	appendLog(line)
}

func (j *Job) doCancel() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.canceled {
		j.canceled = true
		close(j.cancel)
	}
}

func (j *Job) isCanceled() bool {
	select {
	case <-j.cancel:
		return true
	default:
		return false
	}
}

// waitIfPaused blocks while paused; returns false if canceled.
func (j *Job) waitIfPaused() bool {
	for {
		j.mu.Lock()
		p := j.st.Paused
		j.mu.Unlock()
		if !p {
			return !j.isCanceled()
		}
		select {
		case <-j.cancel:
			return false
		case <-time.After(200 * time.Millisecond):
		}
	}
}

var errCanceled = errors.New("отменено")

func startJob(kind JobKind) (*Job, error) {
	jobMu.Lock()
	defer jobMu.Unlock()
	if curJob != nil && curJob.status().Running {
		return nil, errors.New("уже идёт другая операция")
	}
	j := &Job{cancel: make(chan struct{})}
	j.st = JobStatus{Running: true, Kind: kind, Phase: "prepare", PhaseText: "Подготовка…"}
	curJob = j
	go func() {
		err := runJob(j, kind)
		j.set(func(s *JobStatus) {
			s.Running = false
			s.Finished = true
			s.Paused = false
			s.Speed = 0
			if err != nil {
				if errors.Is(err, errCanceled) {
					s.Error = "Операция отменена"
				} else {
					s.Error = err.Error()
				}
				s.Phase = "error"
			} else {
				s.Phase = "done"
				s.PhaseText = "Готово"
			}
		})
		if err != nil {
			j.logf("ОШИБКА: %v", err)
		} else {
			j.logf("Готово")
		}
	}()
	return j, nil
}

func runJob(j *Job, kind JobKind) error {
	s := getSettings()
	dir := s.InstallDir
	if dir == "" {
		return errors.New("не выбрана папка сборки")
	}
	j.logf("Операция %s, папка %s", kind, dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("не удалось создать папку: %w", err)
	}
	j.set(func(st *JobStatus) { st.PhaseText = "Получаю список сборки…" })
	m, err := fetchManifest(true)
	if err != nil {
		return fmt.Errorf("не удалось получить список сборки: %w", err)
	}
	j.logf("Сборка %s, файлов: %d", m.Version, len(m.Files))
	st := loadState(dir)

	if kind == JobReinstall {
		j.set(func(s *JobStatus) { s.PhaseText = "Удаляю старые файлы сборки…" })
		n := 0
		for p := range st.Files {
			if checkRelPath(p) != nil {
				continue
			}
			if err := os.Remove(filepath.Join(dir, filepath.FromSlash(p))); err == nil {
				n++
			}
		}
		j.logf("Удалено файлов сборки: %d (миры и личные настройки не тронуты)", n)
		st = &LocalState{Files: map[string]string{}}
		_ = saveState(dir, st)
	}

	// First install into an existing profile: jars that are not part of the pack (old versions,
	// other mods) would clash with ours, so move them aside instead of deleting them.
	if kind == JobInstall && len(st.Files) == 0 {
		if n, bdir := moveForeignMods(dir, m); n > 0 {
			j.logf("Чужие моды (%d шт.) перенесены в %s — их можно вернуть вручную", n, bdir)
		}
	}

	// Work out which files need downloading.
	j.set(func(s *JobStatus) {
		s.Phase = "check"
		s.PhaseText = "Проверяю файлы…"
		s.Total = len(m.Files)
	})
	var todo []PackFile
	verify := kind == JobRepair || kind == JobInstall || kind == JobReinstall
	for i, f := range m.Files {
		if j.isCanceled() {
			return errCanceled
		}
		j.set(func(s *JobStatus) { s.Done = i; s.Current = f.Path })
		local := filepath.Join(dir, filepath.FromSlash(f.Path))
		fi, statErr := os.Stat(local)
		if f.Policy == "once" {
			if statErr != nil {
				todo = append(todo, f)
			}
			continue
		}
		if statErr != nil || fi.Size() != f.Size {
			todo = append(todo, f)
			continue
		}
		if !verify && st.Files[f.Path] == strings.ToLower(f.SHA1) {
			continue
		}
		h, err := sha1File(local)
		if err != nil || !strings.EqualFold(h, f.SHA1) {
			todo = append(todo, f)
			continue
		}
		st.Files[f.Path] = strings.ToLower(f.SHA1)
	}

	var total int64
	for _, f := range todo {
		total += f.Size
	}
	if need, free := total, freeSpace(dir); free > 0 && need > free {
		return fmt.Errorf("на диске не хватает места: нужно %s, свободно %s", humanBytes(need), humanBytes(free))
	}
	j.logf("Нужно скачать: %d файлов, %s", len(todo), humanBytes(total))
	j.set(func(s *JobStatus) {
		s.Phase = "download"
		s.PhaseText = "Загрузка файлов"
		s.Total = len(todo)
		s.Done = 0
		s.BytesTotal = total
		s.BytesDone = 0
	})
	var auto, manual []PackFile
	for _, f := range todo {
		if f.Manual != "" && f.URL == "" {
			manual = append(manual, f)
		} else {
			auto = append(auto, f)
		}
	}
	if err := downloadAll(j, m, dir, auto, st); err != nil {
		_ = saveState(dir, st)
		return err
	}
	if missing := pickManual(j, dir, manual, st); len(missing) > 0 {
		_ = saveState(dir, st)
		j.set(func(s *JobStatus) { s.Manual = missing })
		return fmt.Errorf("осталось скачать вручную: %d шт. Авторы этих модов разрешают скачивать их только с их страницы", len(missing))
	}

	// Remove files that we installed earlier but that are no longer in the pack.
	want := map[string]bool{}
	for _, f := range m.Files {
		want[f.Path] = true
	}
	var removed []string
	for p := range st.Files {
		if !want[p] {
			removed = append(removed, p)
		}
	}
	sort.Strings(removed)
	for _, p := range removed {
		if checkRelPath(p) == nil {
			_ = os.Remove(filepath.Join(dir, filepath.FromSlash(p)))
			j.logf("Удалён устаревший файл: %s", p)
		}
		delete(st.Files, p)
	}
	st.Version = m.Version
	if err := saveState(dir, st); err != nil {
		return err
	}

	// Loader + profile in the official launcher.
	j.set(func(s *JobStatus) {
		s.Phase = "loader"
		s.PhaseText = "Ставлю NeoForge и профиль в лаунчер…"
		s.Current = ""
	})
	if err := ensureLoaderAndProfile(j, m); err != nil {
		return err
	}
	applyPendingSkin()
	return nil
}

func humanBytes(n int64) string {
	f := float64(n)
	switch {
	case f >= 1<<30:
		return strings.Replace(fmt.Sprintf("%.1f ГБ", f/(1<<30)), ".", ",", 1)
	case f >= 1<<20:
		return strings.Replace(fmt.Sprintf("%.1f МБ", f/(1<<20)), ".", ",", 1)
	case f >= 1<<10:
		return fmt.Sprintf("%.0f КБ", f/(1<<10))
	}
	return fmt.Sprintf("%d Б", n)
}

// ---------------------------------------------------------------- log file

var logMu sync.Mutex

func appendLog(line string) {
	logMu.Lock()
	defer logMu.Unlock()
	p := filepath.Join(appDataDir(), "launcher.log")
	if fi, err := os.Stat(p); err == nil && fi.Size() > 2<<20 {
		_ = os.Rename(p, p+".old")
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(time.Now().Format("2006-01-02 ") + line + "\r\n")
}

// pickManual looks for manually downloaded files in the player's Downloads folder.
func pickManual(j *Job, dir string, files []PackFile, st *LocalState) []ManualItem {
	var missing []ManualItem
	home, _ := os.UserHomeDir()
	dl := filepath.Join(home, "Downloads")
	entries, _ := os.ReadDir(dl)
	for _, f := range files {
		found := ""
		base := filepath.Base(f.Path)
		cands := []string{filepath.Join(dl, base)}
		for _, e := range entries {
			if e.IsDir() || e.Name() == base {
				continue
			}
			if info, err := e.Info(); err == nil && info.Size() == f.Size {
				cands = append(cands, filepath.Join(dl, e.Name()))
			}
		}
		for _, c := range cands {
			if fi, err := os.Stat(c); err != nil || fi.Size() != f.Size {
				continue
			}
			if h, err := sha1File(c); err == nil && strings.EqualFold(h, f.SHA1) {
				found = c
				break
			}
		}
		if found == "" {
			name := f.Name
			if name == "" {
				name = base
			}
			missing = append(missing, ManualItem{Name: name, File: base, URL: f.Manual})
			continue
		}
		dst := filepath.Join(dir, filepath.FromSlash(f.Path))
		_ = os.MkdirAll(filepath.Dir(dst), 0o755)
		if err := copyFileTo(found, dst); err != nil {
			j.logf("Не удалось скопировать %s: %v", found, err)
			missing = append(missing, ManualItem{Name: f.Name, File: base, URL: f.Manual})
			continue
		}
		st.Files[f.Path] = strings.ToLower(f.SHA1)
		j.logf("Взят из «Загрузок»: %s", base)
	}
	return missing
}

func copyFileTo(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return replaceFile(tmp, dst)
}

// moveForeignMods moves mods/*.jar that are not in the manifest into mods_backup_vibeclub_<date>.
func moveForeignMods(dir string, m *Manifest) (int, string) {
	want := map[string]bool{}
	for _, f := range m.Files {
		want[strings.ToLower(f.Path)] = true
	}
	modsDir := filepath.Join(dir, "mods")
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return 0, ""
	}
	bdir := filepath.Join(dir, "mods_backup_vibeclub_"+time.Now().Format("2006-01-02_15-04"))
	n := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".jar") || want[strings.ToLower("mods/"+name)] {
			continue
		}
		if err := os.MkdirAll(bdir, 0o755); err != nil {
			return n, bdir
		}
		if os.Rename(filepath.Join(modsDir, name), filepath.Join(bdir, name)) == nil {
			n++
		}
	}
	return n, bdir
}

// launcherRelease returns the launcher build for this platform.
func (m *Manifest) launcherRelease() *Release {
	if isWindows() {
		return m.Launcher
	}
	return m.LauncherLx
}
