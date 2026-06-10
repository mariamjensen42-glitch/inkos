package llm

import (
	"os"
	"testing"

	"github.com/narcooo/inkos/internal/model"
)

func TestResolverDefault(t *testing.T) {
	cfg := &model.ProjectConfig{
		LLM: model.LLMConfig{
			Provider: "openai",
			BaseURL:  "https://api.openai.com/v1",
			Model:    "gpt-4o",
		},
		CurrentService: "default",
	}
	r := NewResolver(cfg, nil)
	rm, err := r.Resolve("default")
	if err != nil {
		t.Fatal(err)
	}
	if rm.BaseURL != "https://api.openai.com/v1" || rm.Model != "gpt-4o" {
		t.Errorf("got %+v", rm)
	}
}

func TestResolverServiceKeyFromSecrets(t *testing.T) {
	cfg := &model.ProjectConfig{
		LLM: model.LLMConfig{Provider: "custom", BaseURL: "https://x/v1", Model: "m1"},
		Services: map[string]model.LLMConfig{
			"moonshot": {Provider: "custom", BaseURL: "https://api.moonshot.cn/v1", Model: "kimi"},
		},
		CurrentService: "moonshot",
	}
	sec := &model.Secrets{
		Services: map[string]model.SecretEntry{
			"moonshot": {APIKey: "sk-m"},
		},
	}
	r := NewResolver(cfg, sec)
	rm, err := r.Resolve("moonshot")
	if err != nil {
		t.Fatal(err)
	}
	if rm.BaseURL != "https://api.moonshot.cn/v1" || rm.APIKey != "sk-m" || rm.Model != "kimi" {
		t.Errorf("got %+v", rm)
	}
}

func TestResolverServiceFallback(t *testing.T) {
	cfg := &model.ProjectConfig{
		LLM: model.LLMConfig{Provider: "openai", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o"},
	}
	// Asking for an unknown service should fall back to default.
	r := NewResolver(cfg, nil)
	rm, err := r.Resolve("does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	if rm.Model != "gpt-4o" {
		t.Errorf("got %+v", rm)
	}
}

func TestResolverEnvKey(t *testing.T) {
	os.Setenv("INKOS_TEST_KEY", "sk-env")
	defer os.Unsetenv("INKOS_TEST_KEY")
	cfg := &model.ProjectConfig{
		LLM: model.LLMConfig{Provider: "custom", BaseURL: "https://x/v1", Model: "m", APIKeyEnv: "INKOS_TEST_KEY"},
	}
	r := NewResolver(cfg, nil)
	rm, err := r.Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	if rm.APIKey != "sk-env" {
		t.Errorf("got %+v", rm)
	}
}
