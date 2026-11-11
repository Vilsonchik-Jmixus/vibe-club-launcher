package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image/png"
	"os"
	"path/filepath"
)

// Skins are handed to the Quick Skin mod that is part of the pack: the PNG goes into
// <pack>/quickskin/uploads/skins and becomes the active skin in config/quickskin-client.json.
// Quick Skin uploads it to the server, so every player sees it (works on offline-mode servers).

func skinDir(install string) string {
	return filepath.Join(install, "quickskin", "uploads", "skins")
}

func validateSkin(data []byte) error {
	if len(data) > 2<<20 {
		return errors.New("файл слишком большой (больше 2 МБ)")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return errors.New("это не PNG-файл")
	}
	if !(cfg.Width == 64 && (cfg.Height == 64 || cfg.Height == 32)) {
		return errors.New("скин должен быть 64×64 (или старый 64×32)")
	}
	return nil
}

func saveSkin(data []byte, slim bool) error {
	if err := validateSkin(data); err != nil {
		return err
	}
	s := getSettings()
	dir := skinDir(s.InstallDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(dir, "vibeclub-launcher.png"), data); err != nil {
		return err
	}
	_ = writeFileAtomic(filepath.Join(appDataDir(), "skin.png"), data)
	sum := sha256.Sum256(data)
	hash := "sha256-" + hex.EncodeToString(sum[:])
	model := "classic"
	if slim {
		model = "slim"
	}
	updateSettings(func(st *Settings) {
		st.PendingSkinHash = hash
		st.PendingSkinModel = model
		st.PendingSkinActive = true
	})
	applyPendingSkin()
	return nil
}

func resetSkin() {
	s := updateSettings(func(st *Settings) {
		st.PendingSkinHash = ""
		st.PendingSkinModel = ""
		st.PendingSkinActive = true
	})
	_ = os.Remove(filepath.Join(skinDir(s.InstallDir), "vibeclub-launcher.png"))
	_ = os.Remove(filepath.Join(appDataDir(), "skin.png"))
	applyPendingSkin()
}

// applyPendingSkin writes the chosen skin into Quick Skin's config. The config file only exists
// after the first game start, so the choice is kept and applied again later if needed.
func applyPendingSkin() {
	s := getSettings()
	if !s.PendingSkinActive || s.InstallDir == "" {
		return
	}
	cfg := filepath.Join(s.InstallDir, "config", "quickskin-client.json")
	b, err := os.ReadFile(cfg)
	if err != nil {
		return // game not started yet: keep pending
	}
	root := map[string]any{}
	if json.Unmarshal(b, &root) != nil {
		return
	}
	if s.PendingSkinHash == "" {
		root["activeSkinHash"] = ""
		root["activeModelType"] = "auto"
	} else {
		root["activeSkinHash"] = s.PendingSkinHash
		root["activeModelType"] = s.PendingSkinModel
	}
	out, _ := json.MarshalIndent(root, "", "  ")
	if writeFileAtomic(cfg, out) == nil {
		updateSettings(func(st *Settings) { st.PendingSkinActive = false })
	}
}

func currentSkin() ([]byte, bool) {
	b, err := os.ReadFile(filepath.Join(appDataDir(), "skin.png"))
	if err != nil {
		return nil, false
	}
	return b, getSettings().PendingSkinModel == "slim"
}
