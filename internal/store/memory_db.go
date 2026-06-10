package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// MemoryDB is the per-book SQLite time-series memory store. Schema
// mirrors packages/core/src/state/memory-db.ts.
type MemoryDB struct {
	mu sync.Mutex
	db *sql.DB
}

// NewMemoryDB opens (and creates) the memory.db SQLite file.
func NewMemoryDB(path string) (*MemoryDB, error) {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	m := &MemoryDB{db: db}
	if err := m.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return m, nil
}

func (m *MemoryDB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS facts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			kind TEXT NOT NULL,
			subject TEXT,
			predicate TEXT,
			object TEXT,
			chapter INTEGER,
			confidence REAL,
			text TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS hooks (
			id TEXT PRIMARY KEY,
			description TEXT,
			status TEXT,
			chapter INTEGER,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS summaries (
			chapter INTEGER PRIMARY KEY,
			title TEXT,
			summary TEXT,
			updated_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_facts_kind ON facts(kind)`,
		`CREATE INDEX IF NOT EXISTS idx_facts_chapter ON facts(chapter)`,
		`CREATE INDEX IF NOT EXISTS idx_hooks_status ON hooks(status)`,
	}
	for _, s := range stmts {
		if _, err := m.db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

// Close releases the database handle.
func (m *MemoryDB) Close() error { return m.db.Close() }

// AddFact inserts a fact row.
func (m *MemoryDB) AddFact(kind, subject, predicate, obj, text string, chapter int, confidence float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, err := m.db.Exec(
		`INSERT INTO facts(kind, subject, predicate, object, text, chapter, confidence) VALUES (?,?,?,?,?,?,?)`,
		kind, subject, predicate, obj, text, chapter, confidence,
	)
	return err
}

// AddHook inserts/updates a hook row.
func (m *MemoryDB) AddHook(id, description, status string, chapter int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, err := m.db.Exec(
		`INSERT INTO hooks(id, description, status, chapter) VALUES(?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET description=excluded.description, status=excluded.status, chapter=excluded.chapter`,
		id, description, status, chapter,
	)
	return err
}

// UpsertSummary inserts or replaces a chapter summary row.
func (m *MemoryDB) UpsertSummary(chapter int, title, summary, updatedAt string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, err := m.db.Exec(
		`INSERT INTO summaries(chapter, title, summary, updated_at) VALUES(?,?,?,?)
		 ON CONFLICT(chapter) DO UPDATE SET title=excluded.title, summary=excluded.summary, updated_at=excluded.updated_at`,
		chapter, title, summary, updatedAt,
	)
	return err
}

// FactRow is a row in the facts table.
type FactRow struct {
	ID         int64   `json:"id"`
	Kind       string  `json:"kind"`
	Subject    string  `json:"subject,omitempty"`
	Predicate  string  `json:"predicate,omitempty"`
	Object     string  `json:"object,omitempty"`
	Chapter    int     `json:"chapter"`
	Confidence float64 `json:"confidence"`
	Text       string  `json:"text,omitempty"`
}

// RecentFacts returns the most recent N facts.
func (m *MemoryDB) RecentFacts(limit int) ([]FactRow, error) {
	rows, err := m.db.Query(`SELECT id, kind, COALESCE(subject,''), COALESCE(predicate,''), COALESCE(object,''), chapter, confidence, COALESCE(text,'') FROM facts ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FactRow{}
	for rows.Next() {
		var f FactRow
		if err := rows.Scan(&f.ID, &f.Kind, &f.Subject, &f.Predicate, &f.Object, &f.Chapter, &f.Confidence, &f.Text); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func ensureDir(dir string) error {
	return mkdirAll(dir, 0o755)
}
