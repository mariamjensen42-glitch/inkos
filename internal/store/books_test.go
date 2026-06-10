package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/narcooo/inkos/internal/model"
)

func setupBookStore(t *testing.T) (*BookStore, string) {
	t.Helper()
	dir := t.TempDir()
	// books/<id>/{chapters,story,story/state,story/runtime,story/snapshots}
	if err := os.MkdirAll(filepath.Join(dir, "books"), 0o755); err != nil {
		t.Fatal(err)
	}
	return NewBookStore(dir), dir
}

func TestBookStoreCreateList(t *testing.T) {
	bs, _ := setupBookStore(t)
	ids, _ := bs.ListBooks()
	if len(ids) != 0 {
		t.Fatalf("expected 0 books, got %d", len(ids))
	}
	cfg := &model.BookConfig{ID: "test-book", Title: "Test", Genre: "xuanhuan", Status: model.BookStatusActive}
	if err := bs.CreateBook(cfg); err != nil {
		t.Fatal(err)
	}
	ids, _ = bs.ListBooks()
	if len(ids) != 1 || ids[0] != "test-book" {
		t.Errorf("expected [test-book], got %v", ids)
	}
}

func TestBookStoreUnsafeID(t *testing.T) {
	bs, _ := setupBookStore(t)
	cfg := &model.BookConfig{ID: "../escape"}
	if err := bs.CreateBook(cfg); err == nil {
		t.Error("expected unsafe id to fail")
	}
}

func TestBookStoreChapterOps(t *testing.T) {
	bs, _ := setupBookStore(t)
	cfg := &model.BookConfig{ID: "alpha", Title: "A", Status: model.BookStatusActive}
	if err := bs.CreateBook(cfg); err != nil {
		t.Fatal(err)
	}
	if err := bs.WriteChapter("alpha", 1, "Hello, world."); err != nil {
		t.Fatal(err)
	}
	body, err := bs.ReadChapter("alpha", 1)
	if err != nil {
		t.Fatal(err)
	}
	if body != "Hello, world." {
		t.Errorf("got %q, want Hello, world.", body)
	}
	// Update index.
	meta := model.ChapterMeta{Number: 1, Title: "First", Status: model.ChapterStatusDraft, WordCount: 2}
	if err := bs.AddOrUpdateChapter("alpha", meta); err != nil {
		t.Fatal(err)
	}
	idx, err := bs.LoadChapterIndex("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Chapters) != 1 || idx.Chapters[0].Title != "First" {
		t.Errorf("index: %+v", idx)
	}
	next, _ := bs.NextChapterNumber("alpha")
	if next != 2 {
		t.Errorf("next = %d, want 2", next)
	}
}

func TestBookStoreLoadBookConfig(t *testing.T) {
	bs, _ := setupBookStore(t)
	cfg := &model.BookConfig{ID: "gamma", Title: "G", Status: model.BookStatusActive, ChapterWordCount: 3000}
	if err := bs.CreateBook(cfg); err != nil {
		t.Fatal(err)
	}
	got, err := bs.LoadBookConfig("gamma")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "G" || got.ChapterWordCount != 3000 {
		t.Errorf("got %+v", got)
	}
	// JSON round-trip
	data, _ := json.Marshal(got)
	_ = data
}

func TestBookStoreLock(t *testing.T) {
	bs, _ := setupBookStore(t)
	cfg := &model.BookConfig{ID: "delta", Status: model.BookStatusActive}
	if err := bs.CreateBook(cfg); err != nil {
		t.Fatal(err)
	}
	release, err := bs.Lock("delta")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
}
