// Package benchmark measures model throughput (tokens/s) for stet's review workload.
package benchmark

import (
	"context"
	"fmt"
	"math"

	"stet/cli/internal/llm"
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
	Model                      string  // Model name used
	PromptEvalCount            int     // Input tokens processed
	PromptEvalDurationNs       int64   // Time to process prompt (prefill)
	EvalCount                  int     // Output tokens generated
	EvalDurationNs             int64   // Time to generate output
	LoadDurationNs             int64   // Model load time (cold start)
	TotalDurationNs            int64   // Wall-clock time
	EvalRateTPS                float64 // Eval rate (tokens/s) for generation; end-to-end when EndToEndRate is set
	EndToEndRate               bool    // EvalRateTPS covers prefill plus generation (no prefill timing, e.g. OpenAI-compatible)
	PromptEvalRateTPS          float64 // Prompt eval rate (tokens/s) for prefill
	EstimatedSecPerTypicalHunk float64 // Estimated seconds for typical hunk (3k prompt, 200 out)
}

// Run executes a single benchmark run with a review-like prompt and returns metrics.
// opts may be nil (Ollama uses server/model defaults). The model must already exist.
func Run(ctx context.Context, client llm.Client, model string, opts *ollama.GenerateOptions) (*BenchmarkResult, error) {
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
	// Eval rate: tokens per second for generation.
	// Guard: skip when duration is zero to avoid division by zero.
	if r.EvalDuration > 0 && r.EvalCount > 0 {
		rate := float64(r.EvalCount) / (float64(r.EvalDuration) / 1e9)
		if !math.IsNaN(rate) && !math.IsInf(rate, 0) {
			out.EvalRateTPS = rate
		}
	}
	// OpenAI-compatible clients report only client wall clock (no prefill split),
	// so the eval rate spans prefill plus generation.
	out.EndToEndRate = r.PromptEvalDuration == 0 && r.EvalDuration > 0
	// Prompt eval rate: tokens per second for prefill.
	// Guard: skip when duration is zero to avoid division by zero.
	if r.PromptEvalDuration > 0 && r.PromptEvalCount > 0 {
		rate := float64(r.PromptEvalCount) / (float64(r.PromptEvalDuration) / 1e9)
		if !math.IsNaN(rate) && !math.IsInf(rate, 0) {
			out.PromptEvalRateTPS = rate
		}
	}
	// Estimated time for typical hunk: prompt_time + output_time
	out.EstimatedSecPerTypicalHunk = estimateHunkSeconds(out, TypicalPromptTokens, TypicalOutputTokens)
	return out
}

// estimateHunkSeconds estimates wall-clock seconds for one hunk with the given
// prompt and output token counts.
//
// Ollama-shaped results (EndToEndRate == false) report separate prefill and
// generation rates, so the estimate is promptTokens/PromptEvalRateTPS plus
// outputTokens/EvalRateTPS (each part only when its rate is positive).
//
// End-to-end results (EndToEndRate == true, e.g. OpenAI-compatible clients)
// report only the whole request time (EvalDurationNs) for PromptEvalCount
// input plus EvalCount output tokens. Model: split the measured request time
// between prompt and output in proportion to their token counts, then scale
// each part by typical/measured for its token kind
// (promptSec*typicalPrompt/measuredPrompt + outputSec*typicalOutput/measuredOutput).
// This makes the estimate grow with prompt size instead of ignoring it. Every
// division is guarded: zero counts or zero duration give 0, and a NaN or
// negative result collapses to 0.
func estimateHunkSeconds(r *BenchmarkResult, promptTokens, outputTokens int) float64 {
	est := 0.0
	if r.EndToEndRate {
		measuredSec := float64(r.EvalDurationNs) / 1e9
		measuredPrompt := float64(r.PromptEvalCount)
		measuredOutput := float64(r.EvalCount)
		total := measuredPrompt + measuredOutput
		if measuredSec > 0 && total > 0 {
			promptSec := measuredSec * measuredPrompt / total
			outputSec := measuredSec * measuredOutput / total
			if measuredPrompt > 0 {
				est += promptSec * float64(promptTokens) / measuredPrompt
			}
			if measuredOutput > 0 {
				est += outputSec * float64(outputTokens) / measuredOutput
			}
		}
	} else {
		if r.PromptEvalRateTPS > 0 {
			est += float64(promptTokens) / r.PromptEvalRateTPS
		}
		if r.EvalRateTPS > 0 {
			est += float64(outputTokens) / r.EvalRateTPS
		}
	}
	if math.IsNaN(est) || est < 0 {
		return 0
	}
	return est
}
