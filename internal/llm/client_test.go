package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientChatCompletion(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("bad auth: %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"x","object":"chat.completion","created":0,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "sk-test")
	resp, err := c.ChatCompletion(context.Background(), &Request{
		Model:    "m",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "hi" {
		t.Errorf("bad response: %+v", resp)
	}
	if gotBody["model"] != "m" {
		t.Errorf("body: %+v", gotBody)
	}
}

func TestClientChatCompletionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = io.WriteString(w, `{"error":"unauthorized"}`)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "x")
	_, err := c.ChatCompletion(context.Background(), &Request{
		Model: "m", Messages: []Message{{Role: RoleUser, Content: "x"}},
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestClientProbe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/models") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"data":[{"id":"m1"},{"id":"m2"}]}`)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "k")
	models, err := c.Probe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0] != "m1" {
		t.Errorf("got %v", models)
	}
}

func TestExtractJSON(t *testing.T) {
	cases := map[string]string{
		"```json\n{\"a\":1}\n```":          `{"a":1}`,
		"```\n{\"a\":2}\n```":              `{"a":2}`,
		"{\"a\":3}":                        `{"a":3}`,
		"prefix\n```json\n{\"a\":4}\n```\nsuffix": `{"a":4}`,
	}
	for in, want := range cases {
		got, err := ExtractJSON(in)
		if err != nil {
			t.Errorf("ExtractJSON(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ExtractJSON(%q) = %q, want %q", in, got, want)
		}
	}
}
