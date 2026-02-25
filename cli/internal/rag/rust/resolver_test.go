package rust

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
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `/// Returns 42.
pub fn foo() -> i32 {
    42
}
`
	libPath := filepath.Join(srcDir, "lib.rs")
	if err := os.WriteFile(libPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "src/lib.rs")

	r := New()
	opts := rag.ResolveOptions{MaxDefinitions: 5}
	defs, err := r.ResolveSymbols(ctx, dir, "src/lib.rs", "let x = foo();", opts)
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition; got %d", len(defs))
	}
	if defs[0].Symbol != "foo" {
		t.Errorf("Symbol = %q, want foo", defs[0].Symbol)
	}
	if !strings.Contains(defs[0].Signature, "fn foo") {
		t.Errorf("Signature should contain 'fn foo'; got %q", defs[0].Signature)
	}
	if want := "src/lib.rs"; defs[0].File != want {
		t.Errorf("File = %q, want %q", defs[0].File, want)
	}
}

func TestResolveSymbols_maxN_returnsAtMostN(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	initGitRepo(t, dir)
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `fn A() {}
fn B() {}
fn C() {}
`
	libPath := filepath.Join(srcDir, "lib.rs")
	if err := os.WriteFile(libPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "src/lib.rs")

	r := New()
	opts := rag.ResolveOptions{MaxDefinitions: 2}
	defs, err := r.ResolveSymbols(ctx, dir, "src/lib.rs", "A(); B(); C();", opts)
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
	r := New()
	opts := rag.ResolveOptions{MaxDefinitions: 5}
	defs, err := r.ResolveSymbols(ctx, dir, "src/lib.rs", "NonExistent()", opts)
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 0 {
		t.Errorf("expected 0 definitions; got %d", len(defs))
	}
}

func TestExtractSymbols_deduplicatesAndSkipsKeywords(t *testing.T) {
	hunk := `fn bar() { let x = foo(); match baz() { Some(Bar) => {} } struct Qux {} }`
	symbols := extractSymbols(hunk)
	seen := make(map[string]bool)
	for _, s := range symbols {
		if seen[s] {
			t.Errorf("duplicate symbol %q", s)
		}
		seen[s] = true
		if rustKeywords[s] {
			t.Errorf("keyword should not be extracted: %q", s)
		}
	}
	if !seen["foo"] {
		t.Error("expected foo (call site) to be extracted")
	}
	if !seen["Bar"] {
		t.Error("expected Bar to be extracted")
	}
	if !seen["baz"] {
		t.Error("expected baz (call site) to be extracted")
	}
	if !seen["Qux"] {
		t.Error("expected Qux (struct) to be extracted")
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
