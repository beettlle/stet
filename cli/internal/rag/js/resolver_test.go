package js

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/jsts"
	"stet/cli/internal/rag"
	"stet/cli/internal/testutil"
)

// TestMain isolates the package's tests from host git configuration and
// background git daemons (see cli/internal/testutil).
func TestMain(m *testing.M) {
	os.Exit(testutil.RunIsolatedGit(m))
}

func TestResolveSymbols_oneSymbol_returnsDefinition(t *testing.T) {
	ctx := context.Background()
	dir := testutil.TempGitDir(t)
	initGitRepo(t, dir)
	pkgDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `/**
 * Greets the user.
 */
export function greet(name: string): string {
  return "Hello, " + name;
}
`
	fooPath := filepath.Join(pkgDir, "index.ts")
	if err := os.WriteFile(fooPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "src/index.ts")

	r := New()
	opts := rag.ResolveOptions{MaxDefinitions: 5}
	defs, err := r.ResolveSymbols(ctx, dir, "src/index.ts", "greet(x)", opts)
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition; got %d", len(defs))
	}
	if defs[0].Symbol != "greet" {
		t.Errorf("Symbol = %q, want greet", defs[0].Symbol)
	}
	if !strings.Contains(defs[0].Signature, "function greet") {
		t.Errorf("Signature should contain 'function greet'; got %q", defs[0].Signature)
	}
	if want := "src/index.ts"; defs[0].File != want {
		t.Errorf("File = %q, want %q", defs[0].File, want)
	}
}

// TestRegistry_everyJSTSExtension_resolvesDefinition verifies init() registers
// the resolver for every shared JS/TS extension (notably .jsx, .mjs, .cjs) so
// rag.ResolveSymbols dispatches and returns a definition from such files.
func TestRegistry_everyJSTSExtension_resolvesDefinition(t *testing.T) {
	cases := []struct {
		ext     string
		content string
		symbol  string
		hunk    string
	}{
		{".jsx", "export function Button({ label }) {\n  return <button>{label}</button>;\n}\n", "Button", "<div>{Button({ label: \"x\" })}</div>"},
		{".mjs", "export function filterPending(ids) {\n  return ids.filter(Boolean);\n}\n", "filterPending", "const p = filterPending(ids);"},
		{".cjs", "function countItems(items) {\n  return items.length;\n}\nmodule.exports = { countItems };\n", "countItems", "const n = countItems(list);"},
	}
	seen := make(map[string]bool)
	for _, tc := range cases {
		seen[tc.ext] = true
	}
	for _, ext := range []string{".jsx", ".mjs", ".cjs"} {
		if !jsts.IsExt(ext) {
			t.Fatalf("jsts.IsExt(%q) = false; shared list must include it", ext)
		}
	}
	for _, ext := range jsts.Extensions {
		if !seen[ext] {
			cases = append(cases, struct {
				ext     string
				content string
				symbol  string
				hunk    string
			}{ext, "function sharedHelper(x) {\n  return x;\n}\n", "sharedHelper", "sharedHelper(1)"})
		}
	}
	for _, tc := range cases {
		t.Run(tc.ext, func(t *testing.T) {
			ctx := context.Background()
			dir := testutil.TempGitDir(t)
			initGitRepo(t, dir)
			rel := "src/file" + tc.ext
			if err := os.MkdirAll(filepath.Join(dir, "src"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, rel), []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}
			gitAdd(t, dir, rel)

			defs, err := rag.ResolveSymbols(ctx, dir, rel, tc.hunk, rag.ResolveOptions{MaxDefinitions: 5})
			if err != nil {
				t.Fatalf("rag.ResolveSymbols(%s): %v", rel, err)
			}
			var found bool
			for _, d := range defs {
				if d.Symbol == tc.symbol && d.File == rel {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected definition of %q in %s; got %+v", tc.symbol, rel, defs)
			}
		})
	}
}

func TestResolveSymbols_maxN_returnsAtMostN(t *testing.T) {
	ctx := context.Background()
	dir := testutil.TempGitDir(t)
	initGitRepo(t, dir)
	pkgDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `function A() {}
function B() {}
function C() {}
`
	fooPath := filepath.Join(pkgDir, "x.js")
	if err := os.WriteFile(fooPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "src/x.js")

	r := New()
	opts := rag.ResolveOptions{MaxDefinitions: 2}
	defs, err := r.ResolveSymbols(ctx, dir, "src/x.js", "A(); B(); C()", opts)
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) > 2 {
		t.Errorf("expected at most 2 definitions; got %d", len(defs))
	}
}

func TestResolveSymbols_noMatch_returnsNil(t *testing.T) {
	ctx := context.Background()
	dir := testutil.TempGitDir(t)
	initGitRepo(t, dir)
	r := New()
	opts := rag.ResolveOptions{MaxDefinitions: 5}
	defs, err := r.ResolveSymbols(ctx, dir, "src/index.ts", "NonExistent()", opts)
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 0 {
		t.Errorf("expected 0 definitions; got %d", len(defs))
	}
}

func TestExtractSymbols_deduplicatesAndSkipsKeywords(t *testing.T) {
	symbols := extractSymbols("function foo() { bar(); class Baz {} }")
	seen := make(map[string]bool)
	for _, s := range symbols {
		if seen[s] {
			t.Errorf("duplicate symbol %q", s)
		}
		seen[s] = true
		if s == "function" || s == "class" {
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
