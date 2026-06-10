// Package store provides filesystem and database persistence for InkOS.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gofrs/flock"
	"github.com/narcooo/inkos/internal/model"
)

// ProjectStore loads and saves the project configuration (inkos.json)
// and secrets (.inkos/secrets.json) under a project root.
type ProjectStore struct {
	root       string
	configPath string
	secretsPath string
	configMu  sync.RWMutex
	secretsMu sync.RWMutex
}

// NewProjectStore returns a store rooted at the given directory.
// The directory must exist (it is not created).
func NewProjectStore(root string) *ProjectStore {
	return &ProjectStore{
		root:        root,
		configPath:  filepath.Join(root, "inkos.json"),
		secretsPath: filepath.Join(root, ".inkos", "secrets.json"),
	}
}

// Root returns the project root directory.
func (s *ProjectStore) Root() string { return s.root }

// ConfigPath returns the absolute path to inkos.json.
func (s *ProjectStore) ConfigPath() string { return s.configPath }

// SecretsPath returns the absolute path to .inkos/secrets.json.
func (s *ProjectStore) SecretsPath() string { return s.secretsPath }

// LoadConfig reads inkos.json, returning a sensible default if absent.
func (s *ProjectStore) LoadConfig() (*model.ProjectConfig, error) {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.loadConfigLocked()
}

func (s *ProjectStore) loadConfigLocked() (*model.ProjectConfig, error) {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
		}
		return nil, fmt.Errorf("read %s: %w", s.configPath, err)
	}
	cfg := defaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.configPath, err)
	}
	return cfg, nil
}

// SaveConfig atomically writes inkos.json (write-temp + rename + flock).
func (s *ProjectStore) SaveConfig(cfg *model.ProjectConfig) error {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.configPath), 0o755); err != nil {
		return err
	}
	lock := flock.New(s.configPath + ".lock")
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("acquire config lock: %w", err)
	}
	defer lock.Unlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.configPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.configPath)
}

// LoadSecrets reads .inkos/secrets.json (empty if absent).
func (s *ProjectStore) LoadSecrets() (*model.Secrets, error) {
	s.secretsMu.RLock()
	defer s.secretsMu.RUnlock()
	return s.loadSecretsLocked()
}

func (s *ProjectStore) loadSecretsLocked() (*model.Secrets, error) {
	data, err := os.ReadFile(s.secretsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &model.Secrets{Services: map[string]model.SecretEntry{}}, nil
		}
		return nil, fmt.Errorf("read %s: %w", s.secretsPath, err)
	}
	sec := &model.Secrets{}
	if err := json.Unmarshal(data, sec); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.secretsPath, err)
	}
	if sec.Services == nil {
		sec.Services = map[string]model.SecretEntry{}
	}
	return sec, nil
}

// SaveSecrets atomically writes .inkos/secrets.json.
func (s *ProjectStore) SaveSecrets(sec *model.Secrets) error {
	s.secretsMu.Lock()
	defer s.secretsMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.secretsPath), 0o700); err != nil {
		return err
	}
	lock := flock.New(s.secretsPath + ".lock")
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("acquire secrets lock: %w", err)
	}
	defer lock.Unlock()

	data, err := json.MarshalIndent(sec, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.secretsPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.secretsPath)
}

func defaultConfig() *model.ProjectConfig {
	return &model.ProjectConfig{
		Name:     "inkos-project",
		Version:  "0.1.0",
		Language: "zh",
		Notify:   []model.NotifyChannel{},
	}
}
