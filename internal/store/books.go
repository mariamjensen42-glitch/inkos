// Package store: book-level filesystem layout.
//
//	books/<id>/
//	  book.json
//	  chapters/
//	    index.json
//	    0001.md
//	    0001.audit.md
//	    0001.revised.md
//	    ...
//	  story/
//	    story_bible.md
//	    book_rules.md
//	    author_intent.md
//	    current_focus.md
//	    current_state.md
//	    pending_hooks.md
//	    chapter_summaries.md
//	    character_matrix.md
//	    volume_outline.md
//	    state/
//	      current_state.json
//	      hooks.json
//	      chapter_summaries.json
//	      manifest.json
//	    runtime/
//	      chapter-XXXX.intent.md
//	      chapter-XXXX.context.json
//	      chapter-XXXX.rule-stack.yaml
//	      chapter-XXXX.trace.json
//	    memory.db
//	    snapshots/
//	      chapter-XXXX.json
//	    play.db   or play.json
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gofrs/flock"
	"github.com/narcooo/inkos/internal/model"
	"github.com/narcooo/inkos/internal/util"
)

// BookStore handles filesystem operations for the books/ tree.
type BookStore struct {
	root   string
	booksDir string
	locks  sync.Map // per-book flock
}

// NewBookStore returns a store rooted at <root>/books.
func NewBookStore(root string) *BookStore {
	return &BookStore{
		root:     root,
		booksDir: filepath.Join(root, "books"),
	}
}

// BooksDir returns the absolute path to the books/ directory.
func (s *BookStore) BooksDir() string { return s.booksDir }

// ListBooks returns the sorted list of book ids present on disk.
func (s *BookStore) ListBooks() ([]string, error) {
	entries, err := os.ReadDir(s.booksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !util.IsSafeBookID(e.Name()) {
			continue
		}
		ids = append(ids, e.Name())
	}
	sort.Strings(ids)
	return ids, nil
}

// BookDir returns the absolute directory for a given book.
func (s *BookStore) BookDir(id string) string {
	return filepath.Join(s.booksDir, id)
}

// ChapterDir returns the chapters directory for a book.
func (s *BookStore) ChapterDir(id string) string {
	return filepath.Join(s.booksDir, id, "chapters")
}

// StoryDir returns the story/ directory for a book.
func (s *BookStore) StoryDir(id string) string {
	return filepath.Join(s.booksDir, id, "story")
}

// StateDir returns the story/state/ directory.
func (s *BookStore) StateDir(id string) string {
	return filepath.Join(s.booksDir, id, "story", "state")
}

// RuntimeDir returns the story/runtime/ directory.
func (s *BookStore) RuntimeDir(id string) string {
	return filepath.Join(s.booksDir, id, "story", "runtime")
}

// SnapshotsDir returns the story/snapshots/ directory.
func (s *BookStore) SnapshotsDir(id string) string {
	return filepath.Join(s.booksDir, id, "story", "snapshots")
}

// MemoryDBPath returns the SQLite memory.db path.
func (s *BookStore) MemoryDBPath(id string) string {
	return filepath.Join(s.booksDir, id, "story", "memory.db")
}

// PlayDBPath returns the SQLite play.db path.
func (s *BookStore) PlayDBPath(id string) string {
	return filepath.Join(s.booksDir, id, "story", "play.db")
}

// BookExists reports whether the book directory and book.json exist.
func (s *BookStore) BookExists(id string) bool {
	if !util.IsSafeBookID(id) {
		return false
	}
	_, err := os.Stat(filepath.Join(s.BookDir(id), "book.json"))
	return err == nil
}

