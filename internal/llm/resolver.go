package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/narcooo/inkos/internal/model"
)

// ResolvedModel is the LLM configuration selected for a single request.
type ResolvedModel struct {
	BaseURL  string
	APIKey   string
	Model    string
	Provider string
	Service  string
}

// Resolver picks a concrete base URL + key + model for a service, taking
// into account project config and secrets.json.
type Resolver struct {
	Project *model.ProjectConfig
	Secrets *model.Secrets
}

// NewResolver returns a resolver bound to a project config + secrets.
func NewResolver(cfg *model.ProjectConfig, sec *model.Secrets) *Resolver {
	return &Resolver{Project: cfg, Secrets: sec}
}

// Resolve picks the model for a given service. If service is empty,
// uses the current service or "default". Resolution order:
//  1. Project.Services[service] (if present)
//  2. Project.LLM (if service matches "default" or is empty)
//  3. The API key is fetched from Secrets.Services[service], or
//     Secrets.Services["default"], or from the config's APIKey field.
func (r *Resolver) Resolve(service string) (*ResolvedModel, error) {
	if r.Project == nil {
		return nil, errors.New("nil project config")
	}
	chosen := service
	if chosen == "" {
		chosen = r.Project.CurrentService
	}
	if chosen == "" {
		chosen = "default"
	}
	// Build base config.
	var llmCfg *model.LLMConfig
	if chosen == "default" {
		llmCfg = &r.Project.LLM
	} else if cfg, ok := r.Project.Services[chosen]; ok {
		llmCfg = &cfg
	} else {
		// Unknown service: fall back to default.
		llmCfg = &r.Project.LLM
	}
	if llmCfg == nil || llmCfg.Model == "" {
		return nil, fmt.Errorf("no LLM model configured for service %q", chosen)
	}
	baseURL := llmCfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL(llmCfg.Provider, chosen)
	}
	apiKey := llmCfg.APIKey
	if apiKey == "" && r.Secrets != nil {
		if k, ok := r.Secrets.Services[chosen]; ok {
			apiKey = k.APIKey
		}
	}
	if apiKey == "" && llmCfg.APIKeyEnv != "" {
		apiKey = lookupEnv(llmCfg.APIKeyEnv)
	}
	return &ResolvedModel{
		BaseURL:  baseURL,
		APIKey:   apiKey,
		Model:    llmCfg.Model,
		Provider: llmCfg.Provider,
		Service:  chosen,
	}, nil
}

// Client returns an LLM client for the given service.
func (r *Resolver) Client(service string) (*Client, *ResolvedModel, error) {
	rm, err := r.Resolve(service)
	if err != nil {
		return nil, nil, err
	}
	return NewClient(rm.BaseURL, rm.APIKey), rm, nil
}

// defaultBaseURL returns a sensible base URL when none is configured.
// Only OpenAI has a hard-coded default; everything else must be supplied
// by the user.
func defaultBaseURL(provider, service string) string {
	switch provider {
	case "openai", "":
		if service == "openai" || service == "default" {
			return "https://api.openai.com/v1"
		}
	}
	return ""
}

// envLookup is a package-level indirection for os.Getenv so tests can
// override it.
var envLookup = func(key string) string { return getenvDefault(key) }

// lookupEnv is the public hook used by Resolver.
func lookupEnv(key string) string { return envLookup(key) }

// regexJSONFence matches the first JSON code-fence in an LLM response.
var regexJSONFence = regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\n?```")

// ExtractJSON pulls the first JSON code-fence out of an LLM reply.
// If no fence is found, it tries to decode the whole text as JSON.
func ExtractJSON(s string) (string, error) {
	m := regexJSONFence.FindStringSubmatch(s)
	if len(m) >= 2 {
		return strings.TrimSpace(m[1]), nil
	}
	// Try whole text.
	trim := strings.TrimSpace(s)
	if trim == "" {
		return "", errors.New("empty response")
	}
	return trim, nil
}

// ParseJSON parses text into the given value, using ExtractJSON first.
func ParseJSON(text string, out interface{}) error {
	body, err := ExtractJSON(text)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(body), out)
}

// ChatOnce is a convenience wrapper that resolves a service, builds a
// request, and returns the first choice text.
func (r *Resolver) ChatOnce(ctx context.Context, service, system, user string, opts *Request) (string, error) {
	client, rm, err := r.Client(service)
	if err != nil {
		return "", err
	}
	if opts == nil {
		opts = &Request{}
	}
	opts.Model = rm.Model
	opts.Messages = []Message{{Role: RoleSystem, Content: system}, {Role: RoleUser, Content: user}}
	resp, err := client.ChatCompletion(ctx, opts)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("no choices in response")
	}
	return resp.Choices[0].Message.Content, nil
}
