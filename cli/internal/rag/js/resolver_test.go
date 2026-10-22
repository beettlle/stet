package js

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

func TestExtractSymbols_importForms_yieldLocalBindings(t *testing.T) {
	cases := []struct {
		name string
		hunk string
		want []string
		not  []string
	}{
		{"named", `import { compareTaskIds } from "./discover.mjs";`, []string{"compareTaskIds"}, nil},
		{"named multiple", `import { alpha, beta } from "./x.js";`, []string{"alpha", "beta"}, nil},
		{"aliased", `import { orig as localName } from "./x.js";`, []string{"localName"}, []string{"orig"}},
		{"type-only", `import type { Options } from "./x";`, []string{"Options"}, nil},
		{"default", `import discover from "./discover.mjs";`, []string{"discover"}, nil},
		{"default plus named", `import React, { useMemo } from "react";`, []string{"React", "useMemo"}, nil},
		{"namespace", `import * as helpers from "./helpers.js";`, []string{"helpers"}, nil},
		{"require destructuring", `const { readTasks, writeTasks: saveTasks } = require("./tasks");`, []string{"readTasks", "saveTasks"}, []string{"writeTasks"}},
		{"require default", `const loader = require("./loader");`, []string{"loader"}, []string{"require"}},
		{"multi-line diff", "+import {\n+  first,\n+  second\n+    as third,\n+} from \"./m.js\";", []string{"first", "third"}, []string{"second"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractSymbols(tc.hunk)
			set := make(map[string]bool, len(got))
			for _, s := range got {
				set[s] = true
			}
			for _, w := range tc.want {
				if !set[w] {
					t.Errorf("extractSymbols(%q) missing %q; got %v", tc.hunk, w, got)
				}
			}
			for _, n := range tc.not {
				if set[n] {
					t.Errorf("extractSymbols(%q) should not contain %q; got %v", tc.hunk, n, got)
				}
			}
		})
	}
}

func TestExtractSymbols_importBoundNamesComeFirst(t *testing.T) {
	hunk := "+const x = Widget(arr.sort(byName));\n+import { compareTaskIds } from \"./discover.mjs\";\n"
	got := extractSymbols(hunk)
	want := []string{"compareTaskIds", "Widget", "byName"}
	if len(got) != len(want) {
		t.Fatalf("extractSymbols = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("extractSymbols = %v, want %v", got, want)
		}
	}
}

func TestExtractSymbols_callArgumentIdentifiers(t *testing.T) {
	hunk := `arr.sort(compareTaskIds);
emitter.on("done", handler);
run(obj.prop, 42, "s", null, this, ...rest, cb);
wrap(inner(deep), fn(x) + 1);
function declared(paramA, paramB) {
const arrow = (argA) => argA;
class K { method(paramC) { return paramC; } }
`
	got := extractSymbols(hunk)
	set := make(map[string]bool, len(got))
	for _, s := range got {
		set[s] = true
	}
	for _, w := range []string{"compareTaskIds", "handler", "cb", "deep", "x", "run", "wrap", "inner", "declared"} {
		if !set[w] {
			t.Errorf("missing %q; got %v", w, got)
		}
	}
	for _, n := range []string{"obj", "prop", "rest", "paramA", "paramB", "argA", "paramC", "null", "this", "sort"} {
		if set[n] {
			t.Errorf("should not contain %q; got %v", n, got)
		}
	}
}

func TestExtractSymbols_multiLineArgsAndStrings(t *testing.T) {
	hunk := "+process(\n+  \"a,b)\",\n+  `t(${x})`,\n+  onDone\n+);\n"
	got := extractSymbols(hunk)
	found := false
	for _, s := range got {
		if s == "onDone" {
			found = true
		}
		if s == "a" || s == "b" {
			t.Errorf("string content extracted: %v", got)
		}
	}
	if !found {
		t.Errorf("expected onDone from multi-line call; got %v", got)
	}
}

