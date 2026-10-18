package expand

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"stet/cli/internal/diff"
	"stet/cli/internal/jsts"
)

// maxJSTSImportLines caps the collected import block shown to the model.
const maxJSTSImportLines = 60

// jstsImportTokenShare splits the token budget when the import block is
// shown: the imports get at most 1/jstsImportTokenShare of maxTokens, and
// the enclosing function context gets the remainder (maxTokens minus that
// quarter).
const jstsImportTokenShare = 4

// minJSTSTruncateTokens is the smallest token budget for which
// truncateToTokens yields a non-degenerate truncation: under the chars/4
// heuristic a smaller budget leaves no room for truncateMarker next to the
// cut. Import blocks are only shown when their quarter share reaches it.
const minJSTSTruncateTokens = len(truncateMarker)/4 + 2

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

var (
	// jstsReExportStartRegex matches the start of a re-export statement:
	// `export * from "x"`, `export * as ns from "x"`, `export { a, b } from "x"`.
	jstsReExportStartRegex = regexp.MustCompile(`^export\s*(?:\*(?:\s+as\s+[\w$]+)?|\{)`)
	// jstsRequireStartRegex matches CommonJS top-level bindings:
	// `const fs = require("fs")` (also let/var).
	jstsRequireStartRegex = regexp.MustCompile(`^(?:const|let|var)\s+[A-Za-z_$][\w$]*\s*=\s*require\s*\(`)
	// jstsFromModuleRegex matches a module specifier after `from`; import and
	// re-export statements always name a string literal, so `from` inside a
	// comment or identifier does not count.
	jstsFromModuleRegex = regexp.MustCompile(`from\s*["'` + "`" + `]`)
	// jstsTrailingFromRegex matches a line that ends in `from`, whose module
	// specifier continues on the next line.
	jstsTrailingFromRegex = regexp.MustCompile(`\bfrom\s*$`)
)

// jstsImports is the top-of-file import block collected from a JS/TS source.
type jstsImports struct {
	block   string // import statements joined with newlines (capped at maxJSTSImportLines)
	endLine int    // 1-based source line of the last import statement (uncapped)
	found   bool
}

// extractJSTSImports collects the file's top-level import statements: ESM
// imports (`import x from "y"`, side-effect imports), re-exports (`export ...
// from`), CommonJS require bindings (`const x = require(...)`), and TypeScript
// import-equals (`import x = require(...)`). Multi-line statements are
// included whole. Scanning stops at the first line that is not blank, a
// comment, a directive ("use strict", shebang), or an import.
func extractJSTSImports(src []byte) jstsImports {
	lines := strings.Split(string(src), "\n")
	var stmts []string
	res := jstsImports{}
	for i := 0; i < len(lines); {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") ||
			strings.HasPrefix(trimmed, "*") || jstsIsDirective(trimmed) {
			i++
			continue
		}
		if !jstsIsImportStart(trimmed) {
			break // first top-level code line: imports end here
		}
		end, terminated := jstsStatementEnd(lines, i)
		if !terminated {
			break // unterminated statement: stop instead of consuming the rest of the file
		}
		joined := strings.Join(lines[i:end+1], "\n")
		if jstsReExportStartRegex.MatchString(trimmed) && !jstsFromModuleRegex.MatchString(joined) {
			break // re-export of local bindings, not an import: stop scanning
		}
		stmts = append(stmts, joined)
		res.endLine = end + 1
		res.found = true
		i = end + 1
	}
	if !res.found {
		return res
	}
	res.block = strings.Join(stmts, "\n")
	if n := strings.Count(res.block, "\n") + 1; n > maxJSTSImportLines {
		res.block = strings.Join(strings.Split(res.block, "\n")[:maxJSTSImportLines], "\n")
	}
	return res
}

// jstsIsDirective reports whether the line is a shebang or a "use strict"
// directive, which may legally precede imports.
func jstsIsDirective(trimmed string) bool {
	if strings.HasPrefix(trimmed, "#!") {
		return true
	}
	return trimmed == `"use strict"` || trimmed == `'use strict'` ||
		trimmed == `"use strict";` || trimmed == `'use strict';`
}

// jstsIsImportStart reports whether the line starts a top-level import
// statement (ESM import, re-export, require binding, or TS import-equal).
func jstsIsImportStart(trimmed string) bool {
	if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "import{") {
		return true
	}
	return jstsReExportStartRegex.MatchString(trimmed) || jstsRequireStartRegex.MatchString(trimmed)
}

