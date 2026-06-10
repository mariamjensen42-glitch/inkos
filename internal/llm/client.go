// Package llm provides a minimal OpenAI-compatible Chat Completions client.
//
// The InkOS API supports any service that speaks the OpenAI Chat
// Completions protocol (OpenAI, Moonshot, DeepSeek, kkaiapi, OpenRouter,
// Ollama's /v1, etc.). Users configure baseUrl + apiKey + model per
// service in the project config.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Role is a chat message role.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a single chat message.
type Message struct {
	Role    Role        `json:"role"`
	Content string      `json:"content"`
	Name    string      `json:"name,omitempty"`
	ToolCallID string   `json:"tool_call_id,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall is a model-issued function call.
type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function describes the function name and arguments.
type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool is a tool definition sent to the model.
type Tool struct {
	Type     string             `json:"type"`
	Function ToolFunctionSpec   `json:"function"`
}

// ToolFunctionSpec describes a callable function.
type ToolFunctionSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// Request is a Chat Completions request.
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	ToolChoice  string    `json:"tool_choice,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	TopP        *float64  `json:"top_p,omitempty"`
	Extra       map[string]interface{} `json:"-"`
}

// Response is a non-streaming Chat Completions response.
type Response struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// Choice is a single response choice.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage reports token counts.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Client is an OpenAI-compatible chat completions client.
type Client struct {
	BaseURL string
	APIKey  string
	OrgID   string
	HTTP    *http.Client
}

// NewClient returns a Client for the given base URL and API key.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 0},
	}
}

// ChatCompletion issues a non-streaming request and returns the response.
func (c *Client) ChatCompletion(ctx context.Context, req *Request) (*Response, error) {
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := c.newRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, readError(resp)
	}
	out := &Response{}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return out, nil
}

// StreamChunk is one SSE chunk in a streaming response.
type StreamChunk struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

// ChatCompletionStream issues a streaming request. The returned channel
// emits each delta chunk; it is closed when the stream completes or
// an error occurs (in which case the last value carries an error).
func (c *Client) ChatCompletionStream(ctx context.Context, req *Request) (<-chan StreamEvent, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := c.newRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "text/event-stream")
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, readError(resp)
	}
	out := make(chan StreamEvent, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if !errors.Is(err, io.EOF) {
					out <- StreamEvent{Err: err}
				}
				return
			}
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			payload := bytes.TrimSpace(line[5:])
			if bytes.Equal(payload, []byte("[DONE]")) {
				return
			}
			var chunk StreamChunk
			if err := json.Unmarshal(payload, &chunk); err != nil {
				out <- StreamEvent{Err: fmt.Errorf("decode chunk: %w", err)}
				continue
			}
			out <- StreamEvent{Chunk: &chunk}
		}
	}()
	return out, nil
}

// StreamEvent is one message on a streaming channel.
type StreamEvent struct {
	Chunk *StreamChunk
	Err   error
}

func (c *Client) newRequest(ctx context.Context, body []byte) (*http.Request, error) {
	url := c.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	if c.OrgID != "" {
		req.Header.Set("OpenAI-Organization", c.OrgID)
	}
	return req, nil
}

func readError(resp *http.Response) error {
	data, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("llm %s: %s", resp.Status, truncate(string(data), 1000))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// Probe hits GET {BaseURL}/models to check connectivity. Returns the
// raw model id list on success.
func (c *Client) Probe(ctx context.Context) ([]string, error) {
	url := c.BaseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, readError(resp)
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}
