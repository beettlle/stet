// Package openaicompat provides an HTTP client for OpenAI-compatible APIs
// (e.g. LM Studio). It returns ollama-shaped types so callers can use a
// single interface for either Ollama or OpenAI-compat backends.
package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"stet/cli/internal/ollama"
)

const (
	_defaultTimeout   = 10 * time.Second
	_maxRetries      = 3
	_initialBackoff  = 1 * time.Second
	_maxBackoff      = 16 * time.Second
	_maxResponseBytes = 10 * 1024 * 1024
)

// Client calls an OpenAI-compatible API. Zero value is not valid; use NewClient.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient builds a client. baseURL is the API root including /v1 if needed
// (e.g. http://localhost:1234/v1 for LM Studio). If httpClient is nil, a
// default client with 10s timeout is used.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: _defaultTimeout}
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &Client{baseURL: baseURL, httpClient: httpClient}
}

func modelsURL(baseURL string) string {
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + "/models"
	}
	return baseURL + "/v1/models"
}

func chatCompletionsURL(baseURL string) string {
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + "/chat/completions"
	}
	return baseURL + "/v1/chat/completions"
}

func httpStatusError(prefix string, statusCode int) error {
	if statusCode >= 400 && statusCode < 500 {
		return fmt.Errorf("%s: %w: HTTP %d", prefix, ollama.ErrBadRequest, statusCode)
	}
	return fmt.Errorf("%s: %w: HTTP %d", prefix, ollama.ErrUnreachable, statusCode)
}

func sleepWithBackoff(ctx context.Context, attempt int) bool {
	base := _initialBackoff * time.Duration(1<<attempt)
	if base > _maxBackoff {
		base = _maxBackoff
	}
	jitter := time.Duration(float64(base) * (0.15 * (2*rand.Float64() - 1)))
	d := base + jitter
	if d < 0 {
		d = 0
	}
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		return false
	}
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// Check verifies the server is reachable and whether the given model is present.
// It GETs /v1/models and parses the response. Retries on connection/5xx; 4xx returns ErrBadRequest.
func (c *Client) Check(ctx context.Context, model string) (*ollama.CheckResult, error) {
	url := modelsURL(c.baseURL)
	var lastErr error
	for attempt := 0; attempt <= _maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("openai models: %w", ctx.Err())
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("openai models request: %w", err)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("openai models: %w", errors.Join(ollama.ErrUnreachable, err))
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, lastErr
			}
			if attempt == _maxRetries {
				return nil, lastErr
			}
			if !sleepWithBackoff(ctx, attempt) {
				return nil, fmt.Errorf("openai models: %w", ctx.Err())
			}
			continue
		}
		if resp == nil {
			return nil, fmt.Errorf("openai models: unexpected nil response")
		}
		if resp.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			lastErr = httpStatusError("openai models", resp.StatusCode)
			if errors.Is(lastErr, ollama.ErrBadRequest) || attempt == _maxRetries {
				return nil, lastErr
			}
			if !sleepWithBackoff(ctx, attempt) {
				return nil, fmt.Errorf("openai models: %w", ctx.Err())
			}
			continue
		}
		var body modelsResponse
		err = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("openai models: parse response: %w", err)
		}
		names := make([]string, 0, len(body.Data))
		for _, m := range body.Data {
			if m.ID != "" {
				names = append(names, m.ID)
			}
		}
		modelPresent := false
		for _, n := range names {
			if n == model {
				modelPresent = true
				break
			}
		}
		return &ollama.CheckResult{
			Reachable:    true,
			ModelPresent: modelPresent,
			ModelNames:   names,
		}, nil
	}
	return nil, lastErr
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens    int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Generate sends a completion request to /v1/chat/completions and returns
// an ollama-shaped result. opts may be nil. Retries on connection/5xx; 4xx returns ErrBadRequest.
func (c *Client) Generate(ctx context.Context, model, systemPrompt, userPrompt string, opts *ollama.GenerateOptions) (*ollama.GenerateResult, error) {
	return c.generate(ctx, model, systemPrompt, userPrompt, opts)
}

// GeneratePlain is the same as Generate (OpenAI chat API does not distinguish format).
func (c *Client) GeneratePlain(ctx context.Context, model, systemPrompt, userPrompt string, opts *ollama.GenerateOptions) (*ollama.GenerateResult, error) {
	return c.generate(ctx, model, systemPrompt, userPrompt, opts)
}

