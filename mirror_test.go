package main

import "testing"

func TestGithubMirrors(t *testing.T) {
	u := "https://raw.githubusercontent.com/Vilsonchik-Jmixus/vibe-club-launcher/main/pack/files/config/a%20b.zip"
	setPackCommit("")
	m := githubMirrors(u)
	if len(m) != 2 || m[0] != "https://cdn.jsdelivr.net/gh/Vilsonchik-Jmixus/vibe-club-launcher@main/pack/files/config/a%20b.zip" {
		t.Fatalf("got %v", m)
	}
	setPackCommit("0123456789abcdef0123456789abcdef01234567")
	m = githubMirrors(u)
	if m[1] != "https://fastly.jsdelivr.net/gh/Vilsonchik-Jmixus/vibe-club-launcher@0123456789abcdef0123456789abcdef01234567/pack/files/config/a%20b.zip" {
		t.Fatalf("got %v", m)
	}
	setPackCommit("")
	if githubMirrors("https://cdn.modrinth.com/data/x/y.jar") != nil {
		t.Fatal("non-github must have no mirrors")
	}
	f := PackFile{Path: "mods/x.jar", URL: "https://cdn.modrinth.com/x.jar"}
	if n := len(f.urls(&Manifest{})); n != 1 {
		t.Fatalf("modrinth urls %d", n)
	}
	f = PackFile{Path: "config/x", URL: u}
	if n := len(f.urls(&Manifest{})); n != 3 {
		t.Fatalf("github urls %d", n)
	}
}
