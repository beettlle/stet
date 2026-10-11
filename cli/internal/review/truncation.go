package review

import (
	"context"
	"fmt"
	"strings"

	"stet/cli/internal/findings"
	"stet/cli/internal/llm"
	"stet/cli/internal/ollama"
	"stet/cli/internal/trace"
)

// degenerateTailWindowBytes is how much of a truncated response's end
// isDegenerateTail inspects.
const degenerateTailWindowBytes = 2048

// degenerateMinIntegerRun is the shortest trailing run of comma-separated
// integers treated as a counting loop (e.g. "evidence_lines": [44,45,...,1865,).
const degenerateMinIntegerRun = 64

// degenerateMinPeriodBytes and degenerateMaxPeriodBytes bound the length of a
// repeated n-gram; degenerateMinRepeats is how many back-to-back copies at the
// end of the text count as a repetition loop.
const (
	degenerateMinPeriodBytes = 8
	degenerateMaxPeriodBytes = 64
	degenerateMinRepeats     = 8
)

// defaultCompletionCapTokens mirrors the openaicompat client's max_tokens
// default when GenerateOptions.MaxCompletionTokens is unset.
const defaultCompletionCapTokens = 4096

// hunkOutputBudgetMultiplier caps a hunk's total output tokens (first
// response, continuations, and the JSON-only retry) at this many times the
// per-request completion cap.
const hunkOutputBudgetMultiplier = 2

// minFollowUpTokens is the smallest remaining budget worth another request;
// below it (or below the cap, if the cap is smaller) no follow-up is sent.
const minFollowUpTokens = 256

// recoverTruncatedResponse handles a first response cut off at the output cap
// (finish reason "length"). It sends budget-limited continuation rounds unless
// the content is empty or ends in a repetition loop, merges the parts, and
// falls back to salvaging the finding objects that closed before the cut-off.
// Each request's usage is added to usage. A non-nil error means nothing was
// recovered and the caller may still try the JSON-only retry.
func recoverTruncatedResponse(ctx context.Context, client llm.Client, model, system, user, first string, generateOpts *ollama.GenerateOptions, budget *hunkOutputBudget, usage *HunkUsage, parseOpts []ParseOptions, traceOut *trace.Tracer) ([]findings.Finding, error) {
	tracing := traceOut != nil && traceOut.Enabled()
	parts := []string{first}
	skipReason := ""
	switch {
	case strings.TrimSpace(first) == "":
		skipReason = "empty content"
	case isDegenerateTail(first):
		skipReason = "degenerate tail (repetition loop)"
	}
	if skipReason != "" {
		if tracing {
			traceOut.Section("LLM continuation")
			traceOut.Printf("skipped: %s\n", skipReason)
		}
	} else {
		for round := 0; round < maxContinuationRounds; round++ {
			if !budget.canFollowUp() {
				if tracing {
					traceOut.Section("LLM continuation")
					traceOut.Printf("skipped round=%d: per-hunk output budget exhausted (%d of %d tokens)\n", round+1, budget.usedTokens, budget.totalTokens)
				}
				break
			}
			messages := []ollama.Message{
				{Role: "system", Content: system},
				{Role: "user", Content: user},
				{Role: "assistant", Content: parts[len(parts)-1]},
				{Role: "user", Content: "Continue."},
			}
			next, contErr := client.GenerateWithMessages(ctx, model, messages, budget.followUpOptions(generateOpts))
			if contErr != nil || next == nil {
				break
			}
			parts = append(parts, next.Response)
			budget.spend(next.EvalCount)
			usage.add(next)
			if tracing {
				traceOut.Section("LLM continuation")
				traceOut.Printf("round=%d done_reason=%s prompt_eval_count=%d eval_count=%d\n", round+1, next.DoneReason, next.PromptEvalCount, next.EvalCount)
				traceOut.Printf("%s\n", next.Response)
			}
			if next.DoneReason != "length" {
				break
			}
		}
		if list, err := MergeFindingsFromParts(parts, parseOpts...); err == nil {
			return list, nil
		}
	}
	candidates := []string{first}
	if len(parts) > 1 {
		candidates = []string{strings.Join(parts, ""), first}
	}
	for _, cand := range candidates {
		if list, ok := salvageTruncatedFindings(cand, parseOpts...); ok {
			if tracing {
				traceOut.Section("Salvage")
				traceOut.Printf("salvaged %d complete finding(s) from truncated response\n", len(list))
			}
			return list, nil
		}
	}
	return nil, fmt.Errorf("review: truncated response not recoverable")
}

