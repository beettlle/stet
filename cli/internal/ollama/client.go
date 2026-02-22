// Package ollama provides an HTTP client for the Ollama API (health check, model list).
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const _defaultTimeout = 10 * time.Second

const (
	_maxRetries     = 3
	_initialBackoff = 1 * time.Second
	_maxBackoff     = 16 * time.Second
)

// ErrUnreachable indicates the Ollama server could not be reached (connection refused, timeout, or 5xx).
var ErrUnreachable = errors.New("ollama server unreachable")

// ErrBadRequest indicates the server responded with 4xx (bad request, not found, etc.). Not retried.
var ErrBadRequest = errors.New("ollama bad request")

// Client calls the Ollama API. Zero value is not valid; use NewClient.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// CheckResult is the result of a health/model check.
type CheckResult struct {
	Reachable    bool     // Server responded with 200.
	ModelPresent bool     // Requested model name appears in the tags list.
	ModelNames   []string // All model names from /api/tags (for diagnostics).
}

// NewClient builds an Ollama client. baseURL is the API root (e.g. http://localhost:11434).
// If httpClient is nil, a default client with a 10s timeout is used.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: _defaultTimeout}
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &Client{baseURL: baseURL, httpClient: httpClient}
}

// httpStatusError returns the appropriate error for a non-2xx status. 4xx -> ErrBadRequest, 5xx -> ErrUnreachable.
func httpStatusError(prefix string, statusCode int) error {
	if statusCode >= 400 && statusCode < 500 {
		return fmt.Errorf("%s: %w: HTTP %d", prefix, ErrBadRequest, statusCode)
	}
	return fmt.Errorf("%s: %w: HTTP %d", prefix, ErrUnreachable, statusCode)
}

// sleepWithBackoff sleeps for the given attempt (0-based) with exponential backoff and ±15% jitter.
// Returns false if context was cancelled during sleep.
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

type tagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// Check verifies the server is reachable and whether the given model is present.
// It GETs /api/tags and parses the response. Retries on connection/5xx errors; 4xx returns ErrBadRequest.
func (c *Client) Check(ctx context.Context, model string) (*CheckResult, error) {
	url := c.baseURL + "/api/tags"
	var lastErr error
	for attempt := 0; attempt <= _maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("ollama tags: %w", ctx.Err())
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("ollama tags request: %w", err)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("ollama tags: %w", errors.Join(ErrUnreachable, err))
			if !errors.Is(lastErr, ErrUnreachable) || attempt == _maxRetries {
				return nil, lastErr
			}
			if !sleepWithBackoff(ctx, attempt) {
				return nil, fmt.Errorf("ollama tags: %w", ctx.Err())
			}
			continue
		}
		if resp == nil {
			return nil, fmt.Errorf("ollama tags: unexpected nil response")
		}
		if resp.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			lastErr = httpStatusError("ollama tags", resp.StatusCode)
			if errors.Is(lastErr, ErrBadRequest) || attempt == _maxRetries {
				return nil, lastErr
			}
			if !sleepWithBackoff(ctx, attempt) {
				return nil, fmt.Errorf("ollama tags: %w", ctx.Err())
			}
			continue
		}
		var body tagsResponse
		err = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("ollama tags: parse response: %w", err)
		}
		names := make([]string, 0, len(body.Models))
		for _, m := range body.Models {
			names = append(names, m.Name)
		}
		modelPresent := false
		for _, n := range names {
			if n == model {
				modelPresent = true
				break
			}
		}
		return &CheckResult{
			Reachable:    true,
			ModelPresent: modelPresent,
			ModelNames:   names,
		}, nil
	}
	return nil, lastErr
}

// ShowResult is the result of a model show request. ContextLength is the
// model's context size in tokens (0 if not found or on error).
type ShowResult struct {
	ContextLength int
}

type showRequest struct {
	Model string `json:"model"`
}

type showResponse struct {
	Parameters string                 `json:"parameters"`
	ModelInfo  map[string]interface{} `json:"model_info"`
}

// numCtxParamRegex matches "num_ctx 12345" in the parameters text.
var numCtxParamRegex = regexp.MustCompile(`\bnum_ctx\s+(\d+)\b`)

const maxContextLength = 524288 // cap parsed context to 512k tokens