// CreateBook creates the on-disk skeleton for a new book.
func (s *BookStore) CreateBook(cfg *model.BookConfig) error {
	if !util.IsSafeBookID(cfg.ID) {
		return fmt.Errorf("unsafe book id: %q", cfg.ID)
	}
	dirs := []string{
		s.BookDir(cfg.ID),
		s.ChapterDir(cfg.ID),
		s.StoryDir(cfg.ID),
		s.StateDir(cfg.ID),
		s.RuntimeDir(cfg.ID),
		s.SnapshotsDir(cfg.ID),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	// Write book.json atomically.
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	target := filepath.Join(s.BookDir(cfg.ID), "book.json")
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

// LoadBookConfig reads books/<id>/book.json.
func (s *BookStore) LoadBookConfig(id string) (*model.BookConfig, error) {
	if !util.IsSafeBookID(id) {
		return nil, fmt.Errorf("unsafe book id: %q", id)
	}
	data, err := os.ReadFile(filepath.Join(s.BookDir(id), "book.json"))
	if err != nil {
		return nil, err
	}
	cfg := &model.BookConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// SaveBookConfig writes books/<id>/book.json atomically.
func (s *BookStore) SaveBookConfig(cfg *model.BookConfig) error {
	if !util.IsSafeBookID(cfg.ID) {
		return fmt.Errorf("unsafe book id: %q", cfg.ID)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	target := filepath.Join(s.BookDir(cfg.ID), "book.json")
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

// DeleteBook removes a book directory recursively.
func (s *BookStore) DeleteBook(id string) error {
	if !util.IsSafeBookID(id) {
		return fmt.Errorf("unsafe book id: %q", id)
	}
	return os.RemoveAll(s.BookDir(id))
}

// LoadChapterIndex reads books/<id>/chapters/index.json, returning an
// empty index if it does not exist.
func (s *BookStore) LoadChapterIndex(id string) (*model.ChapterIndex, error) {
	if !util.IsSafeBookID(id) {
		return nil, fmt.Errorf("unsafe book id: %q", id)
	}
	data, err := os.ReadFile(filepath.Join(s.ChapterDir(id), "index.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return &model.ChapterIndex{Chapters: []model.ChapterMeta{}}, nil
		}
		return nil, err
	}
	idx := &model.ChapterIndex{}
	if err := json.Unmarshal(data, idx); err != nil {
		return nil, err
	}
	return idx, nil
}

// SaveChapterIndex writes books/<id>/chapters/index.json atomically.
func (s *BookStore) SaveChapterIndex(id string, idx *model.ChapterIndex) error {
	if !util.IsSafeBookID(id) {
		return fmt.Errorf("unsafe book id: %q", id)
	}
	if idx == nil {
		idx = &model.ChapterIndex{}
	}
	if idx.Chapters == nil {
		idx.Chapters = []model.ChapterMeta{}
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	target := filepath.Join(s.ChapterDir(id), "index.json")
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

// ChapterFileName returns the canonical 4-digit chapter file name (e.g. "0001.md").
func ChapterFileName(num int) string {
	return fmt.Sprintf("%04d.md", num)
}

// ChapterPath returns the absolute path to a chapter file.
func (s *BookStore) ChapterPath(id string, num int) string {
	return filepath.Join(s.ChapterDir(id), ChapterFileName(num))
}

// WriteChapter writes a chapter file (numbered 0001.md) atomically.
func (s *BookStore) WriteChapter(id string, num int, body string) error {
	if !util.IsSafeBookID(id) {
		return fmt.Errorf("unsafe book id: %q", id)
	}
	target := s.ChapterPath(id, num)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

// ReadChapter reads a chapter file; returns os.ErrNotExist if missing.
func (s *BookStore) ReadChapter(id string, num int) (string, error) {
	if !util.IsSafeBookID(id) {
		return "", fmt.Errorf("unsafe book id: %q", id)
	}
	data, err := os.ReadFile(s.ChapterPath(id, num))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ChapterExists reports whether a numbered chapter file exists.
func (s *BookStore) ChapterExists(id string, num int) bool {
	_, err := os.Stat(s.ChapterPath(id, num))
	return err == nil
}

// NextChapterNumber returns 1 + max existing chapter number, or 1 if none.
func (s *BookStore) NextChapterNumber(id string) (int, error) {
	idx, err := s.LoadChapterIndex(id)
	if err != nil {
		return 1, err
	}
	maxN := 0
	for _, c := range idx.Chapters {
		if c.Number > maxN {
			maxN = c.Number
		}
	}
	return maxN + 1, nil
}

// AddOrUpdateChapter upserts a row in the chapter index.
func (s *BookStore) AddOrUpdateChapter(id string, meta model.ChapterMeta) error {
	idx, err := s.LoadChapterIndex(id)
	if err != nil {
		return err
	}
	found := false
	for i, c := range idx.Chapters {
		if c.Number == meta.Number {
			idx.Chapters[i] = meta
			found = true
			break
		}
	}
	if !found {
		idx.Chapters = append(idx.Chapters, meta)
	}
	return s.SaveChapterIndex(id, idx)
}

// Lock acquires a per-book exclusive flock for safe concurrent writes.
// The returned function must be called to release the lock.
func (s *BookStore) Lock(id string) (func(), error) {
	if !util.IsSafeBookID(id) {
		return nil, fmt.Errorf("unsafe book id: %q", id)
	}
	v, _ := s.locks.LoadOrStore(id, flock.New(filepath.Join(s.booksDir, id, ".lock")))
	l := v.(*flock.Flock)
	if err := l.Lock(); err != nil {
		return nil, err
	}
	return func() { _ = l.Unlock() }, nil
}

// FormatChapterNumber formats a chapter number as 4-digit zero-padded.
func FormatChapterNumber(n int) string {
	return strconv.Itoa(n)
}

// TruthFileName returns the canonical truth filename (always .md).
func TruthFileName(name string) string {
	if !strings.HasSuffix(name, ".md") {
		return name + ".md"
	}
	return name
}