// isDegenerateTail reports whether a truncated response ends in a repetition
// loop: a long run of comma-separated integers, or a short byte sequence
// repeated back-to-back. Continuing such a response only produces more of it.
func isDegenerateTail(s string) bool {
	tail := strings.TrimRight(s, " \t\r\n")
	if len(tail) > degenerateTailWindowBytes {
		tail = tail[len(tail)-degenerateTailWindowBytes:]
	}
	if trailingIntegerRun(tail) >= degenerateMinIntegerRun {
		return true
	}
	for period := degenerateMinPeriodBytes; period <= degenerateMaxPeriodBytes; period++ {
		if hasTrailingPeriod(tail, period, degenerateMinRepeats) {
			return true
		}
	}
	return false
}

// trailingIntegerRun counts the integers in the run of digits, commas, and
// whitespace at the end of s.
func trailingIntegerRun(s string) int {
	count := 0
	inDigits := false
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			if !inDigits {
				count++
				inDigits = true
			}
		case c == ',' || c == ' ' || c == '\t' || c == '\n' || c == '\r':
			inDigits = false
		default:
			return count
		}
	}
	return count
}

// hasTrailingPeriod reports whether the last period*repeats bytes of s are
// periodic with the given period (the same period-byte sequence repeated).
func hasTrailingPeriod(s string, period, repeats int) bool {
	span := period * repeats
	if len(s) < span {
		return false
	}
	start := len(s) - span
	for i := start + period; i < len(s); i++ {
		if s[i] != s[i-period] {
			return false
		}
	}
	return true
}

// salvageTruncatedFindings recovers the finding objects that close before a
// truncated JSON array is cut off, dropping the incomplete last one. ok is
// false when no complete finding object precedes the cut-off.
func salvageTruncatedFindings(s string, opts ...ParseOptions) ([]findings.Finding, bool) {
	start := findingsArrayStart(s)
	if start < 0 {
		return nil, false
	}
	var objects []string
	i := start + 1
	for len(objects) < maxSpanCandidates {
		for i < len(s) && (s[i] == ',' || s[i] == ' ' || s[i] == '\t' || s[i] == '\r' || s[i] == '\n') {
			i++
		}
		if i >= len(s) || s[i] != '{' {
			break
		}
		obj, ok := balancedJSONSpan(s, i)
		if !ok {
			break
		}
		objects = append(objects, obj)
		i += len(obj)
	}
	if len(objects) == 0 {
		return nil, false
	}
	list, err := parseFindingsStrict("["+strings.Join(objects, ",")+"]", opts...)
	if err != nil || len(list) == 0 {
		return nil, false
	}
	return list, true
}

// findingsArrayStart returns the index of the first '[' whose next
// non-whitespace byte opens an object, or -1.
func findingsArrayStart(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != '[' {
			continue
		}
		j := i + 1
		for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\r' || s[j] == '\n') {
			j++
		}
		if j < len(s) && s[j] == '{' {
			return i
		}
	}
	return -1
}

// hunkOutputBudget tracks output tokens spent on one hunk across the first
// response and every follow-up request.
type hunkOutputBudget struct {
	capTokens   int
	totalTokens int
	usedTokens  int
}

func newHunkOutputBudget(opts *ollama.GenerateOptions, firstEvalCount int) *hunkOutputBudget {
	capTokens := defaultCompletionCapTokens
	if opts != nil && opts.MaxCompletionTokens > 0 {
		capTokens = opts.MaxCompletionTokens
	}
	return &hunkOutputBudget{
		capTokens:   capTokens,
		totalTokens: hunkOutputBudgetMultiplier * capTokens,
		usedTokens:  max(0, firstEvalCount),
	}
}

func (b *hunkOutputBudget) remaining() int {
	return max(0, b.totalTokens-b.usedTokens)
}

// canFollowUp reports whether enough budget remains for another request.
func (b *hunkOutputBudget) canFollowUp() bool {
	return b.remaining() >= min(minFollowUpTokens, b.capTokens)
}

func (b *hunkOutputBudget) spend(evalCount int) {
	b.usedTokens += max(0, evalCount)
}

// followUpOptions returns opts with its completion cap limited to the
// remaining budget. Nil opts stay nil: Ollama sends zero-valued options
// verbatim, so synthesizing them would override server defaults.
func (b *hunkOutputBudget) followUpOptions(opts *ollama.GenerateOptions) *ollama.GenerateOptions {
	if opts == nil || b.remaining() >= b.capTokens {
		return opts
	}
	out := *opts
	out.MaxCompletionTokens = b.remaining()
	return &out
}
