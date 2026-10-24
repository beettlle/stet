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
	// specifier matches a quoted module specifier after "from" or inside
	// require(...); its capture group holds the specifier text ("" when the
	// text is cut off before it or the require is dynamic).
	specifier = `(?:["']([^"']*)["'])?`

	// import { a, b as c } from "..."; import d, { e } from "..."
	// Group 1: brace list. Group 2: module specifier.
	importNamedRe = regexp.MustCompile(`\bimport\s+(?:type\s+)?(?:` + jsIdentPattern + `\s*,\s*)?\{([^}]*)\}\s*from\b\s*` + specifier)
	// import d from "..."; import d, { e } from "..."; import d, * as ns from "..."
	// Group 1: default binding. Groups 2-3: module specifier (after a comma
	// with a named or namespace clause, or directly after "from").
	importDefaultRe = regexp.MustCompile(`\bimport\s+(?:type\s+)?(` + jsIdentPattern + `)\s*(?:,(?:\s*(?:\{[^}]*\}\s*|\*\s*as\s+` + jsIdentPattern + `\s+)?from\b\s*` + specifier + `)?|from\b\s*` + specifier + `)`)
	// import * as ns from "..."; import d, * as ns from "..."
	// Group 1: namespace binding. Group 2: module specifier.
	importNamespaceRe = regexp.MustCompile(`\bimport\s+(?:` + jsIdentPattern + `\s*,\s*)?\*\s*as\s+(` + jsIdentPattern + `)(?:\s+from\b\s*` + specifier + `)?`)
	// const { a, b: c } = require("...")
	// Group 1: brace list. Group 2: module specifier.
	requireDestructRe = regexp.MustCompile(`\b(?:const|let|var)\s+\{([^}]*)\}\s*=\s*require\s*\(\s*` + specifier + `\s*\)?`)
	// const x = require("...")
	// Group 1: local binding. Group 2: module specifier.
	requireDefaultRe = regexp.MustCompile(`\b(?:const|let|var)\s+(` + jsIdentPattern + `)\s*=\s*require\s*\(\s*` + specifier + `\s*\)?`)
	// Bare identifier (whole string).
	bareIdentRe = regexp.MustCompile(`^` + jsIdentPattern + `$`)
)

// sourceDefault is the exported source name for default-style bindings: ESM
// "import d from m" and whole-module CommonJS "const x = require(m)".
const sourceDefault = "default"

// importBinding is one import/require-bound local name together with the
// module specifier it comes from and the exported source name it is bound to
// in that module. Namespace bindings ("* as ns") never get a binding: no
// single exported name corresponds to them, so they resolve via git grep.
type importBinding struct {
	local  string // local name bound in the importing file
	source string // exported name in the module; sourceDefault for default-style
	module string // module specifier as written ("./discover.mjs", "lodash")
}

// Resolver implements rag.Resolver for JavaScript and TypeScript.
type Resolver struct{}

func init() {
	for _, ext := range jsts.Extensions() {
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
	defs, err := lookupDefinitions(ctx, repoRoot, filePath, hunkContent, symbols, maxDefs)
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

// importBindings parses import and require statements in src and returns one
// binding per local name that carries a module specifier, in the same form
// order as importBoundNames. Unlike importBoundNames it drops bindings with
// no usable specifier (dynamic require(m), cut-off hunks) and namespace
// bindings — neither can be resolved in a module, so they fall back to git
// grep. bindingListPairs (not bindingListNames) is used here because both
// sides of an alias must be bare identifiers for module lookup.
func importBindings(src string) []importBinding {
	var bindings []importBinding
	for _, m := range importNamedRe.FindAllStringSubmatch(src, -1) {
		if m[2] == "" {
			continue
		}
		for _, pair := range bindingListPairs(m[1], " as ") {
			bindings = append(bindings, importBinding{local: pair[1], source: pair[0], module: m[2]})
		}
	}
	for _, m := range importDefaultRe.FindAllStringSubmatch(src, -1) {
		spec := m[2]
		if spec == "" {
			spec = m[3]
		}
		if spec != "" {
			bindings = append(bindings, importBinding{local: m[1], source: sourceDefault, module: spec})
		}
	}
	for _, m := range requireDestructRe.FindAllStringSubmatch(src, -1) {
		if m[2] == "" {
			continue
		}
		for _, pair := range bindingListPairs(m[1], ":") {
			bindings = append(bindings, importBinding{local: pair[1], source: pair[0], module: m[2]})
		}
	}
	for _, m := range requireDefaultRe.FindAllStringSubmatch(src, -1) {
		if m[2] != "" {
			bindings = append(bindings, importBinding{local: m[1], source: sourceDefault, module: m[2]})
		}
	}
	return bindings
}

// normalizeBindingItem normalizes one comma-separated item of an import or
// destructuring brace list: strips diff markers, collapses whitespace
// (including newlines, so "b\n+  as c" matches the alias separator), and
// removes a trailing "= default" initializer.
func normalizeBindingItem(item string) string {
	item = strings.Join(strings.Fields(stripDiffMarkers(item)), " ")
	if i := strings.Index(item, "="); i >= 0 {
		item = item[:i]
	}
	return item
}

// bindingListPairs parses the inside of an import/destructuring brace list
// ("a, b as c" or "a, b: c") into (source, local) name pairs: the exported
// source name first, the local alias second. Items whose either side is not
// a bare identifier (string export names, computed keys) are skipped.
func bindingListPairs(list, aliasSep string) [][2]string {
	var pairs [][2]string
	for _, item := range strings.Split(list, ",") {
		item = normalizeBindingItem(item)
		source, local := item, item
		if i := strings.Index(item, aliasSep); i >= 0 {
			source, local = item[:i], item[i+len(aliasSep):]
		}
		source = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(source), "type "))
		local = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(local), "type "))
		if bareIdentRe.MatchString(source) && bareIdentRe.MatchString(local) {
			pairs = append(pairs, [2]string{source, local})
		}
	}
	return pairs
}

