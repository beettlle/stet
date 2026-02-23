// Package python implements RAG symbol resolution for Python files: extract
// symbols from hunk content and look up definitions via git grep.
package python

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
	grepTimeout             = 5 * time.Second
	maxSymbolCandidates     = 30
	maxPrecedingCommentLines = 5
)

var pythonKeywords = map[string]bool{
	"and": true, "as": true, "assert": true, "async": true, "await": true,
	"break": true, "class": true, "continue": true, "def": true, "del": true,
	"elif": true, "else": true, "except": true, "False": true, "finally": true,
	"for": true, "from": true, "global": true, "if": true, "import": true,
	"in": true, "is": true, "lambda": true, "None": true, "nonlocal": true,
	"not": true, "or": true, "pass": true, "raise": true, "return": true,
	"True": true, "try": true, "while": true, "with": true, "yield": true,
}

var defIdent = regexp.MustCompile(`\bdef\s+(\w+)\s*\(`)
var classIdent = regexp.MustCompile(`\bclass\s+(\w+)\s*[:(]`)
var callIdent = regexp.MustCompile(`\b(\w+)\s*\(`)

// Resolver implements rag.Resolver for Python.
type Resolver struct{}

func init() {
	rag.MustRegisterResolver(".py", New())
	rag.MustRegisterResolver(".pyw", New())
}

// New returns a new Python symbol resolver.
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
	for _, re := range []*regexp.Regexp{defIdent, classIdent, callIdent} {
		for _, m := range re.FindAllStringSubmatch(hunkContent, -1) {
			if len(m) < 2 {
				continue
			}
			name := m[1]
			if pythonKeywords[name] || seen[name] {
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
	// Match: def symbol( or class symbol. Use POSIX [[:space:]] so git grep -E works on macOS/BSD.
	quoted := regexp.QuoteMeta(symbol)
	pattern := `(def[[:space:]]+` + quoted + `[[:space:]]*\(|class[[:space:]]+` + quoted + `[[:space:]]*[:(])`
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

// readSignatureAndDoc reads the file at path, line (1-based). Signature = def/class
// line(s) until :. Docstring = first """...""" or '''...''' after signature, or preceding #.
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
	lastSigIdx := lineNum - 1
	trimmed := strings.TrimSpace(sigLine)
	if !strings.HasSuffix(trimmed, ":") {
		for i := lineNum; i < len(lines) && i < lineNum+8; i++ {
			trimmed = strings.TrimSpace(lines[i])
			sigBuilder.WriteString("\n")
			sigBuilder.WriteString(lines[i])
			lastSigIdx = i
			if strings.HasSuffix(trimmed, ":") {
				break
			}
		}
	}
	signature = strings.TrimSpace(sigBuilder.String())

	if lastSigIdx < 0 || lastSigIdx >= len(lines) {
		lastSigIdx = lineNum - 1
	}

	// Docstring: preceding # comments
	var docLines []string
	for i := lineNum - 2; i >= 0 && i >= lineNum-1-maxPrecedingCommentLines; i-- {
		s := strings.TrimSpace(lines[i])
		if s == "" {
			break
		}
		if strings.HasPrefix(s, "#") {
			docLines = append([]string{strings.TrimSpace(s[1:])}, docLines...)
			continue
		}
		break
	}
	if len(docLines) > 0 {
		docstring = strings.Join(docLines, "\n")
		return signature, docstring
	}

	// Docstring: first """...""" or '''...''' after signature
	for i := lastSigIdx + 1; i < len(lines) && i < lastSigIdx+12; i++ {
		line := lines[i]
		for _, q := range []string{`"""`, `'''`} {
			if idx := strings.Index(line, q); idx >= 0 {
				rest := line[idx+len(q):]
				endIdx := strings.Index(rest, q)
				if endIdx >= 0 {
					docstring = strings.TrimSpace(rest[:endIdx])
					return signature, docstring
				}
				var docBuilder strings.Builder
				docBuilder.WriteString(strings.TrimSpace(rest))
				for j := i + 1; j < len(lines) && j < i+10; j++ {
					if endIdx := strings.Index(lines[j], q); endIdx >= 0 {
						docBuilder.WriteString("\n")
						docBuilder.WriteString(strings.TrimSpace(lines[j][:endIdx]))
						break
					}
					docBuilder.WriteString("\n")
					docBuilder.WriteString(lines[j])
				}
				docstring = docBuilder.String()
				return signature, docstring
			}
		}
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
