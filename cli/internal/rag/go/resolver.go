// Package goresolver implements RAG symbol resolution for Go files: extract
// symbols from hunk content and look up definitions via git grep.
package goresolver

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"stet/cli/internal/rag"
)

const (
	maxSymbolCandidates      = 30
	maxPrecedingCommentLines = 5
)

// goKeywords that should not be looked up as symbols.
var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

// typeIdent matches capitalized type names (e.g. MyType, Context).
var typeIdent = regexp.MustCompile(`\b([A-Z][A-Za-z0-9_]*)\b`)

// callIdent matches lowercase identifier followed by ( (function call).
var callIdent = regexp.MustCompile(`\b([a-z][A-Za-z0-9_]*)\s*\(`)

// Resolver implements rag.Resolver for Go.
type Resolver struct{}

func init() {
	rag.MustRegisterResolver(".go", New())
}

// New returns a new Go symbol resolver.
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

func extractSymbols(hunkContent string) []string {
	seen := make(map[string]bool)
	var list []string
	for _, re := range []*regexp.Regexp{typeIdent, callIdent} {
		for _, m := range re.FindAllStringSubmatch(hunkContent, -1) {
			if len(m) < 2 {
				continue
			}
			name := m[1]
			if goKeywords[name] || seen[name] {
				continue
			}
			seen[name] = true
			list = append(list, name)
			if len(list) >= maxSymbolCandidates {
				return list
			}
		}
	}
	return list
}

// lookupDefinitions looks up all symbols with a single batched git grep
// process and reads signature + docstring for each found definition.
func lookupDefinitions(ctx context.Context, repoRoot, fromFile string, symbols []string, maxDefs int) ([]rag.Definition, error) {
	absRepo, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	pairs := make([]rag.SymbolPattern, len(symbols))
	for i, sym := range symbols {
		pairs[i] = rag.SymbolPattern{Symbol: sym, Pattern: symbolPattern(sym)}
	}
	matches, err := rag.BatchGrepDefinitions(ctx, absRepo, pairs)
	if err != nil {
		return nil, err
	}
	var defs []rag.Definition
	for _, m := range matches {
		if len(defs) >= maxDefs {
			break
		}
		if m == nil {
			continue
		}
		sig, doc := readSignatureAndDoc(m.AbsPath, m.Line, m.Content)
		if sig == "" {
			sig = strings.TrimSpace(m.Content)
		}
		defs = append(defs, rag.Definition{
			Symbol:    m.Symbol,
			File:      m.RelPath,
			Line:      m.Line,
			Signature: sig,
			Docstring: doc,
		})
	}
	return defs, nil
}

// symbolPattern returns the ERE matching Go definitions of symbol.
func symbolPattern(symbol string) string {
	// Match: func Symbol, type Symbol, var Symbol, const Symbol. Use POSIX classes
	// so git grep -E works on macOS/BSD (which do not support \b or \s).
	// Trailing anchor includes $ so symbols at end-of-line are matched.
	return `(func|type|var|const)[[:space:]]+` + regexp.QuoteMeta(symbol) + `([^a-zA-Z0-9_]|$)`
}

// readSignatureAndDoc reads the file at path, line (1-based), and returns
// the declaration line(s) as signature and preceding comment as docstring.
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
		if line >= lineNum+10 {
			break
		}
	}
	if lineNum < 1 || lineNum > len(lines) {
		return strings.TrimSpace(declarationLine), ""
	}
	// Signature: from declaration line until we have a complete line (or opening {).
	sigLine := lines[lineNum-1]
	var sigBuilder strings.Builder
	sigBuilder.WriteString(strings.TrimSpace(sigLine))
	for i := lineNum; i < len(lines) && i < lineNum+5; i++ {
		sigBuilder.WriteString("\n")
		sigBuilder.WriteString(lines[i])
		if strings.Contains(lines[i], "{") {
			break
		}
	}
	signature = strings.TrimSpace(sigBuilder.String())

	// Docstring: preceding // or /* */ lines
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
		if strings.HasPrefix(s, "/*") {
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

// capDefinitionsByTokens trims the defs list or content so estimated tokens <= maxTokens.
func capDefinitionsByTokens(defs []rag.Definition, maxTokens int) []rag.Definition {
	// Simple chars/4 heuristic; tokens package is in cli/internal/tokens.
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
