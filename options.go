package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Game settings and controls of the pack.
//
// The pack ships its preset as configureddefaults/options.txt (the ConfiguredDefaults mod
// only copies it when options.txt does not exist yet, so on an existing profile it never
// did anything). The launcher merges the preset into options.txt itself:
//   - no options.txt yet, first install into a profile, or "force": every preset key wins
//     (the old file is backed up as options.txt.vibeclub-backup-<date>);
//   - pack update: a key is changed only if the player did not touch it since the last
//     apply (current value == value we applied) or the key is missing.
// Keys outside the preset (other mods, the player's own extras) are never touched.

const packOptionsPath = "configureddefaults/options.txt"

type optLine struct{ key, val, raw string }

func parseOptions(b []byte) []optLine {
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	var out []optLine
	for _, raw := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		if raw == "" {
			continue
		}
		i := strings.Index(raw, ":")
		if i <= 0 {
			out = append(out, optLine{raw: raw})
			continue
		}
		out = append(out, optLine{key: raw[:i], val: raw[i+1:], raw: raw})
	}
	return out
}

func optMap(lines []optLine) map[string]string {
	m := map[string]string{}
	for _, l := range lines {
		if l.key != "" {
			m[l.key] = l.val
		}
	}
	return m
}

func appliedOptionsPath(dir string) string {
	return filepath.Join(dir, ".vibeclub", "options.applied.txt")
}

// applyPackOptions returns how many settings were changed.
func applyPackOptions(dir string, force bool) (int, error) {
	presetB, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(packOptionsPath)))
	if err != nil {
		return 0, nil // the pack has no preset
	}
	preset := parseOptions(presetB)
	target := filepath.Join(dir, "options.txt")
	curB, curErr := os.ReadFile(target)
	if curErr != nil {
		if err := writeFileAtomic(target, presetB); err != nil {
			return 0, err
		}
		_ = writeFileAtomic(appliedOptionsPath(dir), presetB)
		return len(optMap(preset)), nil
	}
	appliedB, appliedErr := os.ReadFile(appliedOptionsPath(dir))
	full := force || appliedErr != nil
	applied := optMap(parseOptions(appliedB))
	cur := parseOptions(curB)
	curMap := optMap(cur)

	want := map[string]string{}
	for _, p := range preset {
		if p.key == "" {
			continue
		}
		have, ok := curMap[p.key]
		switch {
		case ok && have == p.val:
		case full || !ok:
			want[p.key] = p.val
		case have == applied[p.key]: // the player never changed it — follow the pack
			want[p.key] = p.val
		}
	}
	if len(want) == 0 {
		_ = writeFileAtomic(appliedOptionsPath(dir), presetB)
		return 0, nil
	}
	if full {
		backup := target + ".vibeclub-backup-" + time.Now().Format("2006-01-02_15-04")
		_ = os.WriteFile(backup, curB, 0o644)
	}
	var sb strings.Builder
	done := map[string]bool{}
	for _, l := range cur {
		if v, ok := want[l.key]; ok && l.key != "" {
			sb.WriteString(l.key + ":" + v + "\n")
			done[l.key] = true
			continue
		}
		sb.WriteString(l.raw + "\n")
	}
	for _, p := range preset {
		if v, ok := want[p.key]; ok && !done[p.key] {
			sb.WriteString(p.key + ":" + v + "\n")
			done[p.key] = true
		}
	}
	if err := writeFileAtomic(target, []byte(sb.String())); err != nil {
		return 0, err
	}
	_ = writeFileAtomic(appliedOptionsPath(dir), presetB)
	return len(want), nil
}
