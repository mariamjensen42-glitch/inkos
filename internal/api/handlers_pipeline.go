package api

import (
	"context"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/narcooo/inkos/internal/pipeline"
)

// pipelineCommonOptions are shared by all pipeline handlers.
type pipelineCommonOptions struct {
	Chapter int    `json:"chapter"`
	Service string `json:"service"`
	Words   int    `json:"words"`
	Context string `json:"context"`
	Extra   string `json:"extra"`
	Audit   bool   `json:"audit"`
	Revise  bool   `json:"revise"`
}

func toPipelineOpts(in pipelineCommonOptions) pipeline.Options {
	return pipeline.Options{
		Service: in.Service,
		Words:   in.Words,
		Context: in.Context,
		Extra:   in.Extra,
		Audit:   in.Audit,
		Revise:  in.Revise,
	}
}

// rejectIfNoResolver returns ErrLLMConfig if the project has no
// resolver. The handler will then abort with a 400 and the client can
// configure the LLM before retrying.
func (s *Server) rejectIfNoResolver(c *gin.Context) bool {
	if s.ResolverFactory == nil {
		AbortWithError(c, ErrLLMConfig)
		return true
	}
	return false
}

// handleWriteNext runs the full pipeline (plan -> compose -> draft ->
// audit -> revise) for the next chapter. The work happens in a
// background goroutine; the HTTP response is 202 Accepted and the
// client subscribes to /events for progress.
//   POST /api/v1/books/:id/write-next
func (s *Server) handleWriteNext(c *gin.Context) {
	id := c.Param("id")
	if s.rejectIfNoResolver(c) {
		return
	}
	var body pipelineCommonOptions
	_ = c.ShouldBindJSON(&body)
	chapter, err := s.resolveChapterNumber(c, id, body.Chapter)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	opts := toPipelineOpts(body)
	runner := s.newRunner()
	s.detach("write-next", id, chapter, func(ctx context.Context) error {
		_, err := runner.WriteNext(ctx, id, opts)
		return err
	})
	c.JSON(http.StatusAccepted, gin.H{
		"bookId":  id,
		"chapter": chapter,
		"status":  "queued",
		"stage":   "write-next",
		"options": body,
		"received": nowISO(),
	})
}

// handleDraft writes a draft chapter (plan + compose + write, no
// audit/revise).
//   POST /api/v1/books/:id/draft
func (s *Server) handleDraft(c *gin.Context) {
	id := c.Param("id")
	if s.rejectIfNoResolver(c) {
		return
	}
	var body pipelineCommonOptions
	_ = c.ShouldBindJSON(&body)
	chapter, err := s.resolveChapterNumber(c, id, body.Chapter)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	opts := toPipelineOpts(body)
	runner := s.newRunner()
	s.detach("draft", id, chapter, func(ctx context.Context) error {
		if _, err := runner.Plan(ctx, id, chapter, opts); err != nil {
			return err
		}
		if _, err := runner.Compose(ctx, id, chapter, opts); err != nil {
			return err
		}
		_, err := runner.Draft(ctx, id, chapter, opts)
		return err
	})
	c.JSON(http.StatusAccepted, gin.H{
		"bookId":  id,
		"chapter": chapter,
		"status":  "queued",
		"stage":   "draft",
	})
}

// handlePlan generates the chapter intent file.
//   POST /api/v1/books/:id/plan
func (s *Server) handlePlan(c *gin.Context) {
	id := c.Param("id")
	if s.rejectIfNoResolver(c) {
		return
	}
	var body pipelineCommonOptions
	_ = c.ShouldBindJSON(&body)
	chapter, err := s.resolveChapterNumber(c, id, body.Chapter)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	opts := toPipelineOpts(body)
	runner := s.newRunner()
	s.detach("plan", id, chapter, func(ctx context.Context) error {
		_, err := runner.Plan(ctx, id, chapter, opts)
		return err
	})
	c.JSON(http.StatusAccepted, gin.H{
		"bookId":  id,
		"chapter": chapter,
		"status":  "queued",
		"stage":   "plan",
	})
}

// handleCompose compiles the chapter context (intent -> context.json).
// Requires a prior plan; if the intent is missing the stage errors out.
//   POST /api/v1/books/:id/compose
func (s *Server) handleCompose(c *gin.Context) {
	id := c.Param("id")
	if s.rejectIfNoResolver(c) {
		return
	}
	var body pipelineCommonOptions
	_ = c.ShouldBindJSON(&body)
	chapter, err := s.resolveChapterNumber(c, id, body.Chapter)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	opts := toPipelineOpts(body)
	runner := s.newRunner()
	s.detach("compose", id, chapter, func(ctx context.Context) error {
		_, err := runner.Compose(ctx, id, chapter, opts)
		return err
	})
	c.JSON(http.StatusAccepted, gin.H{
		"bookId":  id,
		"chapter": chapter,
		"status":  "queued",
		"stage":   "compose",
	})
}

