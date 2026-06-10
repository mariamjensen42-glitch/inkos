// Package api: Server bundles all dependencies for HTTP handlers.
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/narcooo/inkos/internal/llm"
	"github.com/narcooo/inkos/internal/store"
)

// Server is the application-wide dependency container.
type Server struct {
	Root        string
	Project     *store.ProjectStore
	Books       *store.BookStore
	Truth       *store.TruthStore
	Broadcaster *Broadcaster

	// Lazy LLM resolver — re-built when project config is reloaded.
	ResolverFactory func() *llm.Resolver
}

// NewServer constructs a server bound to the given project root.
func NewServer(root string) *Server {
	return &Server{
		Root:        root,
		Project:     store.NewProjectStore(root),
		Books:       store.NewBookStore(root),
		Truth:       store.NewTruthStore(root),
		Broadcaster: NewBroadcaster(),
	}
}

// WithResolverFactory installs a custom resolver factory. This is
// optional; if unset, handlers that need an LLM will return an error.
func (s *Server) WithResolverFactory(f func() *llm.Resolver) *Server {
	s.ResolverFactory = f
	return s
}

// DefaultResolverFactory returns a resolver built from the latest
// project config + secrets. Suitable for most use cases.
func (s *Server) DefaultResolverFactory() func() *llm.Resolver {
	return func() *llm.Resolver {
		cfg, err := s.Project.LoadConfig()
		if err != nil {
			return &llm.Resolver{}
		}
		sec, err := s.Project.LoadSecrets()
		if err != nil {
			sec = nil
		}
		return llm.NewResolver(cfg, sec)
	}
}

