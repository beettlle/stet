package python

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
	content := `def bar():
    """doc"""
    return 42
`
	fooPath := filepath.Join(pkgDir, "foo.py")
	if err := os.WriteFile(fooPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "pkg/foo.py")

	r := New()
	opts := rag.ResolveOptions{MaxDefinitions: 5}
	defs, err := r.ResolveSymbols(ctx, dir, "pkg/foo.py", "bar()", opts)
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition; got %d", len(defs))
	}
	if defs[0].Symbol != "bar" {
		t.Errorf("Symbol = %q, want bar", defs[0].Symbol)
	}
	if !strings.Contains(defs[0].Signature, "def bar") {
		t.Errorf("Signature should contain 'def bar'; got %q", defs[0].Signature)
	}
	if !strings.Contains(defs[0].Docstring, "doc") {
		t.Errorf("Docstring should contain 'doc'; got %q", defs[0].Docstring)
	}
	if want := "pkg/foo.py"; defs[0].File != want {
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
	content := `def A(): pass
def B(): pass
def C(): pass
`
	fooPath := filepath.Join(pkgDir, "foo.py")
	if err := os.WriteFile(fooPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "pkg/foo.py")

	r := New()
	opts := rag.ResolveOptions{MaxDefinitions: 2}
	defs, err := r.ResolveSymbols(ctx, dir, "pkg/foo.py", "A(); B(); C()", opts)
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
	defs, err := r.ResolveSymbols(ctx, dir, "pkg/foo.py", "NonExistent()", opts)
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 0 {
		t.Errorf("expected 0 definitions; got %d", len(defs))
	}
}

func TestReadSignatureAndDoc_edgeCases(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "tiny.py")
	if err := os.WriteFile(f, []byte("def foo():\n    pass\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("lineNumBeyondFileLength", func(t *testing.T) {
		sig, doc := readSignatureAndDoc(f, 999, "fallback")
		if sig != "fallback" || doc != "" {
			t.Errorf("lineNum beyond file: got sig=%q doc=%q, want fallback, \"\"", sig, doc)
		}
	})

	t.Run("lineNumZero", func(t *testing.T) {
		sig, doc := readSignatureAndDoc(f, 0, "fallback")
		if sig != "fallback" || doc != "" {
			t.Errorf("lineNum 0: got sig=%q doc=%q, want fallback, \"\"", sig, doc)
		}
	})

	t.Run("invalidPath", func(t *testing.T) {
		sig, doc := readSignatureAndDoc(filepath.Join(dir, "nonexistent.py"), 1, "fallback")
		if sig != "fallback" || doc != "" {
			t.Errorf("invalid path: got sig=%q doc=%q, want fallback, \"\"", sig, doc)
		}
	})
}

func TestExtractSymbols_deduplicatesAndSkipsKeywords(t *testing.T) {
	symbols := extractSymbols("def foo(): bar(); class Baz: pass")
	seen := make(map[string]bool)
	for _, s := range symbols {
		if seen[s] {
			t.Errorf("duplicate symbol %q", s)
		}
		seen[s] = true
		if s == "def" || s == "class" {
			t.Errorf("keyword should not be extracted: %q", s)
		}
	}
	if !seen["foo"] {
		t.Error("expected foo to be extracted")
	}
	if !seen["bar"] {
		t.Error("expected bar to be extracted")
	}
	if !seen["Baz"] {
		t.Error("expected Baz to be extracted")
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
