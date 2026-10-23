// Package js implements RAG symbol resolution for JavaScript and TypeScript
// files: extract symbols from hunk content and look up definitions via git grep.
package js

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"stet/cli/internal/jsts"
	"stet/cli/internal/rag"
)

const (
	grepTimeout              = 5 * time.Second
	maxSymbolCandidates      = 30
	maxPrecedingCommentLines = 5
)

var jsKeywords = map[string]bool{
	"async": true, "await": true, "break": true, "case": true, "catch": true,
	"class": true, "const": true, "continue": true, "debugger": true,
	"default": true, "delete": true, "do": true, "else": true, "export": true,
	"extends": true, "false": true, "finally": true, "for": true, "function": true,
	"if": true, "import": true, "in": true, "instanceof": true, "let": true,
	"new": true, "null": true, "return": true, "super": true, "switch": true,
	"this": true, "throw": true, "true": true, "try": true, "typeof": true,
	"var": true, "void": true, "while": true, "with": true, "yield": true,
	"interface": true, "type": true, "implements": true, "enum": true,
}

var funcIdent = regexp.MustCompile(`\bfunction\s+(\w+)\s*\(`)
var typeIdent = regexp.MustCompile(`\b([A-Z][A-Za-z0-9_]*)\b`)
var callIdent = regexp.MustCompile(`\b([a-zA-Z_$][A-Za-z0-9_$]*)\s*\(`)

// jsBuiltinMethods lists common prototype and static method names (Array,
// String, Map/Set, Promise, Object, console, ...). They are built-ins only
// when used as member calls ("arr.filter(", "m.get(", "console.log("); the
// same spellings are legitimate repo function names, so bare occurrences of
// these names are kept as candidates.
var jsBuiltinMethods = map[string]bool{
	// Array / iterable prototype
	"filter": true, "map": true, "sort": true, "push": true, "pop": true,
	"shift": true, "unshift": true, "splice": true, "slice": true, "concat": true,
	"join": true, "reverse": true, "includes": true, "indexOf": true,
	"lastIndexOf": true, "forEach": true, "reduce": true, "reduceRight": true,
	"find": true, "findIndex": true, "findLast": true, "findLastIndex": true,
	"some": true, "every": true, "flat": true, "flatMap": true, "fill": true,
	"at": true, "keys": true, "values": true, "entries": true,
	// Map / Set prototype
	"has": true, "get": true, "set": true, "add": true, "clear": true,
	// Promise
	"then": true, "allSettled": true, "race": true,
	// String prototype
	"split": true, "trim": true, "trimStart": true, "trimEnd": true,
	"replace": true, "replaceAll": true, "match": true, "matchAll": true,
	"startsWith": true, "endsWith": true, "toLowerCase": true, "toUpperCase": true,
	"padStart": true, "padEnd": true, "charAt": true, "charCodeAt": true,
	"substring": true, "substr": true,
	// Object / Function prototype and statics
	"toString": true, "valueOf": true, "hasOwnProperty": true, "call": true,
	"apply": true, "bind": true, "assign": true, "freeze": true, "isArray": true,
	"stringify": true,
	// console
	"log": true, "warn": true, "error": true, "info": true, "debug": true,
}

// jsBuiltinGlobals lists common built-in global identifiers. They rarely have
// repo definitions, so they are always skipped to save candidate slots —
// including when imported or required under the same spelling.
var jsBuiltinGlobals = map[string]bool{
	"require": true, "console": true, "undefined": true, "NaN": true,
	"Infinity": true, "parseInt": true, "parseFloat": true, "setTimeout": true,
	"clearTimeout": true, "setInterval": true, "clearInterval": true,
	"Object": true, "Array": true, "String": true, "Number": true, "Boolean": true,
	"Promise": true, "Map": true, "Set": true, "WeakMap": true, "WeakSet": true,
	"JSON": true, "Math": true, "Date": true, "Error": true, "TypeError": true,
	"RangeError": true, "RegExp": true, "Symbol": true,
}

const jsIdentPattern = `[A-Za-z_$][A-Za-z0-9_$]*`

var (
	// import { a, b as c } from "..."; import d, { e } from "..."
	importNamedRe = regexp.MustCompile(`\bimport\s+(?:type\s+)?(?:` + jsIdentPattern + `\s*,\s*)?\{([^}]*)\}\s*from\b`)
	// import d from "..."; import d, { e } from "..."; import d, * as ns from "..."
	importDefaultRe = regexp.MustCompile(`\bimport\s+(?:type\s+)?(` + jsIdentPattern + `)\s*(?:,|from\b)`)
	// import * as ns from "..."; import d, * as ns from "..."
	importNamespaceRe = regexp.MustCompile(`\bimport\s+(?:` + jsIdentPattern + `\s*,\s*)?\*\s*as\s+(` + jsIdentPattern + `)`)
	// const { a, b: c } = require("...")
	requireDestructRe = regexp.MustCompile(`\b(?:const|let|var)\s+\{([^}]*)\}\s*=\s*require\s*\(`)
	// const x = require("...")
	requireDefaultRe = regexp.MustCompile(`\b(?:const|let|var)\s+(` + jsIdentPattern + `)\s*=\s*require\s*\(`)
	// Bare identifier (whole string).
	bareIdentRe = regexp.MustCompile(`^` + jsIdentPattern + `$`)
)

