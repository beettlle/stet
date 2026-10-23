// Tests for model throughput benchmarking (tokens/s measurement).
package benchmark

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"stet/cli/internal/llm"
	"stet/cli/internal/ollama"
)

func TestRun_returnsMetricsFromMockServer(t *testing.T) {
	// Do not run in parallel: avoids potential race with httptest.Server.
	wantEvalCount := 312
	wantEvalDur := int64(8000000000) // 8s
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
	client, _ := llm.NewClient("ollama", srv.URL, srv.Client())
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
	if got.EndToEndRate {
		t.Error("EndToEndRate = true for Ollama result with prefill timing, want false")
	}
	// Estimated: 3000/1898 + 200/39 ≈ 1.58 + 5.13 ≈ 6.7s
	if got.EstimatedSecPerTypicalHunk < 5 || got.EstimatedSecPerTypicalHunk > 10 {
		t.Errorf("EstimatedSecPerTypicalHunk = %.2f, want ~6-8", got.EstimatedSecPerTypicalHunk)
	}
}

func TestRun_handlesZeroPromptEvalDuration(t *testing.T) {
	// Do not run in parallel: avoids potential race with httptest.Server.
	// When prompt_eval_duration is zero, PromptEvalRateTPS should be 0 and the
	// result is end-to-end shaped (flag true): the estimate uses the measured
	// request time split by token counts (see estimateHunkSeconds).
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
	client, _ := llm.NewClient("ollama", srv.URL, srv.Client())
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
	if !got.EndToEndRate {
		t.Error("EndToEndRate = false with zero prompt_eval_duration, want true")
	}
	// Estimated via the end-to-end model: 1s measured for 100+50 tokens scales
	// to (3000+200)/150 ≈ 21.3s, so the 3k-token prompt is no longer ignored.
	if got.EstimatedSecPerTypicalHunk < 19 || got.EstimatedSecPerTypicalHunk > 23 {
		t.Errorf("EstimatedSecPerTypicalHunk = %.2f, want ~21.3", got.EstimatedSecPerTypicalHunk)
	}
}

func TestEstimateHunkSeconds_endToEndGrowsWithPromptTokens(t *testing.T) {
	t.Parallel()
	res := resultToBenchmark(&ollama.GenerateResult{
		PromptEvalCount: 500,
		EvalCount:       200,
		EvalDuration:    2_000_000_000, // whole request: 2s for 700 tokens
		TotalDuration:   2_000_000_000,
	})
	if !res.EndToEndRate {
		t.Fatal("EndToEndRate = false, want true for OpenAI-compatible shape")
	}
	small := estimateHunkSeconds(res, 1_000, TypicalOutputTokens)
	large := estimateHunkSeconds(res, 6_000, TypicalOutputTokens)
	if small <= 0 {
		t.Fatalf("estimate for 1000-token prompt = %.4f, want > 0", small)
	}
	if large <= small {
		t.Errorf("estimate with 6000-token prompt = %.4f, want > 1000-token prompt = %.4f", large, small)
	}
	// Documented model: est = measuredSec * (promptTokens + outputTokens) / measuredTokens.
	wantSmall := 2.0 * (1_000.0 + TypicalOutputTokens) / 700.0
	if math.Abs(small-wantSmall) > 0.01 {
		t.Errorf("small = %.4f, want ~%.4f", small, wantSmall)
	}
	wantLarge := 2.0 * (6_000.0 + TypicalOutputTokens) / 700.0
	if math.Abs(large-wantLarge) > 0.01 {
		t.Errorf("large = %.4f, want ~%.4f", large, wantLarge)
	}
}

func TestEstimateHunkSeconds_ollamaShapeUnchanged(t *testing.T) {
	t.Parallel()
	res := resultToBenchmark(&ollama.GenerateResult{
		PromptEvalCount:    1000,
		PromptEvalDuration: 500_000_000, // 2000 t/s prefill
		EvalCount:          100,
		EvalDuration:       2_000_000_000, // 50 t/s generation
		TotalDuration:      2_500_000_000,
	})
	if res.EndToEndRate {
		t.Fatal("EndToEndRate = true, want false for Ollama-shaped result")
	}
	// Unchanged formula: 3000/2000 + 200/50 = 1.5 + 4 = 5.5s
	want := float64(TypicalPromptTokens)/res.PromptEvalRateTPS + float64(TypicalOutputTokens)/res.EvalRateTPS
	got := estimateHunkSeconds(res, TypicalPromptTokens, TypicalOutputTokens)
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("estimate = %.6f, want %.6f (prompt/rate + output/rate formula)", got, want)
	}
	if res.EstimatedSecPerTypicalHunk != got {
		t.Errorf("EstimatedSecPerTypicalHunk = %.6f, want helper result %.6f", res.EstimatedSecPerTypicalHunk, got)
	}
}

