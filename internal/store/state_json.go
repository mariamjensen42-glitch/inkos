package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/narcooo/inkos/internal/model"
)

// StateJSONStore reads/writes the structured JSON state files under
// books/<id>/story/state/:
//
//	current_state.json   — persisted current state blob
//	hooks.json           — PendingHooks
//	chapter_summaries.json — ChapterSummariesState
//	manifest.json        — StateManifest
type StateJSONStore struct {
	root string
}

// NewStateJSONStore returns a store rooted at the project root.
func NewStateJSONStore(root string) *StateJSONStore {
	return &StateJSONStore{root: root}
}

// stateDir returns the absolute path to story/state/ for a book.
func (s *StateJSONStore) stateDir(bookID string) string {
	return filepath.Join(s.root, "books", bookID, "story", "state")
}

// --- current_state.json ---

// ReadCurrentState reads story/state/current_state.json. Returns an empty
// map if the file does not exist.
func (s *StateJSONStore) ReadCurrentState(bookID string) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	err := readJSONFile(s.stateDir(bookID), "current_state.json", &out)
	return out, err
}

// WriteCurrentState writes story/state/current_state.json atomically.
func (s *StateJSONStore) WriteCurrentState(bookID string, state map[string]interface{}) error {
	return writeJSONFile(s.stateDir(bookID), "current_state.json", state)
}

// --- hooks.json ---

// ReadHooks reads story/state/hooks.json. Returns an empty list if missing.
func (s *StateJSONStore) ReadHooks(bookID string) (*model.PendingHooks, error) {
	out := &model.PendingHooks{Hooks: []model.PendingHook{}}
	err := readJSONFile(s.stateDir(bookID), "hooks.json", out)
	return out, err
}

// WriteHooks writes story/state/hooks.json atomically.
func (s *StateJSONStore) WriteHooks(bookID string, hooks *model.PendingHooks) error {
	return writeJSONFile(s.stateDir(bookID), "hooks.json", hooks)
}

// --- chapter_summaries.json ---

// ReadChapterSummaries reads story/state/chapter_summaries.json.
func (s *StateJSONStore) ReadChapterSummaries(bookID string) (*model.ChapterSummariesState, error) {
	out := &model.ChapterSummariesState{Summaries: []model.ChapterSummaryRow{}}
	err := readJSONFile(s.stateDir(bookID), "chapter_summaries.json", out)
	return out, err
}

// WriteChapterSummaries writes story/state/chapter_summaries.json atomically.
func (s *StateJSONStore) WriteChapterSummaries(bookID string, summaries *model.ChapterSummariesState) error {
	return writeJSONFile(s.stateDir(bookID), "chapter_summaries.json", summaries)
}

// --- manifest.json ---

// ReadManifest reads story/state/manifest.json.
func (s *StateJSONStore) ReadManifest(bookID string) (*model.StateManifest, error) {
	out := &model.StateManifest{}
	err := readJSONFile(s.stateDir(bookID), "manifest.json", out)
	return out, err
}

// WriteManifest writes story/state/manifest.json atomically.
func (s *StateJSONStore) WriteManifest(bookID string, manifest *model.StateManifest) error {
	return writeJSONFile(s.stateDir(bookID), "manifest.json", manifest)
}

// --- Init ---

// Init creates empty state JSON files for a new book.
func (s *StateJSONStore) Init(bookID string) error {
	dir := s.stateDir(bookID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("state_json init mkdir: %w", err)
	}
	// Write empty defaults for each file.
	if err := writeJSONFile(dir, "current_state.json", map[string]interface{}{}); err != nil {
		return err
	}
	if err := writeJSONFile(dir, "hooks.json", &model.PendingHooks{Hooks: []model.PendingHook{}}); err != nil {
		return err
	}
	if err := writeJSONFile(dir, "chapter_summaries.json", &model.ChapterSummariesState{Summaries: []model.ChapterSummaryRow{}}); err != nil {
		return err
	}
	if err := writeJSONFile(dir, "manifest.json", &model.StateManifest{
		Version:     1,
		HasLegacy:   false,
		TruthFiles:  []string{},
		StateFiles:  []string{},
		MemoryDB:    false,
		PlayDB:      false,
	}); err != nil {
		return err
	}
	return nil
}

// --- ApplyDelta (skeleton) ---

// ApplyDelta applies a partial state update to a book. This is a skeleton
// that will be fleshed out in a later subproject.
func (s *StateJSONStore) ApplyDelta(bookID string, delta interface{}) error {
	// TODO: implement full delta merging logic.
	return fmt.Errorf("ApplyDelta not yet implemented")
}

// --- helpers ---

func readJSONFile(dir, name string, out interface{}) error {
	path := filepath.Join(dir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", name, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("unmarshal %s: %w", name, err)
	}
	return nil
}

func writeJSONFile(dir, name string, v interface{}) error {
	path := filepath.Join(dir, name)
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", name, err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return os.Rename(tmp, path)
}