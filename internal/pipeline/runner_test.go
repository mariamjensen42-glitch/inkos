package pipeline

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/narcooo/inkos/internal/llm"
	"github.com/narcooo/inkos/internal/model"
	"github.com/narcooo/inkos/internal/store"
)

// fakeBroadcaster captures published events for assertions.
type fakeBroadcaster struct {
	mu     sync.Mutex
	events []SSEEvent
}

func (f *fakeBroadcaster) PublishEvent(name string, data interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, SSEEvent{Event: name, Data: data})
}

func (f *fakeBroadcaster) Names() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.events))
	for _, e := range f.events {
		out = append(out, e.Event)
	}
	return out
}

// makeBook scaffolds a minimal book directory using BookStore.CreateBook
// so the runner can load context. All the on-disk skeleton directories
// (story/, chapters/, runtime/, snapshots/) are created for us.
func makeBook(t *testing.T, books *store.BookStore, id string) {
	t.Helper()
	cfg := &model.BookConfig{
		ID:               id,
		Title:            "Test " + id,
		Genre:            "fantasy",
		Language:         "zh",
		ChapterWordCount: 1000,
	}
	if err := books.CreateBook(cfg); err != nil {
		t.Fatal(err)
	}
}

// TestRunnerNoResolverErrors verifies the runner returns ErrNoResolver
// when no LLM resolver factory is configured.
func TestRunnerNoResolverErrors(t *testing.T) {
	root := t.TempDir()
	truth := store.NewTruthStore(root)
	project := store.NewProjectStore(root)
	books := store.NewBookStore(root)
	makeBook(t, books, "b1")

	bc := &fakeBroadcaster{}
	r := NewRunner(books, truth, project, bc, nil)

	_, err := r.Plan(context.Background(), "b1", 1, Options{})
	if !errors.Is(err, ErrNoResolver) {
		t.Fatalf("expected ErrNoResolver, got %v", err)
	}
	// We should still have published a start and an error event for
	// observability.
	names := bc.Names()
	hasStart := false
	hasError := false
	for _, n := range names {
		if n == "pipeline:plan:start" {
			hasStart = true
		}
		if n == "pipeline:plan:error" {
			hasError = true
		}
	}
	if !hasStart || !hasError {
		t.Fatalf("expected plan:start and plan:error events, got %v", names)
	}
}

// TestRunnerResolverFailsCleanly verifies that a resolver returning an
// error produces a clean error event without crashing.
func TestRunnerResolverFailsCleanly(t *testing.T) {
	root := t.TempDir()
	truth := store.NewTruthStore(root)
	project := store.NewProjectStore(root)
	books := store.NewBookStore(root)
	makeBook(t, books, "b2")

	bc := &fakeBroadcaster{}
	// Resolver factory returns a resolver, but the resolver's Client
	// method always errors.
	rf := func() *llm.Resolver {
		return &llm.Resolver{}
	}
	r := NewRunner(books, truth, project, bc, rf)

	_, err := r.Plan(context.Background(), "b2", 1, Options{})
	if err == nil {
		t.Fatal("expected an error from Plan with a broken resolver")
	}
	if !contains(bc.Names(), "pipeline:plan:error") {
		t.Fatalf("expected plan:error event, got %v", bc.Names())
	}
}

// TestRunnerNextChapterFallback verifies that when the chapter number
// is 0 in the request, the runner uses NextChapterNumber (which returns
// 1 for a fresh book with no chapter files).
func TestRunnerNextChapterFallback(t *testing.T) {
	root := t.TempDir()
	truth := store.NewTruthStore(root)
	project := store.NewProjectStore(root)
	books := store.NewBookStore(root)
	makeBook(t, books, "b3")

	bc := &fakeBroadcaster{}
	rf := func() *llm.Resolver { return &llm.Resolver{} } // broken
	r := NewRunner(books, truth, project, bc, rf)

	// 0 → use NextChapterNumber. Should resolve to 1, then error out
	// at the resolver step.
	_, err := r.Plan(context.Background(), "b3", 0, Options{})
	if err == nil {
		t.Fatal("expected error from broken resolver")
	}
	// The broadcast event should carry chapter=1.
	bc.mu.Lock()
	defer bc.mu.Unlock()
	found := false
	for _, e := range bc.events {
		if e.Event == "pipeline:plan:start" {
			if m, ok := e.Data.(map[string]interface{}); ok {
				if ch, ok := m["chapter"].(int); ok && ch == 1 {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected plan:start event with chapter=1, got %v", bc.events)
	}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
