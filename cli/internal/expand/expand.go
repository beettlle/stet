// Package expand provides hunk expansion for context-aware code review.
// For Go files, when a diff hunk is inside a function, the enclosing function
// body is fetched and injected into the prompt to reduce hallucinations (e.g.
// "variable undefined" when the variable is declared earlier in the same function).
package expand

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"go/ast"
	"go/parser"
	"go/token"

	"stet/cli/internal/diff"
)

// hunkHeaderRegex captures @@ -oldStart,oldCount +newStart,newCount @@
// The +newStart,newCount part gives the line range in the new (post-change) file.
var hunkHeaderRegex = regexp.MustCompile(`@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// HunkLineRange parses the hunk header from RawContent and returns the 1-based
// line range in the new file: start and end (inclusive). ok is false if the
// header cannot be parsed.
func HunkLineRange(hunk diff.Hunk) (start, end int, ok bool) {
	firstLine := strings.SplitN(hunk.RawContent, "\n", 2)[0]
	matches := hunkHeaderRegex.FindStringSubmatch(firstLine)
	if matches == nil {
		return 0, 0, false
	}
	var newStart, newCount int
	if !parsePositiveInt(matches[1], &newStart) {
		return 0, 0, false
	}
	if len(matches) > 2 && matches[2] != "" {
		if !parsePositiveInt(matches[2], &newCount) {
			return 0, 0, false
		}
	} else {
		newCount = 1
	}
	return newStart, newStart + newCount - 1, true
}

func parsePositiveInt(s string, out *int) bool {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return false
	}
	*out = n
	return true
}

const (
	goExt               = ".go"
	truncateMarker      = "// ... (truncated)"
	maxExpandFileSize   = 1024 * 1024 // 1 MiB; skip expansion for larger files
)

// ExpandHunk enriches a hunk with enclosing function context for Go files.
// When the hunk is inside a function, the full function body is fetched and
// prepended to the prompt. Respects maxTokens by truncating; prioritizes
// function signature. Returns the hunk unchanged on any error or for non-Go
// files (fail open). repoRoot is the git repository root; file path is
// relative to it.
func ExpandHunk(repoRoot string, hunk diff.Hunk, maxTokens int) (diff.Hunk, error) {
	if repoRoot == "" || hunk.FilePath == "" {
		return hunk, nil
	}
	if filepath.Ext(hunk.FilePath) != goExt {
		return hunk, nil
	}
	start, end, ok := HunkLineRange(hunk)
	if !ok {
		return hunk, nil
	}

	path := filepath.Join(repoRoot, filepath.FromSlash(hunk.FilePath))
	path = filepath.Clean(path)
	absRepo, err := filepath.Abs(repoRoot)
	if err != nil {
		return hunk, nil
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return hunk, nil
	}
	rel, err := filepath.Rel(absRepo, absPath)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return hunk, nil // path escaped repo
	}
	path = absPath

	info, err := os.Stat(path)
	if err != nil {
		return hunk, nil
	}
	if info.Size() > maxExpandFileSize {
		return hunk, nil
	}
	src, err := readFile(path)
	if err != nil {
		return hunk, nil
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return hunk, nil
	}

	enclosing := findEnclosingFunc(fset, f, start, end)
	if enclosing == nil {
		return hunk, nil
	}

	startOff := fset.Position(enclosing.Pos()).Offset
	endOff := fset.Position(enclosing.End()).Offset
	if startOff < 0 || endOff > len(src) || startOff >= endOff {
		return hunk, nil
	}
	funcSrc := string(src[startOff:endOff])
	if maxTokens > 0 {
		funcSrc = truncateToTokens(funcSrc, maxTokens)
	}

	augmented := "## Enclosing function context\n\n```go\n" + funcSrc + "\n```\n\n## Diff hunk\n\n" + hunk.RawContent
	return diff.Hunk{
		FilePath:   hunk.FilePath,
		RawContent: hunk.RawContent,
		Context:    augmented,
	}, nil
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// findEnclosingFunc returns the smallest *ast.FuncDecl that fully contains
// the given line range (1-based, inclusive). Returns nil if none found.
func findEnclosingFunc(fset *token.FileSet, f *ast.File, startLine, endLine int) *ast.FuncDecl {
	var smallest *ast.FuncDecl
	var smallestSpan int = -1

	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		pos := fset.Position(fn.Pos())
		end := fset.Position(fn.End())
		if pos.Line <= startLine && endLine <= end.Line {
			span := end.Line - pos.Line + 1
			if smallest == nil || span < smallestSpan {
				smallest = fn
				smallestSpan = span
			}
		}
	}
	return smallest
}

// truncateToTokens truncates s to fit within maxTokens (chars/4 heuristic).
// Prioritizes the start (signature); appends truncateMarker when truncated.
func truncateToTokens(s string, maxTokens int) string {
	maxChars := maxTokens * 4
	if len(s) <= maxChars {
		return s
	}
	truncated := s[:maxChars-len(truncateMarker)-1]
	// Try to break at a newline
	if idx := strings.LastIndex(truncated, "\n"); idx > maxChars/2 {
		truncated = truncated[:idx+1]
	}
	return truncated + "\n" + truncateMarker
}
