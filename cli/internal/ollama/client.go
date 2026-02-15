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
	"net/http"
	"regexp"
	"strings"
	"time"
)

const _defaultTimeout = 10 * time.Second

// ErrUnreachable indicates the Ollama server could not be reached (connection refused, timeout, or non-2xx).
var ErrUnreachable = errors.New("ollama server unreachable")

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

type tagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// Check verifies the server is reachable and whether the given model is present.
// It GETs /api/tags and parses the response. On connection/HTTP error returns ErrUnreachable (via %w).
func (c *Client) Check(ctx context.Context, model string) (*CheckResult, error) {
	url := c.baseURL + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ollama tags request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama tags: %w", errors.Join(ErrUnreachable, err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama tags: %w: HTTP %d", ErrUnreachable, resp.StatusCode)
	}
	var body tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
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
// context length when available. On connection/HTTP error returns an error
// (caller should use config). On success with no context found, returns
// ShowResult{ContextLength: 0}, nil.
func (c *Client) Show(ctx context.Context, model string) (*ShowResult, error) {
	body := showRequest{Model: model}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ollama show request: %w", err)
	}
	url := c.baseURL + "/api/show"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("ollama show request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama show: %w", errors.Join(ErrUnreachable, err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("ollama show: %w: HTTP %d", ErrUnreachable, resp.StatusCode)
	}
	var show showResponse
	if err := json.NewDecoder(resp.Body).Decode(&show); err != nil {
		return nil, fmt.Errorf("ollama show: parse response: %w", err)
	}
	ctxLen := parseContextLengthFromShow(show)
	return &ShowResult{ContextLength: ctxLen}, nil
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
type GenerateOptions struct {
	Temperature float64 `json:"temperature"`
	NumCtx      int     `json:"num_ctx"`
}

type generateRequest struct {
	Model   string           `json:"model"`
	System  string           `json:"system,omitempty"`
	Prompt  string           `json:"prompt"`
	Stream  bool             `json:"stream"`
	Format  string           `json:"format,omitempty"`
	Options *GenerateOptions `json:"options,omitempty"`
}

type generateResponse struct {
	Response        string `json:"response"`
	Done            bool   `json:"done"`
	Model           string `json:"model"`
	PromptEvalCount int    `json:"prompt_eval_count"`
	EvalCount       int    `json:"eval_count"`
	EvalDuration    int64  `json:"eval_duration"`
}

// GenerateResult holds the response text and metadata returned by Ollama /api/generate.
// Metadata fields may be zero when the server does not send them.
type GenerateResult struct {
	Response        string
	Model           string
	PromptEvalCount int
	EvalCount       int
	EvalDuration    int64
}

// Generate sends a completion request to /api/generate with the given model,
// system prompt, and user prompt. It uses stream: false and format: "json" so
// the response is a single JSON string. opts may be nil (Ollama uses server/model
// defaults). Returns (*GenerateResult, error): on success, result is non-nil; on
// error, result is nil and err wraps ErrUnreachable on connection/HTTP failure.
func (c *Client) Generate(ctx context.Context, model, systemPrompt, userPrompt string, opts *GenerateOptions) (*GenerateResult, error) {
	body := generateRequest{
		Model:   model,
		System:  systemPrompt,
		Prompt:  userPrompt,
		Stream:  false,
		Format:  "json",
		Options: opts,
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ollama generate request: %w", err)
	}
	url := c.baseURL + "/api/generate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("ollama generate request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama generate: %w", errors.Join(ErrUnreachable, err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("ollama generate: %w: HTTP %d", ErrUnreachable, resp.StatusCode)
	}
	var gen generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&gen); err != nil {
		return nil, fmt.Errorf("ollama generate: parse response: %w", err)
	}
	return &GenerateResult{
		Response:        gen.Response,
		Model:           gen.Model,
		PromptEvalCount: gen.PromptEvalCount,
		EvalCount:       gen.EvalCount,
		EvalDuration:    gen.EvalDuration,
	}, nil
}
