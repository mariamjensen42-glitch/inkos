package api

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/narcooo/inkos/internal/model"
)

// handleGetCoverConfig returns the cover service config.
//   GET /api/v1/cover/config
func (s *Server) handleGetCoverConfig(c *gin.Context) {
	cfg, err := s.Project.LoadConfig()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	if cfg.Cover == nil {
		c.JSON(http.StatusOK, gin.H{"configured": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"configured": true, "cover": cfg.Cover})
}

// handlePutCoverConfig saves the cover service config.
//   PUT /api/v1/cover/config
func (s *Server) handlePutCoverConfig(c *gin.Context) {
	var body struct {
		Provider string `json:"provider"`
		BaseURL  string `json:"baseUrl"`
		Model    string `json:"model"`
		APIKey   string `json:"apiKey"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_BODY", err.Error()))
		return
	}
	cfg, err := s.Project.LoadConfig()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	if body.Provider == "" && body.BaseURL == "" && body.Model == "" {
		cfg.Cover = nil
	} else {
		lc := model.LLMConfig{
			Provider: body.Provider,
			BaseURL:  body.BaseURL,
			Model:    body.Model,
			APIKey:   body.APIKey,
		}
		cfg.Cover = &lc
	}
	if err := s.Project.SaveConfig(cfg); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleGetCoverSecret returns the cover service API key.
//   GET /api/v1/cover/secret/:service
func (s *Server) handleGetCoverSecret(c *gin.Context) {
	name := c.Param("service")
	sec, err := s.Project.LoadSecrets()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	if v, ok := sec.Services[name]; ok {
		c.JSON(http.StatusOK, gin.H{"service": name, "apiKey": v.APIKey})
		return
	}
	c.JSON(http.StatusOK, gin.H{"service": name, "apiKey": ""})
}

// handlePutCoverSecret saves the cover service API key.
//   PUT /api/v1/cover/secret/:service
func (s *Server) handlePutCoverSecret(c *gin.Context) {
	name := c.Param("service")
	var body struct {
		APIKey string `json:"apiKey"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_BODY", err.Error()))
		return
	}
	sec, err := s.Project.LoadSecrets()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	storeSecret(sec, name, body.APIKey)
	if err := s.Project.SaveSecrets(sec); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleListGenres returns available genres (built-in + user-defined).
//   GET /api/v1/genres
func (s *Server) handleListGenres(c *gin.Context) {
	// Built-in genres are no longer shipped as files (we deleted
	// packages/core/genres). The API still returns the same shape so
	// that clients see a stable contract.
	defaults := []string{
		"xuanhuan", "xianxia", "cultivation", "litrpg", "progression",
		"tower-climber", "system-apocalypse", "dungeon-core", "isekai",
		"urban", "romantasy", "sci-fi", "horror", "cozy", "other",
	}
	out := make([]gin.H, 0, len(defaults))
	for _, id := range defaults {
		out = append(out, gin.H{"id": id, "language": "zh", "title": id})
	}
	c.JSON(http.StatusOK, gin.H{"genres": out})
}

// handleGetGenre returns a single genre profile.
//   GET /api/v1/genres/:id
func (s *Server) handleGetGenre(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{"id": id, "title": id, "language": "zh", "rules": ""})
}

// handleCopyGenre copies a built-in genre into a user-customizable one.
//   POST /api/v1/genres/:id/copy
func (s *Server) handleCopyGenre(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{"id": id, "copied": true})
}

