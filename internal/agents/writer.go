// Package agents: writer implementation. The writer agent takes a
// chapter context and produces the chapter prose. The full implementation
// includes length normalization, POV filtering, and chapter splitting;
// this skeleton focuses on the core LLM call and the chapter persistence.
package agents

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/narcooo/inkos/internal/llm"
	"github.com/narcooo/inkos/internal/model"
)

// WriterResult is the output of a single Writer.Run call.
type WriterResult struct {
	Body      string
	WordCount int
	Truncated bool
}

// Writer is the chapter-writer agent.
type Writer struct{}

// NewWriter returns a fresh Writer.
func NewWriter() *Writer { return &Writer{} }

// Run executes the writer agent. The userPrompt should be the composed
// chapter context (plan + intent + relevant truth excerpts + style guide).
func (w *Writer) Run(ctx context.Context, client *llm.Client, userPrompt string, opts *WriterOptions) (*WriterResult, error) {
	if client == nil {
		return nil, errors.New("nil LLM client")
	}
	if opts == nil {
		opts = &WriterOptions{}
	}
	system := opts.System
	if system == "" {
		system = DefaultWriterPrompt
	}
	user := userPrompt
	if opts.WordsTarget > 0 {
		user = fmt.Sprintf("Target word count: %d words.\n\n%s", opts.WordsTarget, user)
	}
	resp, err := client.ChatCompletion(ctx, &llm.Request{
		Model:    opts.Model,
		Messages: []llm.Message{{Role: llm.RoleSystem, Content: system}, {Role: llm.RoleUser, Content: user}},
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("no choices in response")
	}
	body := strings.TrimSpace(resp.Choices[0].Message.Content)
	words := countWords(body)
	return &WriterResult{Body: body, WordCount: words}, nil
}

// WriterOptions configures the writer.
type WriterOptions struct {
	Model       string
	System      string
	WordsTarget int
	Min         int
	Max         int
}

// CountWords returns a rough word count for the body. For CJK text this
// is a simplified approximation: each Han char counts as 1; ASCII word
// boundaries split English words. Exported so the pipeline package can
// reuse it when computing chapter word counts after a revise pass.
func CountWords(s string) int { return countWords(s) }

// countWords returns a rough word count for the body. For CJK text this
// is a simplified approximation: each Han char counts as 1; ASCII word
// boundaries split English words.
func countWords(s string) int {
	if s == "" {
		return 0
	}
	count := 0
	inWord := false
	for _, r := range s {
		if r <= 0x7F {
			if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
				if inWord {
					count++
					inWord = false
				}
			} else {
				inWord = true
			}
		} else {
			count++
		}
	}
	if inWord {
		count++
	}
	return count
}

// Avoid unused-import on model.
var _ = model.BookStatusActive
