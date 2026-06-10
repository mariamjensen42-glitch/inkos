package agents

import (
	"context"
	"errors"
	"fmt"

	"github.com/narcooo/inkos/internal/llm"
)

// Reviser revises a chapter based on an audit result.
type Reviser struct{}

// NewReviser returns a fresh Reviser.
func NewReviser() *Reviser { return &Reviser{} }

// ReviseOptions carries the inputs to a Reviser.Run.
type ReviseOptions struct {
	Audit *AuditorResult
}

// Run executes the reviser agent.
func (r *Reviser) Run(ctx context.Context, client *llm.Client, chapter string, opts ReviseOptions) (string, error) {
	if client == nil {
		return "", errors.New("nil LLM client")
	}
	if opts.Audit == nil {
		return "", errors.New("nil audit")
	}
	system := `You are a chapter reviser. Take the chapter and the audit
findings and produce a revised chapter that resolves every blocker and
addresses every suggestion. Preserve voice, POV, and overall structure.

Output the revised chapter text directly, no JSON.`
	user := fmt.Sprintf("Audit findings:\n%v\n\nOriginal chapter:\n```\n%s\n```",
		opts.Audit, chapter)
	resp, err := client.ChatCompletion(ctx, &llm.Request{
		Messages: []llm.Message{{Role: llm.RoleSystem, Content: system}, {Role: llm.RoleUser, Content: user}},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("no choices")
	}
	return resp.Choices[0].Message.Content, nil
}
