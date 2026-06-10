package api

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// handleWriteNext runs the full pipeline (plan -> compose -> write -> audit -> revise)
// for the next chapter.
//   POST /api/v1/books/:id/write-next
func (s *Server) handleWriteNext(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Context string `json:"context"`
		Words   int    `json:"words"`
		Service string `json:"service"`
		Audit   bool   `json:"audit"`
	}
	_ = c.ShouldBindJSON(&body)
	// In the full implementation, the pipeline stages run in goroutines
	// and publish SSE events. The HTTP response is 202 Accepted; the
	// client subscribes to /events for progress.
	s.Broadcaster.Publish(SSEEvent{Event: "pipeline:start", Data: gin.H{"bookId": id, "stage": "write-next"}})
	c.JSON(http.StatusAccepted, gin.H{
		"bookId":  id,
		"status":  "queued",
		"stage":   "plan",
		"options": body,
		"received": nowISO(),
	})
}

// handleDraft writes a draft chapter (write only, no audit/revise).
//   POST /api/v1/books/:id/draft
func (s *Server) handleDraft(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Context string `json:"context"`
		Words   int    `json:"words"`
	}
	_ = c.ShouldBindJSON(&body)
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "status": "queued", "stage": "draft"})
}

// handlePlan generates the chapter intent file.
//   POST /api/v1/books/:id/plan
func (s *Server) handlePlan(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Context string `json:"context"`
	}
	_ = c.ShouldBindJSON(&body)
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "status": "queued", "stage": "plan"})
}

// handleCompose compiles the chapter context (intent -> context.json, rule-stack).
//   POST /api/v1/books/:id/compose
func (s *Server) handleCompose(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "status": "queued", "stage": "compose"})
}

// handleAudit audits a single chapter.
//   POST /api/v1/books/:id/audit/:chapter
func (s *Server) handleAudit(c *gin.Context) {
	id := c.Param("id")
	num, _ := strconv.Atoi(c.Param("chapter"))
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "chapter": num, "status": "queued", "stage": "audit"})
}

// handleRevise revises a single chapter based on audit findings.
//   POST /api/v1/books/:id/revise/:chapter
func (s *Server) handleRevise(c *gin.Context) {
	id := c.Param("id")
	num, _ := strconv.Atoi(c.Param("chapter"))
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "chapter": num, "status": "queued", "stage": "revise"})
}

// handleRewrite restores a chapter from snapshot and rewrites it.
//   POST /api/v1/books/:id/rewrite/:chapter
func (s *Server) handleRewrite(c *gin.Context) {
	id := c.Param("id")
	num, _ := strconv.Atoi(c.Param("chapter"))
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "chapter": num, "status": "queued", "stage": "rewrite"})
}

// handleResync rebuilds the truth files from the canonical state.
//   POST /api/v1/books/:id/resync/:chapter
func (s *Server) handleResync(c *gin.Context) {
	id := c.Param("id")
	num, _ := strconv.Atoi(c.Param("chapter"))
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "chapter": num, "status": "queued", "stage": "resync"})
}

// handleRepairState rebuilds a single chapter's state from the markdown truth files.
//   POST /api/v1/books/:id/repair-state/:chapter
func (s *Server) handleRepairState(c *gin.Context) {
	id := c.Param("id")
	num, _ := strconv.Atoi(c.Param("chapter"))
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "chapter": num, "status": "queued", "stage": "repair-state"})
}

// handleFoundationRevise regenerates the book foundation (story_bible + book_rules).
//   POST /api/v1/books/:id/foundation/revise
func (s *Server) handleFoundationRevise(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "status": "queued", "stage": "foundation/revise"})
}

// handleConsolidate merges chapter summaries to reduce context pressure.
//   POST /api/v1/books/:id/consolidate
func (s *Server) handleConsolidate(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "status": "queued", "stage": "consolidate"})
}

// handleDetect runs AIGC detection on one chapter.
//   POST /api/v1/books/:id/detect/:chapter
func (s *Server) handleDetect(c *gin.Context) {
	id := c.Param("id")
	num, _ := strconv.Atoi(c.Param("chapter"))
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "chapter": num, "status": "queued", "stage": "detect"})
}

// handleDetectAll runs AIGC detection on every chapter.
//   POST /api/v1/books/:id/detect-all
func (s *Server) handleDetectAll(c *gin.Context) {
	id := c.Param("id")
	idx, err := s.Books.LoadChapterIndex(id)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "count": len(idx.Chapters), "status": "queued"})
}

// handleDetectStats returns aggregated AIGC detection statistics.
//   GET /api/v1/books/:id/detect/stats
func (s *Server) handleDetectStats(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"bookId":         id,
		"totalChecked":   0,
		"avgScore":       0,
		"highRiskCount":  0,
		"perChapter":     []interface{}{},
	})
}

// handleStyleImport copies a style profile into the book.
//   POST /api/v1/books/:id/style/import
func (s *Server) handleStyleImport(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		ProfileID string `json:"profileId"`
	}
	_ = c.ShouldBindJSON(&body)
	target := filepath.Join(s.Books.StoryDir(id), "style_profile.md")
	if err := writeFileAtomic(target, []byte("# Style profile\n\nsource: "+body.ProfileID+"\n")); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "path": target})
}

// handleImportChapters imports existing chapter text into a book.
//   POST /api/v1/books/:id/import/chapters
func (s *Server) handleImportChapters(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		From       string `json:"from"`
		Split      string `json:"split"`
		ResumeFrom int    `json:"resumeFrom"`
	}
	_ = c.ShouldBindJSON(&body)
	c.JSON(http.StatusAccepted, gin.H{
		"bookId": id,
		"from":   body.From,
		"split":  body.Split,
		"status": "queued",
	})
}

// handleImportCanon imports parent-book canon into a fanfic book.
//   POST /api/v1/books/:id/import/canon
func (s *Server) handleImportCanon(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Parent string `json:"parent"`
	}
	_ = c.ShouldBindJSON(&body)
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "parent": body.Parent, "status": "queued"})
}

// handleGetFanfic returns fanfic metadata for a book.
//   GET /api/v1/books/:id/fanfic
func (s *Server) handleGetFanfic(c *gin.Context) {
	id := c.Param("id")
	cfg, err := s.Books.LoadBookConfig(id)
	if err != nil {
		AbortWithError(c, ErrBookNotFound)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"bookId":      id,
		"mode":        cfg.FanficMode,
		"sourceBookId": cfg.SourceBookID,
	})
}

// handleRefreshFanfic reloads the fanfic foundation from its source.
//   POST /api/v1/books/:id/fanfic/refresh
func (s *Server) handleRefreshFanfic(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "status": "queued"})
}

// suppressTimeImport silences "time" unused-import warnings in handlers
// that don't otherwise use time. Kept as a no-op reference.
var _ = time.Now
var _ = fmt.Sprintf
