package main

import "testing"

func TestManualPage(t *testing.T) {
	if g := manualPage("https://www.curseforge.com/api/v1/mods/1725145/files/9369022/download"); g != "https://www.curseforge.com/projects/1725145" {
		t.Fatal(g)
	}
	u := "https://www.curseforge.com/minecraft/mc-mods/lovsplayer/download/9369022"
	if g := manualPage(u); g != u {
		t.Fatal(g)
	}
}
