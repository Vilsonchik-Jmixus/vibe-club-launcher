package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanUserPath(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "options.txt")
	_ = os.WriteFile(f, []byte("x"), 0o644)
	cases := map[string]string{
		`"` + dir + `"`:      dir,
		"  " + dir + "/  ":   dir,
		f:                    dir,
		"«" + dir + "/new»":  filepath.Join(dir, "new"),
		"file:///" + dir[1:]: dir,
	}
	for in, want := range cases {
		got, err := cleanUserPath(in)
		if err != nil || got != want {
			t.Errorf("cleanUserPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "  ", "relative/path"} {
		if _, err := cleanUserPath(bad); err == nil {
			t.Errorf("cleanUserPath(%q) should fail", bad)
		}
	}
	if existingParent(filepath.Join(dir, "a", "b")) != dir {
		t.Error("existingParent")
	}
}
