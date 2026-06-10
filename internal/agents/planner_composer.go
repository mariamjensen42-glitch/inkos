package agents

import (
	"context"
	"errors"
	"fmt"

	"github.com/narcooo/inkos/internal/llm"
)

// Planner generates a chapter intent file (chapter-XXXX.intent.md).
type Planner struct{}

// NewPlanner returns a fresh Planner.
func NewPlanner() *Planner { return &Planner{} }

// Run produces the intent document for the next chapter.
func (p *Planner) Run(ctx context.Context, client *llm.Client, bookContext string) (string, error) {
	if client == nil {
		return "", errors.New("nil LLM client")
	}
	system := `You are the chapter planner. Produce a chapter intent document
that lists: (1) chapter goal, (2) must-keep facts, (3) must-avoid events,
(4) hook in/out, (5) POV. Output a single Markdown document — no JSON.`
	resp, err := client.ChatCompletion(ctx, &llm.Request{
		Messages: []llm.Message{{Role: llm.RoleSystem, Content: system}, {Role: llm.RoleUser, Content: bookContext}},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("no choices")
	}
	return resp.Choices[0].Message.Content, nil
}

// Composer assembles the chapter context (context.json + rule-stack).
type Composer struct{}

// NewComposer returns a fresh Composer.
func NewComposer() *Composer { return &Composer{} }

// Run composes a context JSON for the writer.
func (c *Composer) Run(ctx context.Context, client *llm.Client, plan string) (string, error) {
	if client == nil {
		return "", errors.New("nil LLM client")
	}
	system := "You are the context composer. Take a chapter plan and produce\n" +
		"a single JSON object describing the composed context. Use a ```json```\n" +
		"fence. Schema: {\"context\":\"<text>\",\"must_keep\":[],\"must_avoid\":[]}"
	resp, err := client.ChatCompletion(ctx, &llm.Request{
		Messages: []llm.Message{{Role: llm.RoleSystem, Content: system}, {Role: llm.RoleUser, Content: plan}},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("no choices")
	}
	return resp.Choices[0].Message.Content, nil
}

// Avoid "fmt" unused import for build tags.
var _ = fmt.Sprintf
