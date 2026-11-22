// Rule-section selector: splits Cursor rule content into heading-delimited
// sections and ranks them against one diff hunk using a single System One
// choice question, then greedily fills a token budget with the top-ranked
// sections. Library only; pipeline wiring is a separate task (SP-033).
package rules

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"stet/cli/internal/systemone"
	"stet/cli/internal/tokens"
)

// maxSelectorOptions caps how many sections become choice options in one
// selector request; sections beyond the cap are not candidates.
const maxSelectorOptions = 64

// Selector candidate modes (rules_selector_candidates / STET_RULES_SELECTOR_CANDIDATES).
const (
	// SelectorCandidatesFull is the default: full-rule SplitSections, then positional [:64].
	SelectorCandidatesFull = "full"
	// SelectorCandidatesDigestFair builds digest sections file-specific-first and
	// round-robins into the 64-option cap (Protocol A).
	SelectorCandidatesDigestFair = "digest_fair"
)

// selectorStatePrefix prefixes the request state that carries the hunk content,
// mirroring review/critic_systemone.go.
const selectorStatePrefix = "Code under review:\n"

// selectorMaxHunkLen caps the hunk content in the request state (same cap as
// the review critic's default).
const selectorMaxHunkLen = 4096

// selectorQuestionKey is the single choice question's key in the request.
const selectorQuestionKey = "q0"

// selectorInstructions asks which guideline section is most relevant to
// reviewing the change found in the state. Security: the hunk content is
// attacker-influenced input, so it is passed only in the request state and
// never echoed into instructions or option descriptions.
const selectorInstructions = "A code change to review is provided in the state. Which guideline section is most relevant to reviewing this change? Choose the one option whose section belongs in the project review criteria for this change."

// Section is one heading-delimited slice of a rule's markdown content.
type Section struct {
	// Source is the rule file name (e.g. "go-1-21-development-standards.mdc").
	Source string
	// Heading is the markdown heading text; Source for content before the first heading.
	Heading string
	// Content is the heading line plus body, trimmed.
	Content string
}

// ChoiceDecider is satisfied by *systemone.Client; tests use a fake.
type ChoiceDecider interface {
	DecideChoice(ctx context.Context, req systemone.ChoiceRequest) (*systemone.ChoiceResponse, error)
}

// SplitSections splits r.Content at markdown headings of level 2 or 3
// ("## ", "### "), ignoring headings inside fenced code blocks, and returns
// the non-empty sections in document order. Text before the first heading
// becomes a section whose Heading is r.Source. Sections whose trimmed content
// is empty (e.g. an empty preamble) are dropped.
func SplitSections(r CursorRule) []Section {
	type builder struct {
		heading string
		lines   []string
	}
	var sections []Section
	flush := func(b builder) {
		content := strings.TrimSpace(strings.Join(b.lines, "\n"))
		if content == "" {
			return
		}
		sections = append(sections, Section{Source: r.Source, Heading: b.heading, Content: content})
	}
	cur := builder{heading: r.Source}
	inFence := false
	var fenceChar byte
	fenceLen := 0
	for _, line := range strings.Split(r.Content, "\n") {
		if inFence {
			cur.lines = append(cur.lines, line)
			if isFenceClose(line, fenceChar, fenceLen) {
				inFence = false
			}
			continue
		}
		if c, n, ok := fenceMarker(line); ok {
			inFence = true
			fenceChar, fenceLen = c, n
			cur.lines = append(cur.lines, line)
			continue
		}
		if heading, ok := markdownHeadingLevel23(line); ok {
			flush(cur)
			cur = builder{heading: heading, lines: []string{line}}
			continue
		}
		cur.lines = append(cur.lines, line)
	}
	flush(cur)
	return sections
}

// markdownHeadingLevel23 reports whether line is an ATX heading of level 2 or 3
// ("## " / "### ") and returns the trimmed heading text. Deeper levels
// ("#### " and beyond) are body text, not split points.
func markdownHeadingLevel23(line string) (string, bool) {
	if strings.HasPrefix(line, "### ") {
		return strings.TrimSpace(line[len("### "):]), true
	}
	if strings.HasPrefix(line, "## ") {
		return strings.TrimSpace(line[len("## "):]), true
	}
	return "", false
}

// fenceMarker reports whether line opens a fenced code block: at least three
// backticks or tildes (after optional whitespace). Returns the fence character
// and its run length.
func fenceMarker(line string) (byte, int, bool) {
	t := strings.TrimSpace(line)
	if len(t) < 3 {
		return 0, 0, false
	}
	c := t[0]
	if c != '`' && c != '~' {
		return 0, 0, false
	}
	n := 0
	for n < len(t) && t[n] == c {
		n++
	}
	if n < 3 {
		return 0, 0, false
	}
	return c, n, true
}

// isFenceClose reports whether line closes a fence opened with character c and
// run length n: a whitespace-padded run of c of length >= n.
func isFenceClose(line string, c byte, n int) bool {
	t := strings.TrimSpace(line)
	if len(t) < n {
		return false
	}
	for i := 0; i < len(t); i++ {
		if t[i] != c {
			return false
		}
	}
	return true
}

// NormalizeSelectorCandidates returns a known candidate mode. Empty input
// becomes SelectorCandidatesFull. Unknown values return an error.
func NormalizeSelectorCandidates(mode string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "", SelectorCandidatesFull:
		return SelectorCandidatesFull, nil
	case SelectorCandidatesDigestFair:
		return SelectorCandidatesDigestFair, nil
	default:
		return "", fmt.Errorf("rules select: unknown candidates mode %q (want %q or %q)", mode, SelectorCandidatesFull, SelectorCandidatesDigestFair)
	}
}

