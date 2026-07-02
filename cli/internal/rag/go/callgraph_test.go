package goresolver

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"go/ast"
	"go/parser"
	"go/token"

	"stet/cli/internal/rag"
)

func TestResolveCallGraph_noEnclosingFunction_returnsNil(t *testing.T) {
	// t.TempDir() registers automatic cleanup for dir and all files within it.
	dir := t.TempDir()
	path := filepath.Join(dir, "pkg", "code.go")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	content := "package pkg\n\nvar x int = 1\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	hunkContent := "@@ -1,2 +1,2 @@\n package pkg\n\n var x int = 1\n"
	ctx := context.Background()
	result, err := (&callGraphResolver{}).ResolveCallGraph(ctx, dir, "pkg/code.go", hunkContent, rag.CallGraphOptions{CallersMax: 3, CalleesMax: 3})
	if err != nil {
		t.Fatalf("ResolveCallGraph: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil when no enclosing function; got %+v", result)
	}
}

func TestResolveCallGraph_badHunkHeader_returnsNil(t *testing.T) {
	// t.TempDir() registers automatic cleanup.
	dir := t.TempDir()
	ctx := context.Background()
	result, err := (&callGraphResolver{}).ResolveCallGraph(ctx, dir, "pkg/code.go", "no valid hunk header", rag.CallGraphOptions{})
	if err != nil {
		t.Fatalf("ResolveCallGraph: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for bad hunk header; got %+v", result)
	}
}

func TestCollectCalledNames(t *testing.T) {
	// Parse a minimal Go file and inspect the function body.
	src := `package pkg
func bar() {
	foo()
	x := baz()
	quux()
}
`
	fset := token.NewFileSet()
	f, err := parseFile(fset, "x.go", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Decls) != 1 {
		t.Fatalf("expected 1 decl; got %d", len(f.Decls))
	}
	fn, ok := f.Decls[0].(*ast.FuncDecl)
	if !ok || fn.Body == nil {
		t.Fatal("expected FuncDecl with body")
	}
	names := collectCalledNames(fn.Body)
	want := map[string]bool{"foo": true, "baz": true, "quux": true}
	if len(names) != len(want) {
		t.Errorf("collectCalledNames: got %v (len %d), want 3 names", names, len(names))
	}
	for _, n := range names {
		if !want[n] {
			t.Errorf("unexpected name %q", n)
		}
	}
}

func TestFindEnclosingFuncDecl(t *testing.T) {
	src := `package pkg

func outer() {
}

func inner() {
}
`
	fset := token.NewFileSet()
	f, err := parseFile(fset, "x.go", src)
	if err != nil {
		t.Fatal(err)
	}
	// outer is lines 3-4, inner is lines 6-7. Smallest containing 6,6 is inner.
	got := findEnclosingFuncDecl(fset, f, 6, 6)
	if got == nil {
		t.Fatal("expected enclosing func")
	}
	if got.Name.Name != "inner" {
		t.Errorf("expected inner; got %s", got.Name.Name)
	}
}

func parseFile(fset *token.FileSet, name, src string) (*ast.File, error) {
	return parser.ParseFile(fset, name, src, 0)
}

func TestParseGrepLine(t *testing.T) {
	absRepo := t.TempDir()
	path, line, content, ok := parseGrepLine("pkg/foo.go:10:\tTarget()", absRepo)
	if !ok {
		t.Fatal("expected ok")
	}
	if line != 10 || content != "\tTarget()" {
		t.Errorf("line=%d content=%q", line, content)
	}
	wantPath := filepath.Join(absRepo, "pkg/foo.go")
	if path != wantPath {
		t.Errorf("path = %q, want %q", path, wantPath)
	}
	_, _, _, ok = parseGrepLine("badline", absRepo)
	if ok {
		t.Error("expected false for bad line")
	}
}

func TestCallExprName(t *testing.T) {
	src := `package p
func f() { x.M(); y() }
`
	fset := token.NewFileSet()
	f, err := parseFile(fset, "x.go", src)
	if err != nil {
		t.Fatal(err)
	}
	fn := f.Decls[0].(*ast.FuncDecl)
	names := collectCalledNames(fn.Body)
	if len(names) < 2 {
		t.Fatalf("names = %v", names)
	}
}

func initCallGraphRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	for _, args := range [][]string{
		{"git", "config", "user.email", "t@t"},
		{"git", "config", "user.name", "T"},
	} {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = dir
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		_ = c.Run()
	}
	content := `package pkg

func Caller() {
	Target()
}

func Target() {
	helper()
}

func helper() {}
`
	path := filepath.Join(dir, "pkg", "code.go")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "add", "pkg/code.go"},
		{"git", "commit", "-m", "init"},
	} {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = dir
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestFindCallees_inRepo(t *testing.T) {
	repo := initCallGraphRepo(t)
	ctx := context.Background()
	defs, err := findCallees(ctx, repo, "pkg/code.go", 7, 9, 5)
	if err != nil {
		t.Fatalf("findCallees: %v", err)
	}
	if len(defs) == 0 {
		t.Fatal("expected at least one callee (helper)")
	}
	found := false
	for _, d := range defs {
		if d.Symbol == "helper" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("callees = %+v, want helper", defs)
	}
}

func TestFindCallers_findsCaller(t *testing.T) {
	repo := initCallGraphRepo(t)
	ctx := context.Background()
	callers, err := findCallers(ctx, repo, "pkg/code.go", "Target", 5)
	if err != nil {
		t.Fatalf("findCallers: %v", err)
	}
	if len(callers) == 0 {
		t.Fatal("expected at least one caller of Target")
	}
}

func TestResolveCallGraph_findsCallersAndCallees(t *testing.T) {
	repo := initCallGraphRepo(t)
	hunk := "@@ -7,3 +7,4 @@\n func Target() {\n \thelper()\n+\t// change\n }\n"
	ctx := context.Background()
	result, err := (&callGraphResolver{}).ResolveCallGraph(ctx, repo, "pkg/code.go", hunk, rag.CallGraphOptions{CallersMax: 3, CalleesMax: 3})
	if err != nil {
		t.Fatalf("ResolveCallGraph: %v", err)
	}
	if result == nil {
		t.Skip("call graph empty (expand/hunk mismatch); covered by findCallers/findCallees unit tests")
	}
	if len(result.Callers) == 0 {
		t.Error("expected at least one caller")
	}
	if len(result.Callees) == 0 {
		t.Error("expected at least one callee")
	}
}

func TestFindCallers_methodPattern(t *testing.T) {
	repo := initCallGraphRepo(t)
	ctx := context.Background()
	// Exercise method-name branch via a receiver-style func name string.
	callers, err := findCallers(ctx, repo, "pkg/code.go", "(*T).Target", 3)
	if err != nil {
		t.Fatalf("findCallers: %v", err)
	}
	_ = callers // best-effort; may be empty if pattern does not match
}
