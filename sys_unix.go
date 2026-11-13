//go:build !windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Linux (and other Unix) implementation.

func isWindows() bool { return false }

func hideWindow(cmd *exec.Cmd) {}

func freeSpace(dir string) int64 {
	var st syscall.Statfs_t
	for d := dir; ; {
		if err := syscall.Statfs(d, &st); err == nil {
			return int64(st.Bavail) * int64(st.Bsize)
		}
		p := filepath.Dir(d)
		if p == d {
			return 0
		}
		d = p
	}
}

func messageBox(title, text string) {
	fmt.Fprintln(os.Stderr, title+": "+text)
	if p, err := exec.LookPath("zenity"); err == nil {
		_ = exec.Command(p, "--error", "--title="+title, "--text="+text).Run()
	} else if p, err := exec.LookPath("kdialog"); err == nil {
		_ = exec.Command(p, "--title", title, "--error", text).Run()
	} else if p, err := exec.LookPath("notify-send"); err == nil {
		_ = exec.Command(p, title, text).Run()
	}
}

// errNoPicker makes the UI ask for the path as text.
var errNoPicker = errors.New("NO_PICKER")

func pickFolder(initial string) (string, error) {
	if v := os.Getenv("VC_TEST_PICK"); v != "" {
		return v, nil
	}
	start := initial
	for start != "" {
		if fi, err := os.Stat(start); err == nil && fi.IsDir() {
			break
		}
		if p := filepath.Dir(start); p != start {
			start = p
		} else {
			start = ""
		}
	}
	if start == "" {
		start, _ = os.UserHomeDir()
	}
	var cmd *exec.Cmd
	if p, err := exec.LookPath("zenity"); err == nil {
		cmd = exec.Command(p, "--file-selection", "--directory", "--title=Папка сборки Vibe Club", "--filename="+start+"/")
	} else if p, err := exec.LookPath("kdialog"); err == nil {
		cmd = exec.Command(p, "--getexistingdirectory", start, "--title", "Папка сборки Vibe Club")
	} else if p, err := exec.LookPath("yad"); err == nil {
		cmd = exec.Command(p, "--file", "--directory", "--title=Папка сборки Vibe Club", "--filename="+start+"/")
	} else {
		return "", errNoPicker
	}
	out, err := cmd.Output()
	if err != nil {
		// Cancel returns exit code 1.
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", nil
		}
		return "", errNoPicker
	}
	return strings.TrimSpace(string(out)), nil
}

func xdgOpen(target string) error {
	if p, err := exec.LookPath("xdg-open"); err == nil {
		return exec.Command(p, target).Start()
	}
	if p, err := exec.LookPath("open"); err == nil { // macOS
		return exec.Command(p, target).Start()
	}
	return errors.New("не найден xdg-open")
}

func openFolder(path string) error {
	_ = os.MkdirAll(path, 0o755)
	return xdgOpen(path)
}

func openURL(u string) error { return xdgOpen(u) }

var errNoOfficialLauncher = errors.New("NO_OFFICIAL_LAUNCHER")

// launchOfficialLauncher starts the official Minecraft Launcher (deb/rpm/AUR package,
// tarball in /opt or ~, or the Flatpak).
func launchOfficialLauncher() error {
	for _, n := range []string{"minecraft-launcher", "minecraft"} {
		if p, err := exec.LookPath(n); err == nil {
			appendLog("Запускаю " + p)
			return exec.Command(p).Start()
		}
	}
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		"/opt/minecraft-launcher/minecraft-launcher",
		filepath.Join(home, "minecraft-launcher", "minecraft-launcher"),
		filepath.Join(home, ".local", "share", "minecraft-launcher", "minecraft-launcher"),
	} {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			appendLog("Запускаю " + p)
			return exec.Command(p).Start()
		}
	}
	if fp, err := exec.LookPath("flatpak"); err == nil {
		if exec.Command(fp, "info", "com.mojang.Minecraft").Run() == nil {
			appendLog("Запускаю Flatpak com.mojang.Minecraft")
			return exec.Command(fp, "run", "com.mojang.Minecraft").Start()
		}
	}
	appendLog("Официальный лаунчер Minecraft не найден")
	return errNoOfficialLauncher
}

// openAppWindow shows the UI in a Chromium-based browser's app window, or in the default browser.
func openAppWindow(url string) (*exec.Cmd, error) {
	if os.Getenv("VC_HEADLESS") != "" {
		return nil, nil
	}
	if cmd, ok := openChosenWindow(url); ok {
		return cmd, nil
	}
	for _, n := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "microsoft-edge-stable", "brave-browser", "brave", "vivaldi", "yandex-browser"} {
		p, err := exec.LookPath(n)
		if err != nil {
			continue
		}
		prof := filepath.Join(appDataDir(), "ui-profile")
		cmd := exec.Command(p, "--app="+url, "--user-data-dir="+prof, "--window-size=1296,759",
			"--no-first-run", "--no-default-browser-check", "--disable-sync", "--class=VibeClubLauncher")
		if err := cmd.Start(); err == nil {
			return cmd, nil
		}
	}
	// no Chromium browser in PATH: try a Flatpak one before the default browser
	for _, b := range detectBrowsers() {
		if b.flatpak && b.chromium {
			bb := b
			if cmd, err := startBrowserWindow(&bb, url); err == nil {
				appendLog("Открываю лаунчер в " + b.Name)
				return cmd, nil
			}
		}
	}
	return nil, openURL(url)
}

