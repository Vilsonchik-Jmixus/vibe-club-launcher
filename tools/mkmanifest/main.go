// mkmanifest builds pack/manifest.json from a pack folder.
//
//	go run ./tools/mkmanifest -pack <instance dir> -sources sources.json -base <raw files url> \
//	   -version 1.3 -out pack/manifest.json -copy pack/files -meta meta.json
//
// sources.json maps "mods/<file>.jar" -> {"url": "...", "sha1": "..."} for mods that are
// downloaded straight from Modrinth/CurseForge. Every other file is copied to -copy and served
// from -base (our GitHub repo).
package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type PackFile struct {
	Path   string `json:"path"`
	SHA1   string `json:"sha1"`
	Size   int64  `json:"size"`
	URL    string `json:"url,omitempty"`
	Policy string `json:"policy,omitempty"`
}

// Folders that belong to the pack.
var includeDirs = []string{"mods", "config", "defaultconfigs", "kubejs", "resourcepacks", "shaderpacks", "global_packs", "datapacks", "paxi", "openloader", "configureddefaults", "fancymenu_data", "quickskin/uploads/skins"}

// Files at the root that belong to the pack.
var includeRoot = []string{"options.txt", "servers.dat"}

// Player-owned: installed once, never overwritten.
var oncePrefixes = []string{
	"options.txt", "servers.dat",
	"config/quickskin-client.json", "config/quickskin_preferences.json", "config/quickskin-nicknames.txt",
	"config/xaero/", "config/jei/world/", "config/emi.css", "config/keybindatlas-client.toml",
	"config/sodium-options.json", "config/DistantHorizons.toml", "config/iris.properties",
	"config/fancymenu/user_variables.db", "config/spark/", "config/crash_assistant/",
}

// Never shipped.
var skipContains = []string{"/.git/", ".bak", ".tmp", ".old", ".disabled", "/backups/", "-1.toml.bak", "/mcef-cache/", "/mcef-libraries/", "config/fancymenu/layout_editor/", "Thumbs.db", ".DS_Store"}

func main() {
	pack := flag.String("pack", "", "instance folder")
	sources := flag.String("sources", "", "sources.json")
	base := flag.String("base", "", "base url for own files")
	version := flag.String("version", "1.0", "pack version")
	out := flag.String("out", "pack/manifest.json", "output manifest")
	copyDir := flag.String("copy", "", "copy own files here")
	meta := flag.String("meta", "", "meta.json with pack/server/news/loader")
	flag.Parse()

	src := map[string]struct {
		URL  string `json:"url"`
		SHA1 string `json:"sha1"`
	}{}
	if *sources != "" {
		b, err := os.ReadFile(*sources)
		must(err)
		must(json.Unmarshal(b, &src))
	}
	m := map[string]any{}
	if *meta != "" {
		b, err := os.ReadFile(*meta)
		must(err)
		must(json.Unmarshal(b, &m))
	}

	var files []PackFile
	add := func(rel string) {
		for _, s := range skipContains {
			if strings.Contains("/"+rel, s) {
				return
			}
		}
		full := filepath.Join(*pack, filepath.FromSlash(rel))
		fi, err := os.Stat(full)
		if err != nil || fi.IsDir() {
			return
		}
		h := sha1sum(full)
		f := PackFile{Path: rel, SHA1: h, Size: fi.Size()}
		for _, p := range oncePrefixes {
			if rel == p || (strings.HasSuffix(p, "/") && strings.HasPrefix(rel, p)) {
				f.Policy = "once"
			}
		}
		if s, ok := src[rel]; ok && s.URL != "" {
			if s.SHA1 != "" && !strings.EqualFold(s.SHA1, h) {
				fmt.Fprintf(os.Stderr, "WARN %s: sha1 differs from source, hosting ourselves\n", rel)
			} else {
				f.URL = s.URL
			}
		}
		if f.URL == "" && *copyDir != "" {
			dst := filepath.Join(*copyDir, filepath.FromSlash(rel))
			must(os.MkdirAll(filepath.Dir(dst), 0o755))
			copyFile(full, dst)
		}
		files = append(files, f)
	}
	for _, r := range includeRoot {
		add(r)
	}
	for _, d := range includeDirs {
		root := filepath.Join(*pack, filepath.FromSlash(d))
		_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(*pack, p)
			add(filepath.ToSlash(rel))
			return nil
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	mods := 0
	var hosted, external int64
	for _, f := range files {
		if strings.HasPrefix(f.Path, "mods/") && strings.HasSuffix(f.Path, ".jar") {
			mods++
		}
		if f.URL == "" {
			hosted += f.Size
		} else {
			external += f.Size
		}
	}
	m["version"] = *version
	m["baseUrl"] = *base
	m["modCount"] = mods
	m["files"] = files
	b, _ := json.MarshalIndent(m, "", " ")
	must(os.MkdirAll(filepath.Dir(*out), 0o755))
	must(os.WriteFile(*out, b, 0o644))
	fmt.Printf("files=%d mods=%d external=%.1fMB hosted=%.1fMB\n", len(files), mods, float64(external)/1e6, float64(hosted)/1e6)
}

func sha1sum(p string) string {
	f, err := os.Open(p)
	must(err)
	defer f.Close()
	h := sha1.New()
	_, err = io.Copy(h, f)
	must(err)
	return hex.EncodeToString(h.Sum(nil))
}

func copyFile(a, b string) {
	in, err := os.Open(a)
	must(err)
	defer in.Close()
	out, err := os.Create(b)
	must(err)
	defer out.Close()
	_, err = io.Copy(out, in)
	must(err)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
