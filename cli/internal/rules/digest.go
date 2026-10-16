// Review digest: condenses a Cursor rule to the lines a reviewer checks against.

package rules

import "strings"

// Rule sections mark anti-patterns with a "❌ Bad:" example line and a
// "**Detect:**" checklist line; those are the review-relevant parts of a rule
// that is otherwise written for code authoring.
const (
	digestBadMarker    = "❌ Bad"
	digestDetectMarker = "**Detect:**"
)

// BuildDigest returns the review digest of r: for each section (see
// SplitSections) that has "❌ Bad" or "**Detect:**" lines outside fenced code,
// a "### <heading>" line followed by those lines. Sections without markers
// (persona, communication, policy text) are left out. When no section has a
// marker, the rule does not follow the convention and its full Content is
// returned so third-party rules still reach the reviewer.
func BuildDigest(r CursorRule) string {
	var parts []string
	for _, s := range SplitSections(r) {
		lines := reviewLines(s.Content)
		if len(lines) == 0 {
			continue
		}
		parts = append(parts, "### "+s.Heading+"\n"+strings.Join(lines, "\n"))
	}
	if len(parts) == 0 {
		return r.Content
	}
	return strings.Join(parts, "\n\n")
}

// reviewLines returns the trimmed "❌ Bad" and "**Detect:**" lines of content
// in document order, ignoring lines inside fenced code blocks.
func reviewLines(content string) []string {
	var out []string
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
		if strings.HasPrefix(t, digestBadMarker) || strings.HasPrefix(t, digestDetectMarker) {
			out = append(out, t)
		}
	}
	return out
}

// digestOf returns r.Digest when the loader filled it, otherwise builds it.
func digestOf(r CursorRule) string {
	if r.Digest != "" {
		return r.Digest
	}
	return BuildDigest(r)
}
