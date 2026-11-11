package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyPackOptions(t *testing.T) {
	dir := t.TempDir()
	w := func(p, s string) { _ = writeFileAtomic(filepath.Join(dir, filepath.FromSlash(p)), []byte(s)) }
	r := func(p string) string { b, _ := os.ReadFile(filepath.Join(dir, p)); return string(b) }
	w(packOptionsPath, "lang:ru_ru\nkey_key.jump:key.keyboard.space\nfov:0.5\n")
	w("options.txt", "lang:en_us\nfov:0.9\nsoundDevice:\"x\"\nkey_key.other:key.keyboard.k\n")
	// first apply: preset wins, other keys kept, backup made
	n, err := applyPackOptions(dir, false)
	if err != nil || n != 3 {
		t.Fatalf("first apply n=%d err=%v", n, err)
	}
	got := r("options.txt")
	for _, s := range []string{"lang:ru_ru", "fov:0.5", "soundDevice:\"x\"", "key_key.other:key.keyboard.k", "key_key.jump:key.keyboard.space"} {
		if !strings.Contains(got, s) {
			t.Errorf("missing %q in %q", s, got)
		}
	}
	m, _ := filepath.Glob(filepath.Join(dir, "options.txt.vibeclub-backup-*"))
	if len(m) != 1 {
		t.Errorf("backup not made")
	}
	// player changes fov, pack changes fov and lang: fov stays player's, lang follows pack
	w("options.txt", strings.Replace(got, "fov:0.5", "fov:0.7", 1))
	w(packOptionsPath, "lang:uk_ua\nkey_key.jump:key.keyboard.space\nfov:0.6\nkey_key.new:key.keyboard.n\n")
	n, _ = applyPackOptions(dir, false)
	got = r("options.txt")
	if !strings.Contains(got, "fov:0.7") || !strings.Contains(got, "lang:uk_ua") || !strings.Contains(got, "key_key.new:key.keyboard.n") || n != 2 {
		t.Errorf("update merge wrong (n=%d): %q", n, got)
	}
	// force: everything from the pack
	n, _ = applyPackOptions(dir, true)
	if got = r("options.txt"); !strings.Contains(got, "fov:0.6") || n != 1 {
		t.Errorf("force wrong (n=%d): %q", n, got)
	}
	// fresh folder: preset copied as is
	dir2 := t.TempDir()
	_ = writeFileAtomic(filepath.Join(dir2, "configureddefaults", "options.txt"), []byte("fov:0.5\n"))
	if n, _ := applyPackOptions(dir2, false); n != 1 {
		t.Errorf("fresh n=%d", n)
	}
}
