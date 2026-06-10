package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/narcooo/inkos/internal/model"
)

// handleListBooks returns all books under books/.
//   GET /api/v1/books
func (s *Server) handleListBooks(c *gin.Context) {
	ids, err := s.Books.ListBooks()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	summaries := make([]gin.H, 0, len(ids))
	for _, id := range ids {
		cfg, err := s.Books.LoadBookConfig(id)
		if err != nil {
			continue
		}
		idx, _ := s.Books.LoadChapterIndex(id)
		next, _ := s.Books.NextChapterNumber(id)
		summaries = append(summaries, gin.H{
			"id":              id,
			"title":           cfg.Title,
			"genre":           cfg.Genre,
			"platform":        cfg.Platform,
			"language":        cfg.Language,
			"status":          cfg.Status,
			"chapterCount":    len(idx.Chapters),
			"chapterWordCount": cfg.ChapterWordCount,
			"targetChapters":  cfg.TargetChapters,
			"createdAt":       cfg.CreatedAt,
			"updatedAt":       cfg.UpdatedAt,
			"nextChapter":     next,
		})
	}
	c.JSON(http.StatusOK, gin.H{"books": summaries})
}

// handleGetBook returns the book config + chapter index.
//   GET /api/v1/books/:id
func (s *Server) handleGetBook(c *gin.Context) {
	id := c.Param("id")
	cfg, err := s.Books.LoadBookConfig(id)
	if err != nil {
		AbortWithError(c, ErrBookNotFound)
		return
	}
	idx, _ := s.Books.LoadChapterIndex(id)
	next, _ := s.Books.NextChapterNumber(id)
	c.JSON(http.StatusOK, gin.H{
		"book":        cfg,
		"chapters":    idx.Chapters,
		"nextChapter": next,
	})
}

// handleCreateBookRequest is the body of POST /api/v1/books/create.
type handleCreateBookRequest struct {
	Title           string `json:"title"`
	Genre           string `json:"genre"`
	Language        string `json:"language"`
	Platform        string `json:"platform"`
	ChapterWordCount int   `json:"chapterWordCount"`
	TargetChapters  int    `json:"targetChapters"`
	Blurb           string `json:"blurb"`
	Brief           string `json:"brief"`
}

// handleCreateBook creates a new book and (in the full implementation)
// kicks off the architect agent to produce book foundation. In this
// skeleton it writes the config and seeds the story/ skeleton.
//   POST /api/v1/books/create
func (s *Server) handleCreateBook(c *gin.Context) {
	var req handleCreateBookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_BODY", err.Error()))
		return
	}
	if req.Title == "" {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_TITLE", "title is required"))
		return
	}
	id := bookIDFromTitle(req.Title)
	if id == "" {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_TITLE", "could not derive a valid book id"))
		return
	}
	if s.Books.BookExists(id) {
		AbortWithError(c, NewAPIError(http.StatusConflict, "BOOK_EXISTS", "book already exists"))
		return
	}
	now := nowISO()
	cfg := &model.BookConfig{
		ID:               id,
		Title:            req.Title,
		Genre:            model.Genre(req.Genre),
		Language:         req.Language,
		Platform:         model.Platform(req.Platform),
		ChapterWordCount: req.ChapterWordCount,
		TargetChapters:   req.TargetChapters,
		Status:           model.BookStatusActive,
		Blurb:            req.Blurb,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := createBookOnDisk(s.Root, cfg); err != nil {
		AbortWithError(c, err)
		return
	}
	if err := s.Books.CreateBook(cfg); err != nil {
		AbortWithError(c, err)
		return
	}
	s.Broadcaster.Publish(SSEEvent{Event: "book:creating", Data: gin.H{"bookId": id, "title": req.Title}})
	// Run the architect agent in the background; in the skeleton this
	// just emits a progress event.
	go func() {
		// TODO: run architect agent here.
		s.Broadcaster.Publish(SSEEvent{Event: "book:created", Data: gin.H{"bookId": id}})
	}()
	c.JSON(http.StatusOK, gin.H{"id": id, "status": "creating"})
}

// handleUpdateBook updates mutable book fields.
//   PUT /api/v1/books/:id
func (s *Server) handleUpdateBook(c *gin.Context) {
	id := c.Param("id")
	cfg, err := s.Books.LoadBookConfig(id)
	if err != nil {
		AbortWithError(c, ErrBookNotFound)
		return
	}
	var patch map[string]interface{}
	if err := c.ShouldBindJSON(&patch); err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_BODY", err.Error()))
		return
	}
	patchJSON(cfg, patch)
	cfg.UpdatedAt = nowISO()
	if err := s.Books.SaveBookConfig(cfg); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"book": cfg})
}

