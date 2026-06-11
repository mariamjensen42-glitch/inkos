// Package api: pipeline handler implementation helpers. These wrap the
// internal/pipeline.Runner so the HTTP layer can fire-and-forget the
// plan -> compose -> write -> audit -> revise state machine and return
// 202 Accepted immediately. Progress is broadcast through the SSE
// broadcaster, which clients subscribe to at GET /api/v1/events.
package api

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/narcooo/inkos/internal/llm"
	"github.com/narcooo/inkos/internal/pipeline"
)

// pipelineBroadcaster adapts api.Broadcaster to the pipeline.Broadcaster
// interface.
type pipelineBroadcaster struct{ b *Broadcaster }

func (p *pipelineBroadcaster) PublishEvent(name string, data interface{}) {
	if p == nil || p.b == nil {
		return
	}
	p.b.PublishEvent(name, data)
}

// newRunner constructs a pipeline.Runner bound to the current server
// dependencies. The resolver factory is captured by reference so that
// if the server's resolver is reconfigured at runtime, subsequent
// stages pick up the new resolver.
func (s *Server) newRunner() *pipeline.Runner {
	bc := &pipelineBroadcaster{b: s.Broadcaster}
	rf := func() *llm.Resolver {
		if s.ResolverFactory == nil {
			return nil
		}
		return s.ResolverFactory()
	}
	return pipeline.NewRunner(s.Books, s.Truth, s.Project, bc, rf)
}

// errNoResolver is returned by handlers when the project has no
// configured LLM and a pipeline stage requires one. Surfaced to the
// client as a 400 LLM_CONFIG_ERROR.
var errNoResolver = errors.New("no LLM resolver configured for the active project")

// resolveChapterNumber reads the chapter number from the request body
// (preferred) or falls back to NextChapterNumber. If neither is
// available, returns 0.
func (s *Server) resolveChapterNumber(c *gin.Context, bookID string, bodyChapter int) (int, error) {
	if bodyChapter > 0 {
		return bodyChapter, nil
	}
	return s.Books.NextChapterNumber(bookID)
}

// detach runs fn in a background goroutine and reports the outcome
// through the SSE broadcaster as pipeline:<stage>:error on failure.
// The caller's HTTP response is unaffected.
func (s *Server) detach(stage string, bookID string, chapter int, fn func(ctx context.Context) error) {
	go func() {
		ctx := context.Background()
		if err := fn(ctx); err != nil {
			s.Broadcaster.PublishEvent("pipeline:"+stage+":error", gin.H{
				"bookId":  bookID,
				"chapter": chapter,
				"stage":   stage,
				"error":   err.Error(),
			})
		}
	}()
}
