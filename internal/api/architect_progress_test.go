package api

import (
	"testing"
	"time"
)

func TestArchitectProgressStore_SetGet(t *testing.T) {
	store := NewArchitectProgressStore()

	p := &ArchitectProgress{
		BookID:    "test-book",
		Phase:     ArchitectPhaseStarting,
		Message:   "hello",
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	}
	store.Set(p)

	got := store.Get("test-book")
	if got == nil {
		t.Fatal("expected progress, got nil")
	}
	if got.BookID != "test-book" {
		t.Errorf("expected test-book, got %s", got.BookID)
	}
	if got.Phase != ArchitectPhaseStarting {
		t.Errorf("expected starting, got %s", got.Phase)
	}
	if got.UpdatedAt == "" {
		t.Error("UpdatedAt should be set")
	}
}

func TestArchitectProgressStore_Update(t *testing.T) {
	store := NewArchitectProgressStore()

	store.Set(&ArchitectProgress{
		BookID:    "test-book",
		Phase:     ArchitectPhaseStarting,
		Message:   "starting",
		StartedAt: nowISO(),
	})

	old := store.Get("test-book")

	// Update.
	store.Set(&ArchitectProgress{
		BookID:    "test-book",
		Phase:     ArchitectPhaseFileGen,
		Message:   "generating",
		CurrentFile: "story_bible.md",
		StartedAt: old.StartedAt,
	})

	got := store.Get("test-book")
	if got.Phase != ArchitectPhaseFileGen {
		t.Errorf("expected file_gen, got %s", got.Phase)
	}
	if got.CurrentFile != "story_bible.md" {
		t.Errorf("expected story_bible.md, got %s", got.CurrentFile)
	}
	// UpdatedAt should be set.
	if got.UpdatedAt == "" {
		t.Error("UpdatedAt should be set after update")
	}
}

func TestArchitectProgressStore_GetNonexistent(t *testing.T) {
	store := NewArchitectProgressStore()

	got := store.Get("nonexistent")
	if got != nil {
		t.Errorf("expected nil for nonexistent book, got %+v", got)
	}
}

func TestArchitectProgressStore_Delete(t *testing.T) {
	store := NewArchitectProgressStore()

	store.Set(&ArchitectProgress{
		BookID:    "test-book",
		Phase:     ArchitectPhaseDone,
		Message:   "done",
		StartedAt: nowISO(),
	})

	if store.Get("test-book") == nil {
		t.Fatal("expected progress before delete")
	}

	store.Delete("test-book")

	if store.Get("test-book") != nil {
		t.Error("expected nil after delete")
	}
}

func TestArchitectProgressStore_MultipleBooks(t *testing.T) {
	store := NewArchitectProgressStore()

	store.Set(&ArchitectProgress{
		BookID: "book-1", Phase: ArchitectPhaseStarting,
		StartedAt: nowISO(),
	})
	store.Set(&ArchitectProgress{
		BookID: "book-2", Phase: ArchitectPhaseDone,
		StartedAt: nowISO(),
	})
	store.Set(&ArchitectProgress{
		BookID: "book-3", Phase: ArchitectPhaseError,
		Error:   "something went wrong",
		StartedAt: nowISO(),
	})

	if store.Get("book-1") == nil {
		t.Error("book-1 not found")
	}
	if store.Get("book-2") == nil {
		t.Error("book-2 not found")
	}
	b3 := store.Get("book-3")
	if b3 == nil {
		t.Error("book-3 not found")
	}
	if b3.Error != "something went wrong" {
		t.Errorf("expected error message, got %s", b3.Error)
	}
}