// GenerateWithMessages sends a chat completion with the given message history (for continuation).
func (c *Client) GenerateWithMessages(ctx context.Context, model string, messages []ollama.Message, opts *ollama.GenerateOptions) (*ollama.GenerateResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("openai chat: messages required")
	}
	msgs := make([]message, len(messages))
	for i, m := range messages {
		msgs[i] = message{Role: m.Role, Content: m.Content}
	}
	temp := 0.2
	if opts != nil {
		temp = opts.Temperature
	}
	maxTokens := maxCompletionTokens(opts)
	body := chatRequest{
		Model:       model,
		Messages:    msgs,
		Temperature: temp,
		Stream:      false,
		MaxTokens:   maxTokens,
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai chat request: %w", err)
	}
	url := chatCompletionsURL(c.baseURL)
	var lastErr error
	for attempt := 0; attempt <= _maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("openai chat: %w", ctx.Err())
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(encoded))
		if err != nil {
			return nil, fmt.Errorf("openai chat request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("openai chat: %w", errors.Join(ollama.ErrUnreachable, err))
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, lastErr
			}
			if attempt == _maxRetries {
				return nil, lastErr
			}
			if !sleepWithBackoff(ctx, attempt) {
				return nil, fmt.Errorf("openai chat: %w", ctx.Err())
			}
			continue
		}
		if resp == nil {
			return nil, fmt.Errorf("openai chat: unexpected nil response")
		}
		if resp.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			lastErr = httpStatusError("openai chat", resp.StatusCode)
			if errors.Is(lastErr, ollama.ErrBadRequest) || attempt == _maxRetries {
				return nil, lastErr
			}
			if !sleepWithBackoff(ctx, attempt) {
				return nil, fmt.Errorf("openai chat: %w", ctx.Err())
			}
			continue
		}
		limited := io.LimitReader(resp.Body, _maxResponseBytes)
		var chat chatResponse
		err = json.NewDecoder(limited).Decode(&chat)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("openai chat: parse response: %w", err)
		}
		content := ""
		finishReason := ""
		if len(chat.Choices) > 0 {
			content = chat.Choices[0].Message.Content
			finishReason = chat.Choices[0].FinishReason
		}
		promptTokens := 0
		completionTokens := 0
		if chat.Usage != nil {
			promptTokens = chat.Usage.PromptTokens
			completionTokens = chat.Usage.CompletionTokens
		}
		return &ollama.GenerateResult{
			Response:           content,
			Model:              model,
			DoneReason:         finishReason,
			Usage:              ollama.Usage{PromptEvalCount: promptTokens, EvalCount: completionTokens},
			PromptEvalCount:    promptTokens,
			EvalCount:          completionTokens,
			PromptEvalDuration: 0,
			EvalDuration:       0,
			LoadDuration:       0,
			TotalDuration:      0,
		}, nil
	}
	return nil, lastErr
}

// maxCompletionTokens returns the Chat Completions max_tokens value. NumCtx is the model context
// window (Ollama num_ctx), not the completion cap; conflating them broke large --context values (e.g. 256k).
func maxCompletionTokens(opts *ollama.GenerateOptions) int {
	const defaultCap = 4096
	if opts != nil && opts.MaxCompletionTokens > 0 {
		return opts.MaxCompletionTokens
	}
	return defaultCap
}

func (c *Client) generate(ctx context.Context, model, systemPrompt, userPrompt string, opts *ollama.GenerateOptions) (*ollama.GenerateResult, error) {
	temp := 0.2
	if opts != nil {
		temp = opts.Temperature
	}
	maxTokens := maxCompletionTokens(opts)
	body := chatRequest{
		Model: model,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: temp,
		Stream:      false,
		MaxTokens:   maxTokens,
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai chat request: %w", err)
	}
	url := chatCompletionsURL(c.baseURL)
	var lastErr error
	for attempt := 0; attempt <= _maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("openai chat: %w", ctx.Err())
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(encoded))
		if err != nil {
			return nil, fmt.Errorf("openai chat request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("openai chat: %w", errors.Join(ollama.ErrUnreachable, err))
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, lastErr
			}
			if attempt == _maxRetries {
				return nil, lastErr
			}
			if !sleepWithBackoff(ctx, attempt) {
				return nil, fmt.Errorf("openai chat: %w", ctx.Err())
			}
			continue
		}
		if resp == nil {
			return nil, fmt.Errorf("openai chat: unexpected nil response")
		}
		if resp.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			lastErr = httpStatusError("openai chat", resp.StatusCode)
			if errors.Is(lastErr, ollama.ErrBadRequest) || attempt == _maxRetries {
				return nil, lastErr
			}
			if !sleepWithBackoff(ctx, attempt) {
				return nil, fmt.Errorf("openai chat: %w", ctx.Err())
			}
			continue
		}
		limited := io.LimitReader(resp.Body, _maxResponseBytes)
		var chat chatResponse
		err = json.NewDecoder(limited).Decode(&chat)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("openai chat: parse response: %w", err)
		}
		content := ""
		finishReason := ""
		if len(chat.Choices) > 0 {
			content = chat.Choices[0].Message.Content
			finishReason = chat.Choices[0].FinishReason
		}
		promptTokens := 0
		completionTokens := 0
		if chat.Usage != nil {
			promptTokens = chat.Usage.PromptTokens
			completionTokens = chat.Usage.CompletionTokens
		}
		return &ollama.GenerateResult{
			Response:           content,
			Model:              model,
			DoneReason:         finishReason,
			Usage:              ollama.Usage{PromptEvalCount: promptTokens, EvalCount: completionTokens},
			PromptEvalCount:    promptTokens,
			EvalCount:           completionTokens,
			PromptEvalDuration: 0,
			EvalDuration:       0,
			LoadDuration:       0,
			TotalDuration:      0,
		}, nil
	}
	return nil, lastErr
}
