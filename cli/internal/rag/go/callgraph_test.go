package goresolver

import (
	"context"
	"os"
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
