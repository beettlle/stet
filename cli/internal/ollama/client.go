// Package ollama provides an HTTP client for the Ollama API (health check, model list).
package ollama

import (
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

type generateRequest struct {
	Model  string `json:"model"`
	System string `json:"system,omitempty"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Format string `json:"format,omitempty"`
}

type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Generate sends a completion request to /api/generate with the given model,
// system prompt, and user prompt. It uses stream: false and format: "json" so
// the response is a single JSON string. Returns the response text or an error
// (wrapping ErrUnreachable on connection/HTTP failure).
func (c *Client) Generate(ctx context.Context, model, systemPrompt, userPrompt string) (string, error) {
	body := generateRequest{
		Model:  model,
		System: systemPrompt,
		Prompt: userPrompt,
		Stream: false,
		Format: "json",
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("ollama generate request: %w", err)
	}
	url := c.baseURL + "/api/generate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		return "", fmt.Errorf("ollama generate request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama generate: %w", errors.Join(ErrUnreachable, err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("ollama generate: %w: HTTP %d", ErrUnreachable, resp.StatusCode)
	}
	var gen generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&gen); err != nil {
		return "", fmt.Errorf("ollama generate: parse response: %w", err)
	}
	return gen.Response, nil
}
