package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectProfile(t *testing.T) {
	m := &Manifest{Loader: Loader{Type: "neoforge", Version: "21.1.265"}}
	root := t.TempDir()
	cf := filepath.Join(root, "curseforge", "minecraft", "Instances", "TEST")
	os.MkdirAll(cf, 0o755)
	os.WriteFile(filepath.Join(cf, "minecraftinstance.json"), []byte(`{"name":"TEST","baseModLoader":{"name":"neoforge-21.1.200"}}`), 0o644)
	if pi := detectProfile(cf, m); pi.Kind != KindCurseForge || pi.LoaderOK || pi.Profile != "TEST" {
		t.Fatalf("cf %+v", pi)
	}
	pr := filepath.Join(root, "PrismLauncher", "instances", "vc")
	os.MkdirAll(filepath.Join(pr, "minecraft"), 0o755)
	os.WriteFile(filepath.Join(pr, "instance.cfg"), []byte("name=Vibe Club\n"), 0o644)
	os.WriteFile(filepath.Join(pr, "mmc-pack.json"), []byte(`{"components":[{"uid":"net.neoforged","version":"21.1.265"}]}`), 0o644)
	if suggestSubfolder(pr) != filepath.Join(pr, "minecraft") {
		t.Fatal("prism root must map to minecraft subfolder")
	}
	if pi := detectProfile(filepath.Join(pr, "minecraft"), m); pi.Kind != KindPrism || !pi.LoaderOK || pi.instance != "vc" || pi.Profile != "Vibe Club" {
		t.Fatalf("prism %+v", pi)
	}
	mr := filepath.Join(root, "ModrinthApp", "profiles", "Vibe")
	os.MkdirAll(mr, 0o755)
	if pi := detectProfile(mr, m); pi.Kind != KindModrinth {
		t.Fatalf("modrinth %+v", pi)
	}
	if pi := detectProfile(filepath.Join(root, "Games", "VibeClub"), m); pi.Kind != KindOfficial {
		t.Fatalf("official %+v", pi)
	}
}
