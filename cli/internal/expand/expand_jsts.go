package expand

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"stet/cli/internal/diff"
)

const (
	jsExt  = ".js"
	jsxExt = ".jsx"
	tsExt  = ".ts"
	tsxExt = ".tsx"
)

var jstsFuncDeclPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bfunction\s+[A-Za-z_$][\w$]*\s*\(`),
	regexp.MustCompile(`\basync\s+function\s+[A-Za-z_$][\w$]*\s*\(`),
	regexp.MustCompile(`\b(export\s+)?(default\s+)?function\s+[A-Za-z_$][\w$]*\s*\(`),
	regexp.MustCompile(`\b(const|let|var)\s+[A-Za-z_$][\w$]*\s*=\s*(async\s+)?function\b`),
	regexp.MustCompile(`\b(const|let|var)\s+[A-Za-z_$][\w$]*\s*=\s*(async\s+)?\([^)]*\)\s*=>`),
	regexp.MustCompile(`\b(const|let|var)\s+[A-Za-z_$][\w$]*\s*=\s*(async\s+)?[A-Za-z_$][\w$]*\s*=>`),
	regexp.MustCompile(`^\s*(?:(?:public|private|protected|static|async|readonly|override|abstract)\s+)*[A-Za-z_$][\w$]*\s*\([^)]*\)`),
	regexp.MustCompile(`^\s*(?:get|set)\s+[A-Za-z_$][\w$]*`),
}

func isJSTSExt(ext string) bool {
	switch ext {
	case jsExt, jsxExt, tsExt, tsxExt:
		return true
	default:
		return false
	}
}

func jstsFenceLang(filePath string) string {
	switch filepath.Ext(filePath) {
	case tsExt, tsxExt:
		return "typescript"
	default:
		return "javascript"
	}
}

// expandJSTSHunk enriches a hunk with enclosing function or class-method context
// for JavaScript/TypeScript files. Fail-open: returns hunk unchanged on any error.
func expandJSTSHunk(repoRoot string, hunk diff.Hunk, maxTokens int) (diff.Hunk, error) {
	start, end, ok := HunkLineRange(hunk)
	if !ok {
		return hunk, nil
	}

	src, _, ok := loadRepoSourceFile(repoRoot, hunk.FilePath)
	if !ok {
		return hunk, nil
	}

	scope, ok := findEnclosingJSTSScope(src, start, end)
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

	lang := jstsFenceLang(hunk.FilePath)
	augmented := "## Enclosing function context\n\n```" + lang + "\n" + funcSrc + "\n```\n\n## Diff hunk\n\n" + hunk.RawContent
	return diff.Hunk{
		FilePath:   hunk.FilePath,
		RawContent: hunk.RawContent,
		Context:    augmented,
	}, nil
}

type jstsScope struct {
	startLine int
	endLine   int
}

func findEnclosingJSTSScope(src []byte, startLine, endLine int) (jstsScope, bool) {
	text := string(src)
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && text != "" && strings.HasSuffix(text, "\n") {
		// strings.Split drops final empty element only when no trailing newline;
		// keep line numbers aligned with editor/git diff numbering.
	}

	var best *jstsScope
	bestSpan := -1

	for i, line := range lines {
		lineNum := i + 1
		if !looksLikeJSTSDecl(line) {
			continue
		}
		scopeStart, scopeEnd, ok := jstsDeclLineRange(text, lines, i)
		if !ok {
			continue
		}
		if scopeStart > startLine || endLine > scopeEnd {
			continue
		}
		span := scopeEnd - scopeStart + 1
		if best == nil || span < bestSpan {
			best = &jstsScope{startLine: scopeStart, endLine: scopeEnd}
			bestSpan = span
		}
		_ = lineNum // lineNum used implicitly via scopeStart
	}
	if best == nil {
		return jstsScope{}, false
	}
	return *best, true
}

func looksLikeJSTSDecl(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
		return false
	}
	if strings.HasPrefix(trimmed, "class ") || strings.HasPrefix(trimmed, "export class ") {
		return false
	}
	if strings.HasPrefix(trimmed, "if ") || strings.HasPrefix(trimmed, "if(") ||
		strings.HasPrefix(trimmed, "for ") || strings.HasPrefix(trimmed, "for(") ||
		strings.HasPrefix(trimmed, "while ") || strings.HasPrefix(trimmed, "while(") ||
		strings.HasPrefix(trimmed, "switch ") || strings.HasPrefix(trimmed, "switch(") {
		return false
	}
	if strings.Contains(line, "=>") {
		if strings.Contains(trimmed, "const ") || strings.Contains(trimmed, "let ") || strings.Contains(trimmed, "var ") {
			return true
		}
	}
	for _, re := range jstsFuncDeclPatterns {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

func jstsDeclLineRange(src string, lines []string, declLineIdx int) (startLine, endLine int, ok bool) {
	lineOffsets := lineStartOffsets(src)
	if declLineIdx < 0 || declLineIdx >= len(lines) || declLineIdx >= len(lineOffsets) {
		return 0, 0, false
	}
	declStart := lineOffsets[declLineIdx]

	searchEnd := declLineIdx + 12
	if searchEnd > len(lines) {
		searchEnd = len(lines)
	}
	chunk := strings.Join(lines[declLineIdx:searchEnd], "\n")

	relArrow := findSignificantToken(chunk, "=>")
	if relArrow >= 0 {
		afterArrow := chunk[relArrow+2:]
		relBrace := findSignificantByte(afterArrow, 0, '{')
		if relBrace >= 0 {
			openAbs := declStart + relArrow + 2 + relBrace
			closeAbs := findMatchingBrace(src, openAbs)
			if closeAbs < 0 {
				return 0, 0, false
			}
			return declLineIdx + 1, offsetToLine(src, closeAbs), true
		}
		endIdx := declLineIdx
		for i := declLineIdx; i < searchEnd; i++ {
			if strings.Contains(lines[i], ";") {
				endIdx = i
				break
			}
			endIdx = i
		}
		return declLineIdx + 1, endIdx + 1, true
	}

	openAbs := findFunctionBodyOpenBrace(chunk, declStart)
	if openAbs < 0 {
		return 0, 0, false
	}
	closeAbs := findMatchingBrace(src, openAbs)
	if closeAbs < 0 {
		return 0, 0, false
	}
	return declLineIdx + 1, offsetToLine(src, closeAbs), true
}

// findFunctionBodyOpenBrace locates the opening "{" of a function/method body,
// skipping parameter lists and destructuring in the declaration prefix.
func findFunctionBodyOpenBrace(chunk string, chunkStartOffset int) int {
	scanFrom := 0
	if parenEnd := findMatchingParen(chunk, scanFrom); parenEnd >= 0 {
		scanFrom = parenEnd + 1
	}
	if rel := findSignificantByte(chunk, scanFrom, '{'); rel >= 0 {
		return chunkStartOffset + rel
	}
	return -1
}

func findMatchingParen(s string, from int) int {
	open := findSignificantByte(s, from, '(')
	if open < 0 {
		return -1
	}
	depth := 0
	for i := open; i < len(s); {
		switch s[i] {
		case '(':
			depth++
			i++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
			i++
		default:
			next, ok := skipJSTSToken(s, i)
			if !ok {
				return -1
			}
			if next == i {
				i++
			} else {
				i = next
			}
		}
	}
	return -1
}

func lineStartOffsets(src string) []int {
	offsets := []int{0}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return offsets
}

func offsetToLine(src string, offset int) int {
	if offset < 0 {
		offset = 0
	}
	if offset > len(src) {
		offset = len(src)
	}
	line := 1
	for i := 0; i < offset; i++ {
		if src[i] == '\n' {
			line++
		}
	}
	return line
}

func extractLineRange(src []byte, startLine, endLine int) string {
	lines := strings.Split(string(src), "\n")
	if startLine < 1 || endLine < startLine || startLine > len(lines) {
		return ""
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	return strings.Join(lines[startLine-1:endLine], "\n")
}

func findSignificantByte(s string, from int, target byte) int {
	for i := from; i < len(s); {
		if s[i] == target {
			return i
		}
		next, ok := skipJSTSToken(s, i)
		if !ok {
			return -1
		}
		if next == i {
			i++
		} else {
			i = next
		}
	}
	return -1
}

func findSignificantToken(s, token string) int {
	for i := 0; i <= len(s)-len(token); {
		if strings.HasPrefix(s[i:], token) {
			before := byte(0)
			after := byte(0)
			if i > 0 {
				before = s[i-1]
			}
			if i+len(token) < len(s) {
				after = s[i+len(token)]
			}
			if !isIdentChar(before) && !isIdentChar(after) {
				return i
			}
		}
		next, ok := skipJSTSToken(s, i)
		if !ok {
			return -1
		}
		if next == i {
			i++
		} else {
			i = next
		}
	}
	return -1
}

func isIdentChar(b byte) bool {
	return b == '_' || b == '$' || unicode.IsLetter(rune(b)) || unicode.IsDigit(rune(b))
}

func findMatchingBrace(src string, openIdx int) int {
	if openIdx < 0 || openIdx >= len(src) || src[openIdx] != '{' {
		return -1
	}
	depth := 0
	for i := openIdx; i < len(src); {
		switch src[i] {
		case '{':
			depth++
			i++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
			i++
		default:
			next, ok := skipJSTSToken(src, i)
			if !ok {
				return -1
			}
			if next == i {
				i++
			} else {
				i = next
			}
		}
	}
	return -1
}

// skipJSTSToken advances past comments and string/template literals. Returns false on unterminated token.
func skipJSTSToken(s string, i int) (int, bool) {
	if i >= len(s) {
		return i, true
	}
	switch s[i] {
	case '/':
		if i+1 >= len(s) {
			return i + 1, true
		}
		switch s[i+1] {
		case '/':
			j := i + 2
			for j < len(s) && s[j] != '\n' {
				j++
			}
			return j, true
		case '*':
			j := i + 2
			for j+1 < len(s) {
				if s[j] == '*' && s[j+1] == '/' {
					return j + 2, true
				}
				j++
			}
			return 0, false
		default:
			return i + 1, true
		}
	case '"', '\'':
		quote := s[i]
		j := i + 1
		for j < len(s) {
			if s[j] == '\\' {
				j += 2
				continue
			}
			if s[j] == quote {
				return j + 1, true
			}
			j++
		}
		return 0, false
	case '`':
		j := i + 1
		for j < len(s) {
			if s[j] == '\\' {
				j += 2
				continue
			}
			if s[j] == '`' {
				return j + 1, true
			}
			if s[j] == '$' && j+1 < len(s) && s[j+1] == '{' {
				inner, ok := skipJSTSToken(s, j+1)
				if !ok {
					return 0, false
				}
				close := findMatchingBrace(s, inner)
				if close < 0 {
					return 0, false
				}
				j = close + 1
				continue
			}
			j++
		}
		return 0, false
	default:
		return i, true
	}
}