func TestExtractSymbols_unclosedCall_skipsArguments(t *testing.T) {
	got := extractSymbols("doWork(alpha, beta")
	for _, s := range got {
		if s == "alpha" || s == "beta" {
			t.Errorf("unclosed call args should be skipped; got %v", got)
		}
	}
}

func TestExtractSymbols_skipsBuiltinMethodNames(t *testing.T) {
	hunk := `items.filter(isOpen).map(toId).sort(byId);
seen.has(k); list.push(v); list.includes(v); list.forEach(visit);
p.then(onOk); Promise.all(ps); JSON.stringify(o); console.log(o);
const r = Object.keys(o).reduce(merge, {});
`
	got := extractSymbols(hunk)
	for _, s := range got {
		if jsBuiltinGlobals[s] {
			t.Errorf("built-in global %q should not be a candidate; got %v", s, got)
		}
		if jsBuiltinMethods[s] {
			t.Errorf("member call to built-in method %q should not be a candidate; got %v", s, got)
		}
	}
	set := make(map[string]bool, len(got))
	for _, s := range got {
		set[s] = true
	}
	for _, w := range []string{"isOpen", "toId", "byId", "visit", "onOk", "merge"} {
		if !set[w] {
			t.Errorf("missing argument identifier %q; got %v", w, got)
		}
	}
}

func TestExtractSymbols_respectsCandidateCap(t *testing.T) {
	var b strings.Builder
	b.WriteString("import { ")
	for i := 0; i < maxSymbolCandidates+10; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("imp")
		b.WriteString(strconv.Itoa(i))
	}
	b.WriteString(" } from \"./x\";\n")
	for i := 0; i < 10; i++ {
		b.WriteString("call")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("(argx")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(");\n")
	}
	got := extractSymbols(b.String())
	if len(got) != maxSymbolCandidates {
		t.Fatalf("len = %d, want %d", len(got), maxSymbolCandidates)
	}
	if got[0] != "imp0" || got[maxSymbolCandidates-1] != "imp"+strconv.Itoa(maxSymbolCandidates-1) {
		t.Errorf("import-bound names should fill the cap first; got %v", got)
	}

	// Cap also applies when argument identifiers overflow it.
	b.Reset()
	for i := 0; i < maxSymbolCandidates+5; i++ {
		b.WriteString("f(arg")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(");\n")
	}
	got = extractSymbols(b.String())
	if len(got) != maxSymbolCandidates {
		t.Fatalf("args case len = %d, want %d", len(got), maxSymbolCandidates)
	}
	if got[0] != "f" || got[1] != "arg0" {
		t.Errorf("callee should precede argument identifiers; got %v", got)
	}
}

func TestResolveSymbols_functionReferenceArgument_returnsDefinition(t *testing.T) {
	ctx := context.Background()
	dir := testutil.TempGitDir(t)
	initGitRepo(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	content := `// Orders task ids numerically.
export function compareTaskIds(a, b) {
  return Number(a.slice(3)) - Number(b.slice(3));
}
`
	if err := os.WriteFile(filepath.Join(dir, "scripts", "discover.mjs"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "scripts/discover.mjs")

	defs, err := New().ResolveSymbols(ctx, dir, "scripts/list.mjs", "+  arr.sort(compareTaskIds);\n", rag.ResolveOptions{MaxDefinitions: 5})
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition; got %+v", defs)
	}
	d := defs[0]
	if d.Symbol != "compareTaskIds" || d.File != "scripts/discover.mjs" || d.Line != 2 {
		t.Errorf("definition = %+v, want compareTaskIds at scripts/discover.mjs:2", d)
	}
	if !strings.Contains(d.Signature, "function compareTaskIds") {
		t.Errorf("Signature = %q, want function compareTaskIds", d.Signature)
	}
	if d.Docstring != "Orders task ids numerically." {
		t.Errorf("Docstring = %q", d.Docstring)
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
