package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
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

// currentSkin returns the skin to show in the launcher: a skin chosen in the launcher that the
// game has not picked up yet, otherwise the skin that is active in the game files right now.
func currentSkin() ([]byte, bool) {
	b, slim, _ := currentSkinWithSource()
	return b, slim
}

func currentSkinWithSource() ([]byte, bool, string) {
	s := getSettings()
	if s.PendingSkinActive && s.PendingSkinHash != "" {
		if b, err := os.ReadFile(filepath.Join(appDataDir(), "skin.png")); err == nil {
			return b, s.PendingSkinModel == "slim", "pending"
		}
	}
	if b, slim, src := gameSkin(s.InstallDir); b != nil {
		return b, slim, src
	}
	if b, err := os.ReadFile(filepath.Join(appDataDir(), "skin.png")); err == nil {
		return b, s.PendingSkinModel == "slim", "launcher"
	}
	return nil, false, ""
}

// gameSkin finds the skin Quick Skin currently uses in the pack folder.
func gameSkin(dir string) ([]byte, bool, string) {
	if dir == "" {
		return nil, false, ""
	}
	var cfg struct {
		Active  string `json:"activeSkinHash"`
		Model   string `json:"activeModelType"`
		OwnHash string `json:"playerOwnSkinHash"`
		OwnURL  string `json:"playerOwnSkinTexture"`
	}
	if b, err := os.ReadFile(filepath.Join(dir, "config", "quickskin-client.json")); err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	slimOf := func(b []byte) bool {
		switch cfg.Model {
		case "slim":
			return true
		case "classic", "default":
			return false
		}
		return looksSlim(b)
	}
	var newest []byte
	var newestT int64
	for _, d := range []string{skinDir(dir), filepath.Join(dir, "quickskin_cache")} {
		_ = filepath.Walk(d, func(p string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() || !strings.EqualFold(filepath.Ext(p), ".png") || fi.Size() > 2<<20 {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil || validateSkin(b) != nil {
				return nil
			}
			if cfg.Active != "" {
				sum := sha256.Sum256(b)
				if "sha256-"+hex.EncodeToString(sum[:]) == cfg.Active {
					newest, newestT = b, 1<<62
					return filepath.SkipAll
				}
			}
			if t := fi.ModTime().UnixNano(); t > newestT {
				newest, newestT = b, t
			}
			return nil
		})
		if newestT == 1<<62 {
			return newest, slimOf(newest), "game"
		}
	}
	// Account skin (Mojang texture) that Quick Skin shows when nothing else is chosen.
	if cfg.Active != "" && cfg.Active == cfg.OwnHash && strings.HasPrefix(cfg.OwnURL, "https://textures.minecraft.net/texture/") {
		cache := filepath.Join(appDataDir(), "cache", "account-"+strings.TrimPrefix(cfg.Active, "sha256-")+".png")
		if b, err := os.ReadFile(cache); err == nil && validateSkin(b) == nil {
			return b, slimOf(b), "account"
		}
		if b, err := fetchSmall(cfg.OwnURL); err == nil && validateSkin(b) == nil {
			_ = os.MkdirAll(filepath.Dir(cache), 0o755)
			_ = os.WriteFile(cache, b, 0o644)
			return b, slimOf(b), "account"
		}
	}
	if newest != nil {
		return newest, slimOf(newest), "game"
	}
	return nil, false, ""
}

// looksSlim: in Alex-type skins the 4th column of the right arm is transparent.
func looksSlim(b []byte) bool {
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil || img.Bounds().Dy() != 64 {
		return false
	}
	_, _, _, a := img.At(54, 20).RGBA()
	return a == 0
}

func fetchSmall(u string) ([]byte, error) {
	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New(resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 2<<20))
}
