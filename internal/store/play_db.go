package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/narcooo/inkos/internal/model"

	_ "modernc.org/sqlite"
)

// PlayDB is the per-book SQLite database for "Play" (interactive fiction)
// state. Schema mirrors the TypeScript play engine tables.
type PlayDB struct {
	db *sql.DB
}

// NewPlayDB opens (and creates) the play.db SQLite file.
func NewPlayDB(path string) (*PlayDB, error) {
	if err := mkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	p := &PlayDB{db: db}
	if err := p.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return p, nil
}

func (p *PlayDB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS entities (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL DEFAULT 'character',
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			visibility TEXT NOT NULL DEFAULT 'public',
			tags TEXT DEFAULT '[]',
			rarity TEXT DEFAULT '',
			image_prompt TEXT DEFAULT '',
			image_file TEXT DEFAULT '',
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS edges (
			id TEXT PRIMARY KEY,
			from_id TEXT NOT NULL,
			to_id TEXT NOT NULL,
			label TEXT DEFAULT '',
			weight INTEGER DEFAULT 0,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(from_id) REFERENCES entities(id) ON DELETE CASCADE,
			FOREIGN KEY(to_id) REFERENCES entities(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS state_slots (
			id TEXT PRIMARY KEY,
			entity_id TEXT NOT NULL,
			kind TEXT NOT NULL DEFAULT 'text',
			key TEXT NOT NULL,
			value_num REAL,
			value_text TEXT,
			value_bool INTEGER,
			value_enum TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(entity_id) REFERENCES entities(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS evidence (
			id TEXT PRIMARY KEY,
			entity_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'uncollected',
			description TEXT DEFAULT '',
			chapter INTEGER DEFAULT 0,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(entity_id) REFERENCES entities(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL DEFAULT 'narrative',
			at_chapter INTEGER NOT NULL DEFAULT 0,
			text TEXT NOT NULL DEFAULT '',
			payload TEXT DEFAULT '{}',
			created_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS mutations (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL DEFAULT '',
			payload TEXT NOT NULL DEFAULT '{}',
			created_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_id)`,
		`CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_id)`,
		`CREATE INDEX IF NOT EXISTS idx_state_slots_entity ON state_slots(entity_id)`,
		`CREATE INDEX IF NOT EXISTS idx_evidence_entity ON evidence(entity_id)`,
		`CREATE INDEX IF NOT EXISTS idx_events_chapter ON events(at_chapter)`,
	}
	for _, s := range stmts {
		if _, err := p.db.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// Close releases the database handle.
func (p *PlayDB) Close() error { return p.db.Close() }

// --- Entities ---

// UpsertEntity inserts or replaces an entity.
func (p *PlayDB) UpsertEntity(e *model.PlayEntity) error {
	tags, _ := json.Marshal(e.Tags)
	_, err := p.db.Exec(
		`INSERT INTO entities(id, type, name, description, visibility, tags, rarity, image_prompt, image_file, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,datetime('now'))
		 ON CONFLICT(id) DO UPDATE SET
		 type=excluded.type, name=excluded.name, description=excluded.description,
		 visibility=excluded.visibility, tags=excluded.tags, rarity=excluded.rarity,
		 image_prompt=excluded.image_prompt, image_file=excluded.image_file,
		 updated_at=excluded.updated_at`,
		e.ID, e.Type, e.Name, e.Description, e.Visibility, string(tags),
		e.Rarity, e.ImagePrompt, e.ImageFile,
	)
	return err
}

// GetEntity returns a single entity by ID.
func (p *PlayDB) GetEntity(id string) (*model.PlayEntity, error) {
	row := p.db.QueryRow(`SELECT id, type, name, description, visibility, tags, rarity, image_prompt, image_file FROM entities WHERE id=?`, id)
	e := &model.PlayEntity{}
	var tags string
	err := row.Scan(&e.ID, &e.Type, &e.Name, &e.Description, &e.Visibility, &tags, &e.Rarity, &e.ImagePrompt, &e.ImageFile)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(tags), &e.Tags)
	return e, nil
}

// ListEntities returns all entities, optionally filtered by type.
func (p *PlayDB) ListEntities(typ string) ([]model.PlayEntity, error) {
	var rows *sql.Rows
	var err error
	if typ == "" {
		rows, err = p.db.Query(`SELECT id, type, name, description, visibility, tags, rarity, image_prompt, image_file FROM entities ORDER BY name`)
	} else {
		rows, err = p.db.Query(`SELECT id, type, name, description, visibility, tags, rarity, image_prompt, image_file FROM entities WHERE type=? ORDER BY name`, typ)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.PlayEntity{}
	for rows.Next() {
		e := model.PlayEntity{}
		var tags string
		if err := rows.Scan(&e.ID, &e.Type, &e.Name, &e.Description, &e.Visibility, &tags, &e.Rarity, &e.ImagePrompt, &e.ImageFile); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(tags), &e.Tags)
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteEntity removes an entity by ID.
func (p *PlayDB) DeleteEntity(id string) error {
	_, err := p.db.Exec(`DELETE FROM entities WHERE id=?`, id)
	return err
}

// --- Edges ---

// UpsertEdge inserts or replaces an edge.
func (p *PlayDB) UpsertEdge(edge *model.PlayEdge) error {
	_, err := p.db.Exec(
		`INSERT INTO edges(id, from_id, to_id, label, weight) VALUES(?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET
		 from_id=excluded.from_id, to_id=excluded.to_id, label=excluded.label, weight=excluded.weight`,
		edge.ID, edge.From, edge.To, edge.Label, edge.Weight,
	)
	return err
}

// ListEdges returns all edges, optionally filtered by from_id.
func (p *PlayDB) ListEdges(fromID string) ([]model.PlayEdge, error) {
	var rows *sql.Rows
	var err error
	if fromID == "" {
		rows, err = p.db.Query(`SELECT id, from_id, to_id, label, weight FROM edges ORDER BY id`)
	} else {
		rows, err = p.db.Query(`SELECT id, from_id, to_id, label, weight FROM edges WHERE from_id=? ORDER BY id`, fromID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.PlayEdge{}
	for rows.Next() {
		e := model.PlayEdge{}
		if err := rows.Scan(&e.ID, &e.From, &e.To, &e.Label, &e.Weight); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteEdge removes an edge by ID.
func (p *PlayDB) DeleteEdge(id string) error {
	_, err := p.db.Exec(`DELETE FROM edges WHERE id=?`, id)
	return err
}

// --- State Slots ---

// UpsertStateSlot inserts or replaces a state slot.
func (p *PlayDB) UpsertStateSlot(slot *model.PlayStateSlot) error {
	_, err := p.db.Exec(
		`INSERT INTO state_slots(id, entity_id, kind, key, value_num, value_text, value_bool, value_enum, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,datetime('now'))
		 ON CONFLICT(id) DO UPDATE SET
		 entity_id=excluded.entity_id, kind=excluded.kind, key=excluded.key,
		 value_num=excluded.value_num, value_text=excluded.value_text,
		 value_bool=excluded.value_bool, value_enum=excluded.value_enum,
		 updated_at=excluded.updated_at`,
		slot.ID, slot.EntityID, slot.Kind, slot.Key,
		slot.ValueNum, slot.ValueText, boolToInt(slot.ValueBool), slot.ValueEnum,
	)
	return err
}

// ListStateSlots returns all state slots for a given entity.
func (p *PlayDB) ListStateSlots(entityID string) ([]model.PlayStateSlot, error) {
	rows, err := p.db.Query(`SELECT id, entity_id, kind, key, value_num, value_text, value_bool, value_enum FROM state_slots WHERE entity_id=? ORDER BY key`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.PlayStateSlot{}
	for rows.Next() {
		s := model.PlayStateSlot{}
		var vBool sql.NullInt64
		if err := rows.Scan(&s.ID, &s.EntityID, &s.Kind, &s.Key, &s.ValueNum, &s.ValueText, &vBool, &s.ValueEnum); err != nil {
			return nil, err
		}
		if vBool.Valid {
			b := vBool.Int64 == 1
			s.ValueBool = &b
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DeleteStateSlot removes a state slot by ID.
func (p *PlayDB) DeleteStateSlot(id string) error {
	_, err := p.db.Exec(`DELETE FROM state_slots WHERE id=?`, id)
	return err
}

// --- Events ---

// InsertEvent adds an event row.
func (p *PlayDB) InsertEvent(ev *model.PlayEvent) error {
	payload, _ := json.Marshal(ev.Payload)
	_, err := p.db.Exec(
		`INSERT INTO events(id, kind, at_chapter, text, payload) VALUES(?,?,?,?,?)`,
		ev.ID, ev.Kind, ev.AtChapter, ev.Text, string(payload),
	)
	return err
}

// ListEvents returns events filtered by chapter (0 = all).
func (p *PlayDB) ListEvents(chapter int) ([]model.PlayEvent, error) {
	var rows *sql.Rows
	var err error
	if chapter <= 0 {
		rows, err = p.db.Query(`SELECT id, kind, at_chapter, text, payload, created_at FROM events ORDER BY id`)
	} else {
		rows, err = p.db.Query(`SELECT id, kind, at_chapter, text, payload, created_at FROM events WHERE at_chapter=? ORDER BY id`, chapter)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.PlayEvent{}
	for rows.Next() {
		e := model.PlayEvent{}
		var payload string
		if err := rows.Scan(&e.ID, &e.Kind, &e.AtChapter, &e.Text, &payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(payload), &e.Payload)
		out = append(out, e)
	}
	return out, rows.Err()
}

// --- Mutations ---

// InsertMutation adds a mutation row.
func (p *PlayDB) InsertMutation(mut *model.PlayMutation) error {
	payload, _ := json.Marshal(mut.Payload)
	_, err := p.db.Exec(
		`INSERT INTO mutations(id, kind, payload) VALUES(?,?,?)`,
		mut.ID, mut.Kind, string(payload),
	)
	return err
}

// ListMutations returns all pending mutations (oldest first).
func (p *PlayDB) ListMutations() ([]model.PlayMutation, error) {
	rows, err := p.db.Query(`SELECT id, kind, payload, created_at FROM mutations ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.PlayMutation{}
	for rows.Next() {
		m := model.PlayMutation{}
		var payload string
		if err := rows.Scan(&m.ID, &m.Kind, &payload, &m.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(payload), &m.Payload)
		out = append(out, m)
	}
	return out, rows.Err()
}

// DeleteMutation removes a mutation by ID.
func (p *PlayDB) DeleteMutation(id string) error {
	_, err := p.db.Exec(`DELETE FROM mutations WHERE id=?`, id)
	return err
}

// --- Evidence ---

// UpsertEvidence inserts or replaces an evidence row.
func (p *PlayDB) UpsertEvidence(entityID, status, description string, chapter int) error {
	// Generate a deterministic but unique ID based on entity+chapter.
	id := fmt.Sprintf("%s-ch%d", entityID, chapter)
	_, err := p.db.Exec(
		`INSERT INTO evidence(id, entity_id, status, description, chapter, updated_at)
		 VALUES(?,?,?,?,?,datetime('now'))
		 ON CONFLICT(id) DO UPDATE SET
		 entity_id=excluded.entity_id, status=excluded.status,
		 description=excluded.description,
		 updated_at=excluded.updated_at`,
		id, entityID, status, description, chapter,
	)
	return err
}

// ListEvidence returns all evidence rows, optionally filtered by status.
func (p *PlayDB) ListEvidence(status string) ([]map[string]interface{}, error) {
	var rows *sql.Rows
	var err error
	if status == "" {
		rows, err = p.db.Query(`SELECT id, entity_id, status, description, chapter, created_at, updated_at FROM evidence ORDER BY id`)
	} else {
		rows, err = p.db.Query(`SELECT id, entity_id, status, description, chapter, created_at, updated_at FROM evidence WHERE status=? ORDER BY id`, status)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var id, entityID, st, desc, createdAt, updatedAt string
		var chapter int
		if err := rows.Scan(&id, &entityID, &st, &desc, &chapter, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"id": id, "entityId": entityID, "status": st,
			"description": desc, "chapter": chapter,
			"createdAt": createdAt, "updatedAt": updatedAt,
		})
	}
	return out, rows.Err()
}

func boolToInt(b *bool) *int64 {
	if b == nil {
		return nil
	}
	v := int64(0)
	if *b {
		v = 1
	}
	return &v
}