package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client handles communication with the local Ollama server.
type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

const (
	evaluatorSeed       = 42
	evaluatorThinkLevel = "low"
	ollamaMaxAttempts   = 3
	ollamaRetryDelay    = 250 * time.Millisecond
)

func evaluatorModelOptions() map[string]interface{} {
	return map[string]interface{}{
		"temperature": 0,
		"seed":        evaluatorSeed,
	}
}

// NewClient creates a new Ollama client.
func NewClient(baseURL, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// Chat sends a list of messages to Ollama and returns the model response.
func (c *Client) Chat(
	ctx context.Context,
	messages []Message,
	format interface{},
) (string, error) {
	return c.chat(ctx, messages, format, nil, nil)
}

// ChatWithOptions sends a chat request with optional Ollama model options.
func (c *Client) ChatWithOptions(
	ctx context.Context,
	messages []Message,
	format interface{},
	options map[string]interface{},
) (string, error) {
	return c.chat(ctx, messages, format, options, nil)
}

// ChatWithThinking sends a chat request with optional model options and an
// Ollama thinking level. Evaluators use low reasoning for faster, more stable
// structured output while generation keeps the model defaults.
func (c *Client) ChatWithThinking(
	ctx context.Context,
	messages []Message,
	format interface{},
	options map[string]interface{},
	think interface{},
) (string, error) {
	return c.chat(ctx, messages, format, options, think)
}

func (c *Client) chat(
	ctx context.Context,
	messages []Message,
	format interface{},
	options map[string]interface{},
	think interface{},
) (string, error) {
	requestBody := chatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
		Format:   format,
		Options:  options,
		Think:    think,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal Ollama request: %w", err)
	}

	for attempt := 1; attempt <= ollamaMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		result, retry, err := c.doChatRequest(ctx, body)
		if err == nil {
			return result, nil
		}

		if !retry {
			return "", err
		}
		if attempt == ollamaMaxAttempts {
			return "", fmt.Errorf(
				"Ollama request failed after %d attempt(s): %w",
				attempt,
				err,
			)
		}

		if err := waitForRetry(ctx, time.Duration(attempt)*ollamaRetryDelay); err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("Ollama request failed")
}

func (c *Client) doChatRequest(ctx context.Context, body []byte) (string, bool, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/chat",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", false, fmt.Errorf("create Ollama request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", false, ctx.Err()
		}

		// Network errors and the HTTP client's own timeout are transient from the
		// caller's perspective, so retry them with the same request payload.
		return "", true, fmt.Errorf("send Ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		err := fmt.Errorf(
			"Ollama returned HTTP %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(responseBody)),
		)

		// Retry server-side failures. 4xx errors are normally deterministic
		// request problems and should be surfaced immediately.
		return "", resp.StatusCode >= http.StatusInternalServerError, err
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// A truncated or malformed 2xx response can be transient as well.
		return "", true, fmt.Errorf("decode Ollama response: %w", err)
	}

	content := strings.TrimSpace(result.Message.Content)
	if content == "" {
		return "", true, fmt.Errorf(
			"Ollama returned empty content (thinking_length=%d, done_reason=%q)",
			len(result.Message.Thinking),
			result.DoneReason,
		)
	}

	return result.Message.Content, false, nil
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