// BuildSelectorCandidates returns the section list that becomes choice options
// for SelectSections. mode SelectorCandidatesFull concatenates SplitSections of
// each path-matched rule in FilterRules order (caller/SelectSections then caps
// at 64). mode SelectorCandidatesDigestFair uses digest sections only, orders
// file-specific rules first, and round-robins into at most maxSelectorOptions
// so late language rules remain candidates when alwaysApply packs are large.
func BuildSelectorCandidates(ruleList []CursorRule, filePath, mode string) []Section {
	norm, err := NormalizeSelectorCandidates(mode)
	if err != nil {
		norm = SelectorCandidatesFull
	}
	matched := FilterRules(ruleList, filePath)
	if len(matched) == 0 {
		return nil
	}
	if norm == SelectorCandidatesFull {
		var sections []Section
		for _, r := range matched {
			sections = append(sections, SplitSections(r)...)
		}
		return sections
	}
	ordered := fileSpecificFirst(matched)
	queues := make([][]Section, len(ordered))
	maxLen := 0
	for i, r := range ordered {
		queues[i] = SplitSections(CursorRule{Source: r.Source, Content: digestOf(r)})
		maxLen = max(maxLen, len(queues[i]))
	}
	out := make([]Section, 0, maxSelectorOptions)
	for pass := 0; pass < maxLen && len(out) < maxSelectorOptions; pass++ {
		for _, queue := range queues {
			if pass >= len(queue) {
				continue
			}
			if len(out) >= maxSelectorOptions {
				break
			}
			out = append(out, queue[pass])
		}
	}
	return out
}

// selectorOptionDescription builds the choice option text for s. When
// useDetect is true, the first "**Detect:**" line in s.Content is preferred
// so the decision server sees review signal instead of a bare heading.
func selectorOptionDescription(s Section, useDetect bool) string {
	if useDetect {
		if d := firstDetectLine(s.Content); d != "" {
			return s.Source + ": " + d
		}
	}
	return s.Source + ": " + s.Heading
}

// firstDetectLine returns the first trimmed "**Detect:**" line outside fences,
// or "" when none exists.
func firstDetectLine(content string) string {
	inFence := false
	var fenceChar byte
	fenceLen := 0
	for _, line := range strings.Split(content, "\n") {
		if inFence {
			if isFenceClose(line, fenceChar, fenceLen) {
				inFence = false
			}
			continue
		}
		if c, n, ok := fenceMarker(line); ok {
			inFence = true
			fenceChar, fenceLen = c, n
			continue
		}
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, digestDetectMarker) {
			return t
		}
	}
	return ""
}

// SelectSections asks the decision server one choice question — which guideline
// section is most relevant to reviewing the change in hunkContent — with the
// sections as options, ranks sections by returned probability (descending,
// ties by input order), and greedily keeps sections whose cumulative
// tokens.Estimate fits maxTokens, skipping any section that alone exceeds the
// remaining budget. Returns the kept sections in ranked order. Sections beyond
// maxSelectorOptions are not candidates. When useDetectOptions is true, choice
// option text prefers each section's first "**Detect:**" line (Protocol A).
//
// nil client, empty model, or no sections returns an error so the caller can
// decide the fallback. Decider errors and missing answers are wrapped with
// context.
func SelectSections(ctx context.Context, client ChoiceDecider, model, hunkContent string, sections []Section, maxTokens int, useDetectOptions bool) ([]Section, error) {
	if client == nil {
		return nil, fmt.Errorf("rules select: nil choice decider")
	}
	if model == "" {
		return nil, fmt.Errorf("rules select: empty model")
	}
	if len(sections) == 0 {
		return nil, fmt.Errorf("rules select: no sections")
	}
	if len(sections) > maxSelectorOptions {
		sections = sections[:maxSelectorOptions]
	}
	options := make(map[string]string, len(sections))
	for i, s := range sections {
		options[selectorOptionKey(i)] = selectorOptionDescription(s, useDetectOptions)
	}
	req := systemone.ChoiceRequest{
		Model: model,
		State: buildSelectorState(hunkContent),
		Questions: map[string]systemone.ChoiceQuestion{
			selectorQuestionKey: {Instructions: selectorInstructions, Criteria: options},
		},
	}
	resp, err := client.DecideChoice(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("rules select: decide choice: %w", err)
	}
	answer, ok := resp.Answers[selectorQuestionKey]
	if !ok {
		return nil, fmt.Errorf("rules select: response missing answer for question %q", selectorQuestionKey)
	}

	type ranked struct {
		idx  int
		prob float64
	}
	order := make([]ranked, len(sections))
	for i := range sections {
		order[i] = ranked{idx: i, prob: answer.Probabilities[selectorOptionKey(i)]}
	}
	sort.SliceStable(order, func(a, b int) bool {
		return order[a].prob > order[b].prob
	})

	selected := make([]Section, 0, len(sections))
	used := 0
	for _, r := range order {
		s := sections[r.idx]
		cost := tokens.Estimate(s.Content)
		if used+cost > maxTokens {
			continue
		}
		used += cost
		selected = append(selected, s)
	}
	return selected, nil
}

// selectorOptionKey returns the option key for the section at index i
// ("s0", "s1", ...).
func selectorOptionKey(i int) string {
	return fmt.Sprintf("s%d", i)
}

// buildSelectorState builds the request state: the hunk content only, prefixed
// like the review critic's System One state and truncated at the same default
// cap. Hunk content never appears in instructions or option descriptions.
func buildSelectorState(hunkContent string) string {
	if len(hunkContent) > selectorMaxHunkLen {
		hunkContent = hunkContent[:selectorMaxHunkLen] + "\n[truncated]"
	}
	return selectorStatePrefix + hunkContent
}
