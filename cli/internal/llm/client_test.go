package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"stet/cli/internal/ollama"
)

func TestNewClient_ollama(t *testing.T) {
	t.Parallel()
	c, err := NewClient("ollama", "http://localhost:11434", nil)
	if err != nil {
		t.Fatalf("NewClient(ollama): %v", err)
	}
	if c == nil {
		t.Fatal("NewClient: want non-nil client")
	}
}

func TestNewClient_openai(t *testing.T) {
	t.Parallel()
	c, err := NewClient("openai", "http://localhost:1234/v1", nil)
	if err != nil {
		t.Fatalf("NewClient(openai): %v", err)
	}
	if c == nil {
		t.Fatal("NewClient: want non-nil client")
	}
}

func TestNewClient_normalizesProviderCase(t *testing.T) {
	t.Parallel()
	c, err := NewClient("  OLLAMA  ", "http://localhost:11434", nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c == nil {
		t.Fatal("want non-nil client")
	}
}

func TestNewClient_unsupportedProvider(t *testing.T) {
	t.Parallel()
	_, err := NewClient("anthropic", "http://example.com", nil)
	if err == nil {
		t.Fatal("NewClient: want error for unsupported provider")
	}
}

func TestNewClient_ollamaDelegatesToBackend(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":[{"name":"m"}]}`))
	}))
	defer srv.Close()

	c, err := NewClient("ollama", srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	got, err := c.Check(context.Background(), "m")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !got.Reachable || !got.ModelPresent {
		t.Errorf("Check: got %+v, want reachable with model present", got)
	}
}

func TestErrReexports(t *testing.T) {
	t.Parallel()
	if ErrUnreachable != ollama.ErrUnreachable {
		t.Error("ErrUnreachable should re-export ollama.ErrUnreachable")
	}
	if ErrBadRequest != ollama.ErrBadRequest {
		t.Error("ErrBadRequest should re-export ollama.ErrBadRequest")
	}
}
