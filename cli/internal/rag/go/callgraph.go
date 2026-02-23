// Package goresolver: call-graph resolution for Go (callers and callees of the
// function containing a hunk). Uses expand.EnclosingFuncName and git grep.
package goresolver

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go/ast"
	"go/parser"
	"go/token"

	"stet/cli/internal/diff"
	"stet/cli/internal/expand"
	"stet/cli/internal/rag"
)

const (
	callGraphGrepTimeout = 5 * time.Second
	defaultCallersMax    = 3
	defaultCalleesMax    = 3
	maxCallGraphFileSize = 1024 * 1024 // 1 MiB, same as expand
)

// callGraphResolver implements rag.CallGraphResolver for Go.
type callGraphResolver struct{}

func init() {
	rag.MustRegisterCallGraphResolver(".go", &callGraphResolver{})
}

// ResolveCallGraph returns callers and callees for the function containing the hunk.
func (r *callGraphResolver) ResolveCallGraph(ctx context.Context, repoRoot, filePath, hunkContent string, opts rag.CallGraphOptions) (*rag.CallGraphResult, error) {
	start, end, ok := expand.HunkLineRange(diff.Hunk{FilePath: filePath, RawContent: hunkContent})
	if !ok {
		return nil, nil
	}
	funcName, ok := expand.EnclosingFuncName(repoRoot, filePath, start, end)
	if !ok || funcName == "" {
		return nil, nil
	}
	callersMax := opts.CallersMax
	if callersMax <= 0 {
		callersMax = defaultCallersMax
	}
	calleesMax := opts.CalleesMax
	if calleesMax <= 0 {
		calleesMax = defaultCalleesMax
	}
	callers, err := findCallers(ctx, repoRoot, filePath, funcName, callersMax)
	if err != nil {
		return nil, nil // best-effort; don't fail the pipeline
	}
	callees, err := findCallees(ctx, repoRoot, filePath, start, end, calleesMax)
	if err != nil {
		return nil, nil
	}
	if len(callers) == 0 && len(callees) == 0 {
		return nil, nil
	}
	return &rag.CallGraphResult{Callers: callers, Callees: callees}, nil
}

// findCallers runs git grep for call sites of funcName and returns up to max definitions.
func findCallers(ctx context.Context, repoRoot, hunkFilePath, funcName string, max int) ([]rag.Definition, error) {
	absRepo, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	// Match call sites: for "Foo" match Foo( or .Foo( ; for "(*T).Foo" match .Foo(
	// Use a pattern that matches the method name after a dot (for methods) or at word boundary (for functions).
	var pattern string
	if strings.HasPrefix(funcName, "(") && strings.Contains(funcName, ").") {
		// Method: (*T).Foo -> match .Foo(
		parts := strings.SplitN(funcName, ").", 2)
		if len(parts) != 2 {
			return nil, nil
		}
		methodName := regexp.QuoteMeta(parts[1])
		pattern = `\.` + methodName + `[[:space:]]*\(`
	} else {
		// Function: Foo -> match Foo(
		pattern = regexp.QuoteMeta(funcName) + `[[:space:]]*\(`
	}
	ctx, cancel := context.WithTimeout(ctx, callGraphGrepTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "grep", "-n", "-E", pattern)
	cmd.Dir = repoRoot
	cmd.Env = minimalEnv(repoRoot)
	out, err := cmd.Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var defs []rag.Definition
	for _, line := range lines {
		if len(defs) >= max {
			break
		}
		path, lineNum, content, ok := parseGrepLine(line, absRepo)
		if !ok || path == "" {
			continue
		}
		relPath, _ := filepath.Rel(absRepo, path)
		relPath = filepath.ToSlash(relPath)
		content = strings.TrimSpace(content)
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		defs = append(defs, rag.Definition{
			Symbol:    funcName,
			File:      relPath,
			Line:      lineNum,
			Signature: content,
		})
	}
	return defs, nil
}

func parseGrepLine(line, absRepo string) (absPath string, lineNum int, content string, ok bool) {
	idx := strings.Index(line, ":")
	if idx == -1 {
		return "", 0, "", false
	}
	path := line[:idx]
	if path == "" || strings.Contains(path, "..") {
		return "", 0, "", false
	}
	rest := line[idx+1:]
	idx2 := strings.Index(rest, ":")
	if idx2 == -1 {
		return "", 0, "", false
	}
	lineno, _ := strconv.Atoi(rest[:idx2])
	content = rest[idx2+1:]
	absPath = filepath.Join(absRepo, path)
	return absPath, lineno, content, true
}

// findCallees parses the file, finds the enclosing function, collects called names from the body, and looks up each definition.
func findCallees(ctx context.Context, repoRoot, filePath string, startLine, endLine, max int) ([]rag.Definition, error) {
	path := filepath.Join(repoRoot, filepath.FromSlash(filePath))
	path = filepath.Clean(path)
	absRepo, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(absRepo, absPath)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return nil, nil
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxCallGraphFileSize {
		return nil, nil
	}
	src, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, absPath, src, 0)
	if err != nil {
		return nil, err
	}
	enclosing := findEnclosingFuncDecl(fset, f, startLine, endLine)
	if enclosing == nil || enclosing.Body == nil {
		return nil, nil
	}
	calleeNames := collectCalledNames(enclosing.Body)
	if len(calleeNames) == 0 {
		return nil, nil
	}
	seen := make(map[string]bool)
	var defs []rag.Definition
	for _, name := range calleeNames {
		if len(defs) >= max {
			break
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		path, line, content, err := gitGrepSymbol(ctx, absRepo, name)
		if err != nil || path == "" {
			continue
		}
		relPath, _ := filepath.Rel(absRepo, path)
		relPath = filepath.ToSlash(relPath)
		sig, _ := readSignatureAndDoc(path, line, content)
		if sig == "" {
			sig = strings.TrimSpace(content)
		}
		defs = append(defs, rag.Definition{
			Symbol:    name,
			File:      relPath,
			Line:      line,
			Signature: sig,
		})
	}
	return defs, nil
}

func findEnclosingFuncDecl(fset *token.FileSet, f *ast.File, startLine, endLine int) *ast.FuncDecl {
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

// collectCalledNames walks the AST and collects up to max unique function/method names from CallExpr nodes.
func collectCalledNames(body *ast.BlockStmt) []string {
	var names []string
	seen := make(map[string]bool)
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := callExprName(call.Fun)
		if name == "" || seen[name] || goKeywords[name] {
			return true
		}
		seen[name] = true
		names = append(names, name)
		return true
	})
	return names
}

func callExprName(fun ast.Expr) string {
	switch e := fun.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	default:
		return ""
	}
}
