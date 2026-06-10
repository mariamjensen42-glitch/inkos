package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/narcooo/inkos/internal/util"
)

// AllowedTruthFiles enumerates the truth files that can be read/written
// via the API. All other story/*.md files are not editable through HTTP
// (they are generated artifacts).
var AllowedTruthFiles = map[string]struct{}{
	"story_bible.md":          {},
	"book_rules.md":           {},
	"author_intent.md":        {},
	"current_focus.md":        {},
	"current_state.md":        {},
	"pending_hooks.md":        {},
	"chapter_summaries.md":    {},
	"character_matrix.md":     {},
	"volume_outline.md":       {},
	"style_profile.md":        {},
}

// TruthStore reads/writes files under story/.
type TruthStore struct {
	root string
}

// NewTruthStore returns a truth-file store rooted at the project root.
func NewTruthStore(root string) *TruthStore {
	return &TruthStore{root: root}
}

// TruthPath returns the absolute path to a story truth file.
func (s *TruthStore) TruthPath(bookID, name string) string {
	return filepath.Join(s.root, "books", bookID, "story", TruthFileName(name))
}

// ReadTruth reads a whitelisted truth file, returning ErrNotExist otherwise.
func (s *TruthStore) ReadTruth(bookID, name string) (string, error) {
	if !util.IsSafeBookID(bookID) {
		return "", fmt.Errorf("unsafe book id")
	}
	base := strings.TrimSuffix(name, ".md")
	if _, ok := AllowedTruthFiles[base+".md"]; !ok {
		return "", fmt.Errorf("truth file not allowed: %s", name)
	}
	data, err := os.ReadFile(s.TruthPath(bookID, base))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteTruth writes a whitelisted truth file atomically.
func (s *TruthStore) WriteTruth(bookID, name, body string) error {
	if !util.IsSafeBookID(bookID) {
		return fmt.Errorf("unsafe book id")
	}
	base := strings.TrimSuffix(name, ".md")
	if _, ok := AllowedTruthFiles[base+".md"]; !ok {
		return fmt.Errorf("truth file not allowed: %s", name)
	}
	target := s.TruthPath(bookID, base)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

// ListTruth returns the list of truth files that currently exist on disk.
func (s *TruthStore) ListTruth(bookID string) ([]string, error) {
	if !util.IsSafeBookID(bookID) {
		return nil, fmt.Errorf("unsafe book id")
	}
	dir := filepath.Join(s.root, "books", bookID, "story")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	out := []string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		if _, ok := AllowedTruthFiles[name]; !ok {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}
