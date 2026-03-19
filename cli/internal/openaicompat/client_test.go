package openaicompat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
