package main

// Editions of the pack (2.0 "Optimization update"):
//   "full" — everything;
//   "lite" — for weaker PCs: heavy client-only graphics mods are left out and the game gets
//            cheaper settings (configureddefaults/options.lite.txt on top of the usual preset).
// A file without "editions" belongs to every edition. Old launchers ignore the field and keep
// installing the full pack. Switching the edition is an ordinary update: files of the other
// edition are removed by the usual "no longer in the pack" step.

type Edition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MemoryGB    int    `json:"memoryGB,omitempty"`
	Mods        int    `json:"mods,omitempty"`
	Size        int64  `json:"size,omitempty"`
}

const defaultEdition = "full"

// currentEdition is the edition chosen in the settings ("full" if none was chosen).
func currentEdition() string {
	if e := getSettings().Edition; e != "" {
		return e
	}
	return defaultEdition
}

func validEdition(id string) bool { return id == "full" || id == "lite" }

func (f PackFile) inEdition(ed string) bool {
	if len(f.Editions) == 0 {
		return true
	}
	for _, e := range f.Editions {
		if e == ed {
			return true
		}
	}
	return false
}

// forEdition returns a copy of the manifest with only the files of that edition.
func (m *Manifest) forEdition(ed string) *Manifest {
	c := *m
	c.Files = nil
	c.ModCount = 0
	for _, f := range m.Files {
		if f.inEdition(ed) {
			c.Files = append(c.Files, f)
			if len(f.Path) > 5 && f.Path[:5] == "mods/" {
				c.ModCount++
			}
		}
	}
	return &c
}

func (m *Manifest) edition(id string) *Edition {
	for i := range m.Editions {
		if m.Editions[i].ID == id {
			return &m.Editions[i]
		}
	}
	return nil
}

func editionName(m *Manifest, id string) string {
	if m != nil {
		if e := m.edition(id); e != nil {
			return e.Name
		}
	}
	if id == "lite" {
		return "Облегчённая"
	}
	return "Полная"
}