// handleDeleteBook removes a book.
//   DELETE /api/v1/books/:id
func (s *Server) handleDeleteBook(c *gin.Context) {
	id := c.Param("id")
	if !s.Books.BookExists(id) {
		AbortWithError(c, ErrBookNotFound)
		return
	}
	if err := s.Books.DeleteBook(id); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// handleCreateStatus returns the current creation progress for a book.
//   GET /api/v1/books/:id/create-status
func (s *Server) handleCreateStatus(c *gin.Context) {
	id := c.Param("id")
	cfg, err := s.Books.LoadBookConfig(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "unknown"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"bookId": id,
		"status": "created",
		"book":   cfg,
	})
}

// handleGetChapter returns one chapter file.
//   GET /api/v1/books/:id/chapters/:num
func (s *Server) handleGetChapter(c *gin.Context) {
	id := c.Param("id")
	num, err := strconv.Atoi(c.Param("num"))
	if err != nil || num < 1 {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_CHAPTER", "invalid chapter number"))
		return
	}
	body, err := s.Books.ReadChapter(id, num)
	if err != nil {
		AbortWithError(c, NewAPIError(http.StatusNotFound, "CHAPTER_NOT_FOUND", err.Error()))
		return
	}
	idx, _ := s.Books.LoadChapterIndex(id)
	var meta *struct{}
	_ = meta
	metaFound := false
	for _, ch := range idx.Chapters {
		if ch.Number == num {
			c.JSON(http.StatusOK, gin.H{"chapter": gin.H{"number": num, "title": ch.Title, "status": ch.Status, "body": body}, "meta": ch})
			metaFound = true
			return
		}
	}
	if !metaFound {
		c.JSON(http.StatusOK, gin.H{"chapter": gin.H{"number": num, "body": body}})
	}
}

// handlePutChapter writes one chapter file + updates the index.
//   PUT /api/v1/books/:id/chapters/:num
func (s *Server) handlePutChapter(c *gin.Context) {
	id := c.Param("id")
	num, err := strconv.Atoi(c.Param("num"))
	if err != nil || num < 1 {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_CHAPTER", "invalid chapter number"))
		return
	}
	var body struct {
		Title     string `json:"title"`
		Status    string `json:"status"`
		Body      string `json:"body"`
		WordCount int    `json:"wordCount"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_BODY", err.Error()))
		return
	}
	release, err := s.Books.Lock(id)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	defer release()
	if err := s.Books.WriteChapter(id, num, body.Body); err != nil {
		AbortWithError(c, err)
		return
	}
	now := nowISO()
	meta := chapterMetaFromRequest(num, body, now)
	if err := s.Books.AddOrUpdateChapter(id, meta); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "meta": meta})
}

// handleApproveChapter marks a chapter approved.
//   POST /api/v1/books/:id/chapters/:num/approve
func (s *Server) handleApproveChapter(c *gin.Context) {
	id := c.Param("id")
	num, err := strconv.Atoi(c.Param("num"))
	if err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_CHAPTER", "invalid chapter number"))
		return
	}
	idx, err := s.Books.LoadChapterIndex(id)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	now := nowISO()
	updated := false
	for i := range idx.Chapters {
		if idx.Chapters[i].Number == num {
			idx.Chapters[i].Status = "approved"
			idx.Chapters[i].ApprovedAt = &now
			idx.Chapters[i].UpdatedAt = now
			updated = true
			break
		}
	}
	if !updated {
		AbortWithError(c, NewAPIError(http.StatusNotFound, "CHAPTER_NOT_FOUND", "chapter not in index"))
		return
	}
	if err := s.Books.SaveChapterIndex(id, idx); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleRejectChapter marks a chapter rejected.
//   POST /api/v1/books/:id/chapters/:num/reject
func (s *Server) handleRejectChapter(c *gin.Context) {
	id := c.Param("id")
	num, err := strconv.Atoi(c.Param("num"))
	if err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_CHAPTER", "invalid chapter number"))
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	idx, err := s.Books.LoadChapterIndex(id)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	now := nowISO()
	for i := range idx.Chapters {
		if idx.Chapters[i].Number == num {
			idx.Chapters[i].Status = "rejected"
			idx.Chapters[i].UpdatedAt = now
			break
		}
	}
	if err := s.Books.SaveChapterIndex(id, idx); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "reason": body.Reason})
}

// handleListTruth returns the list of truth files for a book.
//   GET /api/v1/books/:id/truth
func (s *Server) handleListTruth(c *gin.Context) {
	id := c.Param("id")
	files, err := s.Truth.ListTruth(id)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"files": files})
}

// handleGetTruth returns one truth file.
//   GET /api/v1/books/:id/truth/*file
func (s *Server) handleGetTruth(c *gin.Context) {
	id := c.Param("id")
	name := filepath.Base(c.Param("file"))
	body, err := s.Truth.ReadTruth(id, name)
	if err != nil {
		AbortWithError(c, NewAPIError(http.StatusNotFound, "TRUTH_NOT_FOUND", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"file": name, "body": body})
}

// handlePutTruth writes one truth file.
//   PUT /api/v1/books/:id/truth/*file
func (s *Server) handlePutTruth(c *gin.Context) {
	id := c.Param("id")
	name := filepath.Base(c.Param("file"))
	var body struct {
		Body string `json:"body"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_BODY", err.Error()))
		return
	}
	if err := s.Truth.WriteTruth(id, name, body.Body); err != nil {
		AbortWithError(c, err)
		return
	}
	s.Broadcaster.Publish(SSEEvent{Event: "truth:updated", Data: gin.H{"bookId": id, "file": name}})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleBookAnalytics returns aggregated book statistics.
//   GET /api/v1/books/:id/analytics
func (s *Server) handleBookAnalytics(c *gin.Context) {
	id := c.Param("id")
	idx, err := s.Books.LoadChapterIndex(id)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	totalWords, approvedWords, draftCount, approvedCount := 0, 0, 0, 0
	for _, ch := range idx.Chapters {
		totalWords += ch.WordCount
		switch ch.Status {
		case "approved":
			approvedWords += ch.WordCount
			approvedCount++
		case "draft":
			draftCount++
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"bookId":         id,
		"chapterCount":   len(idx.Chapters),
		"approvedCount":  approvedCount,
		"draftCount":     draftCount,
		"totalWords":     totalWords,
		"approvedWords":  approvedWords,
	})
}

// handleBookEval returns a quality evaluation summary (lightweight).
//   GET /api/v1/books/:id/eval
func (s *Server) handleBookEval(c *gin.Context) {
	id := c.Param("id")
	idx, err := s.Books.LoadChapterIndex(id)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	rows := make([]gin.H, 0, len(idx.Chapters))
	for _, ch := range idx.Chapters {
		rows = append(rows, gin.H{
			"number":    ch.Number,
			"title":     ch.Title,
			"status":    ch.Status,
			"wordCount": ch.WordCount,
		})
	}
	c.JSON(http.StatusOK, gin.H{"bookId": id, "rows": rows})
}

// handleBookExport concatenates approved chapters into a single text blob.
//   GET /api/v1/books/:id/export
func (s *Server) handleBookExport(c *gin.Context) {
	id := c.Param("id")
	format := c.DefaultQuery("format", "txt")
	idx, err := s.Books.LoadChapterIndex(id)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	cfg, _ := s.Books.LoadBookConfig(id)
	var out string
	out += "# " + id + "\n\n"
	if cfg != nil {
		out += cfg.Title + "\n\n"
	}
	for _, ch := range idx.Chapters {
		if ch.Status != "approved" {
			continue
		}
		body, err := s.Books.ReadChapter(id, ch.Number)
		if err != nil {
			continue
		}
		out += body + "\n\n"
	}
	switch format {
	case "md":
		c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(out))
	case "txt":
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(out))
	default:
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(out))
	}
}