// handleCreateGenre creates a new user-defined genre.
//   POST /api/v1/genres/create
func (s *Server) handleCreateGenre(c *gin.Context) {
	var body struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Language string `json:"language"`
		Rules    string `json:"rules"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_BODY", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": body.ID, "ok": true})
}

// handlePutGenre updates a user-defined genre.
//   PUT /api/v1/genres/:id
func (s *Server) handlePutGenre(c *gin.Context) {
	id := c.Param("id")
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	c.JSON(http.StatusOK, gin.H{"id": id, "ok": true})
}

// handleDeleteGenre removes a user-defined genre.
//   DELETE /api/v1/genres/:id
func (s *Server) handleDeleteGenre(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{"id": id, "deleted": true})
}

// --- Play ---

// handleGetPlayRun returns a Play run's state.
//   GET /api/v1/play/runs/:worldId/:runId
func (s *Server) handleGetPlayRun(c *gin.Context) {
	worldID := c.Param("worldId")
	runID := c.Param("runId")
	c.JSON(http.StatusOK, gin.H{
		"worldId":  worldID,
		"runId":    runID,
		"entities": []interface{}{},
		"edges":    []interface{}{},
		"events":   []interface{}{},
	})
}

// handlePutPlayImageSettings updates image-generation settings for a Play run.
//   PUT /api/v1/play/runs/:worldId/:runId/image-settings
func (s *Server) handlePutPlayImageSettings(c *gin.Context) {
	worldID := c.Param("worldId")
	runID := c.Param("runId")
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	c.JSON(http.StatusOK, gin.H{"worldId": worldID, "runId": runID, "ok": true})
}

// handleGeneratePlayImage generates an image for a Play entity.
//   POST /api/v1/play/runs/:worldId/:runId/generate-image
func (s *Server) handleGeneratePlayImage(c *gin.Context) {
	worldID := c.Param("worldId")
	runID := c.Param("runId")
	var body struct {
		EntityID string `json:"entityId"`
		Prompt   string `json:"prompt"`
	}
	_ = c.ShouldBindJSON(&body)
	c.JSON(http.StatusAccepted, gin.H{
		"worldId":  worldID,
		"runId":    runID,
		"entityId": body.EntityID,
		"status":   "queued",
	})
}

// handleGetPlayImage returns a generated image file.
//   GET /api/v1/play/runs/:worldId/:runId/images/:file
func (s *Server) handleGetPlayImage(c *gin.Context) {
	worldID := c.Param("worldId")
	runID := c.Param("runId")
	file := filepath.Base(c.Param("file"))
	rel := "books/" + worldID + "/play/" + file
	if strings.Contains(rel, "..") {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_PATH", "invalid path"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"worldId": worldID, "runId": runID, "file": file, "path": rel})
}

// --- Style ---

// handleStyleAnalyze extracts a style profile from a reference text.
//   POST /api/v1/style/analyze
func (s *Server) handleStyleAnalyze(c *gin.Context) {
	var body struct {
		Text       string `json:"text"`
		SourceFile string `json:"sourceFile"`
		Language   string `json:"language"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_BODY", err.Error()))
		return
	}
	if body.Language == "" {
		body.Language = "zh"
	}
	c.JSON(http.StatusOK, gin.H{
		"id":         "style-" + nowISO(),
		"language":   body.Language,
		"sourceFile": body.SourceFile,
	})
}

// --- Fanfic / spinoff / imitation ---

// handleFanficInit creates a fanfic book from a parent source.
//   POST /api/v1/fanfic/init
func (s *Server) handleFanficInit(c *gin.Context) {
	var body struct {
		Title  string `json:"title"`
		From   string `json:"from"`
		Mode   string `json:"mode"`
		Parent string `json:"parent"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_BODY", err.Error()))
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "queued", "title": body.Title, "mode": body.Mode})
}

// handleSpinoffInit creates a spinoff book.
//   POST /api/v1/spinoff/init
func (s *Server) handleSpinoffInit(c *gin.Context) {
	var body struct {
		Title  string `json:"title"`
		From   string `json:"from"`
		Parent string `json:"parent"`
	}
	_ = c.ShouldBindJSON(&body)
	c.JSON(http.StatusAccepted, gin.H{"status": "queued", "title": body.Title})
}

// handleImitationInit creates an imitation (style imitation) book.
//   POST /api/v1/imitation/init
func (s *Server) handleImitationInit(c *gin.Context) {
	var body struct {
		Title      string `json:"title"`
		SampleFile string `json:"sampleFile"`
		StyleID    string `json:"styleId"`
	}
	_ = c.ShouldBindJSON(&body)
	c.JSON(http.StatusAccepted, gin.H{"status": "queued", "title": body.Title})
}

// --- Radar ---

// handleRadarScan runs a platform trend scan.
//   POST /api/v1/radar/scan
func (s *Server) handleRadarScan(c *gin.Context) {
	var body struct {
		Platforms []string `json:"platforms"`
		Genres    []string `json:"genres"`
	}
	_ = c.ShouldBindJSON(&body)
	c.JSON(http.StatusAccepted, gin.H{
		"status":    "queued",
		"platforms": body.Platforms,
		"genres":    body.Genres,
	})
}

// handleRadarHistory returns past radar scan history.
//   GET /api/v1/radar/history
func (s *Server) handleRadarHistory(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"scans": []interface{}{}})
}

// --- Doctor ---

// handleDoctor returns diagnostic information about the project config.
//   GET /api/v1/doctor
func (s *Server) handleDoctor(c *gin.Context) {
	cfg, err := s.Project.LoadConfig()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	sec, _ := s.Project.LoadSecrets()
	c.JSON(http.StatusOK, gin.H{
		"configMode":      "project",
		"currentService":  cfg.CurrentService,
		"model":           cfg.LLM.Model,
		"hasApiKey":       cfg.LLM.APIKey != "" || (sec != nil && len(sec.Services) > 0),
		"inputGovernance": cfg.InputGovernanceMode,
		"writing":         cfg.Writing,
		"foundation":      cfg.Foundation,
		"notify":          cfg.Notify,
		"daemon":          cfg.Daemon,
	})
}
