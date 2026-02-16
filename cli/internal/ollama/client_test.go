package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"syscall"
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
		wantBadRequest  bool
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
			// Package contract: 4xx (including 404) returns ErrBadRequest.
			name:            "404",
			status:          http.StatusNotFound,
			body:            "",
			model:           "any",
			wantReachable:   false,
			wantPresent:     false,
			wantErr:         true,
			wantBadRequest:  true,
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
				if tt.wantBadRequest && !errors.Is(err, ErrBadRequest) {
					t.Errorf("error should wrap ErrBadRequest: %v", err)
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
	if got == nil {
		t.Fatal("Generate: want non-nil result, got nil")
	}
	if got.Response != wantResponse {
		t.Errorf("response = %q, want %q", got.Response, wantResponse)
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
	if got == nil {
		t.Fatal("Generate: want non-nil result, got nil")
	}
	if got.Response != wantResponse {
		t.Errorf("response = %q, want %q", got.Response, wantResponse)
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
	result, err := client.Generate(ctx, "m", "sys", "user", nil)
	if err == nil {
		t.Fatal("Generate: want error, got nil")
	}
	if result != nil {
		t.Error("Generate: want nil result on error")
	}
	if !errors.Is(err, ErrUnreachable) {
		t.Errorf("error should wrap ErrUnreachable: %v", err)
	}
}

func TestClient_Generate_returnsResponseMetadata(t *testing.T) {
	// Do not run in parallel: avoids potential race with httptest.Server.
	wantResponse := `[]`
	wantModel := "qwen3-coder:30b"
	wantPromptEval := 100
	wantPromptEvalDur := int64(50000000)
	wantEval := 42
	wantEvalDur := int64(500000000)
	wantLoadDur := int64(1200000000)
	wantTotalDur := int64(1750000000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"response":               wantResponse,
			"done":                   true,
			"model":                  wantModel,
			"prompt_eval_count":      wantPromptEval,
			"prompt_eval_duration":   wantPromptEvalDur,
			"eval_count":             wantEval,
			"eval_duration":          wantEvalDur,
			"load_duration":          wantLoadDur,
			"total_duration":         wantTotalDur,
		})
	}))
	defer srv.Close()
	client := NewClient(srv.URL, srv.Client())
	ctx := context.Background()
	got, err := client.Generate(ctx, "m", "sys", "user", nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got == nil {
		t.Fatal("Generate: want non-nil result, got nil")
	}
	if got.Response != wantResponse {
		t.Errorf("Response = %q, want %q", got.Response, wantResponse)
	}
	if got.Model != wantModel {
		t.Errorf("Model = %q, want %q", got.Model, wantModel)
	}
	if got.PromptEvalCount != wantPromptEval {
		t.Errorf("PromptEvalCount = %d, want %d", got.PromptEvalCount, wantPromptEval)
	}
	if got.PromptEvalDuration != wantPromptEvalDur {
		t.Errorf("PromptEvalDuration = %d, want %d", got.PromptEvalDuration, wantPromptEvalDur)
	}
	if got.EvalCount != wantEval {
		t.Errorf("EvalCount = %d, want %d", got.EvalCount, wantEval)
	}
	if got.EvalDuration != wantEvalDur {
		t.Errorf("EvalDuration = %d, want %d", got.EvalDuration, wantEvalDur)
	}
	if got.LoadDuration != wantLoadDur {
		t.Errorf("LoadDuration = %d, want %d", got.LoadDuration, wantLoadDur)
	}
	if got.TotalDuration != wantTotalDur {
		t.Errorf("TotalDuration = %d, want %d", got.TotalDuration, wantTotalDur)
	}
}

