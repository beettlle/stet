package review

import (
	"encoding/json"
	"strings"

	"stet/cli/internal/findings"
)

// maxFencedCandidates bounds how many fenced blocks the recovery fallback
// decodes per response.
const maxFencedCandidates = 64

// maxSpanCandidates bounds how many top-level '[' / '{' start positions the
// balanced-span stage examines. Each costs at most one O(n) scan, so prose
// heavy with brackets stays linear in practice; when exceeded, recovery gives
// up rather than return an older payload than the one the model ended with.
const maxSpanCandidates = 1024

// extractFindingsJSON recovers a findings payload embedded in a prose response:
// some models write analysis first and only then emit the JSON array, often
// inside a fenced code block. Fenced blocks are tried newest first; the first
// that decodes into a findings payload (array of finding objects, or a
// {"findings": [...]} wrapper; an empty array counts) wins. Otherwise
// lastBareFindingsSpan scans bare JSON spans. ok is false when nothing decodes
// or a candidate budget is exhausted.
func extractFindingsJSON(s string) (string, bool) {
	blocks := collectFencedBlocks(s)
	tried := 0
	for i := len(blocks) - 1; i >= 0; i-- {
		tried++
		if tried > maxFencedCandidates {
			return "", false
		}
		if isFindingsPayload(blocks[i]) {
			return blocks[i], true
		}
	}
	return lastBareFindingsSpan(s)
}

// lastBareFindingsSpan scans '[' / '{' start positions left to right. A span
// that decodes as any JSON value is skipped over whole, so arrays nested in a
// finding (e.g. "evidence_lines": []) are never candidates on their own. The
// last non-empty findings payload wins; an empty payload is returned only when
// no non-empty one exists, so a stray "[]" in prose cannot hide real findings.
func lastBareFindingsSpan(s string) (string, bool) {
	var lastNonEmpty, lastEmpty string
	examined := 0
	for start := 0; start < len(s); start++ {
		if c := s[start]; c != '[' && c != '{' {
			continue
		}
		examined++
		if examined > maxSpanCandidates {
			return "", false
		}
		span, ok := balancedJSONSpan(s, start)
		if !ok || !json.Valid([]byte(span)) {
			continue
		}
		if n, isPayload := findingsPayloadLen(span); isPayload {
			if n > 0 {
				lastNonEmpty = span
			} else {
				lastEmpty = span
			}
		}
		start += len(span) - 1
	}
	if lastNonEmpty != "" {
		return lastNonEmpty, true
	}
	if lastEmpty != "" {
		return lastEmpty, true
	}
	return "", false
}

// collectFencedBlocks returns the inner text of every fenced code block
// (```...```) in s, in document order. The line after an opening fence is
// treated as the info string (language tag) when empty or tag-shaped, matching
// stripMarkdownCodeFence. Unterminated fences yield no block; the JSON inside
// can still be recovered as a balanced span.
func collectFencedBlocks(s string) []string {
	var blocks []string
	i := 0
	for {
		idx := strings.Index(s[i:], markdownFence)
		if idx < 0 {
			return blocks
		}
		rest := i + idx + len(markdownFence)
		contentStart := rest
		if nl := strings.IndexByte(s[rest:], '\n'); nl >= 0 {
			firstLine := strings.TrimSpace(s[rest : rest+nl])
			if firstLine == "" || isLikelyFenceLanguageTag(firstLine) {
				contentStart = rest + nl + 1
			}
		}
		closeRel := strings.Index(s[contentStart:], markdownFence)
		if closeRel < 0 {
			// Unterminated fence; stop collecting fenced candidates.
			return blocks
		}
		blocks = append(blocks, s[contentStart:contentStart+closeRel])
		i = contentStart + closeRel + len(markdownFence)
	}
}

// balancedJSONSpan returns the balanced JSON object/array in s starting at
// position start ('[' or '{'), honoring string literals and escapes. ok is
// false when the structure is unbalanced (e.g. a truncated response).
func balancedJSONSpan(s string, start int) (string, bool) {
	open, close := s[start], byte(']')
	if open == '{' {
		close = '}'
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return s[start : i+1], true
			}
		}
	}
	return "", false
}

// isFindingsPayload reports whether cand decodes into a findings payload: a
// JSON array of finding objects (including an empty array) or a wrapper object
// with a "findings" array. Single finding objects are rejected; json.Decoder
// ignores trailing text after the first value.
func isFindingsPayload(cand string) bool {
	_, ok := findingsPayloadLen(cand)
	return ok
}

// findingsPayloadLen decodes cand like isFindingsPayload and also returns how
// many entries carry a non-empty message, so placeholder arrays such as [{}]
// do not outrank a real findings array.
func findingsPayloadLen(cand string) (int, bool) {
	dec := json.NewDecoder(strings.NewReader(cand))
	var arr []findings.Finding
	if err := dec.Decode(&arr); err == nil {
		return countWithMessage(arr), true
	}
	dec = json.NewDecoder(strings.NewReader(cand))
	var wrapper map[string]json.RawMessage
	if err := dec.Decode(&wrapper); err != nil {
		return 0, false
	}
	raw, has := wrapper["findings"]
	if !has {
		return 0, false
	}
	var list []findings.Finding
	if err := json.Unmarshal(raw, &list); err != nil {
		return 0, false
	}
	return countWithMessage(list), true
}

func countWithMessage(list []findings.Finding) int {
	n := 0
	for i := range list {
		if strings.TrimSpace(list[i].Message) != "" {
			n++
		}
	}
	return n
}
