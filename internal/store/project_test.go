package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/narcooo/inkos/internal/model"
)

func TestProjectStoreLoadDefault(t *testing.T) {
	dir := t.TempDir()
	ps := NewProjectStore(dir)
	cfg, err := ps.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "inkos-project" {
		t.Errorf("got %+v", cfg)
	}
}

func TestProjectStoreSaveLoad(t *testing.T) {
	dir := t.TempDir()
	ps := NewProjectStore(dir)
	cfg, _ := ps.LoadConfig()
	cfg.Name = "my-project"
	cfg.LLM.Provider = "openai"
	cfg.LLM.Model = "gpt-4o"
	cfg.Services = map[string]model.LLMConfig{
		"openai": {Provider: "openai", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o"},
	}
	if err := ps.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, _ := ps.LoadConfig()
	if loaded.Name != "my-project" {
		t.Errorf("got %+v", loaded)
	}
	if loaded.LLM.Model != "gpt-4o" {
		t.Errorf("model lost: %+v", loaded.LLM)
	}
	if _, ok := loaded.Services["openai"]; !ok {
		t.Errorf("services lost: %+v", loaded.Services)
	}
}

func TestProjectStoreSecrets(t *testing.T) {
	dir := t.TempDir()
	ps := NewProjectStore(dir)
	sec, _ := ps.LoadSecrets()
	if sec.Services == nil {
		t.Error("expected non-nil services map")
	}
	sec.Services["openai"] = model.SecretEntry{APIKey: "sk-test"}
	if err := ps.SaveSecrets(sec); err != nil {
		t.Fatal(err)
	}
	loaded, _ := ps.LoadSecrets()
	if loaded.Services["openai"].APIKey != "sk-test" {
		t.Errorf("got %+v", loaded)
	}
	// file should have 0600 perms.
	st, _ := os.Stat(filepath.Join(dir, ".inkos", "secrets.json"))
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Errorf("secrets perm = %o, want 0600", perm)
	}
}
