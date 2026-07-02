package expand

import (
	"regexp"
	"strings"

	"stet/cli/internal/diff"
)

const pyExt = ".py"

var pyDefDecl = regexp.MustCompile(`^\s*(async\s+)?def\s+\w+\s*(\(|:)`)

// expandPythonHunk enriches a hunk with enclosing function or class-method context
// for Python files. Fail-open: returns hunk unchanged on any error.
func expandPythonHunk(repoRoot string, hunk diff.Hunk, maxTokens int) (diff.Hunk, error) {
	start, end, ok := HunkLineRange(hunk)
	if !ok {
		return hunk, nil
	}

	src, _, ok := loadRepoSourceFile(repoRoot, hunk.FilePath)
	if !ok {
		return hunk, nil
	}

	scope, ok := findEnclosingPythonScope(src, start, end)
	if !ok {
		return hunk, nil
	}

	funcSrc := extractLineRange(src, scope.startLine, scope.endLine)
	if funcSrc == "" {
		return hunk, nil
	}
	if maxTokens > 0 {
		funcSrc = truncateToTokens(funcSrc, maxTokens)
	}

	augmented := "## Enclosing function context\n\n```python\n" + funcSrc + "\n```\n\n## Diff hunk\n\n" + hunk.RawContent
	return diff.Hunk{
		FilePath:   hunk.FilePath,
		RawContent: hunk.RawContent,
		Context:    augmented,
	}, nil
}

type pyScope struct {
	startLine int
	endLine   int
}

func findEnclosingPythonScope(src []byte, startLine, endLine int) (pyScope, bool) {
	lines := strings.Split(string(src), "\n")

	var best *pyScope
	bestSpan := -1

	for i, line := range lines {
		if !looksLikePythonDef(line) {
			continue
		}
		scopeStart, scopeEnd, ok := pythonDefLineRange(lines, i)
		if !ok {
			continue
		}
		if scopeStart > startLine || endLine > scopeEnd {
			continue
		}
		span := scopeEnd - scopeStart + 1
		if best == nil || span < bestSpan {
			best = &pyScope{startLine: scopeStart, endLine: scopeEnd}
			bestSpan = span
		}
	}
	if best == nil {
		return pyScope{}, false
	}
	return *best, true
}

func looksLikePythonDef(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return false
	}
	if strings.HasPrefix(trimmed, "class ") {
		return false
	}
	return pyDefDecl.MatchString(line)
}

func pythonDefLineRange(lines []string, defLineIdx int) (startLine, endLine int, ok bool) {
	if defLineIdx < 0 || defLineIdx >= len(lines) {
		return 0, 0, false
	}
	headerEnd, ok := pythonDefHeaderEndLine(lines, defLineIdx)
	if !ok {
		return 0, 0, false
	}
	scopeStart := pythonDeclStartLine(lines, defLineIdx) + 1
	scopeEnd := pythonBlockEndLine(lines, defLineIdx, headerEnd)
	if scopeEnd < scopeStart {
		return 0, 0, false
	}
	return scopeStart, scopeEnd, true
}

func pythonDeclStartLine(lines []string, defLineIdx int) int {
	defIndent := lineIndent(lines[defLineIdx])
	start := defLineIdx
	for i := defLineIdx - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			break
		}
		if lineIndent(lines[i]) != defIndent {
			break
		}
		if strings.HasPrefix(trimmed, "@") {
			start = i
			continue
		}
		break
	}
	return start
}

func pythonDefHeaderEndLine(lines []string, defLineIdx int) (int, bool) {
	for i := defLineIdx; i < len(lines) && i < defLineIdx+24; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasSuffix(trimmed, ":") {
			return i, true
		}
	}
	return 0, false
}

func pythonBlockEndLine(lines []string, defLineIdx, headerEndLine int) int {
	defIndent := indentLevel(lineIndent(lines[defLineIdx]))
	lastBody := headerEndLine
	for i := headerEndLine + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		if indentLevel(lineIndent(line)) <= defIndent {
			return lastBody + 1
		}
		lastBody = i
	}
	return lastBody + 1
}

func lineIndent(line string) string {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return line[:i]
}

func indentLevel(indent string) int {
	level := 0
	for _, c := range indent {
		switch c {
		case ' ':
			level++
		case '\t':
			level += 8
		}
	}
	return level
}
