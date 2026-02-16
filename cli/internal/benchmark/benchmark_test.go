package benchmark

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"stet/cli/internal/ollama"
)

func TestRun_returnsMetricsFromMockServer(t *testing.T) {
	// Do not run in parallel: avoids potential race with httptest.Server.
	wantEvalCount := 312
	wantEvalDur := int64(8000000000)   // 8s
	wantPromptEvalCount := 2847
	wantPromptEvalDur := int64(1500000000) // 1.5s
	wantLoadDur := int64(1200000000)       // 1.2s
	wantTotalDur := int64(10700000000)     // 10.7s
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" || r.Method != http.MethodPost {
			t.Errorf("path = %q method = %q", r.URL.Path, r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"response":             `[]`,
			"done":                 true,
			"model":                "qwen3-coder:30b",
			"prompt_eval_count":    wantPromptEvalCount,
			"prompt_eval_duration": wantPromptEvalDur,
			"eval_count":           wantEvalCount,
			"eval_duration":        wantEvalDur,
			"load_duration":        wantLoadDur,
			"total_duration":       wantTotalDur,
		})
	}))
	defer srv.Close()
	client := ollama.NewClient(srv.URL, srv.Client())
	ctx := context.Background()

	got, err := Run(ctx, client, "qwen3-coder:30b", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got == nil {
		t.Fatal("Run: want non-nil result")
	}
	if got.Model != "qwen3-coder:30b" {
		t.Errorf("Model = %q, want qwen3-coder:30b", got.Model)
	}
	if got.PromptEvalCount != wantPromptEvalCount {
		t.Errorf("PromptEvalCount = %d, want %d", got.PromptEvalCount, wantPromptEvalCount)
	}
	if got.EvalCount != wantEvalCount {
		t.Errorf("EvalCount = %d, want %d", got.EvalCount, wantEvalCount)
	}
	// Eval rate = 312 / 8 = 39 t/s
	wantEvalRate := float64(wantEvalCount) / (float64(wantEvalDur) / 1e9)
	if got.EvalRateTPS < wantEvalRate-0.1 || got.EvalRateTPS > wantEvalRate+0.1 {
		t.Errorf("EvalRateTPS = %.2f, want ~%.2f", got.EvalRateTPS, wantEvalRate)
	}
	// Prompt eval rate = 2847 / 1.5 = 1898 t/s
	wantPromptRate := float64(wantPromptEvalCount) / (float64(wantPromptEvalDur) / 1e9)
	if got.PromptEvalRateTPS < wantPromptRate-1 || got.PromptEvalRateTPS > wantPromptRate+1 {
		t.Errorf("PromptEvalRateTPS = %.2f, want ~%.2f", got.PromptEvalRateTPS, wantPromptRate)
	}
	if got.LoadDurationNs != wantLoadDur {
		t.Errorf("LoadDurationNs = %d, want %d", got.LoadDurationNs, wantLoadDur)
	}
	if got.TotalDurationNs != wantTotalDur {
		t.Errorf("TotalDurationNs = %d, want %d", got.TotalDurationNs, wantTotalDur)
	}
	// Estimated: 3000/1898 + 200/39 ≈ 1.58 + 5.13 ≈ 6.7s
	if got.EstimatedSecPerTypicalHunk < 5 || got.EstimatedSecPerTypicalHunk > 10 {
		t.Errorf("EstimatedSecPerTypicalHunk = %.2f, want ~6-8", got.EstimatedSecPerTypicalHunk)
	}
}

func TestRun_handlesZeroPromptEvalDuration(t *testing.T) {
	// Do not run in parallel: avoids potential race with httptest.Server.
	// When prompt_eval_duration is zero, PromptEvalRateTPS should be 0; EstimatedSecPerTypicalHunk uses only eval rate.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"response":             `[]`,
			"done":                 true,
			"model":                "test",
			"prompt_eval_count":    100,
			"prompt_eval_duration": 0,
			"eval_count":           50,
			"eval_duration":        1000000000,
		})
	}))
	defer srv.Close()
	client := ollama.NewClient(srv.URL, srv.Client())
	ctx := context.Background()

	got, err := Run(ctx, client, "test", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.PromptEvalRateTPS != 0 {
		t.Errorf("PromptEvalRateTPS = %.2f when prompt_eval_duration=0, want 0", got.PromptEvalRateTPS)
	}
	// Eval rate should still be computed: 50 / 1 = 50 t/s
	if got.EvalRateTPS < 49 || got.EvalRateTPS > 51 {
		t.Errorf("EvalRateTPS = %.2f, want ~50", got.EvalRateTPS)
	}
	// Estimated: only output part (3000/promptRate is 0 when promptRate=0, 200/50=4)
	if got.EstimatedSecPerTypicalHunk < 3.5 || got.EstimatedSecPerTypicalHunk > 5 {
		t.Errorf("EstimatedSecPerTypicalHunk = %.2f, want ~4", got.EstimatedSecPerTypicalHunk)
	}
}

func TestRun_handlesZeroLoadDuration(t *testing.T) {
	// Do not run in parallel: when load_duration is omitted/zero, should not crash.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"response":             `[]`,
			"done":                 true,
			"model":                "test",
			"prompt_eval_count":    100,
			"prompt_eval_duration": 50000000,
			"eval_count":           42,
			"eval_duration":        500000000,
		})
	}))
	defer srv.Close()
	client := ollama.NewClient(srv.URL, srv.Client())
	ctx := context.Background()

	got, err := Run(ctx, client, "test", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.LoadDurationNs != 0 {
		t.Errorf("LoadDurationNs = %d when omitted, want 0", got.LoadDurationNs)
	}
	if got.EvalRateTPS < 80 || got.EvalRateTPS > 86 {
		t.Errorf("EvalRateTPS = %.2f, want ~84", got.EvalRateTPS)
	}
}
