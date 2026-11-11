package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPickManual(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dl := filepath.Join(home, "Downloads")
	os.MkdirAll(dl, 0o755)
	data := []byte("hello mod")
	os.WriteFile(filepath.Join(dl, "renamed (1).jar"), data, 0o644)
	dir := t.TempDir()
	st := &LocalState{Files: map[string]string{}}
	j := &Job{cancel: make(chan struct{})}
	files := []PackFile{
		{Path: "mods/a.jar", SHA1: "6fca5f0e96d4bf38be0d7e1d0ae5d4e3e2e66e11", Size: int64(len(data)), Manual: "https://x"},
		{Path: "mods/b.jar", SHA1: "00", Size: 5, Manual: "https://y", Name: "B"},
	}
	h, _ := sha1File(filepath.Join(dl, "renamed (1).jar"))
	files[0].SHA1 = h
	miss := pickManual(j, dir, files, st)
	if len(miss) != 1 || miss[0].Name != "B" {
		t.Fatalf("missing=%v", miss)
	}
	if _, err := os.Stat(filepath.Join(dir, "mods/a.jar")); err != nil {
		t.Fatal(err)
	}
}