// handleBookExportSave writes the export to disk under the book dir.
//   POST /api/v1/books/:id/export-save
func (s *Server) handleBookExportSave(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Format string `json:"format"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Format == "" {
		req.Format = "txt"
	}
	idx, err := s.Books.LoadChapterIndex(id)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	cfg, _ := s.Books.LoadBookConfig(id)
	var out string
	if cfg != nil {
		out += cfg.Title + "\n\n"
	}
	for _, ch := range idx.Chapters {
		if ch.Status != "approved" {
			continue
		}
		body, err := s.Books.ReadChapter(id, ch.Number)
		if err != nil {
			continue
		}
		out += body + "\n\n"
	}
	target := filepath.Join(s.Books.BookDir(id), "final."+req.Format)
	if err := writeFileAtomic(target, []byte(out)); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": target, "bytes": len(out)})
}

// --- helpers ---

func chapterMetaFromRequest(num int, body struct {
	Title     string `json:"title"`
	Status    string `json:"status"`
	Body      string `json:"body"`
	WordCount int    `json:"wordCount"`
}, now string) model.ChapterMeta {
	status := body.Status
	if status == "" {
		status = "draft"
	}
	return model.ChapterMeta{
		Number:    num,
		Title:     body.Title,
		Status:    model.ChapterStatus(status),
		WordCount: body.WordCount,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// patchJSON applies a shallow JSON merge to cfg.
func patchJSON(cfg *model.BookConfig, patch map[string]interface{}) {
	data, _ := json.Marshal(cfg)
	_ = json.Unmarshal(data, cfg)
	if v, ok := patch["title"].(string); ok {
		cfg.Title = v
	}
	if v, ok := patch["genre"].(string); ok {
		cfg.Genre = model.Genre(v)
	}
	if v, ok := patch["platform"].(string); ok {
		cfg.Platform = model.Platform(v)
	}
	if v, ok := patch["language"].(string); ok {
		cfg.Language = v
	}
	if v, ok := patch["status"].(string); ok {
		cfg.Status = model.BookStatus(v)
	}
	if v, ok := patch["blurb"].(string); ok {
		cfg.Blurb = v
	}
	if v, ok := patch["brief"].(string); ok {
		cfg.Brief = v
	}
	if v, ok := patch["chapterWordCount"].(float64); ok {
		cfg.ChapterWordCount = int(v)
	}
	if v, ok := patch["targetChapters"].(float64); ok {
		cfg.TargetChapters = int(v)
	}
}
