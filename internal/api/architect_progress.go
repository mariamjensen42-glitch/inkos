// Package api: architect progress tracking via sync.Map.
package api

import (
	"sync"
	"time"
)

// ArchitectPhase represents a stage in the architect agent lifecycle.
type ArchitectPhase string

const (
	ArchitectPhaseStarting ArchitectPhase = "starting"
	ArchitectPhasePlanning ArchitectPhase = "planning"
	ArchitectPhaseFileGen  ArchitectPhase = "file_gen"
	ArchitectPhaseDone     ArchitectPhase = "done"
	ArchitectPhaseError    ArchitectPhase = "error"
)

// ArchitectProgress tracks the current progress of an architect run for a book.
type ArchitectProgress struct {
	BookID      string         `json:"bookId"`
	Phase       ArchitectPhase `json:"phase"`
	Message     string         `json:"message"`
	CurrentFile string         `json:"currentFile,omitempty"`
	Error       string         `json:"error,omitempty"`
	StartedAt   string         `json:"startedAt"`
	UpdatedAt   string         `json:"updatedAt"`
}

// ArchitectProgressStore holds per-book progress in a sync.Map.
type ArchitectProgressStore struct {
	m sync.Map // bookID -> *ArchitectProgress
}

// NewArchitectProgressStore returns a ready-to-use store.
func NewArchitectProgressStore() *ArchitectProgressStore {
	return &ArchitectProgressStore{}
}

// Set stores (or replaces) progress for a book.
func (s *ArchitectProgressStore) Set(p *ArchitectProgress) {
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	s.m.Store(p.BookID, p)
}

// Get retrieves progress for a book, or nil if not found.
func (s *ArchitectProgressStore) Get(bookID string) *ArchitectProgress {
	v, ok := s.m.Load(bookID)
	if !ok {
		return nil
	}
	return v.(*ArchitectProgress)
}

// Delete removes progress for a book.
func (s *ArchitectProgressStore) Delete(bookID string) {
	s.m.Delete(bookID)
}