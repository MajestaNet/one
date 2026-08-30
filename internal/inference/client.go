// Package inference routes agent LLM calls to install-local BYO providers
// or Native DigitalOcean Serverless Inference (BP-052).
package inference

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/egress"
)

// ActiveSource selects which backend the router uses.
type ActiveSource string

const (
	SourceNone         ActiveSource = "none"
	SourceDigitalOcean ActiveSource = "digitalocean"
	SourceBYO          ActiveSource = "byo"
)

// ErrNotConfigured means no active inference source is usable.
var ErrNotConfigured = errors.New("inference: not configured")

// ErrDOTokenMissing means Native DO is selected but DIGITALOCEAN_API_TOKEN is unset.
var ErrDOTokenMissing = errors.New("inference: DigitalOcean API token not configured")

// ErrEgressDenied means the BYO host is not on the install egress allowlist.
var ErrEgressDenied = errors.New("inference: provider host not on install egress allowlist")

// Route is a resolved OpenAI-compatible chat target.
type Route struct {
	Source        ActiveSource `json:"source"`
	BaseURL       string       `json:"baseUrl"`
	APIKey        string       `json:"-"`
	Model         string       `json:"model"`
	ProviderName  string       `json:"providerApiName,omitempty"`
	DOMode        Mode         `json:"doMode,omitempty"`
	BillingNotice string       `json:"billingNotice,omitempty"`
	Prepaid       bool         `json:"prepaid,omitempty"`
	// AllowDevLocal is set when Resolve ran with AllowDevLocal (non-production).
	AllowDevLocal bool `json:"-"`
}

// Message is an OpenAI-compatible chat message.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// Tool is an OpenAI-compatible tools[] entry (type=function).
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction is the JSON Schema function description advertised to the model.
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ToolCall is an assistant-requested function call.
type ToolCall struct {
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction holds the function name and JSON-encoded arguments.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatRequest is a subset of chat/completions.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
}

// ChatResponse is a non-streaming completion subset.
type ChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int     `json:"index"`
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
}

// Client calls OpenAI-compatible chat completions via an SSRF-safe transport.
type Client struct {
	HTTP *http.Client
}

// NewClient returns a client with dial-time SSRF guards and redirects disabled.
func NewClient() *Client {
	return NewClientWithOptions(false)
}

// NewClientWithOptions builds a client; allowDevLocal permits loopback Ollama dials.
func NewClientWithOptions(allowDevLocal bool) *Client {
	return &Client{HTTP: egress.NewSafeClientWithOptions(120*time.Second, func(_ *http.Request, _ []*http.Request) error {
		return fmt.Errorf("redirects disabled for inference")
	}, egress.DialOptions{AllowDevLocalHosts: allowDevLocal})}
}

// NewStreamClient is like NewClient but with no wall-clock Timeout (SSE).
func NewStreamClient() *Client {
	return NewStreamClientWithOptions(false)
}

// NewStreamClientWithOptions is NewStreamClient with optional local-dev dial relaxations.
func NewStreamClientWithOptions(allowDevLocal bool) *Client {
	return &Client{HTTP: egress.NewSafeClientWithOptions(0, func(_ *http.Request, _ []*http.Request) error {
		return fmt.Errorf("redirects disabled for inference")
	}, egress.DialOptions{AllowDevLocalHosts: allowDevLocal})}
}

// ClientForRoute picks a dial policy matching the resolved route.
func ClientForRoute(route *Route) *Client {
	allow := route != nil && route.AllowDevLocal
	return NewClientWithOptions(allow)
}

// StreamClientForRoute is ClientForRoute for SSE.
func StreamClientForRoute(route *Route) *Client {
	allow := route != nil && route.AllowDevLocal
	return NewStreamClientWithOptions(allow)
}

func (c *Client) http() *http.Client {
	if c != nil && c.HTTP != nil {
		return c.HTTP
	}
	return NewClient().HTTP
}

func chatURL(base string) string {
	b := strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(b, "/chat/completions") {
		return b
	}
	return b + "/chat/completions"
}