// bindingListNames parses the inside of an import/destructuring brace list
// ("a, b as c" or "a, b: c = 1") and returns the local names. aliasSep is the
// token that separates the source name from the local alias. Unlike
// bindingListPairs only the local side must be a bare identifier.
func bindingListNames(list, aliasSep string) []string {
	var names []string
	for _, item := range strings.Split(list, ",") {
		item = normalizeBindingItem(item)
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

func lookupDefinitions(ctx context.Context, repoRoot, fromFile, hunkContent string, symbols []string, maxDefs int) ([]rag.Definition, error) {
	absRepo, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	bindings := moduleBindings(hunkContent, absRepo, fromFile)
	seen := make(map[string]bool)
	var defs []rag.Definition
	for _, sym := range symbols {
		if len(defs) >= maxDefs {
			break
		}
		if seen[sym] {
			continue
		}
		// Import-bound names resolve in the module they come from first;
		// git grep is the fallback for everything else and every failure.
		if def, ok := resolveImportedDefinition(absRepo, fromFile, sym, bindings[sym]); ok {
			seen[sym] = true
			defs = append(defs, def)
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

// moduleBindings maps local import/require-bound names to their module
// bindings for definition lookup. Bindings come from the edited file's import
// block (read from disk at fromFile) and from the hunk itself — a hunk that
// edits an import line wins over the stale file content. Names without a
// usable module binding are absent from the map and resolve via git grep.
func moduleBindings(hunkContent, absRepo, fromFile string) map[string]importBinding {
	bindings := make(map[string]importBinding)
	absFile := filepath.Join(absRepo, filepath.FromSlash(fromFile))
	if pathInsideRepo(absRepo, absFile) {
		if src, err := os.ReadFile(absFile); err == nil {
			for _, b := range importBindings(string(src)) {
				bindings[b.local] = b
			}
		}
	}
	for _, b := range importBindings(hunkContent) {
		bindings[b.local] = b
	}
	return bindings
}

// resolveImportedDefinition resolves an import-bound symbol to its exported
// declaration in the module it was imported from. Only relative specifiers
// ("./x", "../y") are resolved; bare package specifiers ("lodash",
// "node:fs") return false so the caller falls back to git grep. Every
// failure — missing module file, no matching export, unreadable path —
// returns false: the lookup never fails the review.
func resolveImportedDefinition(absRepo, fromFile, symbol string, b importBinding) (rag.Definition, bool) {
	if b.module == "" || b.source == "" {
		return rag.Definition{}, false
	}
	if !strings.HasPrefix(b.module, "./") && !strings.HasPrefix(b.module, "../") {
		return rag.Definition{}, false
	}
	fromDir := filepath.Dir(filepath.Join(absRepo, filepath.FromSlash(fromFile)))
	modulePath := resolveModulePath(absRepo, fromDir, b.module)
	if modulePath == "" {
		return rag.Definition{}, false
	}
	line, decl, ok := findExportedDecl(modulePath, b.source)
	if !ok {
		return rag.Definition{}, false
	}
	relPath, err := filepath.Rel(absRepo, modulePath)
	if err != nil {
		return rag.Definition{}, false
	}
	sig, doc := readSignatureAndDoc(modulePath, line, decl)
	if sig == "" {
		sig = strings.TrimSpace(decl)
	}
	return rag.Definition{
		Symbol:    symbol,
		File:      filepath.ToSlash(relPath),
		Line:      line,
		Signature: sig,
		Docstring: doc,
	}, true
}

// resolveModulePath resolves a relative module specifier against fromDir the
// way Node resolves file modules: the exact path, then the path with each
// JS/TS extension appended (extensionless specifiers), then index with each
// extension inside the directory (directory imports). The result must be a
// regular file inside repoRoot — candidates that escape via ".." or via
// symlinks are rejected — otherwise "" is returned.
func resolveModulePath(repoRoot, fromDir, specifier string) string {
	if !strings.HasPrefix(specifier, "./") && !strings.HasPrefix(specifier, "../") {
		return ""
	}
	target := filepath.Clean(filepath.Join(fromDir, specifier))
	candidates := []string{target}
	if filepath.Ext(target) == "" {
		for _, ext := range jsts.Extensions() {
			candidates = append(candidates, target+ext)
		}
	}
	for _, ext := range jsts.Extensions() {
		candidates = append(candidates, filepath.Join(target, "index"+ext))
	}
	for _, cand := range candidates {
		info, err := os.Stat(cand)
		if err != nil || info.IsDir() {
			continue
		}
		if !pathInsideRepo(repoRoot, cand) {
			continue
		}
		return cand
	}
	return ""
}

// pathInsideRepo reports whether path lies inside repoRoot after cleaning
// and after resolving symlinks in both, so a module (or importing file)
// outside the repository is never read even when reached through a symlink.
// Both paths are evaluated so a repo root reached through a symlink (e.g.
// macOS /tmp) still compares correctly.
func pathInsideRepo(repoRoot, path string) bool {
	if !withinClean(repoRoot, path) {
		return false
	}
	realRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return false
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	return withinClean(realRoot, realPath)
}

// withinClean reports whether cleaned path lies inside cleaned root (root
// itself counts as inside; callers require a regular file separately).
func withinClean(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Export declaration patterns matched per line of a resolved module file.
// Lists are matched single-line; multi-line export lists fall back to git grep.
var (
	// export function name / export async function name / export const name /
	// export class name (also with "default": export default function name)
	// — group 1 is the declared name.
	exportDeclRe = regexp.MustCompile(`^\s*export\s+(?:default\s+)?(?:async\s+)?(?:function|const|class)\b\s*\*?\s*(` + jsIdentPattern + `)`)
	// export default ...
	exportDefaultRe = regexp.MustCompile(`^\s*export\s+default\b`)
	// export { a, b as c } / export type { A } — group 1 is the brace list.
	exportListRe = regexp.MustCompile(`^\s*export\s+(?:type\s+)?\{([^}]*)\}`)
	// export { a } from "..." — a re-export, not a declaration.
	exportListFromRe = regexp.MustCompile(`^\s*export\s+(?:type\s+)?\{[^}]*\}\s*from\b`)
	// module.exports.name = / exports.name = — group 1 is the exported name.
	cjsNamedExportRe = regexp.MustCompile(`^\s*(?:module\.)?exports\.` + `(` + jsIdentPattern + `)\s*=`)
	// module.exports = ... — whole-object export for default-style requires.
	cjsModuleExportsRe = regexp.MustCompile(`^\s*module\.exports\s*=`)
)

// findExportedDecl scans a module file for the exported declaration of source
// and returns its 1-based line number and the line content. Supported forms:
// "export function|const|class name", single-line "export { ... name ... }"
// lists, "export default" for default-style imports, and CommonJS
// "module.exports.name =" / "exports.name =" (whole-object
// "module.exports =" for default-style requires). Re-export lists
// ("export { x } from ...") are skipped. ok is false when nothing matches or
// the file cannot be read.
func findExportedDecl(absPath, source string) (line int, decl string, ok bool) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return 0, "", false
	}
	for i, text := range strings.Split(string(data), "\n") {
		if exportListFromRe.MatchString(text) {
			continue
		}
		if source == sourceDefault {
			if exportDefaultRe.MatchString(text) || cjsModuleExportsRe.MatchString(text) {
				return i + 1, text, true
			}
			continue
		}
		if m := exportDeclRe.FindStringSubmatch(text); m != nil && m[1] == source {
			return i + 1, text, true
		}
		if m := exportListRe.FindStringSubmatch(text); m != nil {
			for _, pair := range bindingListPairs(m[1], " as ") {
				// In an export list the exported name follows " as ".
				if pair[1] == source {
					return i + 1, text, true
				}
			}
		}
		if m := cjsNamedExportRe.FindStringSubmatch(text); m != nil && m[1] == source {
			return i + 1, text, true
		}
	}
	return 0, "", false
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
