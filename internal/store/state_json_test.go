package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/narcooo/inkos/internal/model"
)

func TestStateJSON_ReadWriteCurrentState(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStateJSONStore(tmpDir)

	// Init first.
	if err := store.Init("test-book"); err != nil {
		t.Fatal(err)
	}

	// Read empty state.
	state, err := store.ReadCurrentState("test-book")
	if err != nil {
		t.Fatal(err)
	}
	if len(state) != 0 {
		t.Errorf("expected empty state, got %v", state)
	}

	// Write state.
	input := map[string]interface{}{"chapter": 5, "status": "writing"}
	if err := store.WriteCurrentState("test-book", input); err != nil {
		t.Fatal(err)
	}

	// Read back.
	state, err = store.ReadCurrentState("test-book")
	if err != nil {
		t.Fatal(err)
	}
	if state["chapter"].(float64) != 5 {
		t.Errorf("expected chapter=5, got %v", state["chapter"])
	}
	if state["status"] != "writing" {
		t.Errorf("expected status=writing, got %v", state["status"])
	}
}

func TestStateJSON_ReadWriteHooks(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStateJSONStore(tmpDir)

	if err := store.Init("test-book"); err != nil {
		t.Fatal(err)
	}

	hooks, err := store.ReadHooks("test-book")
	if err != nil {
		t.Fatal(err)
	}
	if len(hooks.Hooks) != 0 {
		t.Errorf("expected empty hooks, got %d", len(hooks.Hooks))
	}

	input := &model.PendingHooks{
		Hooks: []model.PendingHook{
			{ID: "h1", Description: "A mysterious stranger", Status: model.HookStatusOpen},
		},
	}
	if err := store.WriteHooks("test-book", input); err != nil {
		t.Fatal(err)
	}

	hooks, err = store.ReadHooks("test-book")
	if err != nil {
		t.Fatal(err)
	}
	if len(hooks.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks.Hooks))
	}
	if hooks.Hooks[0].ID != "h1" {
		t.Errorf("expected h1, got %s", hooks.Hooks[0].ID)
	}
}

func TestStateJSON_ReadWriteChapterSummaries(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStateJSONStore(tmpDir)

	if err := store.Init("test-book"); err != nil {
		t.Fatal(err)
	}

	summaries, err := store.ReadChapterSummaries("test-book")
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries.Summaries) != 0 {
		t.Errorf("expected empty summaries, got %d", len(summaries.Summaries))
	}

	input := &model.ChapterSummariesState{
		Summaries: []model.ChapterSummaryRow{
			{Number: 1, Title: "Chapter 1", Summary: "The beginning"},
		},
	}
	if err := store.WriteChapterSummaries("test-book", input); err != nil {
		t.Fatal(err)
	}

	summaries, err = store.ReadChapterSummaries("test-book")
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries.Summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries.Summaries))
	}
}

func TestStateJSON_ReadWriteManifest(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStateJSONStore(tmpDir)

	if err := store.Init("test-book"); err != nil {
		t.Fatal(err)
	}

	manifest, err := store.ReadManifest("test-book")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != 1 {
		t.Errorf("expected version=1, got %d", manifest.Version)
	}
}

func TestStateJSON_Init(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStateJSONStore(tmpDir)

	if err := store.Init("test-book"); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(tmpDir, "books", "test-book", "story", "state")
	files := []string{"current_state.json", "hooks.json", "chapter_summaries.json", "manifest.json"}
	for _, name := range files {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", name)
		}
	}
}

func TestStateJSON_ApplyDeltaSkeleton(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStateJSONStore(tmpDir)

	if err := store.Init("test-book"); err != nil {
		t.Fatal(err)
	}

	err := store.ApplyDelta("test-book", map[string]interface{}{"status": "updated"})
	if err == nil {
		t.Fatal("expected error from ApplyDelta skeleton")
	}
	if err.Error() != "ApplyDelta not yet implemented" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStateJSON_NonExistentBook(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStateJSONStore(tmpDir)

	// Reading non-existent book should return defaults, not errors.
	state, err := store.ReadCurrentState("no-such-book")
	if err != nil {
		t.Fatal(err)
	}
	if len(state) != 0 {
		t.Errorf("expected empty state, got %+v", state)
	}

	hooks, err := store.ReadHooks("no-such-book")
	if err != nil {
		t.Fatal(err)
	}
	if hooks == nil || len(hooks.Hooks) != 0 {
		t.Errorf("expected empty hooks, got %+v", hooks)
	}
}