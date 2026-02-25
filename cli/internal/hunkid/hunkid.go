// Package hunkid provides deterministic IDs for diff hunks and findings: strict
// and semantic hunk hashes (for "already reviewed" matching in Phase 2.3) and
// stable finding IDs (for tool-generated IDs in Phase 3).
package hunkid

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// StrictHunkID returns a deterministic ID for a hunk from path and raw content.
// Content is normalized (CRLF to LF) so identical content yields the same ID.
// Used for exact cache hits in the "already reviewed" set.
func StrictHunkID(path, rawContent string) string {
	norm := normalizeCRLF(rawContent)
	return hashString(path + ":" + norm)
}

// SemanticHunkID returns a deterministic ID that ignores comment and
// whitespace-only differences. Language is derived from the file extension;
// comments are stripped per language (Go, Python, JS/TS, Shell). Used to
// detect "same code, different formatting/comments" for auto-approve or
// format-only review.
func SemanticHunkID(path, rawContent string) string {
	lang := langFromPath(path)
	code := codeOnly(normalizeCRLF(rawContent), lang)
	return hashString(path + ":" + code)
}

// StableFindingID returns a deterministic ID for a finding from its location
// and message. Use file:start:end when rangeStart and rangeEnd are both > 0
// and rangeStart <= rangeEnd; otherwise fall back to file:line (clamped to 1
// when line <= 0 so the hash is never built from a non-positive line number).
// Message stem is trimmed and internal whitespace collapsed. Used when
// assigning tool-generated finding IDs (Phase 3).
func StableFindingID(file string, line int, rangeStart, rangeEnd int, message string) string {
	var loc string
	if rangeStart > 0 && rangeEnd > 0 && rangeStart <= rangeEnd {
		loc = file + ":" + strconv.Itoa(rangeStart) + ":" + strconv.Itoa(rangeEnd)
	} else {
		effectiveLine := line
		if effectiveLine <= 0 {
			effectiveLine = 1
		}
		loc = file + ":" + strconv.Itoa(effectiveLine)
	}
	stem := messageStem(message)
	return hashString(loc + ":" + stem)
}

func normalizeCRLF(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func messageStem(message string) string {
	s := strings.TrimSpace(message)
	return collapseWhitespace(s)
}

func collapseWhitespace(s string) string {
	// Replace runs of whitespace (space, tab, newline) with single space.
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(s, " "))
}

func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// langFromPath returns a language key from the file path for comment stripping.
// Supported: go, python, js, sh. Unknown extensions return "" (no comment strip).
func langFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py", ".pyw":
		return "python"
	case ".js", ".mjs", ".cjs", ".ts", ".tsx":
		return "js"
	case ".sh", ".bash", ".zsh":
		return "sh"
	default:
		return ""
	}
}

// codeOnly strips comments (by language) and collapses whitespace.
// For unknown lang, only whitespace is collapsed.
func codeOnly(content, lang string) string {
	var stripped string
	switch lang {
	case "go", "js":
		stripped = stripGoStyleComments(content)
	case "python":
		stripped = stripPythonComments(content)
	case "sh":
		stripped = stripShellComments(content)
	default:
		stripped = content
	}
	return collapseWhitespace(stripped)
}

// stripGoStyleComments removes // line comments and /* */ block comments.
// Used for Go and JavaScript/TypeScript.
func stripGoStyleComments(content string) string {
	// Block comments first (non-greedy). (?s) makes . match newline.
	blockRe := regexp.MustCompile(`(?s)/\*.*?\*/`)
	s := blockRe.ReplaceAllString(content, " ")
	// Line comments: // to end of line.
	lineRe := regexp.MustCompile(`//[^\n]*`)
	return lineRe.ReplaceAllString(s, " ")
}

// stripPythonComments removes # line comments. Docstrings not stripped for simplicity.
func stripPythonComments(content string) string {
	re := regexp.MustCompile(`#[^\n]*`)
	return re.ReplaceAllString(content, " ")
}

// stripShellComments removes # line comments (simple: # to EOL).
func stripShellComments(content string) string {
	re := regexp.MustCompile(`#[^\n]*`)
	return re.ReplaceAllString(content, " ")
}