func TestClient_Show_modelInfo_returnsContextLength(t *testing.T) {
	t.Parallel()
	body := `{"parameters":"","model_info":{"gemma3.context_length":131072,"gemma3.attention.head_count":8}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/show" || r.Method != http.MethodPost {
			t.Errorf("path = %q method = %q", r.URL.Path, r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	client := NewClient(srv.URL, srv.Client())
	ctx := context.Background()
	got, err := client.Show(ctx, "gemma3")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if got == nil {
		t.Fatal("Show: want non-nil result, got nil")
	}
	if got.ContextLength != 131072 {
		t.Errorf("ContextLength = %d, want 131072", got.ContextLength)
	}
}

func TestClient_Show_parameters_fallback(t *testing.T) {
	t.Parallel()
	body := `{"parameters":"temperature 0.7\nnum_ctx 8192\n","model_info":{}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	client := NewClient(srv.URL, srv.Client())
	ctx := context.Background()
	got, err := client.Show(ctx, "m")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if got == nil {
		t.Fatal("Show: want non-nil result, got nil")
	}
	if got.ContextLength != 8192 {
		t.Errorf("ContextLength = %d, want 8192", got.ContextLength)
	}
}

func TestClient_Show_empty_returnsZero(t *testing.T) {
	t.Parallel()
	body := `{"parameters":"temperature 0.7","model_info":{"general.architecture":"llama"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	client := NewClient(srv.URL, srv.Client())
	ctx := context.Background()
	got, err := client.Show(ctx, "m")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if got == nil {
		t.Fatal("Show: want non-nil result, got nil")
	}
	if got.ContextLength != 0 {
		t.Errorf("ContextLength = %d, want 0", got.ContextLength)
	}
}

func TestClient_Show_nonNumeric_contextLength_ignored(t *testing.T) {
	t.Parallel()
	body := `{"parameters":"","model_info":{"llama.context_length":"invalid"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	client := NewClient(srv.URL, srv.Client())
	ctx := context.Background()
	got, err := client.Show(ctx, "m")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if got == nil {
		t.Fatal("Show: want non-nil result, got nil")
	}
	if got.ContextLength != 0 {
		t.Errorf("ContextLength = %d, want 0 (non-numeric ignored)", got.ContextLength)
	}
}

// TestClient_Show_httpError_returnsError verifies 4xx (e.g. 404) returns ErrBadRequest per package contract.
func TestClient_Show_httpError_returnsError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	client := NewClient(srv.URL, srv.Client())
	ctx := context.Background()
	got, err := client.Show(ctx, "m")
	if err == nil {
		t.Fatal("Show: want error, got nil")
	}
	if got != nil {
		t.Error("Show: want nil result on error")
	}
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("error should wrap ErrBadRequest (404): %v", err)
	}
}

func TestClient_Show_badJSON_returnsError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{`))
	}))
	defer srv.Close()
	client := NewClient(srv.URL, srv.Client())
	ctx := context.Background()
	_, err := client.Show(ctx, "m")
	if err == nil {
		t.Fatal("Show: want error on bad JSON, got nil")
	}
}

func TestClient_Show_capsLargeContext(t *testing.T) {
	t.Parallel()
	// maxContextLength is 524288; model reports 1e6
	body := `{"parameters":"","model_info":{"llama.context_length":1000000}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	client := NewClient(srv.URL, srv.Client())
	ctx := context.Background()
	got, err := client.Show(ctx, "m")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	wantCap := 524288
	if got.ContextLength != wantCap {
		t.Errorf("ContextLength = %d, want capped %d", got.ContextLength, wantCap)
	}
}

func TestClient_Check_5xxRetryThenSuccess(t *testing.T) {
	t.Parallel()
	var attempt atomic.Int32
	validBody := `{"models":[{"name":"m","modified_at":"2024-01-01T00:00:00Z","size":0,"digest":"","details":{}}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempt.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validBody))
	}))
	defer srv.Close()
	client := NewClient(srv.URL, srv.Client())
	ctx := context.Background()
	got, err := client.Check(ctx, "m")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !got.ModelPresent {
		t.Error("Check: want ModelPresent after retry")
	}
	if attempt.Load() < 3 {
		t.Errorf("expected at least 3 attempts after 503 retries, got %d", attempt.Load())
	}
}

func TestClient_Check_4xxNoRetry(t *testing.T) {
	t.Parallel()
	var attempt atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	client := NewClient(srv.URL, srv.Client())
	ctx := context.Background()
	_, err := client.Check(ctx, "any")
	if err == nil {
		t.Fatal("Check: want error on 400")
	}
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("error should wrap ErrBadRequest: %v", err)
	}
	if n := attempt.Load(); n != 1 {
		t.Errorf("4xx should not be retried: got %d requests", n)
	}
}

func TestClient_Check_parseErrorNoRetry(t *testing.T) {
	t.Parallel()
	var attempt atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{`))
	}))
	defer srv.Close()
	client := NewClient(srv.URL, srv.Client())
	ctx := context.Background()
	_, err := client.Check(ctx, "any")
	if err == nil {
		t.Fatal("Check: want error on invalid JSON")
	}
	if n := attempt.Load(); n != 1 {
		t.Errorf("parse error should not be retried: got %d requests", n)
	}
}

func TestClient_Check_contextCancelDuringRetry(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	wg.Add(1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		wg.Done()
	}))
	defer srv.Close()
	client := NewClient(srv.URL, srv.Client())
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		wg.Wait()
		cancel()
	}()
	_, err := client.Check(ctx, "any")
	if err == nil {
		t.Fatal("Check: want error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error should be context.Canceled (or wrap it): %v", err)
	}
}

func TestClient_Check_connectionRefusedRetry(t *testing.T) {
	t.Parallel()
	validBody := `{"models":[{"name":"m","modified_at":"2024-01-01T00:00:00Z","size":0,"digest":"","details":{}}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validBody))
	}))
	defer srv.Close()
	var attempt atomic.Int32
	failFor := int32(2)
	transport := &connectionRefusedRoundTripper{
		base:    http.DefaultTransport,
		attempt: &attempt,
		failFor: failFor,
	}
	client := NewClient(srv.URL, &http.Client{Transport: transport})
	ctx := context.Background()
	got, err := client.Check(ctx, "m")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !got.ModelPresent {
		t.Error("Check: want ModelPresent after retries")
	}
	if n := attempt.Load(); n < 3 {
		t.Errorf("expected at least 3 attempts after connection refused, got %d", n)
	}
}

// connectionRefusedRoundTripper fails the first failFor requests with connection refused,
// then forwards to base. Used to test retry on connection errors.
type connectionRefusedRoundTripper struct {
	base    http.RoundTripper
	attempt *atomic.Int32
	failFor int32
}

func (r *connectionRefusedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	n := r.attempt.Add(1)
	if n <= r.failFor {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: syscall.Errno(syscall.ECONNREFUSED)}
	}
	return r.base.RoundTrip(req)
}