// installDesktopEntry adds the launcher to the application menu (once per binary location).
func installDesktopEntry() {
	exe, err := os.Executable()
	if err != nil || strings.HasPrefix(exe, os.TempDir()) {
		return
	}
	home, _ := os.UserHomeDir()
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		data = filepath.Join(home, ".local", "share")
	}
	icon := filepath.Join(data, "icons", "hicolor", "128x128", "apps", "vibeclub-launcher.png")
	if b, err := webFS.ReadFile("web/assets/profile-icon.png"); err == nil {
		_ = os.MkdirAll(filepath.Dir(icon), 0o755)
		_ = os.WriteFile(icon, b, 0o644)
	}
	desk := filepath.Join(data, "applications", "vibeclub-launcher.desktop")
	q := strings.ReplaceAll(exe, `"`, `\"`)
	content := "[Desktop Entry]\nType=Application\nName=Vibe Club Launcher\nComment=Сборка Vibe Club для Minecraft\nExec=\"" + q + "\"\nIcon=" + icon + "\nTerminal=false\nCategories=Game;\nStartupWMClass=VibeClubLauncher\n"
	if old, err := os.ReadFile(desk); err == nil && string(old) == content {
		return
	}
	_ = os.MkdirAll(filepath.Dir(desk), 0o755)
	_ = os.WriteFile(desk, []byte(content), 0o644)
}

func startDetached(exe string, args ...string) error { return exec.Command(exe, args...).Start() }

func lookFirst(names ...string) string {
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	return ""
}

func flatpakApp(id string) bool {
	fp, err := exec.LookPath("flatpak")
	return err == nil && exec.Command(fp, "info", id).Run() == nil
}

func findCurseForge() (string, []string, bool) {
	if p := lookFirst("curseforge", "CurseForge"); p != "" {
		return p, nil, true
	}
	return "", nil, false
}

func findModrinthApp() (string, []string, bool) {
	if p := lookFirst("modrinth-app", "ModrinthApp", "modrinth"); p != "" {
		return p, nil, true
	}
	if flatpakApp("com.modrinth.ModrinthApp") {
		return "flatpak", []string{"run", "com.modrinth.ModrinthApp"}, true
	}
	return "", nil, false
}

func findPrism(dataDir string) (string, []string, bool) {
	if p := lookFirst("prismlauncher", "PrismLauncher"); p != "" {
		return p, nil, true
	}
	if flatpakApp("org.prismlauncher.PrismLauncher") {
		return "flatpak", []string{"run", "org.prismlauncher.PrismLauncher"}, true
	}
	return "", nil, false
}

// detectBrowsers lists browsers installed as normal packages or as Flatpak apps
// (Steam Deck / SteamOS ships browsers only as Flatpaks).
func detectBrowsers() []browserOpt {
	var out []browserOpt
	seen := map[string]bool{}
	add := func(b browserOpt) {
		if !seen[b.ID] {
			seen[b.ID] = true
			out = append(out, b)
		}
	}
	bins := []struct {
		id, name string
		names    []string
		chromium bool
	}{
		{"chrome", "Google Chrome", []string{"google-chrome", "google-chrome-stable"}, true},
		{"chromium", "Chromium", []string{"chromium", "chromium-browser"}, true},
		{"edge", "Microsoft Edge", []string{"microsoft-edge", "microsoft-edge-stable"}, true},
		{"brave", "Brave", []string{"brave-browser", "brave"}, true},
		{"vivaldi", "Vivaldi", []string{"vivaldi", "vivaldi-stable"}, true},
		{"yandex", "Яндекс Браузер", []string{"yandex-browser", "yandex-browser-stable"}, true},
		{"opera", "Opera", []string{"opera"}, true},
		{"firefox", "Firefox", []string{"firefox"}, false},
	}
	for _, b := range bins {
		for _, n := range b.names {
			if p, err := exec.LookPath(n); err == nil {
				add(browserOpt{ID: b.id, Name: b.name, argv: []string{p}, chromium: b.chromium})
				break
			}
		}
	}
	fp, err := exec.LookPath("flatpak")
	if err != nil {
		return out
	}
	home, _ := os.UserHomeDir()
	flats := []struct {
		id, name, app string
		chromium      bool
	}{
		{"chrome", "Google Chrome", "com.google.Chrome", true},
		{"chromium", "Chromium", "org.chromium.Chromium", true},
		{"ungoogled", "Ungoogled Chromium", "io.github.ungoogled_software.ungoogled_chromium", true},
		{"edge", "Microsoft Edge", "com.microsoft.Edge", true},
		{"brave", "Brave", "com.brave.Browser", true},
		{"vivaldi", "Vivaldi", "com.vivaldi.Vivaldi", true},
		{"opera", "Opera", "com.opera.Opera", true},
		{"firefox", "Firefox", "org.mozilla.firefox", false},
	}
	for _, f := range flats {
		if dirExists(filepath.Join("/var/lib/flatpak/app", f.app)) || dirExists(filepath.Join(home, ".local/share/flatpak/app", f.app)) {
			add(browserOpt{ID: f.id, Name: f.name + " (Flatpak)", argv: []string{fp, "run", f.app}, chromium: f.chromium, flatpak: true})
		}
	}
	return out
}

func platformDownloadDir() string { return "" }
