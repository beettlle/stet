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
	"time"

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

func TestClient_Check_success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" && r.URL.Path != "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{{"id": "gpt-4"}, {"id": "other"}},
		})
	}))
	defer srv.Close()
	client := NewClient(srv.URL+"/v1", srv.Client())
	got, err := client.Check(context.Background(), "gpt-4")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !got.Reachable || !got.ModelPresent {
		t.Errorf("Check: got %+v, want reachable with model present", got)
	}
	if len(got.ModelNames) != 2 {
		t.Errorf("ModelNames len = %d, want 2", len(got.ModelNames))
	}
}

func TestClient_Check_modelAbsent(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{{"id": "other"}},
		})
	}))
	defer srv.Close()
	client := NewClient(srv.URL+"/v1", srv.Client())
	got, err := client.Check(context.Background(), "missing")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !got.Reachable || got.ModelPresent {
		t.Errorf("Check: got %+v, want reachable without model", got)
	}
}

func TestClient_GeneratePlain_delegatesToGenerate(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "plain"}, "finish_reason": "stop"},
			},
		})
	}))
	defer srv.Close()
	client := NewClient(srv.URL+"/v1", srv.Client())
	got, err := client.GeneratePlain(context.Background(), "m", "sys", "user", nil)
	if err != nil {
		t.Fatalf("GeneratePlain: %v", err)
	}
	if got.Response != "plain" {
		t.Errorf("Response = %q, want plain", got.Response)
	}
}

func TestClient_GenerateWithMessages_success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "continued"}, "finish_reason": "stop"},
			},
			"usage": map[string]interface{}{"prompt_tokens": 5, "completion_tokens": 3},
		})
	}))
	defer srv.Close()
	client := NewClient(srv.URL+"/v1", srv.Client())
	got, err := client.GenerateWithMessages(context.Background(), "m", []ollama.Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}, nil)
	if err != nil {
		t.Fatalf("GenerateWithMessages: %v", err)
	}
	if got.Response != "continued" {
		t.Errorf("Response = %q", got.Response)
	}
	if got.PromptEvalCount != 5 || got.EvalCount != 3 {
		t.Errorf("usage: prompt=%d completion=%d", got.PromptEvalCount, got.EvalCount)
	}
}

func TestClient_GenerateWithMessages_emptyMessages(t *testing.T) {
	t.Parallel()
	client := NewClient("http://localhost/v1", nil)
	_, err := client.GenerateWithMessages(context.Background(), "m", nil, nil)
	if err == nil {
		t.Fatal("GenerateWithMessages: want error for empty messages")
	}
}

func TestClient_Check_retriesOn503(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{{"id": "m"}},
		})
	}))
	defer srv.Close()
	client := NewClient(srv.URL+"/v1", srv.Client())
	got, err := client.Check(context.Background(), "m")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !got.ModelPresent {
		t.Error("want model present after retry")
	}
	if calls < 2 {
		t.Errorf("want at least 2 calls (retry), got %d", calls)
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

func TestNewClient_trimsTrailingSlash(t *testing.T) {
	c := NewClient("http://localhost:1234/v1/", nil)
	if c.baseURL != "http://localhost:1234/v1" {
		t.Errorf("baseURL = %q", c.baseURL)
	}
}

func TestClient_Check_connectionRefused(t *testing.T) {
	t.Parallel()
	client := NewClient("http://127.0.0.1:1", &http.Client{Timeout: 100 * time.Millisecond})
	_, err := client.Check(context.Background(), "m")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ollama.ErrUnreachable) {
		t.Errorf("got %v", err)
	}
}

func TestClient_Generate_retriesOn503(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("busy"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "ok"}, "finish_reason": "stop"},
			},
		})
	}))
	defer srv.Close()
	client := NewClient(srv.URL, srv.Client())
	got, err := client.Generate(context.Background(), "m", "s", "u", nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got.Response != "ok" || calls < 2 {
		t.Errorf("calls=%d response=%q", calls, got.Response)
	}
}

func TestHttpStatusError_unreachableWithoutBody(t *testing.T) {
	err := httpStatusError("test", 503, "")
	if !errors.Is(err, ollama.ErrUnreachable) {
		t.Errorf("got %v", err)
	}
}

func TestModelsURL_withoutV1Suffix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []map[string]string{}})
	}))
	defer srv.Close()
	client := NewClient(srv.URL, srv.Client())
	if _, err := client.Check(context.Background(), "m"); err != nil {
		t.Fatalf("Check: %v", err)
	}
}
