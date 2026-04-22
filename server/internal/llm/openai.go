package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const openAIChatEndpoint = "https://api.openai.com/v1/chat/completions"

// OpenAIClient is a minimal Chat Completions client. Phase 5 uses one
// retry on 5xx / timeout, no streaming, no batching, no caching.
type OpenAIClient struct {
	apiKey       string
	http         *http.Client
	defaultModel string
}

// NewOpenAIClient reads OPENAI_KEY (or OPENAI_API_KEY as a fallback)
// from env. Returns an error if missing so the server can decide
// whether to fatal or continue in degraded mode (civ endpoint 503s;
// ship endpoint unaffected).
func NewOpenAIClient() (*OpenAIClient, error) {
	key := os.Getenv("OPENAI_KEY")
	if key == "" {
		key = os.Getenv("OPENAI_API_KEY")
	}
	if key == "" {
		return nil, fmt.Errorf("OPENAI_KEY not set")
	}
	return &OpenAIClient{
		apiKey:       key,
		http:         &http.Client{Timeout: 60 * time.Second},
		defaultModel: "gpt-4o-mini",
	}, nil
}

func (c *OpenAIClient) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
	cfg := applyDefaults(callConfig{
		Model:       c.defaultModel,
		Temperature: 0.9,
		Timeout:     30 * time.Second,
	}, opts)

	body := chatRequest{
		Model:       cfg.Model,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
	}

	resp, err := c.postWithRetry(ctx, body, cfg.Timeout)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("llm: empty choices")
	}
	return resp.Choices[0].Message.Content, nil
}

func (c *OpenAIClient) CompleteJSON(ctx context.Context, prompt string, schema string, out any, opts ...Option) error {
	cfg := applyDefaults(callConfig{
		Model:       c.defaultModel,
		Temperature: 0.2,
		Timeout:     30 * time.Second,
	}, opts)

	// response_format json_schema requires a name and the schema object.
	rf := map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   "response",
			"strict": true,
			"schema": json.RawMessage(schema),
		},
	}

	body := chatRequest{
		Model:          cfg.Model,
		Temperature:    cfg.Temperature,
		MaxTokens:      cfg.MaxTokens,
		ResponseFormat: rf,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
	}

	resp, err := c.postWithRetry(ctx, body, cfg.Timeout)
	if err != nil {
		return err
	}
	if len(resp.Choices) == 0 {
		return fmt.Errorf("llm: empty choices")
	}
	raw := resp.Choices[0].Message.Content
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return fmt.Errorf("%w: %v (raw: %s)", ErrValidation, err, raw)
	}
	return nil
}

// postWithRetry POSTs the request once; on 5xx or context deadline,
// retries exactly once. 4xx bails immediately.
func (c *OpenAIClient) postWithRetry(ctx context.Context, body chatRequest, timeout time.Duration) (*chatResponse, error) {
	attempt := func() (*chatResponse, bool, error) {
		rctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		payload, err := json.Marshal(body)
		if err != nil {
			return nil, false, err
		}
		req, err := http.NewRequestWithContext(rctx, "POST", openAIChatEndpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, false, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, true, fmt.Errorf("%w: %v", ErrTransient, err)
		}
		defer resp.Body.Close()

		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 500 {
			return nil, true, fmt.Errorf("%w: status %d: %s", ErrTransient, resp.StatusCode, raw)
		}
		if resp.StatusCode >= 400 {
			return nil, false, fmt.Errorf("llm: status %d: %s", resp.StatusCode, raw)
		}

		var parsed chatResponse
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, false, fmt.Errorf("llm: parse response: %w", err)
		}
		return &parsed, false, nil
	}

	parsed, retryable, err := attempt()
	if err == nil {
		return parsed, nil
	}
	if !retryable {
		return nil, err
	}
	parsed, _, err = attempt()
	return parsed, err
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string        `json:"model"`
	Temperature    float64       `json:"temperature"`
	MaxTokens      int           `json:"max_tokens,omitempty"`
	Messages       []chatMessage `json:"messages"`
	ResponseFormat any           `json:"response_format,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}
