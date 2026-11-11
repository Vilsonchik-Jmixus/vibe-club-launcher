package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestGameSkin(t *testing.T) {
	png, err := os.ReadFile("/tmp/claude-0/sk/test.png")
	if err != nil {
		t.Skip("no test skin")
	}
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "quickskin", "uploads", "skins"), 0o755)
	os.MkdirAll(filepath.Join(dir, "config"), 0o755)
	os.WriteFile(filepath.Join(dir, "quickskin", "uploads", "skins", "Player.png"), png, 0o644)
	sum := sha256.Sum256(png)
	os.WriteFile(filepath.Join(dir, "config", "quickskin-client.json"), []byte(`{"activeSkinHash":"sha256-`+hex.EncodeToString(sum[:])+`","activeModelType":"slim"}`), 0o644)
	b, slim, src := gameSkin(dir)
	if b == nil || !slim || src != "game" {
		t.Fatalf("got %v %v %q", b != nil, slim, src)
	}
}
