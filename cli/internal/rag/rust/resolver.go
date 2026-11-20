// Package rust implements RAG symbol resolution for Rust files: extract symbols
// from hunk content and look up definitions via git grep.
package rust

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

var rustKeywords = map[string]bool{
	"fn": true, "struct": true, "enum": true, "trait": true, "impl": true,
	"match": true, "let": true, "mut": true, "pub": true, "use": true, "mod": true,
	"async": true, "move": true, "self": true, "Self": true, "true": true, "false": true,
	"type": true, "where": true, "if": true, "else": true, "for": true, "in": true,
	"loop": true, "while": true, "break": true, "continue": true, "return": true,
	"const": true, "static": true, "ref": true, "dyn": true, "unsafe": true,
	"extern": true, "crate": true, "super": true, "as": true, "box": true,
	"default": true, "union": true, "virtual": true, "macro": true,
}

var (
	reFn        = regexp.MustCompile(`\bfn\s+(\w+)\s*[<(]`)
	reStruct    = regexp.MustCompile(`\bstruct\s+(\w+)\s*[{<]`)
	reEnum      = regexp.MustCompile(`\benum\s+(\w+)\s*[{]`)
	reTrait     = regexp.MustCompile(`\btrait\s+(\w+)\s*[{]`)
	reImplFor   = regexp.MustCompile(`\bimpl\s+.*?\s+for\s+(\w+)\s*[<{ ]`)
	reCall      = regexp.MustCompile(`\b(\w+)\s*\(`)
	reTypeIdent = regexp.MustCompile(`\b([A-Z][A-Za-z0-9_]*)\b`)
)

// Resolver implements rag.Resolver for Rust.
type Resolver struct{}

func init() {
	rag.MustRegisterResolver(".rs", New())
}

// New returns a new Rust symbol resolver.
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
	regexes := []*regexp.Regexp{reFn, reStruct, reEnum, reTrait, reImplFor, reCall, reTypeIdent}
	for _, re := range regexes {
		for _, m := range re.FindAllStringSubmatch(hunkContent, -1) {
			if len(m) < 2 {
				continue
			}
			name := m[1]
			if rustKeywords[name] || seen[name] {
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

// symbolPattern returns the ERE matching Rust definitions of symbol.
func symbolPattern(symbol string) string {
	quoted := regexp.QuoteMeta(symbol)
	// POSIX [[:space:]] for macOS/BSD git grep -E. Match: fn/struct/enum/trait Symbol, or "for Symbol" (impl).
	return `(fn[[:space:]]+` + quoted + `[[:space:]]*[<(]|struct[[:space:]]+` + quoted + `[[:space:]]*[{<]|enum[[:space:]]+` + quoted + `[[:space:]]*[{]|trait[[:space:]]+` + quoted + `[[:space:]]*[{]|for[[:space:]]+` + quoted + `[[:space:]]*[<{ ])`
}

// readSignatureAndDoc reads the file at path, line (1-based). Signature = declaration line(s) up to { or ;.
// Docstring = preceding ///, //!, or /** */ lines (Rust doc comments).
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
		if strings.Contains(lines[i], "{") || strings.Contains(lines[i], ";") {
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
		if strings.HasPrefix(s, "///") {
			docLines = append([]string{strings.TrimSpace(s[3:])}, docLines...)
			continue
		}
		if strings.HasPrefix(s, "//!") {
			docLines = append([]string{strings.TrimSpace(s[3:])}, docLines...)
			continue
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
