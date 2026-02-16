// Package benchmark measures model throughput (tokens/s) for stet's review workload.
package benchmark

import (
	"context"
	"fmt"
	"math"

	"stet/cli/internal/ollama"
	"stet/cli/internal/prompt"
)

// TypicalPromptTokens and TypicalOutputTokens are used to estimate time per hunk.
const (
	TypicalPromptTokens = 3000
	TypicalOutputTokens = 200
)

// benchmarkUserPrompt is a realistic diff hunk that exercises code review
// (logic, potential nil check, style). Yields ~200-400 output tokens.
const benchmarkUserPrompt = `File: internal/auth/auth.go

@@ -45,12 +45,14 @@ func ValidateToken(ctx context.Context, token string) (*User, error) {
 	if token == "" {
 		return nil, ErrInvalidToken
 	}
 	u, err := store.LookupByToken(ctx, token)
 	if err != nil {
 		return nil, err
 	}
+	if u == nil {
+		return nil, ErrInvalidToken
+	}
 	if u.ExpiresAt != nil && u.ExpiresAt.Before(time.Now()) {
 		return nil, ErrTokenExpired
 	}
 	return u, nil
`

// BenchmarkResult holds raw and derived metrics from a benchmark run.
type BenchmarkResult struct {
	Model                    string  // Model name used
	PromptEvalCount          int     // Input tokens processed
	PromptEvalDurationNs     int64   // Time to process prompt (prefill)
	EvalCount                int     // Output tokens generated
	EvalDurationNs           int64   // Time to generate output
	LoadDurationNs           int64   // Model load time (cold start)
	TotalDurationNs          int64   // Wall-clock time
	EvalRateTPS              float64 // Eval rate (tokens/s) for generation
	PromptEvalRateTPS        float64 // Prompt eval rate (tokens/s) for prefill
	EstimatedSecPerTypicalHunk float64 // Estimated seconds for typical hunk (3k prompt, 200 out)
}

// Run executes a single benchmark run with a review-like prompt and returns metrics.
// opts may be nil (Ollama uses server/model defaults). The model must already exist.
func Run(ctx context.Context, client *ollama.Client, model string, opts *ollama.GenerateOptions) (*BenchmarkResult, error) {
	system := prompt.DefaultSystemPrompt
	user := benchmarkUserPrompt
	result, err := client.Generate(ctx, model, system, user, opts)
	if err != nil {
		return nil, fmt.Errorf("benchmark generate: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("benchmark generate: unexpected nil result")
	}
	return resultToBenchmark(result), nil
}

func resultToBenchmark(r *ollama.GenerateResult) *BenchmarkResult {
	out := &BenchmarkResult{
		Model:                r.Model,
		PromptEvalCount:      r.PromptEvalCount,
		PromptEvalDurationNs: r.PromptEvalDuration,
		EvalCount:            r.EvalCount,
		EvalDurationNs:       r.EvalDuration,
		LoadDurationNs:       r.LoadDuration,
		TotalDurationNs:      r.TotalDuration,
	}
	// Eval rate: tokens per second for generation
	if r.EvalDuration > 0 && r.EvalCount > 0 {
		out.EvalRateTPS = float64(r.EvalCount) / (float64(r.EvalDuration) / 1e9)
	}
	// Prompt eval rate: tokens per second for prefill
	if r.PromptEvalDuration > 0 && r.PromptEvalCount > 0 {
		out.PromptEvalRateTPS = float64(r.PromptEvalCount) / (float64(r.PromptEvalDuration) / 1e9)
	}
	// Estimated time for typical hunk: prompt_time + output_time
	if out.PromptEvalRateTPS > 0 {
		out.EstimatedSecPerTypicalHunk += float64(TypicalPromptTokens) / out.PromptEvalRateTPS
	}
	if out.EvalRateTPS > 0 {
		out.EstimatedSecPerTypicalHunk += float64(TypicalOutputTokens) / out.EvalRateTPS
	}
	if math.IsNaN(out.EstimatedSecPerTypicalHunk) || out.EstimatedSecPerTypicalHunk < 0 {
		out.EstimatedSecPerTypicalHunk = 0
	}
	return out
}
