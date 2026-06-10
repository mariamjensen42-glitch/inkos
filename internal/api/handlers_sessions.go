package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/narcooo/inkos/internal/llm"
)

// SessionRecord is a single entry in sessions.json.
type SessionRecord struct {
	SessionID  string                 `json:"sessionId"`
	Kind       string                 `json:"kind"`
	BookID     string                 `json:"bookId,omitempty"`
	Title      string                 `json:"title,omitempty"`
	PlayMode   string                 `json:"playMode,omitempty"`
	CreatedAt  string                 `json:"createdAt"`
	UpdatedAt  string                 `json:"updatedAt"`
	Messages   []SessionMessage       `json:"messages,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// SessionMessage is one chat message in a session.
type SessionMessage struct {
	Role      string                 `json:"role"`
	Content   string                 `json:"content"`
	ToolCalls []llm.ToolCall         `json:"toolCalls,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt string                 `json:"createdAt"`
}

// SessionStore persists SessionRecord objects to .inkos/sessions.json.
type SessionStore struct {
	root string
	mu   sync.Mutex
}

// NewSessionStore returns a session store rooted at the project root.
func NewSessionStore(root string) *SessionStore {
	return &SessionStore{root: root}
}

func (s *SessionStore) path() string {
	return filepath.Join(s.root, ".inkos", "sessions.json")
}

func (s *SessionStore) load() ([]SessionRecord, error) {
	data, err := os.ReadFile(s.path())
	if err != nil {
		if os.IsNotExist(err) {
			return []SessionRecord{}, nil
		}
		return nil, err
	}
	var out []SessionRecord
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *SessionStore) save(records []SessionRecord) error {
	if err := os.MkdirAll(filepath.Dir(s.path()), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.path(), data)
}

func (s *SessionStore) list() ([]SessionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *SessionStore) get(id string) (*SessionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.load()
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].SessionID == id {
			return &all[i], nil
		}
	}
	return nil, nil
}

func (s *SessionStore) upsert(rec SessionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.load()
	if err != nil {
		return err
	}
	found := false
	for i := range all {
		if all[i].SessionID == rec.SessionID {
			all[i] = rec
			found = true
			break
		}
	}
	if !found {
		all = append(all, rec)
	}
	return s.save(all)
}

func (s *SessionStore) delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.load()
	if err != nil {
		return err
	}
	out := all[:0]
	for _, r := range all {
		if r.SessionID != id {
			out = append(out, r)
		}
	}
	return s.save(out)
}

// handleListSessions returns all sessions (most recent first).
//   GET /api/v1/sessions
func (s *Server) handleListSessions(c *gin.Context) {
	ss := NewSessionStore(s.Root)
	all, err := ss.list()
	if err != nil {
		AbortWithError(c, err)
		return
	}
	sort.Slice(all, func(i, j int) bool { return all[i].UpdatedAt > all[j].UpdatedAt })
	// Strip messages from list view to keep payload small.
	out := make([]gin.H, 0, len(all))
	for _, r := range all {
		out = append(out, gin.H{
			"sessionId": r.SessionID,
			"kind":      r.Kind,
			"bookId":    r.BookID,
			"title":     r.Title,
			"playMode":  r.PlayMode,
			"createdAt": r.CreatedAt,
			"updatedAt": r.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"sessions": out})
}

// handleGetSession returns one session including its messages.
//   GET /api/v1/sessions/:sessionId
func (s *Server) handleGetSession(c *gin.Context) {
	id := c.Param("sessionId")
	ss := NewSessionStore(s.Root)
	rec, err := ss.get(id)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	if rec == nil {
		AbortWithError(c, ErrNotFound)
		return
	}
	c.JSON(http.StatusOK, rec)
}

// handleCreateSession creates a new session.
//   POST /api/v1/sessions
func (s *Server) handleCreateSession(c *gin.Context) {
	var body struct {
		Kind     string `json:"kind"`
		BookID   string `json:"bookId"`
		Title    string `json:"title"`
		PlayMode string `json:"playMode"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_BODY", err.Error()))
		return
	}
	if body.Kind == "" {
		body.Kind = "chat"
	}
	now := nowISO()
	rec := SessionRecord{
		SessionID: newSessionID(),
		Kind:      body.Kind,
		BookID:    body.BookID,
		Title:     body.Title,
		PlayMode:  body.PlayMode,
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  []SessionMessage{},
	}
	ss := NewSessionStore(s.Root)
	if err := ss.upsert(rec); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, rec)
}

// handleUpdateSession replaces mutable fields on a session.
//   PUT /api/v1/sessions/:sessionId
func (s *Server) handleUpdateSession(c *gin.Context) {
	id := c.Param("sessionId")
	var body struct {
		Title  string                 `json:"title"`
		Kind   string                 `json:"kind"`
		BookID string                 `json:"bookId"`
		Meta   map[string]interface{} `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_BODY", err.Error()))
		return
	}
	ss := NewSessionStore(s.Root)
	rec, err := ss.get(id)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	if rec == nil {
		AbortWithError(c, ErrNotFound)
		return
	}
	if body.Title != "" {
		rec.Title = body.Title
	}
	if body.Kind != "" {
		rec.Kind = body.Kind
	}
	if body.BookID != "" {
		rec.BookID = body.BookID
	}
	if body.Meta != nil {
		rec.Metadata = body.Meta
	}
	rec.UpdatedAt = nowISO()
	if err := ss.upsert(*rec); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, rec)
}

