// Package review provides the per-hunk review pipeline and the optional
// critic (second-pass verification). This file implements the critic.
package review

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"stet/cli/internal/findings"
	"stet/cli/internal/ollama"
)

const (
	criticSystemPrompt = "You are a code review critic. Output only valid JSON."
	// Default max characters of hunk content to include in the critic prompt to keep prompts small.
	defaultCriticMaxHunkLen = 4096
)

// CriticOptions configures VerifyFinding. Zero value uses defaults.
type CriticOptions struct {
	// MaxHunkContentLen caps the hunk content in the prompt (0 = defaultCriticMaxHunkLen).
	MaxHunkContentLen int
	// RetryOnParseError when true retries the LLM call once on parse failure; then treat as no (drop finding).
	RetryOnParseError bool
	// KeepAlive, when set, is passed to Ollama GenerateOptions (e.g. -1 to keep model loaded). When nil, 0 is used so the critic model is unloaded after the call. Set when critic uses the same model as the main review to avoid unloading between hunks.
	KeepAlive interface{}
}

// verdictResponse is the JSON shape expected from the critic model.
type verdictResponse struct {
	Verdict string `json:"verdict"`
	Reason  string `json:"reason"`
}

// BuildCriticPrompt returns the user prompt for the critic: finding details plus code under review.
// maxHunkLen caps the hunk content (0 = defaultCriticMaxHunkLen). Callers may truncate hunkContent before calling.
func BuildCriticPrompt(f findings.Finding, hunkContent string, maxHunkLen int) string {
	if maxHunkLen <= 0 {
		maxHunkLen = defaultCriticMaxHunkLen
	}
	if len(hunkContent) > maxHunkLen {
		hunkContent = hunkContent[:maxHunkLen] + "\n[truncated]"
	}
	loc := fmt.Sprintf("line %d", f.Line)
	if f.Range != nil && f.Range.Start != f.Range.End {
		loc = fmt.Sprintf("lines %d-%d", f.Range.Start, f.Range.End)
	}
	var b strings.Builder
	b.WriteString("Finding:\n")
	b.WriteString("- file: ")
	b.WriteString(f.File)
	b.WriteString("\n- location: ")
	b.WriteString(loc)
	b.WriteString("\n- severity: ")
	b.WriteString(string(f.Severity))
	b.WriteString("\n- category: ")
	b.WriteString(string(f.Category))
	b.WriteString("\n- message: ")
	b.WriteString(f.Message)
	if f.Suggestion != "" {
		b.WriteString("\n- suggestion: ")
		b.WriteString(f.Suggestion)
	}
	b.WriteString("\n\nCode under review:\n```\n")
	b.WriteString(hunkContent)
	b.WriteString("\n```\n\nIs this finding correct and actionable for this code? Answer with a JSON object only: {\"verdict\": \"yes\" or \"no\", \"reason\": \"brief reason\"}. No other text.")
	return b.String()
}

// ParseCriticVerdict parses the critic model response. Returns true only when verdict is "yes" (case-insensitive).
// Invalid JSON or missing verdict returns false (drop finding). Reason is ignored for the keep/drop decision.
func ParseCriticVerdict(response string) (keep bool) {
	response = strings.TrimSpace(response)
	if response == "" {
		return false
	}
	var v verdictResponse
	if err := json.Unmarshal([]byte(response), &v); err != nil {
		return false
	}
	return strings.TrimSpace(strings.ToLower(v.Verdict)) == "yes"
}

// CriticClient is the minimal interface used by VerifyFinding so callers can pass *ollama.Client or a test double.
type CriticClient interface {
	Generate(ctx context.Context, model, systemPrompt, userPrompt string, opts *ollama.GenerateOptions) (*ollama.GenerateResult, error)
}

// VerifyFinding runs the critic model on the given finding and returns whether to keep it.
// On context cancel or Ollama error, returns (false, err). On parse failure: if opts.RetryOnParseError
// is true (default when opts is nil), retries the Generate call once; then treats as no and returns (false, nil).
func VerifyFinding(ctx context.Context, client CriticClient, criticModel string, f findings.Finding, hunkContent string, opts *CriticOptions) (keep bool, err error) {
	if client == nil || criticModel == "" {
		return false, nil
	}
	maxHunk := defaultCriticMaxHunkLen
	retryParse := true
	if opts != nil {
		if opts.MaxHunkContentLen > 0 {
			maxHunk = opts.MaxHunkContentLen
		}
		retryParse = opts.RetryOnParseError
	}
	userPrompt := BuildCriticPrompt(f, hunkContent, maxHunk)
	genOpts := &ollama.GenerateOptions{KeepAlive: 0}
	if opts != nil && opts.KeepAlive != nil {
		genOpts.KeepAlive = opts.KeepAlive
	}
	result, genErr := client.Generate(ctx, criticModel, criticSystemPrompt, userPrompt, genOpts)
	if genErr != nil {
		return false, genErr
	}
	parsed := parseCriticVerdictWithParsed(result.Response)
	if parsed.parsed {
		return parsed.keep, nil
	}
	if retryParse {
		result2, genErr2 := client.Generate(ctx, criticModel, criticSystemPrompt, userPrompt, genOpts)
		if genErr2 != nil {
			return false, fmt.Errorf("critic retry after parse failure: %w", genErr2)
		}
		parsed2 := parseCriticVerdictWithParsed(result2.Response)
		if parsed2.parsed {
			return parsed2.keep, nil
		}
	}
	return false, nil
}

type parsedVerdict struct {
	keep   bool
	parsed bool
}

func parseCriticVerdictWithParsed(response string) parsedVerdict {
	response = strings.TrimSpace(response)
	if response == "" {
		return parsedVerdict{keep: false, parsed: false}
	}
	var v verdictResponse
	if err := json.Unmarshal([]byte(response), &v); err != nil {
		return parsedVerdict{keep: false, parsed: false}
	}
	verdict := strings.TrimSpace(strings.ToLower(v.Verdict))
	return parsedVerdict{keep: verdict == "yes", parsed: true}
}