// Engine builds the gin.Engine with all routes mounted.
func (s *Server) Engine() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(CORS())
	r.Use(ErrorHandler())

	api := r.Group("/api/v1")

	// Project config.
	api.GET("/project", s.handleGetProject)
	api.PUT("/project", s.handlePutProject)
	api.POST("/project/language", s.handlePostProjectLanguage)
	api.GET("/project/input-governance-mode", s.handleGetGovernanceMode)
	api.PUT("/project/input-governance-mode", s.handlePutGovernanceMode)
	api.GET("/project/detection", s.handleGetDetection)
	api.PUT("/project/detection", s.handlePutDetection)
	api.GET("/project/model-overrides", s.handleGetModelOverrides)
	api.PUT("/project/model-overrides", s.handlePutModelOverrides)
	api.GET("/project/chapter-review-mode", s.handleGetReviewMode)
	api.PUT("/project/chapter-review-mode", s.handlePutReviewMode)
	api.GET("/project/notify", s.handleGetNotify)
	api.PUT("/project/notify", s.handlePutNotify)
	api.GET("/project/files/:file{.+}", s.handleGetProjectFile)

	// Books (bookId validated by middleware).
	books := api.Group("/books", BookIDMiddleware())
	books.GET("", s.handleListBooks)
	books.POST("/create", s.handleCreateBook)
	books.GET("/:id", s.handleGetBook)
	books.PUT("/:id", s.handleUpdateBook)
	books.DELETE("/:id", s.handleDeleteBook)
	books.GET("/:id/create-status", s.handleCreateStatus)
	books.GET("/:id/analytics", s.handleBookAnalytics)
	books.GET("/:id/eval", s.handleBookEval)
	books.GET("/:id/export", s.handleBookExport)
	books.POST("/:id/export-save", s.handleBookExportSave)
	books.GET("/:id/chapters/:num", s.handleGetChapter)
	books.PUT("/:id/chapters/:num", s.handlePutChapter)
	books.POST("/:id/chapters/:num/approve", s.handleApproveChapter)
	books.POST("/:id/chapters/:num/reject", s.handleRejectChapter)
	books.GET("/:id/truth", s.handleListTruth)
	books.GET("/:id/truth/*file", s.handleGetTruth)
	books.PUT("/:id/truth/*file", s.handlePutTruth)

	// Pipeline endpoints.
	books.POST("/:id/write-next", s.handleWriteNext)
	books.POST("/:id/draft", s.handleDraft)
	books.POST("/:id/plan", s.handlePlan)
	books.POST("/:id/compose", s.handleCompose)
	books.POST("/:id/audit/:chapter", s.handleAudit)
	books.POST("/:id/revise/:chapter", s.handleRevise)
	books.POST("/:id/rewrite/:chapter", s.handleRewrite)
	books.POST("/:id/resync/:chapter", s.handleResync)
	books.POST("/:id/repair-state/:chapter", s.handleRepairState)
	books.POST("/:id/foundation/revise", s.handleFoundationRevise)
	books.POST("/:id/consolidate", s.handleConsolidate)
	books.POST("/:id/detect/:chapter", s.handleDetect)
	books.POST("/:id/detect-all", s.handleDetectAll)
	books.GET("/:id/detect/stats", s.handleDetectStats)
	books.POST("/:id/style/import", s.handleStyleImport)
	books.POST("/:id/import/chapters", s.handleImportChapters)
	books.POST("/:id/import/canon", s.handleImportCanon)
	books.GET("/:id/fanfic", s.handleGetFanfic)
	books.POST("/:id/fanfic/refresh", s.handleRefreshFanfic)

	// LLM services.
	api.GET("/services", s.handleListServices)
	api.GET("/services/config", s.handleGetServicesConfig)
	api.POST("/services/config/import-env", s.handleImportEnv)
	api.PUT("/services/config", s.handlePutServicesConfig)
	api.GET("/services/models", s.handleListModels)
	api.GET("/services/models/custom", s.handleListCustomModels)
	api.GET("/services/:service/secret", s.handleGetSecret)
	api.PUT("/services/:service/secret", s.handlePutSecret)
	api.DELETE("/services/:service", s.handleDeleteService)
	api.POST("/services/:service/test", s.handleTestService)
	api.GET("/services/:service/models", s.handleServiceModels)

	// Cover.
	api.GET("/cover/config", s.handleGetCoverConfig)
	api.PUT("/cover/config", s.handlePutCoverConfig)
	api.GET("/cover/secret/:service", s.handleGetCoverSecret)
	api.PUT("/cover/secret/:service", s.handlePutCoverSecret)

	// Genres.
	api.GET("/genres", s.handleListGenres)
	api.GET("/genres/:id", s.handleGetGenre)
	api.POST("/genres/:id/copy", s.handleCopyGenre)
	api.POST("/genres/create", s.handleCreateGenre)
	api.PUT("/genres/:id", s.handlePutGenre)
	api.DELETE("/genres/:id", s.handleDeleteGenre)

	// Play.
	api.GET("/play/runs/:worldId/:runId", s.handleGetPlayRun)
	api.PUT("/play/runs/:worldId/:runId/image-settings", s.handlePutPlayImageSettings)
	api.POST("/play/runs/:worldId/:runId/generate-image", s.handleGeneratePlayImage)
	api.GET("/play/runs/:worldId/:runId/images/:file", s.handleGetPlayImage)

	// Sessions.
	api.GET("/sessions", s.handleListSessions)
	api.GET("/sessions/:sessionId", s.handleGetSession)
	api.POST("/sessions", s.handleCreateSession)
	api.PUT("/sessions/:sessionId", s.handleUpdateSession)
	api.PUT("/sessions/:sessionId/play-mode", s.handleSetPlayMode)
	api.DELETE("/sessions/:sessionId", s.handleDeleteSession)

	// Interaction / agent.
	api.GET("/interaction/session", s.handleGetInteractionSession)
	api.POST("/agent", s.handleAgent)

	// SSE.
	api.GET("/events", s.handleEvents)

	// Daemon.
	api.GET("/daemon", s.handleGetDaemon)
	api.POST("/daemon/start", s.handleStartDaemon)
	api.POST("/daemon/stop", s.handleStopDaemon)

	// Logs.
	api.GET("/logs", s.handleGetLogs)

	// Style.
	api.POST("/style/analyze", s.handleStyleAnalyze)

	// Fanfic/spinoff/imitation.
	api.POST("/fanfic/init", s.handleFanficInit)
	api.POST("/spinoff/init", s.handleSpinoffInit)
	api.POST("/imitation/init", s.handleImitationInit)

	// Radar.
	api.POST("/radar/scan", s.handleRadarScan)
	api.GET("/radar/history", s.handleRadarHistory)

	// Doctor.
	api.GET("/doctor", s.handleDoctor)

	// Health.
	api.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	return r
}
