package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForEdition(t *testing.T) {
	m := &Manifest{Files: []PackFile{
		{Path: "mods/create.jar"},
		{Path: "mods/DistantHorizons.jar", Editions: []string{"full"}},
		{Path: "configureddefaults/options.lite.txt", Editions: []string{"lite"}},
		{Path: "config/a.toml"},
	}}
	full, lite := m.forEdition("full"), m.forEdition("lite")
	if len(full.Files) != 3 || full.ModCount != 2 {
		t.Fatalf("full: %+v", full.Files)
	}
	if len(lite.Files) != 3 || lite.ModCount != 1 {
		t.Fatalf("lite: %+v", lite.Files)
	}
	for _, f := range lite.Files {
		if strings.Contains(f.Path, "Distant") {
			t.Fatal("lite must not contain DH")
		}
	}
	if len(m.Files) != 4 {
		t.Fatal("original manifest changed")
	}
}

func TestOverlayOptions(t *testing.T) {
	out := string(overlayOptions([]byte("renderDistance:12\nfov:0.5\nkey_a:key.keyboard.g\n"), []byte("renderDistance:6\ngraphicsMode:0\n")))
	want := "renderDistance:6\nfov:0.5\nkey_a:key.keyboard.g\ngraphicsMode:0\n"
	if out != want {
		t.Fatalf("got %q", out)
	}
}

// Lite applies its values over the preset; switching back to full restores the pack values
// the player did not touch, and keeps what the player changed.
func TestApplyPackOptionsEditions(t *testing.T) {
	dir := t.TempDir()
	oldAppData := os.Getenv("APPDATA")
	t.Setenv("APPDATA", t.TempDir())
	defer os.Setenv("APPDATA", oldAppData)
	loadSettings()
	w := func(p, s string) { _ = writeFileAtomic(filepath.Join(dir, filepath.FromSlash(p)), []byte(s)) }
	r := func() string { b, _ := os.ReadFile(filepath.Join(dir, "options.txt")); return string(b) }
	w(packOptionsPath, "renderDistance:12\ngraphicsMode:1\nfov:0.5\n")
	w(packOptionsLitePath, "renderDistance:6\ngraphicsMode:0\n")

	updateSettings(func(s *Settings) { s.Edition = "lite" })
	if _, err := applyPackOptions(dir, false); err != nil {
		t.Fatal(err)
	}
	if got := r(); !strings.Contains(got, "renderDistance:6") || !strings.Contains(got, "graphicsMode:0") || !strings.Contains(got, "fov:0.5") {
		t.Fatalf("lite not applied: %q", got)
	}
	// the player changes graphics by hand
	w("options.txt", strings.Replace(r(), "graphicsMode:0", "graphicsMode:2", 1))
	updateSettings(func(s *Settings) { s.Edition = "full" })
	if _, err := applyPackOptions(dir, false); err != nil {
		t.Fatal(err)
	}
	if got := r(); !strings.Contains(got, "renderDistance:12") || !strings.Contains(got, "graphicsMode:2") {
		t.Fatalf("full not restored / player change lost: %q", got)
	}
}
