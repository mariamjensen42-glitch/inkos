package api

import (
	"context"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/narcooo/inkos/internal/llm"
	"github.com/narcooo/inkos/internal/model"
	"github.com/narcooo/inkos/internal/store"
)

// handleListServices returns the list of configured service names.
//   GET /api/v1/services
func (s *Server) handleListServices(c *gin.Context) {
	cfg, err := s.Project.LoadConfig()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	names := make([]string, 0, len(cfg.Services))
	for k := range cfg.Services {
		names = append(names, k)
	}
	sort.Strings(names)
	out := make([]gin.H, 0, len(names))
	for _, n := range names {
		v := cfg.Services[n]
		out = append(out, gin.H{
			"service":      n,
			"baseUrl":      v.BaseURL,
			"model":        v.Model,
			"provider":     v.Provider,
			"hasSecret":    hasSecret(s.Root, n),
			"defaultModel": v.Model,
		})
	}
	c.JSON(http.StatusOK, gin.H{"services": out})
}

// handleGetServicesConfig returns the full services config object.
//   GET /api/v1/services/config
func (s *Server) handleGetServicesConfig(c *gin.Context) {
	cfg, err := s.Project.LoadConfig()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"services":       cfg.Services,
		"currentService": cfg.CurrentService,
		"defaultModel":   cfg.DefaultModel,
		"llm":            cfg.LLM,
	})
}

// handlePutServicesConfig replaces the services config.
//   PUT /api/v1/services/config
func (s *Server) handlePutServicesConfig(c *gin.Context) {
	var body struct {
		Services       map[string]model.LLMConfig `json:"services"`
		CurrentService string                     `json:"currentService"`
		DefaultModel   string                     `json:"defaultModel"`
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
	cfg.Services = body.Services
	cfg.CurrentService = body.CurrentService
	cfg.DefaultModel = body.DefaultModel
	if cfg.LLM.Model == "" && body.CurrentService != "" {
		if v, ok := body.Services[body.CurrentService]; ok {
			cfg.LLM = v
		}
	}
	if err := s.Project.SaveConfig(cfg); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleImportEnv imports INKOS_LLM_* environment variables into the
// project config. Does NOT save API keys (they remain env-only).
//   POST /api/v1/services/config/import-env
func (s *Server) handleImportEnv(c *gin.Context) {
	cfg, err := s.Project.LoadConfig()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	if v := getenv("INKOS_LLM_PROVIDER"); v != "" {
		cfg.LLM.Provider = v
	}
	if v := getenv("INKOS_LLM_SERVICE"); v != "" {
		cfg.CurrentService = v
	}
	if v := getenv("INKOS_LLM_BASE_URL"); v != "" {
		cfg.LLM.BaseURL = v
	}
	if v := getenv("INKOS_LLM_MODEL"); v != "" {
		cfg.LLM.Model = v
		cfg.DefaultModel = v
	}
	if err := s.Project.SaveConfig(cfg); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "llm": cfg.LLM, "currentService": cfg.CurrentService})
}

// handleGetSecret returns the API key for a service (without revealing it
// in normal listings).
//   GET /api/v1/services/:service/secret
func (s *Server) handleGetSecret(c *gin.Context) {
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

// handlePutSecret saves the API key for a service.
//   PUT /api/v1/services/:service/secret
func (s *Server) handlePutSecret(c *gin.Context) {
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
	// Service is a map; the inner struct is anonymous in the model so we
	// use a local copy of the same shape.
	storeSecret(sec, name, body.APIKey)
	if err := s.Project.SaveSecrets(sec); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleDeleteService removes a service from the config.
//   DELETE /api/v1/services/:service
func (s *Server) handleDeleteService(c *gin.Context) {
	name := c.Param("service")
	cfg, err := s.Project.LoadConfig()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	if _, ok := cfg.Services[name]; ok {
		delete(cfg.Services, name)
	}
	if cfg.CurrentService == name {
		cfg.CurrentService = ""
	}
	if err := s.Project.SaveConfig(cfg); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleTestService probes the LLM endpoint for a service.
//   POST /api/v1/services/:service/test
func (s *Server) handleTestService(c *gin.Context) {
	name := c.Param("service")
	cfg, err := s.Project.LoadConfig()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	sec, err := s.Project.LoadSecrets()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	resolver := llm.NewResolver(cfg, sec)
	client, rm, err := resolver.Client(name)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10_000_000_000)
	defer cancel()
	models, err := client.Probe(ctx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"service":   name,
			"ok":        false,
			"error":     err.Error(),
			"model":     rm.Model,
			"baseUrl":   rm.BaseURL,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"service":  name,
		"ok":       true,
		"model":    rm.Model,
		"baseUrl":  rm.BaseURL,
		"models":   models,
	})
}

// handleListModels returns models for the default service.
//   GET /api/v1/services/models
func (s *Server) handleListModels(c *gin.Context) {
	cfg, err := s.Project.LoadConfig()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"defaultModel": cfg.DefaultModel,
		"currentService": cfg.CurrentService,
		"llm": cfg.LLM,
	})
}

// handleListCustomModels is a placeholder for future custom-model discovery.
//   GET /api/v1/services/models/custom
func (s *Server) handleListCustomModels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"models": []string{}})
}

// handleServiceModels probes a specific service for its model list.
//   GET /api/v1/services/:service/models
func (s *Server) handleServiceModels(c *gin.Context) {
	name := c.Param("service")
	cfg, err := s.Project.LoadConfig()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	sec, err := s.Project.LoadSecrets()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	resolver := llm.NewResolver(cfg, sec)
	client, rm, err := resolver.Client(name)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8_000_000_000)
	defer cancel()
	models, err := client.Probe(ctx)
	if err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadGateway, "PROBE_FAILED", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"service": name, "model": rm.Model, "models": models})
}

// --- helpers ---

func hasSecret(root, name string) bool {
	ps := store.NewProjectStore(root)
	sec, err := ps.LoadSecrets()
	if err != nil {
		return false
	}
	_, ok := sec.Services[name]
	return ok
}

// storeSecret writes an apiKey into a Secrets struct without depending
// on the anonymous-struct literal in the model package.
func storeSecret(sec *model.Secrets, name, apiKey string) {
	if sec.Services == nil {
		sec.Services = model.NewSecretMap()
	}
	sec.Services[name] = model.NewSecretEntry(apiKey)
}
