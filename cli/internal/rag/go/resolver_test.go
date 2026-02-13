package goresolver

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/rag"
)

func TestResolveSymbols_oneSymbol_returnsDefinition(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	initGitRepo(t, dir)
	pkgDir := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `package pkg

// Bar does something.
func Bar() int {
	return 42
}
`
	fooPath := filepath.Join(pkgDir, "foo.go")
	if err := os.WriteFile(fooPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "pkg/foo.go")

	r := New()
	opts := rag.ResolveOptions{MaxDefinitions: 5}
	defs, err := r.ResolveSymbols(ctx, dir, "pkg/foo.go", "x := Bar()", opts)
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition; got %d", len(defs))
	}
	if defs[0].Symbol != "Bar" {
		t.Errorf("Symbol = %q, want Bar", defs[0].Symbol)
	}
	if !strings.Contains(defs[0].Signature, "func Bar") {
		t.Errorf("Signature should contain 'func Bar'; got %q", defs[0].Signature)
	}
	if want := "pkg/foo.go"; defs[0].File != want {
		t.Errorf("File = %q, want %q", defs[0].File, want)
	}
}

func TestResolveSymbols_maxN_returnsAtMostN(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	initGitRepo(t, dir)
	pkgDir := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `package pkg
func A() {}
func B() {}
func C() {}
`
	fooPath := filepath.Join(pkgDir, "foo.go")
	if err := os.WriteFile(fooPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "pkg/foo.go")

	r := New()
	opts := rag.ResolveOptions{MaxDefinitions: 2}
	defs, err := r.ResolveSymbols(ctx, dir, "pkg/foo.go", "A(); B(); C()", opts)
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) > 2 {
		t.Errorf("expected at most 2 definitions; got %d", len(defs))
	}
}

func TestResolveSymbols_noMatch_returnsNil(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	initGitRepo(t, dir)
	// No Go files; symbol "NonExistent" won't be found.
	r := New()
	opts := rag.ResolveOptions{MaxDefinitions: 5}
	defs, err := r.ResolveSymbols(ctx, dir, "pkg/foo.go", "NonExistent()", opts)
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 0 {
		t.Errorf("expected 0 definitions; got %d", len(defs))
	}
}

func TestExtractSymbols_deduplicatesAndSkipsKeywords(t *testing.T) {
	symbols := extractSymbols("func foo() { return Bar(); type X struct {}; if err != nil {} }")
	seen := make(map[string]bool)
	for _, s := range symbols {
		if seen[s] {
			t.Errorf("duplicate symbol %q", s)
		}
		seen[s] = true
		if s == "func" || s == "return" || s == "type" || s == "if" || s == "struct" {
			t.Errorf("keyword should not be extracted: %q", s)
		}
	}
	if !seen["Bar"] {
		t.Error("expected Bar (type/call) to be extracted")
	}
	if !seen["X"] {
		t.Error("expected X (type) to be extracted")
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test")
	runGit(t, dir, "config", "user.name", "Test")
}

func gitAdd(t *testing.T, dir, path string) {
	t.Helper()
	runGit(t, dir, "add", path)
	runGit(t, dir, "commit", "-m", "add")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