// Show fetches model details from POST /api/show and returns the model's
// context length when available. Retries on connection/5xx errors; 4xx returns ErrBadRequest.
// On success with no context found, returns ShowResult{ContextLength: 0}, nil.
func (c *Client) Show(ctx context.Context, model string) (*ShowResult, error) {
	body := showRequest{Model: model}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ollama show request: %w", err)
	}
	url := c.baseURL + "/api/show"
	var lastErr error
	for attempt := 0; attempt <= _maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("ollama show: %w", ctx.Err())
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(encoded))
		if err != nil {
			return nil, fmt.Errorf("ollama show request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("ollama show: %w", errors.Join(ErrUnreachable, err))
			if !errors.Is(lastErr, ErrUnreachable) || attempt == _maxRetries {
				return nil, lastErr
			}
			if !sleepWithBackoff(ctx, attempt) {
				return nil, fmt.Errorf("ollama show: %w", ctx.Err())
			}
			continue
		}
		if resp == nil {
			return nil, fmt.Errorf("ollama show: unexpected nil response")
		}
		if resp.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			lastErr = httpStatusError("ollama show", resp.StatusCode)
			if errors.Is(lastErr, ErrBadRequest) || attempt == _maxRetries {
				return nil, lastErr
			}
			if !sleepWithBackoff(ctx, attempt) {
				return nil, fmt.Errorf("ollama show: %w", ctx.Err())
			}
			continue
		}
		var show showResponse
		err = json.NewDecoder(resp.Body).Decode(&show)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("ollama show: parse response: %w", err)
		}
		ctxLen := parseContextLengthFromShow(show)
		return &ShowResult{ContextLength: ctxLen}, nil
	}
	return nil, lastErr
}

// parseContextLengthFromShow extracts context length from model_info
// (*.context_length) or parameters (num_ctx N). Returns 0 if none found.
// Values are capped at maxContextLength.
func parseContextLengthFromShow(show showResponse) int {
	if show.ModelInfo != nil {
		var maxVal float64
		for k, v := range show.ModelInfo {
			if !strings.HasSuffix(k, ".context_length") {
				continue
			}
			switch n := v.(type) {
			case float64:
				if n > maxVal {
					maxVal = n
				}
			case int:
				if float64(n) > maxVal {
					maxVal = float64(n)
				}
			}
		}
		if maxVal > 0 {
			// Clamp to MaxInt to avoid overflow when converting float64 to int.
			if maxVal > float64(math.MaxInt) {
				maxVal = float64(math.MaxInt)
			}
			return capContextLength(int(maxVal))
		}
	}
	if show.Parameters != "" {
		if matches := numCtxParamRegex.FindStringSubmatch(show.Parameters); len(matches) >= 2 {
			var n int
			if _, err := fmt.Sscanf(matches[1], "%d", &n); err == nil && n > 0 {
				return capContextLength(n)
			}
		}
	}
	return 0
}

func capContextLength(n int) int {
	if n <= 0 {
		return 0
	}
	if n > maxContextLength {
		return maxContextLength
	}
	return n
}

// GenerateOptions holds model runtime options sent to Ollama /api/generate.
// Zero values are sent as-is; omitempty is not used so the API receives explicit values.
// KeepAlive is not sent inside options; it is sent at the top level of the request (see generateRequest).
type GenerateOptions struct {
	Temperature float64     `json:"temperature"`
	NumCtx      int         `json:"num_ctx"`
	KeepAlive   interface{} `json:"-"` // Top-level keep_alive; -1 = indefinitely, or string e.g. "60m"
}

type generateRequest struct {
	Model     string           `json:"model"`
	System    string           `json:"system,omitempty"`
	Prompt    string           `json:"prompt"`
	Stream    bool             `json:"stream"`
	Format    string           `json:"format,omitempty"`
	Options   *GenerateOptions `json:"options,omitempty"`
	KeepAlive interface{}      `json:"keep_alive,omitempty"` // How long to keep model loaded; -1 = indefinitely
}

type generateResponse struct {
	Response             string `json:"response"`
	Done                 bool   `json:"done"`
	Model                string `json:"model"`
	PromptEvalCount      int    `json:"prompt_eval_count"`
	PromptEvalDuration   int64  `json:"prompt_eval_duration"`
	EvalCount            int    `json:"eval_count"`
	EvalDuration         int64  `json:"eval_duration"`
	LoadDuration         int64  `json:"load_duration"`
	TotalDuration        int64  `json:"total_duration"`
}

// Usage holds token counts and durations from Ollama /api/generate for a single
// completion. Used for impact reporting (Phase 9) and history. Durations are in
// nanoseconds (Ollama API convention).
type Usage struct {
	PromptEvalCount    int   // Input tokens processed
	EvalCount          int   // Output tokens generated
	PromptEvalDuration int64 // Time to process prompt, ns
	EvalDuration       int64 // Time to generate output, ns
}

