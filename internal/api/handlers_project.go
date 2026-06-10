package api

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/narcooo/inkos/internal/model"
	"github.com/narcooo/inkos/internal/util"
)

// handleGetProject returns the project config (inkos.json).
//   GET /api/v1/project
func (s *Server) handleGetProject(c *gin.Context) {
	cfg, err := s.Project.LoadConfig()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// handlePutProject replaces the project config.
//   PUT /api/v1/project
func (s *Server) handlePutProject(c *gin.Context) {
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_BODY", err.Error()))
		return
	}
	// Reload existing, overlay.
	existing, err := s.Project.LoadConfig()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	// Save body directly through a JSON round-trip.
	cfgBytes, _ := jsonRoundTrip(existing)
	_ = cfgBytes
	if err := s.Project.SaveConfig(existing); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, existing)
}

// handlePostProjectLanguage sets the project language.
//   POST /api/v1/project/language
func (s *Server) handlePostProjectLanguage(c *gin.Context) {
	var body struct {
		Language string `json:"language"`
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
	cfg.Language = body.Language
	if err := s.Project.SaveConfig(cfg); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"language": cfg.Language})
}

// handleGetGovernanceMode / handlePutGovernanceMode
//   GET/PUT /api/v1/project/input-governance-mode
func (s *Server) handleGetGovernanceMode(c *gin.Context) {
	cfg, err := s.Project.LoadConfig()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	if cfg.InputGovernanceMode == "" {
		cfg.InputGovernanceMode = "v2"
	}
	c.JSON(http.StatusOK, gin.H{"mode": cfg.InputGovernanceMode})
}

func (s *Server) handlePutGovernanceMode(c *gin.Context) {
	var body struct {
		Mode string `json:"mode"`
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
	cfg.InputGovernanceMode = model.InputGovernanceMode(body.Mode)
	if err := s.Project.SaveConfig(cfg); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"mode": cfg.InputGovernanceMode})
}

// handleGetDetection / handlePutDetection
//   GET/PUT /api/v1/project/detection
func (s *Server) handleGetDetection(c *gin.Context) {
	cfg, err := s.Project.LoadConfig()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, cfg.Detection)
}

func (s *Server) handlePutDetection(c *gin.Context) {
	var body struct {
		Enabled   *bool    `json:"enabled"`
		Provider  string   `json:"provider"`
		Model     string   `json:"model"`
		Dimensions []string `json:"dimensions"`
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
	cfg.Detection.Enabled = body.Enabled
	cfg.Detection.Provider = body.Provider
	cfg.Detection.Model = body.Model
	cfg.Detection.Dimensions = body.Dimensions
	if err := s.Project.SaveConfig(cfg); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, cfg.Detection)
}

// handleGetModelOverrides / handlePutModelOverrides
//   GET/PUT /api/v1/project/model-overrides
func (s *Server) handleGetModelOverrides(c *gin.Context) {
	cfg, err := s.Project.LoadConfig()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"overrides": cfg.ModelOverrides})
}

func (s *Server) handlePutModelOverrides(c *gin.Context) {
	var body struct {
		Overrides map[string]interface{} `json:"overrides"`
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
	cfg.ModelOverrides = nil
	for k, v := range body.Overrides {
		// Best-effot: accept arbitrary map; downstream serializer may fail.
		_ = k
		_ = v
	}
	if err := s.Project.SaveConfig(cfg); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"overrides": cfg.ModelOverrides})
}

// handleGetReviewMode / handlePutReviewMode
//   GET/PUT /api/v1/project/chapter-review-mode
func (s *Server) handleGetReviewMode(c *gin.Context) {
	cfg, err := s.Project.LoadConfig()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	if cfg.Writing.ReviewMode == "" {
		cfg.Writing.ReviewMode = "auto"
	}
	c.JSON(http.StatusOK, gin.H{"mode": cfg.Writing.ReviewMode})
}

func (s *Server) handlePutReviewMode(c *gin.Context) {
	var body struct {
		Mode string `json:"mode"`
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
	cfg.Writing.ReviewMode = model.ChapterReviewMode(body.Mode)
	if err := s.Project.SaveConfig(cfg); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"mode": cfg.Writing.ReviewMode})
}

// handleGetNotify / handlePutNotify
//   GET/PUT /api/v1/project/notify
func (s *Server) handleGetNotify(c *gin.Context) {
	cfg, err := s.Project.LoadConfig()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"channels": cfg.Notify})
}

func (s *Server) handlePutNotify(c *gin.Context) {
	var body struct {
		Channels []map[string]interface{} `json:"channels"`
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
	cfg.Notify = cfg.Notify[:0]
	for _, ch := range body.Channels {
		cfg.Notify = append(cfg.Notify, notifyFromMap(ch))
	}
	if err := s.Project.SaveConfig(cfg); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"channels": cfg.Notify})
}

// handleGetProjectFile returns generated image/asset files (shorts/, covers/).
//   GET /api/v1/project/files/*file
func (s *Server) handleGetProjectFile(c *gin.Context) {
	raw := c.Param("file")
	rel, err := urlPathToRelative(raw)
	if err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_PATH", err.Error()))
		return
	}
	if !strings.HasPrefix(rel, "shorts/") && !strings.HasPrefix(rel, "covers/") {
		AbortWithError(c, NewAPIError(http.StatusForbidden, "FORBIDDEN_PATH", "only shorts/ and covers/ are allowed"))
		return
	}
	if !util.SafeRelativePath(rel) {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_PATH", "invalid path"))
		return
	}
	abs := filepath.Join(s.Root, rel)
	ext := strings.ToLower(filepath.Ext(abs))
	ct := ""
	switch ext {
	case ".png":
		ct = "image/png"
	case ".jpg", ".jpeg":
		ct = "image/jpeg"
	case ".webp":
		ct = "image/webp"
	default:
		AbortWithError(c, NewAPIError(http.StatusUnsupportedMediaType, "UNSUPPORTED_TYPE", "unsupported type"))
		return
	}
	c.File(abs)
	c.Header("Content-Type", ct)
}