// Resolver implements rag.Resolver for JavaScript and TypeScript.
type Resolver struct{}

func init() {
	for _, ext := range jsts.Extensions {
		rag.MustRegisterResolver(ext, New())
	}
}

// New returns a new JavaScript/TypeScript symbol resolver.
func New() *Resolver {
	return &Resolver{}
}

// ResolveSymbols extracts symbols from hunkContent and looks up their
// definitions in the repo. Returns up to opts.MaxDefinitions; total size
// may be capped by opts.MaxTokens.
func (r *Resolver) ResolveSymbols(ctx context.Context, repoRoot, filePath, hunkContent string, opts rag.ResolveOptions) ([]rag.Definition, error) {
	symbols := extractSymbols(hunkContent)
	if len(symbols) == 0 {
		return nil, nil
	}
	maxDefs := opts.MaxDefinitions
	if maxDefs <= 0 {
		maxDefs = 10
	}
	defs, err := lookupDefinitions(ctx, repoRoot, filePath, symbols, maxDefs)
	if err != nil || len(defs) == 0 {
		return nil, err
	}
	if opts.MaxTokens > 0 {
		defs = capDefinitionsByTokens(defs, opts.MaxTokens)
	}
	return defs, nil
}

// extractSymbols returns candidate symbol names from hunkContent in priority
// order: import/require-bound local names, then function declarations, type
// identifiers and callees, then bare identifiers passed as call arguments.
// Keywords and jsBuiltinGlobals are always skipped; jsBuiltinMethods names are
// skipped only when the occurrence is a member call (".name("). The list is
// deduplicated and capped at maxSymbolCandidates.
func extractSymbols(hunkContent string) []string {
	seen := make(map[string]bool)
	var list []string
	add := func(name string, memberCall bool) bool {
		if name == "" || jsKeywords[name] || jsBuiltinGlobals[name] || seen[name] {
			return len(list) >= maxSymbolCandidates
		}
		if memberCall && jsBuiltinMethods[name] {
			return len(list) >= maxSymbolCandidates
		}
		seen[name] = true
		list = append(list, name)
		return len(list) >= maxSymbolCandidates
	}
	// Import/require-bound names are repo symbols by construction, so they are
	// never dropped for being in the member-method set.
	for _, name := range importBoundNames(hunkContent) {
		if add(name, false) {
			return list
		}
	}
	for _, re := range []*regexp.Regexp{funcIdent, typeIdent} {
		for _, m := range re.FindAllStringSubmatch(hunkContent, -1) {
			if len(m) < 2 {
				continue
			}
			if add(m[1], false) {
				return list
			}
		}
	}
	for _, loc := range callIdent.FindAllStringSubmatchIndex(hunkContent, -1) {
		if add(hunkContent[loc[2]:loc[3]], isMemberCall(hunkContent, loc[2])) {
			return list
		}
	}
	// Bare call arguments are not member accesses.
	for _, name := range callArgumentIdents(hunkContent) {
		if add(name, false) {
			return list
		}
	}
	return list
}

// isMemberCall reports whether the identifier starting at start in src is part
// of a member access, i.e. preceded by "." with optional whitespace between.
func isMemberCall(src string, start int) bool {
	i := start
	for i > 0 && (src[i-1] == ' ' || src[i-1] == '\t' || src[i-1] == '\r' || src[i-1] == '\n') {
		i--
	}
	return i > 0 && src[i-1] == '.'
}

// importBoundNames returns local binding names introduced by import and
// require statements in src, in source order per statement form.
func importBoundNames(src string) []string {
	var names []string
	for _, m := range importNamedRe.FindAllStringSubmatch(src, -1) {
		names = append(names, bindingListNames(m[1], " as ")...)
	}
	for _, re := range []*regexp.Regexp{importDefaultRe, importNamespaceRe} {
		for _, m := range re.FindAllStringSubmatch(src, -1) {
			names = append(names, m[1])
		}
	}
	for _, m := range requireDestructRe.FindAllStringSubmatch(src, -1) {
		names = append(names, bindingListNames(m[1], ":")...)
	}
	for _, m := range requireDefaultRe.FindAllStringSubmatch(src, -1) {
		names = append(names, m[1])
	}
	return names
}

