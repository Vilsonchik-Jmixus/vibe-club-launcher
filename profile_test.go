package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfileFolder(t *testing.T) {
	prof := t.TempDir()
	os.MkdirAll(filepath.Join(prof, "mods"), 0o755)
	os.WriteFile(filepath.Join(prof, "mods", "keep.jar"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(prof, "mods", "old-create.jar"), []byte("b"), 0o644)
	if suggestSubfolder(prof) != prof {
		t.Fatal("profile folder must be used as is")
	}
	other := t.TempDir()
	os.WriteFile(filepath.Join(other, "photo.jpg"), []byte("x"), 0o644)
	if suggestSubfolder(other) != filepath.Join(other, "VibeClub") {
		t.Fatal("random folder must get VibeClub subfolder")
	}
	m := &Manifest{Files: []PackFile{{Path: "mods/keep.jar"}}}
	n, b := moveForeignMods(prof, m)
	if n != 1 {
		t.Fatalf("moved %d", n)
	}
	if _, err := os.Stat(filepath.Join(b, "old-create.jar")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(prof, "mods", "keep.jar")); err != nil {
		t.Fatal(err)
	}
}
