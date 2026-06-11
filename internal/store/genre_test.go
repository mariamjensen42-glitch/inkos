package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/narcooo/inkos/internal/model"
)

func TestGenreStore_AllBuiltIn(t *testing.T) {
	gs := NewGenreStore("")
	all := gs.All()
	if len(all) < 10 {
		t.Errorf("expected at least 10 built-in genres, got %d", len(all))
	}

	// Verify sorted order.
	for i := 1; i < len(all); i++ {
		if all[i-1].ID >= all[i].ID {
			t.Errorf("genres not sorted: %s >= %s", all[i-1].ID, all[i].ID)
		}
	}
}

func TestGenreStore_Get(t *testing.T) {
	gs := NewGenreStore("")

	g, ok := gs.Get("xianxia")
	if !ok {
		t.Fatal("xianxia genre not found")
	}
	if g.Title != "仙侠" {
		t.Errorf("expected 仙侠, got %s", g.Title)
	}

	_, ok = gs.Get("nonexistent")
	if ok {
		t.Error("nonexistent genre should not be found")
	}
}

func TestGenreStore_CustomCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	genresDir := filepath.Join(tmpDir, "genres")
	gs := NewGenreStore(genresDir)

	// Create custom genre.
	custom := &model.GenreProfile{
		ID:          "cyberpunk",
		Title:       "Cyberpunk",
		Language:    "en",
		Description: "High tech, low life",
		Tags:        []string{"cyberpunk", "dystopia", "AI"},
	}
	if err := gs.Create(custom); err != nil {
		t.Fatal(err)
	}

	// Verify file exists on disk.
	path := filepath.Join(genresDir, "cyberpunk.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("cyberpunk.json not created on disk")
	}

	// Get custom genre.
	g, ok := gs.Get("cyberpunk")
	if !ok {
		t.Fatal("cyberpunk genre not found after create")
	}
	if g.Title != "Cyberpunk" {
		t.Errorf("expected Cyberpunk, got %s", g.Title)
	}

	// All should include both built-in and custom.
	all := gs.All()
	foundBuiltIn := false
	foundCustom := false
	for _, g := range all {
		if g.ID == "xianxia" {
			foundBuiltIn = true
		}
		if g.ID == "cyberpunk" {
			foundCustom = true
		}
	}
	if !foundBuiltIn {
		t.Error("built-in xianxia not in All()")
	}
	if !foundCustom {
		t.Error("custom cyberpunk not in All()")
	}

	// Update.
	custom.Title = "CYBERPUNK"
	if err := gs.Update("cyberpunk", custom); err != nil {
		t.Fatal(err)
	}
	g, ok = gs.Get("cyberpunk")
	if !ok || g.Title != "CYBERPUNK" {
		t.Errorf("update failed, title is %s", g.Title)
	}

	// Delete.
	if err := gs.Delete("cyberpunk"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("cyberpunk.json not deleted from disk")
	}
	_, ok = gs.Get("cyberpunk")
	if ok {
		t.Error("cyberpunk should not be found after delete")
	}
}

func TestGenreStore_CannotOverwriteBuiltIn(t *testing.T) {
	tmpDir := t.TempDir()
	gs := NewGenreStore(filepath.Join(tmpDir, "genres"))

	err := gs.Create(&model.GenreProfile{ID: "xianxia", Title: "Override"})
	if err == nil {
		t.Fatal("expected error when creating built-in genre")
	}

	err = gs.Update("xianxia", &model.GenreProfile{ID: "xianxia", Title: "Override"})
	if err == nil {
		t.Fatal("expected error when updating built-in genre")
	}

	err = gs.Delete("xianxia")
	if err == nil {
		t.Fatal("expected error when deleting built-in genre")
	}
}

func TestGenreStore_Copy(t *testing.T) {
	gs := NewGenreStore("")

	cp, err := gs.Copy("xianxia")
	if err != nil {
		t.Fatal(err)
	}
	if cp.ID != "xianxia" {
		t.Errorf("expected xianxia, got %s", cp.ID)
	}
	if len(cp.Tags) == 0 {
		t.Error("expected non-empty tags")
	}

	// Modifying the copy should not affect the original.
	cp.Tags[0] = "modified"
	orig, _ := gs.Get("xianxia")
	if orig.Tags[0] == "modified" {
		t.Error("copy modification affected original")
	}

	// Copy nonexistent.
	_, err = gs.Copy("nonexistent")
	if err == nil {
		t.Error("expected error for copying nonexistent genre")
	}
}

func TestGenreStore_CustomOverridesBuiltIn(t *testing.T) {
	tmpDir := t.TempDir()
	genresDir := filepath.Join(tmpDir, "genres")

	// Write a custom genre file with the same ID as built-in.
	// The merged result should pick the custom one.
	customData := `{"id": "xianxia", "title": "Custom Xianxia", "description": "Custom"}`
	os.MkdirAll(genresDir, 0o755)
	os.WriteFile(filepath.Join(genresDir, "xianxia.json"), []byte(customData), 0o644)

	// Force cache reload by creating a new store.
	gs2 := NewGenreStore(genresDir)
	g, ok := gs2.Get("xianxia")
	if !ok {
		t.Fatal("xianxia not found")
	}
	if g.Title != "Custom Xianxia" {
		t.Errorf("custom should override built-in, got %s", g.Title)
	}
}