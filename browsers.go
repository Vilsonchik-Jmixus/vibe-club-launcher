package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Which browser shows the launcher window. "auto" keeps the old behaviour (Edge on Windows,
// the first Chromium-family browser on Linux), "default" uses the system default browser,
// anything else is an id from detectBrowsers().

type browserOpt struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	argv     []string // command (+ fixed args) to start it
	chromium bool     // supports --app windows
	flatpak  bool     // sandboxed: no custom --user-data-dir
}

func browserSetting() string {
	if v := os.Getenv("VIBECLUB_BROWSER"); v != "" {
		return v
	}
	if b := getSettings().Browser; b != "" {
		return b
	}
	return "auto"
}

func findBrowser(id string) *browserOpt {
	for _, b := range detectBrowsers() {
		if b.ID == id {
			bb := b
			return &bb
		}
	}
	return nil
}

// startBrowserWindow opens url in b; Chromium-family browsers get a chromeless app window.
func startBrowserWindow(b *browserOpt, url string) (*exec.Cmd, error) {
	args := append([]string{}, b.argv[1:]...)
	if b.chromium {
		args = append(args, "--app="+url, "--window-size=1296,759", "--no-first-run", "--no-default-browser-check")
		if !b.flatpak {
			args = append(args, "--user-data-dir="+filepath.Join(appDataDir(), "ui-profile-"+b.ID), "--disable-sync")
		}
		if isWindows() && strings.Contains(strings.ToLower(b.argv[0]), "msedge") {
			args = append(args, "--disable-features=Translate,msEdgeSidebarV2,msUndersideButton")
		}
		if !isWindows() {
			args = append(args, "--class=VibeClubLauncher")
		}
	} else {
		args = append(args, "--new-window", url)
	}
	cmd := exec.Command(b.argv[0], args...)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// openChosenWindow honours the player's choice; ok=false means "use the automatic way".
func openChosenWindow(url string) (cmd *exec.Cmd, ok bool) {
	choice := browserSetting()
	switch choice {
	case "auto", "":
		return nil, false
	case "default":
		appendLog("Открываю лаунчер в браузере по умолчанию")
		_ = openURL(url)
		return nil, true
	}
	b := findBrowser(choice)
	if b == nil {
		appendLog("Выбранный браузер «" + choice + "» не найден, открываю как обычно")
		return nil, false
	}
	c, err := startBrowserWindow(b, url)
	if err != nil {
		appendLog("Не удалось открыть " + b.Name + ": " + err.Error())
		return nil, false
	}
	appendLog("Открываю лаунчер в " + b.Name)
	return c, true
}

func browserList() []map[string]string {
	out := []map[string]string{
		{"id": "auto", "name": "Автоматически"},
		{"id": "default", "name": "Браузер по умолчанию"},
	}
	for _, b := range detectBrowsers() {
		out = append(out, map[string]string{"id": b.ID, "name": b.Name})
	}
	return out
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