// GenerateResult holds the response text and metadata returned by Ollama /api/generate.
// Metadata fields may be zero when the server does not send them.
//
// Duration fields are in nanoseconds (Ollama API convention). Eval rate (tokens/s) =
// EvalCount / (EvalDuration / 1e9). Prompt eval rate = PromptEvalCount / (PromptEvalDuration / 1e9).
// LoadDuration is model load time (cold start); TotalDuration is wall-clock.
type GenerateResult struct {
	Response           string
	Model              string
	Usage              Usage // Token counts and eval durations from /api/generate
	PromptEvalCount    int   // Input tokens processed (mirrors Usage for backward compatibility)
	PromptEvalDuration int64 // Time to process prompt, ns
	EvalCount          int   // Output tokens generated
	EvalDuration       int64 // Time to generate output, ns
	LoadDuration       int64 // Model load time (cold start), ns
	TotalDuration      int64 // Wall-clock time, ns
}

// Generate sends a completion request to /api/generate with the given model,
// system prompt, and user prompt. It uses stream: false and format: "json" so
// the response is a single JSON string. opts may be nil (Ollama uses server/model
// defaults). Retries on connection/5xx errors; 4xx returns ErrBadRequest.
func (c *Client) Generate(ctx context.Context, model, systemPrompt, userPrompt string, opts *GenerateOptions) (*GenerateResult, error) {
	return c.generateWithFormat(ctx, model, systemPrompt, userPrompt, "json", opts)
}

// GeneratePlain sends a completion request to /api/generate with the given model,
// system prompt, and user prompt. It uses stream: false and no JSON format so the
// response is plain text (e.g. for commit messages). opts may be nil.
func (c *Client) GeneratePlain(ctx context.Context, model, systemPrompt, userPrompt string, opts *GenerateOptions) (*GenerateResult, error) {
	return c.generateWithFormat(ctx, model, systemPrompt, userPrompt, "", opts)
}

func (c *Client) generateWithFormat(ctx context.Context, model, systemPrompt, userPrompt, format string, opts *GenerateOptions) (*GenerateResult, error) {
	body := generateRequest{
		Model:   model,
		System:  systemPrompt,
		Prompt:  userPrompt,
		Stream:  false,
		Format:  format,
		Options: opts,
	}
	if opts != nil && opts.KeepAlive != nil {
		body.KeepAlive = opts.KeepAlive
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ollama generate request: %w", err)
	}
	url := c.baseURL + "/api/generate"
	var lastErr error
	for attempt := 0; attempt <= _maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("ollama generate: %w", ctx.Err())
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(encoded))
		if err != nil {
			return nil, fmt.Errorf("ollama generate request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("ollama generate: %w", errors.Join(ErrUnreachable, err))
			if !errors.Is(lastErr, ErrUnreachable) || attempt == _maxRetries {
				return nil, lastErr
			}
			if !sleepWithBackoff(ctx, attempt) {
				return nil, fmt.Errorf("ollama generate: %w", ctx.Err())
			}
			continue
		}
		if resp == nil {
			return nil, fmt.Errorf("ollama generate: unexpected nil response")
		}
		if resp.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			lastErr = httpStatusError("ollama generate", resp.StatusCode)
			if errors.Is(lastErr, ErrBadRequest) || attempt == _maxRetries {
				return nil, lastErr
			}
			if !sleepWithBackoff(ctx, attempt) {
				return nil, fmt.Errorf("ollama generate: %w", ctx.Err())
			}
			continue
		}
		var gen generateResponse
		err = json.NewDecoder(resp.Body).Decode(&gen)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("ollama generate: parse response: %w", err)
		}
		u := Usage{
			PromptEvalCount:    gen.PromptEvalCount,
			EvalCount:          gen.EvalCount,
			PromptEvalDuration: gen.PromptEvalDuration,
			EvalDuration:       gen.EvalDuration,
		}
		return &GenerateResult{
			Response:           gen.Response,
			Model:              gen.Model,
			Usage:              u,
			PromptEvalCount:    gen.PromptEvalCount,
			PromptEvalDuration: gen.PromptEvalDuration,
			EvalCount:          gen.EvalCount,
			EvalDuration:       gen.EvalDuration,
			LoadDuration:       gen.LoadDuration,
			TotalDuration:      gen.TotalDuration,
		}, nil
	}
	if lastErr == nil {
		lastErr = errors.New("ollama generate: no response after retries")
	}
	return nil, lastErr
}