// ValidateProviderBaseURL enforces HTTPS + SSRF rules for a BYO base URL.
// When allowDevLocal is true (APP_ENV != production), http:// to localhost /
// 127.0.0.1 / host.docker.internal is accepted for local Ollama.
func ValidateProviderBaseURL(raw string, allowDevLocal bool) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("empty base URL")
	}
	if err := egress.ValidateURLWithOptions(raw, egress.ValidateOptions{AllowDevLocalHosts: allowDevLocal}); err != nil {
		u, perr := url.Parse(raw)
		if perr != nil || u.Scheme == "" || u.Host == "" {
			return err
		}
		return err
	}
	return nil
}

// ProviderHost returns the hostname of a provider base URL.
func ProviderHost(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" {
		return "", fmt.Errorf("invalid base URL")
	}
	return u.Hostname(), nil
}

// Complete performs a non-streaming chat completion.
func (c *Client) Complete(ctx context.Context, route *Route, req ChatRequest) (*ChatResponse, error) {
	if route == nil || route.BaseURL == "" || route.APIKey == "" {
		return nil, ErrNotConfigured
	}
	if err := ValidateProviderBaseURL(route.BaseURL, route.AllowDevLocal); err != nil {
		return nil, fmt.Errorf("inference: %w", err)
	}
	if req.Model == "" {
		req.Model = route.Model
	}
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL(route.BaseURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+route.APIKey)
	resp, err := c.http().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, egress.MaxResponseBytes))
	if resp.StatusCode >= 300 {
		// Do not echo upstream bodies (may contain secrets / prompt fragments).
		return nil, fmt.Errorf("inference: upstream HTTP %d", resp.StatusCode)
	}
	var out ChatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("inference: decode response")
	}
	return &out, nil
}

// StreamChunk is one streamed token delta.
type StreamChunk struct {
	Delta        string
	FinishReason string
	Model        string
	Done         bool
	ToolCalls    []ToolCall
}

// Stream performs SSE chat completion and invokes onChunk for each delta.
func (c *Client) Stream(ctx context.Context, route *Route, req ChatRequest, onChunk func(StreamChunk) error) error {
	if route == nil || route.BaseURL == "" || route.APIKey == "" {
		return ErrNotConfigured
	}
	if err := ValidateProviderBaseURL(route.BaseURL, route.AllowDevLocal); err != nil {
		return fmt.Errorf("inference: %w", err)
	}
	if req.Model == "" {
		req.Model = route.Model
	}
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL(route.BaseURL), bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+route.APIKey)
	httpReq.Header.Set("Accept", "text/event-stream")
	resp, err := c.http().Do(httpReq)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("inference: upstream HTTP %d", resp.StatusCode)
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	assembler := newToolCallAssembler()
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			if onChunk != nil {
				chunk := StreamChunk{Done: true, ToolCalls: assembler.Result()}
				_ = onChunk(chunk)
			}
			return nil
		}
		var payload struct {
			Model   string `json:"model"`
			Choices []struct {
				Delta struct {
					Content   string          `json:"content"`
					ToolCalls []toolCallDelta `json:"tool_calls"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			continue
		}
		chunk := StreamChunk{Model: payload.Model}
		if len(payload.Choices) > 0 {
			choice := payload.Choices[0]
			// Tool-call deltas are not token text.
			if choice.Delta.Content != "" {
				chunk.Delta = choice.Delta.Content
			}
			if len(choice.Delta.ToolCalls) > 0 {
				assembler.Add(choice.Delta.ToolCalls)
			}
			if choice.FinishReason != nil {
				chunk.FinishReason = *choice.FinishReason
			}
			if chunk.FinishReason == "tool_calls" {
				chunk.ToolCalls = assembler.Result()
			}
		}
		if onChunk != nil {
			if err := onChunk(chunk); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}

// TextContent extracts assistant text from a completion.
func TextContent(resp *ChatResponse) string {
	if resp == nil || len(resp.Choices) == 0 {
		return ""
	}
	return resp.Choices[0].Message.Content
}

// ResponseToolCalls extracts native tool_calls from a completion.
func ResponseToolCalls(resp *ChatResponse) []ToolCall {
	if resp == nil || len(resp.Choices) == 0 {
		return nil
	}
	return resp.Choices[0].Message.ToolCalls
}