func TestEstimateHunkSeconds_zeroCountsAndDurations(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		res  *BenchmarkResult
	}{
		{"end-to-end zero everything", &BenchmarkResult{EndToEndRate: true}},
		{"end-to-end zero counts", &BenchmarkResult{EndToEndRate: true, EvalDurationNs: 2_000_000_000}},
		{"end-to-end zero duration", &BenchmarkResult{EndToEndRate: true, PromptEvalCount: 500, EvalCount: 200}},
		{"ollama-shaped zero rates", &BenchmarkResult{}},
	}
	for _, tc := range cases {
		if got := estimateHunkSeconds(tc.res, TypicalPromptTokens, TypicalOutputTokens); got != 0 {
			t.Errorf("%s: estimate = %.4f, want 0 (no NaN)", tc.name, got)
		}
	}
	if v := resultToBenchmark(&ollama.GenerateResult{}).EstimatedSecPerTypicalHunk; v != 0 {
		t.Errorf("resultToBenchmark zero-result estimate = %.4f, want 0", v)
	}
}

func TestRun_handlesAllZeroDurations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"response":             `[]`,
			"done":                 true,
			"model":                "test",
			"prompt_eval_count":    0,
			"prompt_eval_duration": 0,
			"eval_count":           0,
			"eval_duration":        0,
		})
	}))
	defer srv.Close()
	client, _ := llm.NewClient("ollama", srv.URL, srv.Client())
	ctx := context.Background()

	got, err := Run(ctx, client, "test", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.EvalRateTPS != 0 {
		t.Errorf("EvalRateTPS = %.2f when all durations zero, want 0", got.EvalRateTPS)
	}
	if got.PromptEvalRateTPS != 0 {
		t.Errorf("PromptEvalRateTPS = %.2f when all durations zero, want 0", got.PromptEvalRateTPS)
	}
	if got.EstimatedSecPerTypicalHunk != 0 {
		t.Errorf("EstimatedSecPerTypicalHunk = %.2f when all durations zero, want 0", got.EstimatedSecPerTypicalHunk)
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
	client, _ := llm.NewClient("ollama", srv.URL, srv.Client())
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

func TestRun_openAICompatEndToEndRate(t *testing.T) {
	// Do not run in parallel: avoids potential race with httptest.Server.
	// Chat Completions carry no timings; the client wall clock yields an end-to-end rate.
	const delay = 25 * time.Millisecond
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "[]"}, "finish_reason": "stop"},
			},
			"usage": map[string]interface{}{"prompt_tokens": 500, "completion_tokens": 200},
		})
	}))
	defer srv.Close()
	client, err := llm.NewClient("openai", srv.URL+"/v1", srv.Client())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	got, err := Run(context.Background(), client, "local-model", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !got.EndToEndRate {
		t.Error("EndToEndRate = false, want true for OpenAI-compatible result")
	}
	if got.EvalRateTPS <= 0 {
		t.Errorf("EvalRateTPS = %.2f, want > 0", got.EvalRateTPS)
	}
	if got.TotalDurationNs < delay.Nanoseconds() || got.EvalDurationNs < delay.Nanoseconds() {
		t.Errorf("durations total=%d eval=%d, want >= %d", got.TotalDurationNs, got.EvalDurationNs, delay.Nanoseconds())
	}
	if got.PromptEvalRateTPS != 0 {
		t.Errorf("PromptEvalRateTPS = %.2f, want 0 (no prefill timing)", got.PromptEvalRateTPS)
	}
}

func TestResultToBenchmark_endToEndFlag(t *testing.T) {
	t.Parallel()
	e2e := resultToBenchmark(&ollama.GenerateResult{EvalCount: 100, EvalDuration: 2_000_000_000, TotalDuration: 2_000_000_000})
	if !e2e.EndToEndRate || e2e.EvalRateTPS < 49 || e2e.EvalRateTPS > 51 {
		t.Errorf("end-to-end: flag=%v rate=%.2f, want true ~50", e2e.EndToEndRate, e2e.EvalRateTPS)
	}
	ol := resultToBenchmark(&ollama.GenerateResult{
		PromptEvalCount: 1000, PromptEvalDuration: 500_000_000,
		EvalCount: 100, EvalDuration: 2_000_000_000,
	})
	if ol.EndToEndRate {
		t.Error("Ollama-shaped: EndToEndRate = true, want false")
	}
	if ol.EvalRateTPS < 49 || ol.EvalRateTPS > 51 || ol.PromptEvalRateTPS < 1999 || ol.PromptEvalRateTPS > 2001 {
		t.Errorf("Ollama-shaped rates eval=%.2f prompt=%.2f, want ~50 and ~2000", ol.EvalRateTPS, ol.PromptEvalRateTPS)
	}
	none := resultToBenchmark(&ollama.GenerateResult{})
	if none.EndToEndRate || none.EvalRateTPS != 0 {
		t.Errorf("zero durations: flag=%v rate=%.2f, want false 0", none.EndToEndRate, none.EvalRateTPS)
	}
}
