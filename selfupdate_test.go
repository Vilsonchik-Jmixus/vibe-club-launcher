package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceSwap(t *testing.T) {
	d := t.TempDir()
	exe := filepath.Join(d, "Лаунчер 1.0.2.exe")
	os.WriteFile(exe, []byte("old"), 0o755)
	os.WriteFile(exe+".new", []byte("#!/bin/sh\nexit 0\n"), 0o755)
	quitCh = make(chan struct{})
	if err := replaceAndRestart(exe, exe+".new"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(exe); string(b[:2]) != "#!" {
		t.Fatal("not swapped")
	}
	if _, err := os.Stat(exe + ".old"); err != nil {
		t.Fatal(err)
	}
}
