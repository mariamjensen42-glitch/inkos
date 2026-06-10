// Package api: pipeline handler implementation. These functions wire
// the agent layer to the HTTP layer so the /write-next, /draft, /plan,
// /compose, /audit, /revise endpoints actually invoke an LLM.
package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/narcooo/inkos/internal/agents"
	"github.com/narcooo/inkos/internal/llm"
)

// pipelineCommonOptions are shared by all pipeline handlers.
type pipelineCommonOptions struct {
	Service string `json:"service"`
	Words   int    `json:"words"`
	Context string `json:"context"`
	Extra   string `json:"extra"`
	Audit   bool   `json:"audit"`
	Revise  bool   `json:"revise"`
}

// resolveLLM returns an LLM client for the requested service.
func (s *Server) resolveLLM(service string) (*llm.Client, error) {
	if s.ResolverFactory == nil {
		return nil, errors.New("no resolver factory configured")
	}
	resolver := s.ResolverFactory()
	if resolver == nil {
		return nil, errors.New("nil resolver")
	}
	client, _, err := resolver.Client(service)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// runAgent invokes the LLM and returns the assistant's reply text. The
// book id is prepended for context if non-empty.
func (s *Server) runAgent(ctx context.Context, sessionID, message, bookID, mode string) (string, error) {
	client, err := s.resolveLLM("")
	if err != nil {
		return "", err
	}
	user := message
	if bookID != "" {
		user = fmt.Sprintf("[book: %s] %s", bookID, message)
	}
	writer := agents.NewWriter()
	res, err := writer.Run(ctx, client, user, &agents.WriterOptions{
		System: "You are InkOS, an assistant for novelists.",
	})
	if err != nil {
		return "", err
	}
	_ = sessionID
	_ = mode
	return res.Body, nil
}
