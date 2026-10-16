// Review criteria: orders rule digests and fits them into the rule token budget.

package rules

import (
	"strings"
	"unicode/utf8"

	"stet/cli/internal/tokens"
)

// Default rule budget for the review prompt: a share of the context window,
// clamped so small windows still get MaxRuleTokens and large windows leave
// room for the hunk, RAG context, and the response.
const (
	reviewRuleBudgetPercent = 15
	maxReviewRuleTokens     = 8192
	// minTrimmedSectionTokens is the smallest slice worth keeping of a section
	// that can never fit whole; smaller slices are dropped instead.
	minTrimmedSectionTokens = 64
	sectionSeparator        = "\n\n"
	truncatedMarker         = "\n\n[truncated]"
)

// ReviewRuleBudget returns the token budget for rule text in the review
// prompt. override > 0 (rules_max_tokens) wins; otherwise the budget is
// reviewRuleBudgetPercent of contextLimit clamped to
// [MaxRuleTokens, maxReviewRuleTokens], or MaxRuleTokens when contextLimit <= 0.
func ReviewRuleBudget(contextLimit, override int) int {
	if override > 0 {
		return override
	}
	if contextLimit <= 0 {
		return MaxRuleTokens
	}
	budget := int(int64(contextLimit) * reviewRuleBudgetPercent / 100)
	return min(max(budget, MaxRuleTokens), maxReviewRuleTokens)
}

// Criteria is the rule text chosen for one file's review prompt. Included and
// Dropped label sections as "<rule file>: <heading>" for trace and warnings.
type Criteria struct {
	Text     string
	Included []string
	Dropped  []string
}

// BuildReviewCriteria selects the review digests of the rules in ruleList that
// apply to filePath. File-specific rules (glob matches) come before alwaysApply
// rules. Sections are taken round-robin across rules, so every rule gets its
// first sections before any rule gets later ones; a section that does not fit
// the remaining budget is dropped whole. A section larger than the entire
// budget is trimmed to the rule's fair share (budget / rule count) instead, so
// a rule without headings is not dropped outright. budget <= 0 keeps
// everything. Text groups sections by rule in that order.
func BuildReviewCriteria(ruleList []CursorRule, filePath string, budget int) Criteria {
	ordered := fileSpecificFirst(FilterRules(ruleList, filePath))
	if len(ordered) == 0 {
		return Criteria{}
	}
	queues := make([][]Section, len(ordered))
	maxLen := 0
	for i, r := range ordered {
		queues[i] = SplitSections(CursorRule{Source: r.Source, Content: digestOf(r)})
		maxLen = max(maxLen, len(queues[i]))
	}
	picked := make([][]string, len(ordered))
	var c Criteria
	remaining := budget
	share := budget / len(ordered)
	for pass := 0; pass < maxLen; pass++ {
		for i, queue := range queues {
			if pass >= len(queue) {
				continue
			}
			s := queue[pass]
			label := s.Source + ": " + s.Heading
			cost := tokens.Estimate(s.Content + sectionSeparator)
			switch {
			case budget <= 0 || cost <= remaining:
				picked[i] = append(picked[i], s.Content)
				c.Included = append(c.Included, label)
				remaining -= cost
			case cost > budget && min(share, remaining) >= minTrimmedSectionTokens:
				trimmed := trimToTokens(s.Content, min(share, remaining))
				picked[i] = append(picked[i], trimmed)
				c.Included = append(c.Included, label+" (truncated)")
				remaining -= tokens.Estimate(trimmed + sectionSeparator)
			default:
				c.Dropped = append(c.Dropped, label)
			}
		}
	}
	var parts []string
	for _, p := range picked {
		parts = append(parts, p...)
	}
	c.Text = strings.Join(parts, sectionSeparator)
	return c
}

// fileSpecificFirst returns rules with non-alwaysApply (glob-matched) rules
// first and alwaysApply rules after, each group in its original order.
func fileSpecificFirst(matched []CursorRule) []CursorRule {
	out := make([]CursorRule, 0, len(matched))
	for _, r := range matched {
		if !r.AlwaysApply {
			out = append(out, r)
		}
	}
	for _, r := range matched {
		if r.AlwaysApply {
			out = append(out, r)
		}
	}
	return out
}

// trimToTokens cuts s so that it plus truncatedMarker fits maxTokens (chars/4
// estimate), backing off to a UTF-8 rune boundary.
func trimToTokens(s string, maxTokens int) string {
	n := maxTokens*4 - len(truncatedMarker)
	if n <= 0 {
		return ""
	}
	if n >= len(s) {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + truncatedMarker
}
