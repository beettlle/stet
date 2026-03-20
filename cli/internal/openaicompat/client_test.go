package openaicompat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stet/cli/internal/ollama"
)

func TestClient_Generate_doneReasonLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" && r.URL.Path != "/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message":  map[string]interface{}{"content": "[{\"file\":\"a.go\""},
					"finish_reason": "length",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 100,
			},
		})
	}))
	defer srv.Close()
	client := NewClient(srv.URL+"/v1", srv.Client())
	ctx := context.Background()
	got, err := client.Generate(ctx, "m", "sys", "user", nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got.DoneReason != "length" {
		t.Errorf("DoneReason = %q, want %q", got.DoneReason, "length")
	}
	if got.Response != "[{\"file\":\"a.go\"" {
		t.Errorf("Response = %q", got.Response)
	}
}

func TestGenerate_maxTokensFromMaxCompletionTokens_notNumCtx(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "ok"}, "finish_reason": "stop"},
			},
			"usage": map[string]interface{}{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer srv.Close()
	client := NewClient(srv.URL+"/v1", srv.Client())
	ctx := context.Background()
	// Large NumCtx must not set max_tokens (that was the 256k regression).
	_, err := client.Generate(ctx, "m", "sys", "user", &ollama.GenerateOptions{
		NumCtx:              262144,
		MaxCompletionTokens: 8192,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	mt, ok := gotBody["max_tokens"].(float64)
	if !ok || int(mt) != 8192 {
		t.Fatalf("max_tokens = %v, want 8192", gotBody["max_tokens"])
	}
}

func TestGenerate_maxTokensDefaultWhenMaxCompletionTokensZero(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "x"}, "finish_reason": "stop"},
			},
		})
	}))
	defer srv.Close()
	client := NewClient(srv.URL+"/v1", srv.Client())
	_, err := client.Generate(context.Background(), "m", "s", "u", &ollama.GenerateOptions{NumCtx: 262144})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	mt, ok := gotBody["max_tokens"].(float64)
	if !ok || int(mt) != 4096 {
		t.Fatalf("max_tokens = %v, want 4096 default", gotBody["max_tokens"])
	}
}

func TestCheck_modelsHTTP400IncludesBody(t *testing.T) {
	const wantSub = `{"error":{"message":"model not found"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(wantSub + "\n"))
	}))
	defer srv.Close()
	client := NewClient(srv.URL+"/v1", srv.Client())
	_, err := client.Check(context.Background(), "m")
	if err == nil {
		t.Fatal("Check: want error")
	}
	if !errors.Is(err, ollama.ErrBadRequest) {
		t.Errorf("errors.Is ErrBadRequest: %v", err)
	}
	s := err.Error()
	if !strings.Contains(s, "HTTP 400") || !strings.Contains(s, wantSub) {
		t.Fatalf("error should include status and body: %q", s)
	}
}

func TestGenerate_HTTP400includesBody(t *testing.T) {
	const wantSub = "context length exceeded"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"` + wantSub + `"}}`))
	}))
	defer srv.Close()
	client := NewClient(srv.URL+"/v1", srv.Client())
	_, err := client.Generate(context.Background(), "m", "sys", "user", nil)
	if err == nil {
		t.Fatal("Generate: want error")
	}
	if !errors.Is(err, ollama.ErrBadRequest) {
		t.Errorf("errors.Is ErrBadRequest: %v", err)
	}
	s := err.Error()
	if !strings.Contains(s, "HTTP 400") || !strings.Contains(s, wantSub) {
		t.Fatalf("error should include status and body: %q", s)
	}
}
