// Package llm provides a provider-agnostic LLM client interface and factory.
// Use NewClient(provider, baseURL, httpClient) to get a Client that talks to
// Ollama or an OpenAI-compatible server (e.g. LM Studio).
package llm

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"stet/cli/internal/ollama"
	"stet/cli/internal/openaicompat"
)

// Client is the interface for LLM backends (Ollama or OpenAI-compat).
// All methods use ollama-shaped types so callers do not depend on the provider.
type Client interface {
	Check(ctx context.Context, model string) (*ollama.CheckResult, error)
	Generate(ctx context.Context, model, systemPrompt, userPrompt string, opts *ollama.GenerateOptions) (*ollama.GenerateResult, error)
	GeneratePlain(ctx context.Context, model, systemPrompt, userPrompt string, opts *ollama.GenerateOptions) (*ollama.GenerateResult, error)
}

// ErrUnreachable and ErrBadRequest are re-exported from ollama so callers can
// use errors.Is(err, llm.ErrUnreachable) regardless of provider.
var (
	ErrUnreachable = ollama.ErrUnreachable
	ErrBadRequest  = ollama.ErrBadRequest
)

// NewClient returns a Client for the given provider and base URL.
// provider must be "ollama" or "openai". baseURL is the API root (e.g.
// http://localhost:11434 for Ollama, http://localhost:1234/v1 for LM Studio).
// httpClient may be nil to use a default 10s timeout client.
func NewClient(provider, baseURL string, httpClient *http.Client) (Client, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	switch provider {
	case "ollama":
		return ollama.NewClient(baseURL, httpClient), nil
	case "openai":
		return openaicompat.NewClient(baseURL, httpClient), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider %q (use ollama or openai)", provider)
	}
}
