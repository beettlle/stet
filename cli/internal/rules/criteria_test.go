package rules

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestReviewRuleBudget(t *testing.T) {
	tests := []struct {
		name                   string
		contextLimit, override int
		want                   int
	}{
		{name: "override_wins", contextLimit: 32768, override: 2500, want: 2500},
		{name: "no_context_limit_uses_floor", contextLimit: 0, want: MaxRuleTokens},
		{name: "default_context_scales", contextLimit: 32768, want: 4915},
		{name: "small_context_clamped_to_floor", contextLimit: 4096, want: MaxRuleTokens},
		{name: "large_context_clamped_to_cap", contextLimit: 262144, want: maxReviewRuleTokens},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ReviewRuleBudget(tt.contextLimit, tt.override); got != tt.want {
				t.Errorf("ReviewRuleBudget(%d, %d) = %d, want %d", tt.contextLimit, tt.override, got, tt.want)
			}
		})
	}
}

func sectionsRule(source string, always bool, globs []string, headings ...string) CursorRule {
	var b strings.Builder
	for _, h := range headings {
		b.WriteString("### " + h + "\n**Detect:** " + strings.Repeat(h, 20) + "\n\n")
	}
	return CursorRule{Source: source, AlwaysApply: always, Globs: globs, Content: b.String()}
}

func TestBuildReviewCriteria_fileSpecificFirst(t *testing.T) {
	ruleList := []CursorRule{
		sectionsRule("always.mdc", true, nil, "A1"),
		sectionsRule("python.mdc", false, []string{"**/*.py"}, "P1"),
	}
	c := BuildReviewCriteria(ruleList, "pkg/app.py", 0)
	if strings.Index(c.Text, "### P1") > strings.Index(c.Text, "### A1") {
		t.Errorf("python digest should precede alwaysApply digest:\n%s", c.Text)
	}
	if len(c.Dropped) != 0 || len(c.Included) != 2 {
		t.Errorf("Included = %v, Dropped = %v; want 2 included, none dropped", c.Included, c.Dropped)
	}
}

func TestBuildReviewCriteria_roundRobinFairShare(t *testing.T) {
	ruleList := []CursorRule{
		sectionsRule("a.mdc", true, nil, "A1", "A2", "A3"),
		sectionsRule("b.mdc", true, nil, "B1", "B2", "B3"),
	}
	full := BuildReviewCriteria(ruleList, "x.go", 0)
	perSection := len(full.Text) / 6 / 4
	c := BuildReviewCriteria(ruleList, "x.go", perSection*4+8)
	want := []string{"a.mdc: A1", "b.mdc: B1", "a.mdc: A2", "b.mdc: B2"}
	if strings.Join(c.Included, ",") != strings.Join(want, ",") {
		t.Errorf("Included = %v, want %v", c.Included, want)
	}
	if len(c.Dropped) != 2 {
		t.Errorf("Dropped = %v, want A3 and B3", c.Dropped)
	}
	if strings.Contains(c.Text, "[truncated]") {
		t.Errorf("fitting sections must not be cut:\n%s", c.Text)
	}
	if strings.Index(c.Text, "### A2") > strings.Index(c.Text, "### B1") {
		t.Errorf("text should group sections by rule:\n%s", c.Text)
	}
}

func TestBuildReviewCriteria_oversizedSectionTrimmed(t *testing.T) {
	ruleList := []CursorRule{{Source: "big.mdc", Globs: []string{"*"}, Content: strings.Repeat("x", 5000)}}
	c := BuildReviewCriteria(ruleList, "any.go", 100)
	if !strings.HasSuffix(c.Text, "[truncated]") || len(c.Text) > 400 {
		t.Errorf("Text length %d, want trimmed to ~400 chars with marker", len(c.Text))
	}
	if len(c.Included) != 1 || !strings.HasSuffix(c.Included[0], "(truncated)") {
		t.Errorf("Included = %v, want one truncated label", c.Included)
	}
}

func TestBuildReviewCriteria_oversizedSectionDroppedWhenShareTooSmall(t *testing.T) {
	ruleList := []CursorRule{{Source: "big.mdc", Globs: []string{"*"}, Content: strings.Repeat("x", 5000)}}
	c := BuildReviewCriteria(ruleList, "any.go", minTrimmedSectionTokens-1)
	if c.Text != "" || len(c.Dropped) != 1 {
		t.Errorf("Text = %q, Dropped = %v; want empty text and one drop", c.Text, c.Dropped)
	}
}

func TestBuildReviewCriteria_noMatch(t *testing.T) {
	c := BuildReviewCriteria([]CursorRule{sectionsRule("go.mdc", false, []string{"*.go"}, "G")}, "app.ts", 1000)
	if c.Text != "" || c.Included != nil || c.Dropped != nil {
		t.Errorf("no match: got %+v, want zero Criteria", c)
	}
}

func TestTrimToTokens(t *testing.T) {
	if got := trimToTokens("short", 100); got != "short" {
		t.Errorf("trimToTokens(short) = %q", got)
	}
	if got := trimToTokens("abc", 1); got != "" {
		t.Errorf("trimToTokens(budget below marker) = %q, want empty", got)
	}
	s := strings.Repeat("❌", 200)
	got := trimToTokens(s, 20)
	if !utf8.ValidString(got) || !strings.HasSuffix(got, "[truncated]") {
		t.Errorf("trimToTokens(multibyte) = %q, want valid UTF-8 with marker", got)
	}
}
