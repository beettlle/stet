package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient_normalizesBaseURL(t *testing.T) {
	t.Parallel()
	c := NewClient("http://localhost:11434/", nil)
	if c.baseURL != "http://localhost:11434" {
		t.Errorf("baseURL = %q, want no trailing slash", c.baseURL)
	}
}

func TestClient_Check(t *testing.T) {
	t.Parallel()

	validWithModel := `{"models":[{"name":"qwen3-coder:30b","modified_at":"2024-01-01T00:00:00Z","size":0,"digest":"","details":{}}]}`
	validWithoutModel := `{"models":[{"name":"other:7b","modified_at":"2024-01-01T00:00:00Z","size":0,"digest":"","details":{}}]}`
	invalidJSON := `{`

	tests := []struct {
		name            string
		status          int
		body            string
		model           string
		wantReachable   bool
		wantPresent     bool
		wantErr         bool
		wantUnreachable bool
	}{
		{
			name:            "200_with_model",
			status:          http.StatusOK,
			body:            validWithModel,
			model:           "qwen3-coder:30b",
			wantReachable:   true,
			wantPresent:     true,
			wantErr:         false,
			wantUnreachable: false,
		},
		{
			name:            "200_without_model",
			status:          http.StatusOK,
			body:            validWithoutModel,
			model:           "qwen3-coder:30b",
			wantReachable:   true,
			wantPresent:     false,
			wantErr:         false,
			wantUnreachable: false,
		},
		{
			name:            "200_empty_models",
			status:          http.StatusOK,
			body:            `{"models":[]}`,
			model:           "any",
			wantReachable:   true,
			wantPresent:     false,
			wantErr:         false,
			wantUnreachable: false,
		},
		{
			name:            "200_invalid_json",
			status:          http.StatusOK,
			body:            invalidJSON,
			model:           "any",
			wantReachable:   false,
			wantPresent:     false,
			wantErr:         true,
			wantUnreachable: false,
		},
		{
			name:            "404",
			status:          http.StatusNotFound,
			body:            "",
			model:           "any",
			wantReachable:   false,
			wantPresent:     false,
			wantErr:         true,
			wantUnreachable: true,
		},
		{
			name:            "500",
			status:          http.StatusInternalServerError,
			body:            "",
			model:           "any",
			wantReachable:   false,
			wantPresent:     false,
			wantErr:         true,
			wantUnreachable: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/tags" {
					t.Errorf("path = %q, want /api/tags", r.URL.Path)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			client := NewClient(srv.URL, srv.Client())
			ctx := context.Background()
			got, err := client.Check(ctx, tt.model)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Check: want error, got nil")
				}
				if tt.wantUnreachable && !errors.Is(err, ErrUnreachable) {
					t.Errorf("error should wrap ErrUnreachable: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			if got.Reachable != tt.wantReachable {
				t.Errorf("Reachable = %v, want %v", got.Reachable, tt.wantReachable)
			}
			if got.ModelPresent != tt.wantPresent {
				t.Errorf("ModelPresent = %v, want %v", got.ModelPresent, tt.wantPresent)
			}
		})
	}
}

func TestClient_Check_connectionRefused(t *testing.T) {
	t.Parallel()
	// Bind and release a port so nothing is listening.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	client := NewClient("http://"+addr, nil)
	ctx := context.Background()
	_, err = client.Check(ctx, "any")
	if err == nil {
		t.Fatal("Check: want error on connection refused, got nil")
	}
	if !errors.Is(err, ErrUnreachable) {
		t.Errorf("error should wrap ErrUnreachable: %v", err)
	}
}

func TestClient_Generate(t *testing.T) {
	t.Parallel()
	wantResponse := `[{"file":"a.go","line":1,"severity":"warning","category":"style","message":"fix"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("path = %q, want /api/generate", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		var body struct {
			Model   string           `json:"model"`
			System  string           `json:"system"`
			Prompt  string           `json:"prompt"`
			Stream  bool             `json:"stream"`
			Format  string           `json:"format"`
			Options *GenerateOptions `json:"options"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.Model != "m1" {
			t.Errorf("model = %q, want m1", body.Model)
		}
		if body.System != "sys" {
			t.Errorf("system = %q, want sys", body.System)
		}
		if body.Prompt != "user" {
			t.Errorf("prompt = %q, want user", body.Prompt)
		}
		if body.Stream {
			t.Error("stream should be false")
		}
		if body.Format != "json" {
			t.Errorf("format = %q, want json", body.Format)
		}
		w.WriteHeader(http.StatusOK)
		out := map[string]interface{}{"response": wantResponse, "done": true}
		_ = json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	ctx := context.Background()
	got, err := client.Generate(ctx, "m1", "sys", "user", nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got != wantResponse {
		t.Errorf("response = %q, want %q", got, wantResponse)
	}
}

func TestClient_Generate_withOptions_sendsOptionsInRequest(t *testing.T) {
	t.Parallel()
	wantResponse := `[]`
	var receivedOpts *GenerateOptions
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model   string           `json:"model"`
			Options *GenerateOptions `json:"options"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		receivedOpts = body.Options
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": wantResponse, "done": true})
	}))
	defer srv.Close()
	client := NewClient(srv.URL, srv.Client())
	ctx := context.Background()
	opts := &GenerateOptions{Temperature: 0.3, NumCtx: 4096}
	got, err := client.Generate(ctx, "m", "sys", "user", opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got != wantResponse {
		t.Errorf("response = %q, want %q", got, wantResponse)
	}
	if receivedOpts == nil {
		t.Fatal("request body options should be non-nil")
	}
	if receivedOpts.Temperature != 0.3 {
		t.Errorf("options.Temperature = %f, want 0.3", receivedOpts.Temperature)
	}
	if receivedOpts.NumCtx != 4096 {
		t.Errorf("options.NumCtx = %d, want 4096", receivedOpts.NumCtx)
	}
}

func TestClient_Generate_nonOK_returnsError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	client := NewClient(srv.URL, srv.Client())
	ctx := context.Background()
	_, err := client.Generate(ctx, "m", "sys", "user", nil)
	if err == nil {
		t.Fatal("Generate: want error, got nil")
	}
	if !errors.Is(err, ErrUnreachable) {
		t.Errorf("error should wrap ErrUnreachable: %v", err)
	}
}