// bindingListNames parses the inside of an import/destructuring brace list
// ("a, b as c" or "a, b: c = 1") and returns the local names. aliasSep is the
// token that separates the source name from the local alias.
func bindingListNames(list, aliasSep string) []string {
	var names []string
	for _, item := range strings.Split(list, ",") {
		// Normalize whitespace (including newlines and diff markers) so
		// "b\n+  as c" matches the alias separator.
		item = strings.Join(strings.Fields(stripDiffMarkers(item)), " ")
		if i := strings.Index(item, "="); i >= 0 {
			item = item[:i]
		}
		if i := strings.Index(item, aliasSep); i >= 0 {
			item = item[i+len(aliasSep):]
		}
		item = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(item), "type "))
		if bareIdentRe.MatchString(item) {
			names = append(names, item)
		}
	}
	return names
}

// callArgumentIdents returns bare identifiers passed as top-level arguments
// to calls in src (e.g. compareTaskIds in arr.sort(compareTaskIds)). Literals,
// member expressions, and complex expressions are skipped, as are parameter
// lists of function/method declarations and arrow functions.
func callArgumentIdents(src string) []string {
	var names []string
	for _, loc := range callIdent.FindAllStringSubmatchIndex(src, -1) {
		if jsKeywords[src[loc[2]:loc[3]]] || followsFunctionKeyword(src[:loc[2]]) {
			continue
		}
		args, closeIdx := splitCallArgs(src, loc[1])
		if closeIdx < 0 || isDeclarationBody(src[closeIdx+1:]) {
			continue
		}
		for _, a := range args {
			a = stripDiffMarkers(a)
			if bareIdentRe.MatchString(a) {
				names = append(names, a)
			}
		}
	}
	return names
}

// splitCallArgs scans src from start (just after an opening parenthesis) and
// returns the top-level comma-separated arguments plus the index of the
// matching closing parenthesis. String and template literals are skipped.
// closeIdx is -1 when the call is not closed within src.
func splitCallArgs(src string, start int) (args []string, closeIdx int) {
	depth := 0
	argStart := start
	for i := start; i < len(src); i++ {
		switch c := src[i]; c {
		case '"', '\'', '`':
			i = skipStringLiteral(src, i)
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth == 0 {
				if c != ')' {
					return nil, -1
				}
				return append(args, src[argStart:i]), i
			}
			depth--
		case ',':
			if depth == 0 {
				args = append(args, src[argStart:i])
				argStart = i + 1
			}
		}
	}
	return nil, -1
}

// skipStringLiteral returns the index of the closing quote for the literal
// opening at src[i], or len(src)-1 if it is unterminated.
func skipStringLiteral(src string, i int) int {
	quote := src[i]
	for j := i + 1; j < len(src); j++ {
		switch src[j] {
		case '\\':
			j++
		case quote:
			return j
		}
	}
	return len(src) - 1
}

// followsFunctionKeyword reports whether before (text preceding a callee
// name) ends with "function" or "function*", i.e. the match is a declaration.
func followsFunctionKeyword(before string) bool {
	before = strings.TrimRight(before, " \t\r\n")
	before = strings.TrimRight(strings.TrimSuffix(before, "*"), " \t\r\n")
	if !strings.HasSuffix(before, "function") {
		return false
	}
	rest := before[:len(before)-len("function")]
	return rest == "" || !isIdentByte(rest[len(rest)-1])
}

