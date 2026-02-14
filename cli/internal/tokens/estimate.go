// Package tokens provides simple token estimation for prompts and
// context-limit checks. Estimation uses a byte-based chars/4 heuristic;
// model-specific estimators can be added later.
package tokens

import (
	"fmt"
	"math"
)

// charsPerToken is the divisor for the simple byte-based estimator
// (roughly 4 bytes per token for typical English/code).
const charsPerToken = 4

// DefaultResponseReserve is the default number of tokens reserved for
// model response when checking total context. Total context is
// prompt tokens + response reserve for warning purposes.
const DefaultResponseReserve = 2048

// Estimate returns an estimated token count for the given prompt text.
// It uses a simple heuristic: (len(prompt)+3)/4 (bytes), so 0–3 bytes
// map to 1 token, 4–7 to 2, etc. Empty string returns 0.
// This is byte-based to align with typical tokenizer behavior.
func Estimate(prompt string) int {
	n := len(prompt)
	if n == 0 {
		return 0
	}
	return (n + charsPerToken - 1) / charsPerToken
}

// WarnIfOver returns a non-empty warning string when the total estimated
// tokens (promptTokens + responseReserve) meet or exceed the warn threshold
// of the context limit. All token counts (promptTokens, responseReserve) are
// in tokens. contextLimit and warnThreshold come from config
// (e.g. context_limit and warn_threshold). If contextLimit <= 0, returns "".
// The warning describes estimated total, percentage, and limit.
func WarnIfOver(promptTokens, responseReserve, contextLimit int, warnThreshold float64) string {
	if contextLimit <= 0 {
		return ""
	}
	if promptTokens < 0 || responseReserve < 0 {
		return ""
	}
	if responseReserve > math.MaxInt-promptTokens {
		return fmt.Sprintf("token estimate overflow (prompt %d + reserve %d)", promptTokens, responseReserve)
	}
	total := promptTokens + responseReserve
	limit := float64(contextLimit) * warnThreshold
	threshold := int(limit)
	if limit > float64(threshold) {
		threshold++
	}
	if total < threshold {
		return ""
	}
	pct := warnThreshold * 100
	return fmt.Sprintf("estimated tokens %d (prompt %d + reserve %d) exceeds %.0f%% of context limit %d",
		total, promptTokens, responseReserve, pct, contextLimit)
}
