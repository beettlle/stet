// Package minify provides AST-preserving minification of diff hunk content
// to reduce prompt tokens. Go and Rust hunks are minified by per-line
// whitespace reduction (trim leading, collapse runs of spaces); semantics are unchanged.
package minify

import (
	"regexp"
	"strings"
)

// hunkHeader matches @@ -oldStart,oldCount +newStart,newCount @@ (same as diff package).
var hunkHeaderRegex = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+\d+(?:,\d+)? @@`)

// minifyUnifiedHunkContent reduces whitespace in unified-diff hunk content:
// keeps the @@ header and each line's first character (space, -, +); for the
// rest of each line, trims leading whitespace and collapses runs of spaces to one.
// Returns the original content on empty input or if the first line is not a hunk header.
func minifyUnifiedHunkContent(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}
	var out []string
	for i, line := range lines {
		if i == 0 {
			if hunkHeaderRegex.MatchString(line) {
				out = append(out, line)
			} else {
				return content
			}
			continue
		}
		if len(line) == 0 {
			out = append(out, "")
			continue
		}
		// Empty lines handled above; len(line) >= 1 so line[0:1] is safe.
		prefix := line[0:1]
		rest := line[1:]
		rest = strings.TrimLeft(rest, " \t")
		rest = collapseSpaces(rest)
		out = append(out, prefix+rest)
	}
	return strings.Join(out, "\n")
}

// MinifyGoHunkContent reduces whitespace in unified-diff hunk content for Go
// files. Keeps the @@ header and each line's first character (space, -, +); for
// the rest of each line, trims leading whitespace and collapses runs of spaces
// to one. Preserves semantics. Returns the original content on empty input or
// if the first line is not a hunk header. Callers should only pass raw hunk content.
func MinifyGoHunkContent(content string) string {
	return minifyUnifiedHunkContent(content)
}

// MinifyRustHunkContent applies the same per-line minification as MinifyGoHunkContent
// for Rust files. Callers should only pass raw hunk content.
func MinifyRustHunkContent(content string) string {
	return minifyUnifiedHunkContent(content)
}

// collapseSpaces replaces runs of spaces (and tabs) with a single space.
// Does not modify newlines or other characters.
func collapseSpaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	wasSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if !wasSpace {
				b.WriteRune(' ')
				wasSpace = true
			}
			continue
		}
		wasSpace = false
		b.WriteRune(r)
	}
	return b.String()
}
