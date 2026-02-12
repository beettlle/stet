// Package ollama provides an HTTP client for the Ollama API (health check, model list).
package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
