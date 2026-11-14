package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Settings are stored per Windows user in %APPDATA%\VibeClubLauncher\settings.json.
type Settings struct {
	InstallDir        string `json:"installDir"`
	MemoryGB          int    `json:"memoryGB"`
	CheckUpdates      bool   `json:"checkUpdates"`
	CloseAfterLaunch  bool   `json:"closeAfterLaunch"`
	ManifestURL       string `json:"manifestUrl,omitempty"`
	Browser           string `json:"browser,omitempty"`
	Edition           string `json:"edition,omitempty"` // "full" or "lite"; "" = not chosen yet (full)
	PendingSkinHash   string `json:"pendingSkinHash,omitempty"`
	PendingSkinModel  string `json:"pendingSkinModel,omitempty"`
	PendingSkinActive bool   `json:"pendingSkinActive,omitempty"`
}

var (
	settingsMu sync.Mutex
	settings   Settings
)

func appDataDir() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	d := filepath.Join(base, "VibeClubLauncher")
	_ = os.MkdirAll(d, 0o755)
	return d
}

func defaultInstallDir() string {
	if v := os.Getenv("SystemDrive"); v != "" {
		return filepath.Join(v+`\`, "Games", "VibeClub")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Games", "VibeClub")
}

func loadSettings() {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	settings = Settings{InstallDir: defaultInstallDir(), MemoryGB: 8, CheckUpdates: true}
	b, err := os.ReadFile(filepath.Join(appDataDir(), "settings.json"))
	if err == nil {
		_ = json.Unmarshal(b, &settings)
	}
	if settings.MemoryGB < 2 || settings.MemoryGB > 32 {
		settings.MemoryGB = 8
	}
	if settings.InstallDir == "" {
		settings.InstallDir = defaultInstallDir()
	}
}

func getSettings() Settings {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	return settings
}

func updateSettings(fn func(s *Settings)) Settings {
	settingsMu.Lock()
	fn(&settings)
	s := settings
	settingsMu.Unlock()
	b, _ := json.MarshalIndent(s, "", "  ")
	_ = writeFileAtomic(filepath.Join(appDataDir(), "settings.json"), b)
	return s
}

func manifestURL() string {
	if v := os.Getenv("VIBECLUB_MANIFEST"); v != "" {
		return v
	}
	if s := getSettings(); s.ManifestURL != "" {
		return s.ManifestURL
	}
	return DefaultManifestURL
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
