package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/narcooo/inkos/internal/model"
)

// BuiltInGenres is the hard-coded set of built-in genres. These match
// the original TypeScript packages/core/genres/ definitions.
var BuiltInGenres = map[string]model.GenreProfile{
	"xianxia": {
		ID:          "xianxia",
		Title:       "仙侠",
		Language:    "zh",
		Description: "中国传统修仙世界观，包含修炼境界、法术、法宝、天劫等元素",
		Tags:        []string{"修仙", "东方玄幻", "修炼", "境界"},
	},
	"wuxia": {
		ID:          "wuxia",
		Title:       "武侠",
		Language:    "zh",
		Description: "中国传统武侠世界，以内功、招式、江湖恩怨为核心",
		Tags:        []string{"江湖", "内功", "剑客", "门派"},
	},
	"xuanhuan": {
		ID:          "xuanhuan",
		Title:       "玄幻",
		Language:    "zh",
		Description: "东方玄幻，融合东方文化元素的幻想世界，不拘泥于传统修仙体系",
		Tags:        []string{"东方玄幻", "异世", "架空"},
	},
	"qihuan": {
		ID:          "qihuan",
		Title:       "奇幻",
		Language:    "zh",
		Description: "西式奇幻，包含魔法、精灵、龙、剑与魔法等传统奇幻元素",
		Tags:        []string{"魔法", "西方奇幻", "龙", "精灵"},
	},
	"kehuan": {
		ID:          "kehuan",
		Title:       "科幻",
		Language:    "zh",
		Description: "科幻题材，包含未来科技、星际航行、人工智能等",
		Tags:        []string{"科幻", "未来", "AI", "星际"},
	},
	"dushi": {
		ID:          "dushi",
		Title:       "都市",
		Language:    "zh",
		Description: "现代都市背景，包含职场、校园、生活等题材",
		Tags:        []string{"现代", "都市", "职场", "校园"},
	},
	"yanqing": {
		ID:          "yanqing",
		Title:       "言情",
		Language:    "zh",
		Description: "以恋爱为核心主线的故事",
		Tags:        []string{"恋爱", "情感", "甜宠"},
	},
	"xuanyi": {
		ID:          "xuanyi",
		Title:       "悬疑",
		Language:    "zh",
		Description: "悬疑推理题材，包含侦探、犯罪、解谜等元素",
		Tags:        []string{"推理", "侦探", "犯罪", "解谜"},
	},
	"kongbu": {
		ID:          "kongbu",
		Title:       "恐怖",
		Language:    "zh",
		Description: "恐怖惊悚题材",
		Tags:        []string{"恐怖", "惊悚", "灵异"},
	},
	"lishi": {
		ID:          "lishi",
		Title:       "历史",
		Language:    "zh",
		Description: "历史题材，包含架空历史、穿越历史等",
		Tags:        []string{"历史", "穿越", "架空"},
	},
	"youxi": {
		ID:          "youxi",
		Title:       "游戏",
		Language:    "zh",
		Description: "游戏异界/网游题材",
		Tags:        []string{"游戏", "网游", "虚拟现实"},
	},
	"junshi": {
		ID:          "junshi",
		Title:       "军事",
		Language:    "zh",
		Description: "军事战争题材",
		Tags:        []string{"军事", "战争", "特种兵"},
	},
	"tiyu": {
		ID:          "tiyu",
		Title:       "体育",
		Language:    "zh",
		Description: "体育竞技题材",
		Tags:        []string{"体育", "竞技", "热血"},
	},
}

// GenreStore manages the union of built-in + user-custom genres.
type GenreStore struct {
	mu          sync.RWMutex
	genresDir   string
	builtIn     map[string]model.GenreProfile
	customCache map[string]model.GenreProfile
}

// NewGenreStore returns a store that reads custom genres from the
// given directory on the filesystem, merged with BuiltInGenres.
func NewGenreStore(genresDir string) *GenreStore {
	return &GenreStore{
		genresDir:   genresDir,
		builtIn:     BuiltInGenres,
		customCache: nil,
	}
}

// All returns all genres (built-in + custom), sorted by ID.
func (s *GenreStore) All() []model.GenreProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	merged := s.mergeLocked()
	out := make([]model.GenreProfile, 0, len(merged))
	for _, g := range merged {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Get returns a single genre by ID, looking first in custom then built-in.
func (s *GenreStore) Get(id string) (*model.GenreProfile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	merged := s.mergeLocked()
	g, ok := merged[id]
	if !ok {
		return nil, false
	}
	return &g, true
}

// Create persists a new custom genre to disk.
func (s *GenreStore) Create(profile *model.GenreProfile) error {
	if profile.ID == "" {
		return fmt.Errorf("genre id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.builtIn[profile.ID]; ok {
		return fmt.Errorf("genre %q is built-in and cannot be overwritten", profile.ID)
	}
	return s.writeCustomFileLocked(profile)
}

// Update modifies an existing custom genre on disk.
func (s *GenreStore) Update(id string, profile *model.GenreProfile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.builtIn[id]; ok {
		return fmt.Errorf("genre %q is built-in and cannot be updated", id)
	}
	profile.ID = id
	return s.writeCustomFileLocked(profile)
}

// Delete removes a custom genre file from disk.
func (s *GenreStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.builtIn[id]; ok {
		return fmt.Errorf("genre %q is built-in and cannot be deleted", id)
	}
	path := filepath.Join(s.genresDir, id+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	// Invalidate cache on next read.
	s.customCache = nil
	return nil
}

// Copy returns a deep copy of a genre so callers can use it to seed new
// custom genres.
func (s *GenreStore) Copy(id string) (*model.GenreProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	merged := s.mergeLocked()
	g, ok := merged[id]
	if !ok {
		return nil, fmt.Errorf("genre %q not found", id)
	}
	cp := g
	cp.Tags = append([]string{}, g.Tags...)
	return &cp, nil
}

// mergeLocked builds the unified genre map. Caller must hold at least RLock.
func (s *GenreStore) mergeLocked() map[string]model.GenreProfile {
	custom := s.loadCustomLocked()
	out := make(map[string]model.GenreProfile, len(s.builtIn)+len(custom))
	for k, v := range s.builtIn {
		out[k] = v
	}
	for k, v := range custom {
		out[k] = v
	}
	return out
}

// loadCustomLocked reads all *.json files from the genres directory.
func (s *GenreStore) loadCustomLocked() map[string]model.GenreProfile {
	if s.customCache != nil {
		return s.customCache
	}
	cache := map[string]model.GenreProfile{}
	if s.genresDir == "" {
		s.customCache = cache
		return cache
	}
	entries, err := os.ReadDir(s.genresDir)
	if err != nil {
		s.customCache = cache
		return cache
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(s.genresDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var g model.GenreProfile
		if err := json.Unmarshal(data, &g); err != nil {
			continue
		}
		if g.ID == "" {
			continue
		}
		cache[g.ID] = g
	}
	s.customCache = cache
	return cache
}

// writeCustomFileLocked persists a genre profile to disk. Invalidates cache.
func (s *GenreStore) writeCustomFileLocked(profile *model.GenreProfile) error {
	if err := os.MkdirAll(s.genresDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.genresDir, profile.ID+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	// Invalidate cache so next read picks up the change.
	s.customCache = nil
	return nil
}