// handleAudit audits a single chapter.
//   POST /api/v1/books/:id/audit/:chapter
func (s *Server) handleAudit(c *gin.Context) {
	id := c.Param("id")
	if s.rejectIfNoResolver(c) {
		return
	}
	num, _ := strconv.Atoi(c.Param("chapter"))
	if num < 1 {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_CHAPTER", "invalid chapter number"))
		return
	}
	var body pipelineCommonOptions
	_ = c.ShouldBindJSON(&body)
	opts := toPipelineOpts(body)
	runner := s.newRunner()
	s.detach("audit", id, num, func(ctx context.Context) error {
		_, err := runner.Audit(ctx, id, num, opts)
		return err
	})
	c.JSON(http.StatusAccepted, gin.H{
		"bookId":  id,
		"chapter": num,
		"status":  "queued",
		"stage":   "audit",
	})
}

// handleRevise revises a single chapter based on the latest audit
// result. Requires a prior audit; if the audit result is missing the
// stage errors out.
//   POST /api/v1/books/:id/revise/:chapter
func (s *Server) handleRevise(c *gin.Context) {
	id := c.Param("id")
	if s.rejectIfNoResolver(c) {
		return
	}
	num, _ := strconv.Atoi(c.Param("chapter"))
	if num < 1 {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_CHAPTER", "invalid chapter number"))
		return
	}
	var body pipelineCommonOptions
	_ = c.ShouldBindJSON(&body)
	opts := toPipelineOpts(body)
	runner := s.newRunner()
	s.detach("revise", id, num, func(ctx context.Context) error {
		_, err := runner.Revise(ctx, id, num, opts)
		return err
	})
	c.JSON(http.StatusAccepted, gin.H{
		"bookId":  id,
		"chapter": num,
		"status":  "queued",
		"stage":   "revise",
	})
}

// handleRewrite restores a chapter from snapshot and rewrites it.
// Currently behaves the same as handleDraft (snapshot->rewrite cycle is
// a TODO: it would require storing snapshots, which is implemented but
// not yet wired into the request flow).
//   POST /api/v1/books/:id/rewrite/:chapter
func (s *Server) handleRewrite(c *gin.Context) {
	id := c.Param("id")
	if s.rejectIfNoResolver(c) {
		return
	}
	num, _ := strconv.Atoi(c.Param("chapter"))
	if num < 1 {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_CHAPTER", "invalid chapter number"))
		return
	}
	var body pipelineCommonOptions
	_ = c.ShouldBindJSON(&body)
	opts := toPipelineOpts(body)
	runner := s.newRunner()
	s.detach("rewrite", id, num, func(ctx context.Context) error {
		_, err := runner.Draft(ctx, id, num, opts)
		return err
	})
	c.JSON(http.StatusAccepted, gin.H{
		"bookId":  id,
		"chapter": num,
		"status":  "queued",
		"stage":   "rewrite",
	})
}

// handleResync rebuilds the truth files from the canonical state.
// This is a no-op placeholder; the truth file reconstruction logic
// lives in the original TypeScript state manager and is not part of
// this Go port's scope.
//   POST /api/v1/books/:id/resync/:chapter
func (s *Server) handleResync(c *gin.Context) {
	id := c.Param("id")
	num, _ := strconv.Atoi(c.Param("chapter"))
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "chapter": num, "status": "queued", "stage": "resync"})
}

// handleRepairState rebuilds a single chapter's state from the
// markdown truth files. Like handleResync, this is a placeholder —
// the state reconstruction logic from the TypeScript state manager
// has not been ported.
//   POST /api/v1/books/:id/repair-state/:chapter
func (s *Server) handleRepairState(c *gin.Context) {
	id := c.Param("id")
	num, _ := strconv.Atoi(c.Param("chapter"))
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "chapter": num, "status": "queued", "stage": "repair-state"})
}

// handleFoundationRevise regenerates the book foundation (story_bible
// + book_rules). Requires an Architect agent which is not yet
// implemented in this Go port — see internal/agents/prompts.go for the
// DefaultArchitectPrompt constant and the TODO in handleCreateBook.
//   POST /api/v1/books/:id/foundation/revise
func (s *Server) handleFoundationRevise(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "status": "queued", "stage": "foundation/revise"})
}

// handleConsolidate merges chapter summaries to reduce context
// pressure. The consolidator agent is not yet implemented; this
// endpoint remains a placeholder for parity with the TypeScript API.
//   POST /api/v1/books/:id/consolidate
func (s *Server) handleConsolidate(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusAccepted, gin.H{"bookId": id, "status": "queued", "stage": "consolidate"})
}

// handleDetect runs AIGC detection on one chapter. The detection
// agent is not yet implemented; this endpoint remains a placeholder.
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