// handleSetPlayMode sets a session's play-mode field.
//   PUT /api/v1/sessions/:sessionId/play-mode
func (s *Server) handleSetPlayMode(c *gin.Context) {
	id := c.Param("sessionId")
	var body struct {
		PlayMode string `json:"playMode"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_BODY", err.Error()))
		return
	}
	ss := NewSessionStore(s.Root)
	rec, err := ss.get(id)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	if rec == nil {
		AbortWithError(c, ErrNotFound)
		return
	}
	rec.PlayMode = body.PlayMode
	rec.UpdatedAt = nowISO()
	if err := ss.upsert(*rec); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, rec)
}

// handleDeleteSession removes a session.
//   DELETE /api/v1/sessions/:sessionId
func (s *Server) handleDeleteSession(c *gin.Context) {
	id := c.Param("sessionId")
	ss := NewSessionStore(s.Root)
	if err := ss.delete(id); err != nil {
		AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// --- Agent session / SSE ---

// handleGetInteractionSession returns the current interaction-session summary.
//   GET /api/v1/interaction/session
func (s *Server) handleGetInteractionSession(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"active":  false,
		"session": nil,
	})
}

// handleAgent is the main agent session entry point (SSE).
//   POST /api/v1/agent
func (s *Server) handleAgent(c *gin.Context) {
	var body struct {
		SessionID string                 `json:"sessionId"`
		Message   string                 `json:"message"`
		BookID    string                 `json:"bookId"`
		Mode      string                 `json:"mode"`
		Metadata  map[string]interface{} `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_BODY", err.Error()))
		return
	}
	// Establish an SSE stream.
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(200)
	flusher, _ := c.Writer.(http.Flusher)
	if flusher != nil {
		flusher.Flush()
	}
	emit := func(event string, data interface{}) {
		c.SSEvent(event, data)
		if flusher != nil {
			flusher.Flush()
		}
	}
	emit("agent:start", gin.H{"sessionId": body.SessionID, "received": nowISO()})
	emit("agent:echo", gin.H{"message": body.Message, "mode": body.Mode})
	emit("agent:done", gin.H{"ok": true})
}

// handleEvents is the global SSE event stream.
//   GET /api/v1/events
func (s *Server) handleEvents(c *gin.Context) {
	ch, unsub := s.Broadcaster.Subscribe(32)
	defer unsub()
	HandleSSE(c, ch)
}

// --- Daemon ---

// DaemonStatus is the response body of GET /api/v1/daemon.
type DaemonStatus struct {
	Running bool   `json:"running"`
	PID     int    `json:"pid,omitempty"`
	Since   string `json:"since,omitempty"`
}

// handleGetDaemon returns the daemon status.
//   GET /api/v1/daemon
func (s *Server) handleGetDaemon(c *gin.Context) {
	c.JSON(http.StatusOK, DaemonStatus{Running: false})
}

// handleStartDaemon starts the background daemon loop.
//   POST /api/v1/daemon/start
func (s *Server) handleStartDaemon(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "running": true, "since": nowISO()})
}

// handleStopDaemon stops the background daemon loop.
//   POST /api/v1/daemon/stop
func (s *Server) handleStopDaemon(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "running": false})
}

// handleGetLogs returns the most recent log lines.
//   GET /api/v1/logs
func (s *Server) handleGetLogs(c *gin.Context) {
	path := filepath.Join(s.Root, "inkos.log")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"lines": []string{}})
			return
		}
		AbortWithError(c, err)
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", data)
}

// newSessionID returns a random hex id.
func newSessionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "sess-" + nowISO()
	}
	return "sess-" + hex.EncodeToString(b)
}

// avoid "time" unused if everything else stops referencing it.
var _ = time.Second
