package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/narcooo/inkos/internal/model"
)

func TestPlayDB_SchemaCreation(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "play.db")

	db, err := NewPlayDB(path)
	if err != nil {
		t.Fatalf("NewPlayDB failed: %v", err)
	}
	defer db.Close()

	// Verify the file exists.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("play.db not created")
	}
}

func TestPlayDB_Entities(t *testing.T) {
	db := newTestPlayDB(t)
	defer db.Close()

	entity := &model.PlayEntity{
		ID:          "char-1",
		Type:        model.PlayEntityCharacter,
		Name:        "Hero",
		Description: "The main hero",
		Visibility:  model.PlayVisibilityPublic,
		Tags:        []string{"protagonist", "brave"},
		Rarity:      "common",
	}

	if err := db.UpsertEntity(entity); err != nil {
		t.Fatal(err)
	}

	// Get by ID.
	e, err := db.GetEntity("char-1")
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "Hero" {
		t.Errorf("expected Hero, got %s", e.Name)
	}
	if len(e.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(e.Tags))
	}

	// List all.
	entities, err := db.ListEntities("")
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}

	// Filter by type.
	entities, err = db.ListEntities(string(model.PlayEntityCharacter))
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 1 {
		t.Errorf("expected 1 entity matching type, got %d", len(entities))
	}

	// Filter by wrong type.
	entities, err = db.ListEntities(string(model.PlayEntityItem))
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 0 {
		t.Errorf("expected 0 entities matching wrong type, got %d", len(entities))
	}

	// Delete.
	if err := db.DeleteEntity("char-1"); err != nil {
		t.Fatal(err)
	}
	entities, err = db.ListEntities("")
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 0 {
		t.Errorf("expected 0 entities after delete, got %d", len(entities))
	}
}

func TestPlayDB_Edges(t *testing.T) {
	db := newTestPlayDB(t)
	defer db.Close()

	// Need entities first.
	db.UpsertEntity(&model.PlayEntity{ID: "a", Type: model.PlayEntityCharacter, Name: "A"})
	db.UpsertEntity(&model.PlayEntity{ID: "b", Type: model.PlayEntityCharacter, Name: "B"})

	edge := &model.PlayEdge{
		ID:     "e1",
		From:   "a",
		To:     "b",
		Label:  "knows",
		Weight: 5,
	}
	if err := db.UpsertEdge(edge); err != nil {
		t.Fatal(err)
	}

	edges, err := db.ListEdges("")
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Label != "knows" {
		t.Errorf("expected knows, got %s", edges[0].Label)
	}

	// Filter by from_id.
	edges, err = db.ListEdges("a")
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 {
		t.Errorf("expected 1 edge from 'a', got %d", len(edges))
	}

	edges, err = db.ListEdges("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges from nonexistent, got %d", len(edges))
	}

	if err := db.DeleteEdge("e1"); err != nil {
		t.Fatal(err)
	}
	edges, _ = db.ListEdges("")
	if len(edges) != 0 {
		t.Errorf("expected 0 edges after delete, got %d", len(edges))
	}
}

func TestPlayDB_StateSlots(t *testing.T) {
	db := newTestPlayDB(t)
	defer db.Close()

	db.UpsertEntity(&model.PlayEntity{ID: "char-1", Type: model.PlayEntityCharacter, Name: "Hero"})

	b := true
	slot := &model.PlayStateSlot{
		ID:        "slot-1",
		EntityID:  "char-1",
		Kind:      model.PlayStateSlotBoolean,
		Key:       "is_alive",
		ValueBool: &b,
	}
	if err := db.UpsertStateSlot(slot); err != nil {
		t.Fatal(err)
	}

	slots, err := db.ListStateSlots("char-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(slots))
	}
	if slots[0].ValueBool == nil || !*slots[0].ValueBool {
		t.Errorf("expected is_alive=true, got %v", slots[0].ValueBool)
	}

	if err := db.DeleteStateSlot("slot-1"); err != nil {
		t.Fatal(err)
	}
	slots, _ = db.ListStateSlots("char-1")
	if len(slots) != 0 {
		t.Errorf("expected 0 slots after delete, got %d", len(slots))
	}
}

func TestPlayDB_Events(t *testing.T) {
	db := newTestPlayDB(t)
	defer db.Close()

	ev := &model.PlayEvent{
		ID:        "ev-1",
		Kind:      "narrative",
		AtChapter: 3,
		Text:      "The hero arrived at the castle.",
		Payload:   map[string]interface{}{"location": "castle"},
	}
	if err := db.InsertEvent(ev); err != nil {
		t.Fatal(err)
	}

	events, err := db.ListEvents(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Text != "The hero arrived at the castle." {
		t.Errorf("unexpected event text: %s", events[0].Text)
	}

	// Filter by chapter.
	events, err = db.ListEvents(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event at chapter 3, got %d", len(events))
	}

	events, err = db.ListEvents(99)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events at chapter 99, got %d", len(events))
	}
}

func TestPlayDB_Mutations(t *testing.T) {
	db := newTestPlayDB(t)
	defer db.Close()

	mut := &model.PlayMutation{
		ID:      "mut-1",
		Kind:    "update_entity",
		Payload: map[string]interface{}{"field": "hp", "value": 10},
	}
	if err := db.InsertMutation(mut); err != nil {
		t.Fatal(err)
	}

	mutations, err := db.ListMutations()
	if err != nil {
		t.Fatal(err)
	}
	if len(mutations) != 1 {
		t.Fatalf("expected 1 mutation, got %d", len(mutations))
	}
	if mutations[0].Kind != "update_entity" {
		t.Errorf("expected update_entity, got %s", mutations[0].Kind)
	}

	if err := db.DeleteMutation("mut-1"); err != nil {
		t.Fatal(err)
	}
	mutations, _ = db.ListMutations()
	if len(mutations) != 0 {
		t.Errorf("expected 0 mutations after delete, got %d", len(mutations))
	}
}

func TestPlayDB_Evidence(t *testing.T) {
	db := newTestPlayDB(t)
	defer db.Close()

	db.UpsertEntity(&model.PlayEntity{ID: "item-1", Type: model.PlayEntityItem, Name: "Key"})

	if err := db.UpsertEvidence("item-1", "collected", "A rusty key found under the mat", 2); err != nil {
		t.Fatal(err)
	}

	evidence, err := db.ListEvidence("")
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 {
		t.Fatalf("expected 1 evidence, got %d", len(evidence))
	}
	if evidence[0]["status"] != "collected" {
		t.Errorf("expected collected, got %v", evidence[0]["status"])
	}

	// Filter by status.
	evidence, err = db.ListEvidence("collected")
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 {
		t.Errorf("expected 1 collected evidence, got %d", len(evidence))
	}

	evidence, err = db.ListEvidence("analyzed")
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 0 {
		t.Errorf("expected 0 analyzed evidence, got %d", len(evidence))
	}
}

func newTestPlayDB(t *testing.T) *PlayDB {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "play.db")
	db, err := NewPlayDB(path)
	if err != nil {
		t.Fatalf("NewPlayDB failed: %v", err)
	}
	return db
}