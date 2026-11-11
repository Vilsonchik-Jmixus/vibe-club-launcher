package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Which launcher owns the pack folder. "Играть" opens that launcher.
const (
	KindOfficial   = "official"
	KindCurseForge = "curseforge"
	KindModrinth   = "modrinth"
	KindPrism      = "prism"
)

type ProfileInfo struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`     // launcher name for the UI
	Profile  string `json:"profile"`  // profile/instance name inside that launcher
	Loader   string `json:"loader"`   // loader the profile is set up with, if known
	LoaderOK bool   `json:"loaderOk"` // false = profile uses another loader version
	Found    bool   `json:"found"`    // the launcher app was found on this computer
	instance string // Prism instance id
}

var kindNames = map[string]string{
	KindOfficial:   "Minecraft Launcher",
	KindCurseForge: "CurseForge",
	KindModrinth:   "Modrinth App",
	KindPrism:      "Prism Launcher",
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// detectProfile works out which launcher the folder belongs to.
func detectProfile(dir string, m *Manifest) ProfileInfo {
	pi := ProfileInfo{Kind: KindOfficial, LoaderOK: true}
	clean := filepath.Clean(dir)
	low := strings.ToLower(filepath.ToSlash(clean))
	parent := filepath.Dir(clean)
	base := strings.ToLower(filepath.Base(clean))
	want := ""
	if m != nil {
		want = m.Loader.Version
	}
	switch {
	case (base == "minecraft" || base == ".minecraft") && exists(filepath.Join(parent, "instance.cfg")):
		pi.Kind = KindPrism
		pi.instance = filepath.Base(parent)
		pi.Profile = prismName(parent)
		if v := prismLoader(parent); v != "" {
			pi.Loader = "neoforge-" + v
			pi.LoaderOK = want == "" || v == want
		}
	case exists(filepath.Join(clean, "minecraftinstance.json")) || (strings.Contains(low, "curseforge") && strings.Contains(low, "/instances/")):
		pi.Kind = KindCurseForge
		pi.Profile = filepath.Base(clean)
		if n, l := curseforgeInfo(clean); n != "" || l != "" {
			if n != "" {
				pi.Profile = n
			}
			if l != "" {
				pi.Loader = l
				pi.LoaderOK = want == "" || strings.EqualFold(l, "neoforge-"+want)
			}
		}
	case strings.Contains(low, "modrinthapp/profiles/") || strings.Contains(low, "com.modrinth.theseus/profiles/"):
		pi.Kind = KindModrinth
		pi.Profile = filepath.Base(clean)
	}
	pi.Name = kindNames[pi.Kind]
	switch pi.Kind {
	case KindCurseForge:
		_, _, pi.Found = findCurseForge()
	case KindModrinth:
		_, _, pi.Found = findModrinthApp()
	case KindPrism:
		_, _, pi.Found = findPrism(filepath.Dir(parent))
	default:
		pi.Found = true
	}
	return pi
}

func curseforgeInfo(dir string) (name, loader string) {
	b, err := os.ReadFile(filepath.Join(dir, "minecraftinstance.json"))
	if err != nil {
		return "", ""
	}
	var v struct {
		Name          string `json:"name"`
		BaseModLoader *struct {
			Name string `json:"name"`
		} `json:"baseModLoader"`
	}
	if json.Unmarshal(b, &v) != nil {
		return "", ""
	}
	if v.BaseModLoader != nil {
		loader = v.BaseModLoader.Name
	}
	return v.Name, loader
}

func prismName(inst string) string {
	b, err := os.ReadFile(filepath.Join(inst, "instance.cfg"))
	if err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "name=") {
				return strings.TrimSpace(strings.TrimPrefix(line, "name="))
			}
		}
	}
	return filepath.Base(inst)
}

func prismLoader(inst string) string {
	b, err := os.ReadFile(filepath.Join(inst, "mmc-pack.json"))
	if err != nil {
		return ""
	}
	var v struct {
		Components []struct {
			UID     string `json:"uid"`
			Version string `json:"version"`
		} `json:"components"`
	}
	if json.Unmarshal(b, &v) != nil {
		return ""
	}
	for _, c := range v.Components {
		if c.UID == "net.neoforged" {
			return c.Version
		}
	}
	return ""
}

// launchFor opens the launcher that owns the profile. It returns what the player has to do next.
func launchFor(pi ProfileInfo, dir string) (string, error) {
	switch pi.Kind {
	case KindCurseForge:
		exe, args, ok := findCurseForge()
		if !ok {
			return "", errors.New("NO_LAUNCHER:CurseForge")
		}
		appendLog("Играть: открываю CurseForge")
		return "Открываю CurseForge: выбери профиль «" + pi.Profile + "» и нажми Play", startDetached(exe, args...)
	case KindModrinth:
		exe, args, ok := findModrinthApp()
		if !ok {
			return "", errors.New("NO_LAUNCHER:Modrinth App")
		}
		appendLog("Играть: открываю Modrinth App")
		return "Открываю Modrinth App: выбери профиль «" + pi.Profile + "» и нажми Play", startDetached(exe, args...)
	case KindPrism:
		exe, args, ok := findPrism(filepath.Dir(filepath.Dir(filepath.Clean(dir))))
		if !ok {
			return "", errors.New("NO_LAUNCHER:Prism Launcher")
		}
		appendLog("Играть: Prism Launcher --launch " + pi.instance)
		return "Запускаю Minecraft через Prism Launcher…", startDetached(exe, append(args, "--launch", pi.instance)...)
	}
	if err := launchOfficialLauncher(); err != nil {
		return "", err
	}
	return "Открываю лаунчер Minecraft: выбери профиль «Vibe Club» и нажми «Играть»", nil
}
