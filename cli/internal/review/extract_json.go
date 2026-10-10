package review

import (
	"encoding/json"
	"strings"

	"stet/cli/internal/findings"
)

// maxExtractCandidates bounds how many JSON candidates (fenced blocks plus
// balanced spans) the recovery fallback will try per response, so pathological
// prose cannot make parsing O(n^2) unbounded.
const maxExtractCandidates = 64

// extractFindingsJSON recovers a findings payload embedded in a prose response:
// some models write analysis first and only then emit the JSON array, often
// inside a fenced code block. Candidates are tried newest first (fenced blocks
// from last to first, then balanced JSON spans scanning start positions from
// the end of the text backwards). The first candidate that decodes into a
// findings payload (array of finding objects, or a {"findings": [...]} wrapper)
// is returned. An empty array is a valid payload. ok is false when no candidate
// decodes (or the candidate budget is exhausted).
func extractFindingsJSON(s string) (string, bool) {
	attempts := 0
	// 1. Fenced code blocks (``` with optional language tag), newest first.
	blocks := collectFencedBlocks(s)
	for i := len(blocks) - 1; i >= 0; i-- {
		attempts++
		if attempts > maxExtractCandidates {
			return "", false
		}
		if isFindingsPayload(blocks[i]) {
			return blocks[i], true
		}
	}
	// 2. Balanced JSON spans starting at [ or {, newest (last) first.
	for start := len(s) - 1; start >= 0; start-- {
		if c := s[start]; c != '[' && c != '{' {
			continue
		}
		attempts++
		if attempts > maxExtractCandidates {
			return "", false
		}
		span, ok := balancedJSONSpan(s, start)
		if !ok {
			continue
		}
		if isFindingsPayload(span) {
			return span, true
		}
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
	dec := json.NewDecoder(strings.NewReader(cand))
	var arr []findings.Finding
	if err := dec.Decode(&arr); err == nil {
		return true
	}
	dec = json.NewDecoder(strings.NewReader(cand))
	var wrapper map[string]json.RawMessage
	if err := dec.Decode(&wrapper); err != nil {
		return false
	}
	raw, has := wrapper["findings"]
	if !has {
		return false
	}
	var list []findings.Finding
	return json.Unmarshal(raw, &list) == nil
}
