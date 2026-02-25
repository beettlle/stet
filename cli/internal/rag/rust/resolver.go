// Package rust implements RAG symbol resolution for Rust files: extract symbols
// from hunk content and look up definitions via git grep.
package rust

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

	"stet/cli/internal/rag"
)

const (
	grepTimeout                = 5 * time.Second
	maxSymbolCandidates       = 30
	maxPrecedingCommentLines   = 5
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
	reFn       = regexp.MustCompile(`\bfn\s+(\w+)\s*[<(]`)
	reStruct   = regexp.MustCompile(`\bstruct\s+(\w+)\s*[{<]`)
	reEnum     = regexp.MustCompile(`\benum\s+(\w+)\s*[{]`)
	reTrait    = regexp.MustCompile(`\btrait\s+(\w+)\s*[{]`)
	reImplFor  = regexp.MustCompile(`\bimpl\s+.*?\s+for\s+(\w+)\s*[<{ ]`)
	reCall     = regexp.MustCompile(`\b(\w+)\s*\(`)
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
	// POSIX [[:space:]] for macOS/BSD git grep -E. Match: fn/struct/enum/trait Symbol, or "for Symbol" (impl).
	pattern := `(fn[[:space:]]+` + quoted + `[[:space:]]*[<(]|struct[[:space:]]+` + quoted + `[[:space:]]*[{<]|enum[[:space:]]+` + quoted + `[[:space:]]*[{]|trait[[:space:]]+` + quoted + `[[:space:]]*[{]|for[[:space:]]+` + quoted + `[[:space:]]*[<{ ])`
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