func isIdentByte(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// isDeclarationBody reports whether rest (text after a closing parenthesis)
// starts a function body or arrow, meaning the parenthesized list was a
// parameter list rather than call arguments.
// Only the leading whitespace (and diff markers after newlines) is scanned.
func isDeclarationBody(rest string) bool {
	i := 0
	for i < len(rest) {
		switch rest[i] {
		case ' ', '\t', '\r':
			i++
		case '\n':
			i++
			if i < len(rest) && (rest[i] == '+' || rest[i] == '-') {
				i++
			}
		default:
			rest = rest[i:]
			return strings.HasPrefix(rest, "{") || strings.HasPrefix(rest, "=>")
		}
	}
	return false
}

// stripDiffMarkers removes a leading unified-diff marker (+, -, or space)
// from every line after the first in s, then trims surrounding whitespace.
// This lets multi-line argument lists from raw hunks be recognized.
func stripDiffMarkers(s string) string {
	if !strings.Contains(s, "\n") {
		return strings.TrimSpace(s)
	}
	lines := strings.Split(s, "\n")
	for i := 1; i < len(lines); i++ {
		if l := lines[i]; l != "" && (l[0] == '+' || l[0] == '-' || l[0] == ' ') {
			lines[i] = l[1:]
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func lookupDefinitions(ctx context.Context, repoRoot, fromFile string, symbols []string, maxDefs int) ([]rag.Definition, error) {
	absRepo, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var defs []rag.Definition
	for _, sym := range symbols {
		if len(defs) >= maxDefs {
			break
		}
		if seen[sym] {
			continue
		}
		path, line, content, err := gitGrepSymbol(ctx, absRepo, sym)
		if err != nil || path == "" {
			continue
		}
		seen[sym] = true
		relPath, _ := filepath.Rel(absRepo, path)
		relPath = filepath.ToSlash(relPath)
		sig, doc := readSignatureAndDoc(path, line, content)
		if sig == "" {
			sig = strings.TrimSpace(content)
		}
		defs = append(defs, rag.Definition{
			Symbol:    sym,
			File:      relPath,
			Line:      line,
			Signature: sig,
			Docstring: doc,
		})
	}
	return defs, nil
}

func gitGrepSymbol(ctx context.Context, repoRoot, symbol string) (absPath string, line int, lineContent string, err error) {
	quoted := regexp.QuoteMeta(symbol)
	// Use POSIX [[:space:]] and [^a-zA-Z0-9_] so git grep -E works on macOS/BSD.
	pattern := `(function[[:space:]]+` + quoted + `[[:space:]]*\(|const[[:space:]]+` + quoted + `[[:space:]]*=|class[[:space:]]+` + quoted + `[^a-zA-Z0-9_]|interface[[:space:]]+` + quoted + `[^a-zA-Z0-9_])`
	ctx, cancel := context.WithTimeout(ctx, grepTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "grep", "-n", "-E", pattern)
	cmd.Dir = repoRoot
	cmd.Env = minimalEnv(repoRoot)
	out, err := cmd.Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 {
			return "", 0, "", nil
		}
		return "", 0, "", err
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return "", 0, "", nil
	}
	first := strings.SplitN(trimmed, "\n", 2)[0]
	idx := strings.Index(first, ":")
	if idx == -1 {
		return "", 0, "", nil
	}
	path := first[:idx]
	if path == "" || strings.Contains(path, "..") {
		return "", 0, "", nil
	}
	rest := first[idx+1:]
	idx2 := strings.Index(rest, ":")
	if idx2 == -1 {
		return "", 0, "", nil
	}
	lineno, errParse := strconv.Atoi(rest[:idx2])
	if errParse != nil || lineno < 1 {
		return "", 0, "", nil
	}
	lineContent = rest[idx2+1:]
	absPath = filepath.Join(repoRoot, path)
	return absPath, lineno, lineContent, nil
}

func minimalEnv(repoRoot string) []string {
	gitDir := filepath.Join(repoRoot, ".git")
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_DIR=" + gitDir,
		"GIT_WORK_TREE=" + repoRoot,
	}
}

// readSignatureAndDoc reads the file at path, line (1-based). Signature up to { or =>.
// Docstring = preceding //, /* */, or /** */ (JSDoc).
func readSignatureAndDoc(absPath string, lineNum int, declarationLine string) (signature, docstring string) {
	f, err := os.Open(absPath)
	if err != nil {
		return declarationLine, ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var lines []string
	line := 0
	for sc.Scan() {
		line++
		lines = append(lines, sc.Text())
		if line >= lineNum+15 {
			break
		}
	}
	if lineNum < 1 || lineNum > len(lines) {
		return strings.TrimSpace(declarationLine), ""
	}
	sigLine := lines[lineNum-1]
	var sigBuilder strings.Builder
	sigBuilder.WriteString(strings.TrimSpace(sigLine))
	for i := lineNum; i < len(lines) && i < lineNum+5; i++ {
		sigBuilder.WriteString("\n")
		sigBuilder.WriteString(lines[i])
		if strings.Contains(lines[i], "{") || strings.Contains(lines[i], "=>") {
			break
		}
	}
	signature = strings.TrimSpace(sigBuilder.String())

	var docLines []string
	for i := lineNum - 2; i >= 0 && i >= lineNum-1-maxPrecedingCommentLines; i-- {
		s := strings.TrimSpace(lines[i])
		if s == "" {
			break
		}
		if strings.HasPrefix(s, "//") {
			docLines = append([]string{strings.TrimSpace(s[2:])}, docLines...)
			continue
		}
		if strings.HasPrefix(s, "/*") || strings.HasPrefix(s, "/**") {
			docLines = append([]string{s}, docLines...)
			break
		}
		break
	}
	if len(docLines) > 0 {
		docstring = strings.Join(docLines, "\n")
	}
	return signature, docstring
}

func capDefinitionsByTokens(defs []rag.Definition, maxTokens int) []rag.Definition {
	estimate := func(s string) int {
		return (len(s) + 3) / 4
	}
	used := 0
	for i := range defs {
		d := &defs[i]
		n := estimate(d.Signature) + estimate(d.Docstring)
		if used+n > maxTokens && i > 0 {
			return defs[:i]
		}
		used += n
	}
	return defs
}
