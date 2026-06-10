package agents

import (
	"context"
	"errors"
	"fmt"

	"github.com/narcooo/inkos/internal/llm"
)

// AuditorResult mirrors the audit JSON shape.
type AuditorResult struct {
	Overall     map[string]interface{}      `json:"overall"`
	Dimensions  map[string]AuditDimension   `json:"dimensions"`
	Blockers    []string                    `json:"blockers"`
	Suggestions []string                    `json:"suggestions"`
}

// AuditDimension is a single audit finding.
type AuditDimension struct {
	Score    int    `json:"score"`
	Notes    string `json:"notes,omitempty"`
	Evidence string `json:"evidence,omitempty"`
}

// Auditor audits a single chapter.
type Auditor struct{}

// NewAuditor returns a fresh Auditor.
func NewAuditor() *Auditor { return &Auditor{} }

// Run executes the auditor agent.
func (a *Auditor) Run(ctx context.Context, client *llm.Client, chapter string, opts map[string]interface{}) (*AuditorResult, error) {
	if client == nil {
		return nil, errors.New("nil LLM client")
	}
	user := "Audit the following chapter:\n\n```\n" + chapter + "\n```\n\nReturn a JSON object."
	resp, err := client.ChatCompletion(ctx, &llm.Request{
		Messages: []llm.Message{{Role: llm.RoleSystem, Content: DefaultAuditorPrompt}, {Role: llm.RoleUser, Content: user}},
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("no choices in response")
	}
	out := &AuditorResult{
		Overall:     map[string]interface{}{},
		Dimensions:  map[string]AuditDimension{},
		Blockers:    []string{},
		Suggestions: []string{},
	}
	if err := llm.ParseJSON(resp.Choices[0].Message.Content, out); err != nil {
		return nil, fmt.Errorf("parse audit json: %w", err)
	}
	return out, nil
}
