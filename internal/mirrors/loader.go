package mirrors

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

//go:embed embedded.yml
var embeddedYAML []byte

type Store struct {
	File       File
	byCategory map[string][]Mirror
	byID       map[string]Mirror
}

// Load reads the embedded mirrors.yml, then optionally merges in
// ~/.config/cm/mirrors.yml (user override) and any provided extra
// files (used by plugins).
func Load(extraFiles ...string) (*Store, error) {
	base, err := parse(embeddedYAML, "builtin")
	if err != nil {
		return nil, fmt.Errorf("parse embedded mirrors.yml: %w", err)
	}

	if home, err := os.UserHomeDir(); err == nil {
		userFile := filepath.Join(home, ".config", "cm", "mirrors.yml")
		if data, err := os.ReadFile(userFile); err == nil {
			overlay, err := parse(data, "user")
			if err != nil {
				return nil, fmt.Errorf("parse %s: %w", userFile, err)
			}
			base = merge(base, overlay)
		}
	}

	for _, p := range extraFiles {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		overlay, err := parse(data, p)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
		base = merge(base, overlay)
	}

	return index(base), nil
}

func parse(data []byte, source string) (File, error) {
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return File{}, err
	}
	for i := range f.Mirrors {
		f.Mirrors[i].Source = source
	}
	return f, nil
}

// merge appends overlay mirrors and replaces by id when collisions occur.
// Categories are merged map-style (overlay wins per key).
func merge(base, overlay File) File {
	existing := make(map[string]int, len(base.Mirrors))
	for i, m := range base.Mirrors {
		existing[m.ID] = i
	}
	for _, m := range overlay.Mirrors {
		if idx, ok := existing[m.ID]; ok {
			base.Mirrors[idx] = m
		} else {
			base.Mirrors = append(base.Mirrors, m)
		}
	}
	if base.Categories == nil {
		base.Categories = map[string]Category{}
	}
	for k, v := range overlay.Categories {
		base.Categories[k] = v
	}
	return base
}

func index(f File) *Store {
	s := &Store{
		File:       f,
		byCategory: make(map[string][]Mirror),
		byID:       make(map[string]Mirror, len(f.Mirrors)),
	}
	for _, m := range f.Mirrors {
		s.byCategory[m.Category] = append(s.byCategory[m.Category], m)
		s.byID[m.ID] = m
	}
	for cat, list := range s.byCategory {
		sort.SliceStable(list, func(i, j int) bool {
			return list[i].Priority < list[j].Priority
		})
		s.byCategory[cat] = list
	}
	return s
}

func (s *Store) All() []Mirror { return s.File.Mirrors }

func (s *Store) ByCategory(category string) []Mirror {
	return s.byCategory[category]
}

func (s *Store) Get(id string) (Mirror, bool) {
	m, ok := s.byID[id]
	return m, ok
}

// Default returns the first active mirror in a category, ordered by priority.
func (s *Store) Default(category string) (Mirror, bool) {
	for _, m := range s.byCategory[category] {
		if m.Status == StatusActive {
			return m, true
		}
	}
	return Mirror{}, false
}

func (s *Store) Categories() map[string]Category { return s.File.Categories }
