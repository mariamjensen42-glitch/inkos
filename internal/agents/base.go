// Package agents provides LLM-driven agent runners. Each agent wraps a
// prompt + a structured-output parser. The base helpers in this file
// power the streaming agent session, the chapter write/draft/audit/
// revise endpoints, and the LLM-driven portion of the pipeline.
package agents

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/narcooo/inkos/internal/llm"
)

// Agent is a named LLM agent.
type Agent struct {
	Name  string
	RunFn func(ctx context.Context, client *llm.Client, sys, user string) (string, error)
}

// Registry holds named agents.
type Registry struct {
	mu     sync.RWMutex
	agents map[string]Agent
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{agents: map[string]Agent{}}
}

// Register adds (or replaces) an agent.
func (r *Registry) Register(a Agent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[a.Name] = a
}

// Get returns an agent by name.
func (r *Registry) Get(name string) (Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.agents[name]
	if !ok {
		return Agent{}, fmt.Errorf("agent %q not found", name)
	}
	return a, nil
}

// Names returns the list of registered agent names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.agents))
	for n := range r.agents {
		out = append(out, n)
	}
	return out
}

// SimpleAgent is the default implementation of an agent: a system prompt
// plus a user-prompt construction function.
type SimpleAgent struct {
	Name        string
	Description string
	System      string
	BuildUser   func(input map[string]interface{}) (string, error)
}

// Run executes the agent.
func (a *SimpleAgent) Run(ctx context.Context, client *llm.Client, sys, user string) (string, error) {
	if client == nil {
		return "", errors.New("nil LLM client")
	}
	prompt := sys
	if prompt == "" {
		prompt = a.System
	}
	resp, err := client.ChatCompletion(ctx, &llm.Request{
		Model:    "",
		Messages: []llm.Message{{Role: llm.RoleSystem, Content: prompt}, {Role: llm.RoleUser, Content: user}},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("no choices in response")
	}
	return resp.Choices[0].Message.Content, nil
}