// jstsStatementEnd returns the index of the last line of the (possibly
// multi-line) import statement starting at line index start. The statement
// continues while significant braces remain open. ok is false when the braces
// never close (unterminated statement); the caller then stops scanning rather
// than consuming the rest of the file as one statement.
func jstsStatementEnd(lines []string, start int) (end int, ok bool) {
	depth := 0
	for i := start; i < len(lines); i++ {
		depth += jstsBraceDelta(lines[i])
		if depth <= 0 {
			// `from` may sit on its own line before the module specifier.
			if jstsTrailingFromRegex.MatchString(lines[i]) && i+1 < len(lines) {
				return i + 1, true
			}
			return i, true
		}
	}
	return len(lines) - 1, false
}

// jstsBraceDelta counts the net significant braces on a line, skipping
// comments and string/template literals via skipJSTSToken.
func jstsBraceDelta(line string) int {
	delta := 0
	for i := 0; i < len(line); {
		switch line[i] {
		case '{':
			delta++
			i++
		case '}':
			delta--
			i++
		default:
			next, ok := skipJSTSToken(line, i)
			if !ok {
				return delta // fail open on unterminated token
			}
			if next == i {
				i++
			} else {
				i = next
			}
		}
	}
	return delta
}

func isJSTSExt(ext string) bool {
	return jsts.IsExt(ext)
}

func jstsFenceLang(filePath string) string {
	switch filepath.Ext(filePath) {
	case ".ts", ".tsx":
		return "typescript"
	default:
		return "javascript"
	}
}

// expandJSTSHunk enriches a hunk with enclosing function or class-method context
// for JavaScript/TypeScript files. When the file has top-level imports above the
// hunk, a `## File imports` section is prepended so the model does not report
// imports it cannot see as missing. Fail-open: returns hunk unchanged on any error.
func expandJSTSHunk(repoRoot string, hunk diff.Hunk, maxTokens int) (diff.Hunk, error) {
	start, end, ok := HunkLineRange(hunk)
	if !ok {
		return hunk, nil
	}

	src, _, ok := loadRepoSourceFile(repoRoot, hunk.FilePath)
	if !ok {
		return hunk, nil
	}

	imports := extractJSTSImports(src)
	// A hunk that overlaps the import block has no separate import context above it.
	showImports := imports.found && start > imports.endLine

	importBudget := 0
	if showImports && maxTokens > 0 {
		importBudget = maxTokens / jstsImportTokenShare
		if importBudget < minJSTSTruncateTokens {
			// Budget too small for a meaningful import block; fail open
			// without it and keep the full budget for the function context.
			showImports = false
			importBudget = 0
		} else {
			imports.block = truncateToTokens(imports.block, importBudget)
		}
	}
	lang := jstsFenceLang(hunk.FilePath)
	importSection := ""
	if showImports {
		importSection = "## File imports\n\n```" + lang + "\n" + imports.block + "\n```\n\n"
	}

	scope, ok := findEnclosingJSTSScope(src, start, end)
	if !ok {
		if !showImports {
			return hunk, nil
		}
		augmented := importSection + "## Diff hunk\n\n" + hunk.RawContent
		return diff.Hunk{
			FilePath:   hunk.FilePath,
			RawContent: hunk.RawContent,
			Context:    augmented,
		}, nil
	}

	funcSrc := extractLineRange(src, scope.startLine, scope.endLine)
	if funcSrc == "" {
		return hunk, nil
	}
	if maxTokens > 0 {
		funcSrc = truncateToTokens(funcSrc, maxTokens-importBudget)
	}

	augmented := importSection + "## Enclosing function context\n\n```" + lang + "\n" + funcSrc + "\n```\n\n## Diff hunk\n\n" + hunk.RawContent
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
	if relArrow >= 0 && !arrowInsideFunctionBody(src, chunk, declStart, relArrow) {
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

// arrowInsideFunctionBody reports whether the first "=>" in chunk (at relative
// offset relArrow) belongs to a callback inside the declaration's own body
// rather than to the declaration itself, e.g. `function f(ids) {
// ids.filter((id) => ok(id)); }`. That holds when the body brace opens before
// the arrow and closes after it. A brace pair that closes before the arrow
// (such as a TypeScript return type `(x): { a: number } => ...`) does not
// count, so arrow declarations keep their arrow-based scope.
func arrowInsideFunctionBody(src, chunk string, declStart, relArrow int) bool {
	openAbs := findFunctionBodyOpenBrace(chunk, declStart)
	arrowAbs := declStart + relArrow
	if openAbs < 0 || openAbs >= arrowAbs {
		return false
	}
	return findMatchingBrace(src, openAbs) > arrowAbs
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
