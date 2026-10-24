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
	for _, ext := range jsts.Extensions() {
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
		{"builtin-method named", `import { get } from "./cache.mjs";`, []string{"get"}, nil},
		{"global named", `import { console } from "./x.js";`, nil, []string{"console"}},
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

func TestImportBindings_pairsWithModuleAndSource(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []importBinding
	}{
		{"named", `import { compareTaskIds } from "./discover.mjs";`,
			[]importBinding{{local: "compareTaskIds", source: "compareTaskIds", module: "./discover.mjs"}}},
		{"named multiple with alias", `import { alpha, beta as gamma } from "./m.js";`,
			[]importBinding{
				{local: "alpha", source: "alpha", module: "./m.js"},
				{local: "gamma", source: "beta", module: "./m.js"},
			}},
		{"type-only", `import type { Options } from "./x";`,
			[]importBinding{{local: "Options", source: "Options", module: "./x"}}},
		{"default", `import discover from "./discover.mjs";`,
			[]importBinding{{local: "discover", source: sourceDefault, module: "./discover.mjs"}}},
		{"default plus named", `import React, { useMemo } from "react";`,
			[]importBinding{
				{local: "useMemo", source: "useMemo", module: "react"},
				{local: "React", source: sourceDefault, module: "react"},
			}},
		{"namespace binding is skipped", `import * as helpers from "./helpers.js";`, nil},
		{"default plus namespace", `import React, * as ns from "react";`,
			[]importBinding{{local: "React", source: sourceDefault, module: "react"}}},
		{"single quotes", `import { alpha } from './m.js';`,
			[]importBinding{{local: "alpha", source: "alpha", module: "./m.js"}}},
		{"require destructuring", `const { readTasks, writeTasks: saveTasks } = require("./tasks");`,
			[]importBinding{
				{local: "readTasks", source: "readTasks", module: "./tasks"},
				{local: "saveTasks", source: "writeTasks", module: "./tasks"},
			}},
		{"require default", `const loader = require("./loader");`,
			[]importBinding{{local: "loader", source: sourceDefault, module: "./loader"}}},
		{"dynamic require has no specifier", `const loader = require(resolve("./loader"));`, nil},
		{"cut-off hunk has no specifier", `import { alpha } from`, nil},
		{"multi-line diff", "+import {\n+  first,\n+  second\n+    as third,\n+} from \"./m.js\";", []importBinding{
			{local: "first", source: "first", module: "./m.js"},
			{local: "third", source: "second", module: "./m.js"},
		}},
		{"multi-line require", "const loader = require(\n  \"./loader\"\n);", []importBinding{
			{local: "loader", source: sourceDefault, module: "./loader"},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := importBindings(tc.src)
			if len(got) != len(tc.want) {
				t.Fatalf("importBindings(%q) = %+v, want %+v", tc.src, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("importBindings(%q)[%d] = %+v, want %+v", tc.src, i, got[i], tc.want[i])
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

func TestExtractSymbols_memberCallsSkipBuiltins_bareCallsAndDeclsKept(t *testing.T) {
	// Member calls to built-in method names add no candidates; bare call
	// arguments are still extracted.
	memberHunk := `arr.filter(fn);
m.get(k);
console.log(x);
`
	got := extractSymbols(memberHunk)
	set := make(map[string]bool, len(got))
	for _, s := range got {
		set[s] = true
	}
	for _, n := range []string{"filter", "get", "log", "console"} {
		if set[n] {
			t.Errorf("member call to built-in %q should not be a candidate; got %v", n, got)
		}
	}
	for _, w := range []string{"fn", "k", "x"} {
		if !set[w] {
			t.Errorf("missing bare argument %q; got %v", w, got)
		}
	}

	// The same spellings as bare callees are legitimate repo symbols.
	bareHunk := "filter(fn);\nget(k);\nlog(x);\n"
	got = extractSymbols(bareHunk)
	set = make(map[string]bool, len(got))
	for _, s := range got {
		set[s] = true
	}
	for _, w := range []string{"filter", "get", "log", "fn"} {
		if !set[w] {
			t.Errorf("bare call %q should be a candidate; got %v", w, got)
		}
	}

	// function declarations with method-like names are not member accesses.
	got = extractSymbols("function map(items) { return items; }\nmap(list);\n")
	set = make(map[string]bool, len(got))
	for _, s := range got {
		set[s] = true
	}
	if !set["map"] {
		t.Errorf("declaration of map should be a candidate; got %v", got)
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

func TestResolveSymbols_importBoundBuiltinMethodName_returnsDefinition(t *testing.T) {
	ctx := context.Background()
	dir := testutil.TempGitDir(t)
	initGitRepo(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0755); err != nil {
		t.Fatal(err)
	}
	content := `// Returns the cached value for key.
export function get(key) {
  return cache[key];
}
`
	if err := os.WriteFile(filepath.Join(dir, "lib", "cache.mjs"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "lib/cache.mjs")

	// Issue #28: `get` was previously skipped by name before reaching RAG,
	// producing "get is not defined" false positives.
	hunk := "+import { get } from \"./cache.mjs\";\n+  const v = get(key);\n"
	defs, err := New().ResolveSymbols(ctx, dir, "src/app.mjs", hunk, rag.ResolveOptions{MaxDefinitions: 5})
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition; got %+v", defs)
	}
	d := defs[0]
	if d.Symbol != "get" || d.File != "lib/cache.mjs" || d.Line != 2 {
		t.Errorf("definition = %+v, want get at lib/cache.mjs:2", d)
	}
	if !strings.Contains(d.Signature, "function get") {
		t.Errorf("Signature = %q, want function get", d.Signature)
	}
}

func TestResolveModulePath_forms(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	writeRepoFile(t, repo, "src/mod.js", "export const a = 1;\n")
	writeRepoFile(t, repo, "src/util.ts", "export const b = 2;\n")
	writeRepoFile(t, repo, "src/pkg/index.js", "export const c = 3;\n")
	writeRepoFile(t, repo, "top.js", "export const d = 4;\n")
	writeRepoFile(t, base, "outside.js", "export const x = 9;\n")
	srcDir := filepath.Join(repo, "src")

	cases := []struct {
		name      string
		specifier string
		want      string // repo-relative; "" when unresolved
	}{
		{"exact file", "./mod.js", "src/mod.js"},
		{"extensionless appends extension", "./util", "src/util.ts"},
		{"directory index", "./pkg", "src/pkg/index.js"},
		{"dot-dot stays inside", "../top.js", "top.js"},
		{"missing file", "./nope.js", ""},
		{"bare package rejected", "lodash", ""},
		{"node protocol rejected", "node:fs", ""},
		{"parent escape rejected", "../../outside.js", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveModulePath(repo, srcDir, tc.specifier)
			if tc.want == "" {
				if got != "" {
					t.Fatalf("resolveModulePath(%q) = %q, want none", tc.specifier, got)
				}
				return
			}
			if want := filepath.Join(repo, filepath.FromSlash(tc.want)); got != want {
				t.Fatalf("resolveModulePath(%q) = %q, want %q", tc.specifier, got, want)
			}
		})
	}

	t.Run("symlink escape rejected", func(t *testing.T) {
		outside := filepath.Join(t.TempDir(), "secret.js")
		if err := os.WriteFile(outside, []byte("export const x = 1;\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(srcDir, "link.js")); err != nil {
			t.Skip("symlinks unavailable:", err)
		}
		if got := resolveModulePath(repo, srcDir, "./link.js"); got != "" {
			t.Fatalf("resolveModulePath through symlink = %q, want none", got)
		}
	})
}

func TestPathInsideRepo(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	writeRepoFile(t, repo, "src/in.js", "x")
	writeRepoFile(t, base, "out.js", "x")
	inside := filepath.Join(repo, "src", "in.js")
	if !pathInsideRepo(repo, inside) {
		t.Errorf("pathInsideRepo(inside) = false, want true")
	}
	if pathInsideRepo(repo, filepath.Join(base, "out.js")) {
		t.Errorf("pathInsideRepo(outside) = true, want false")
	}
	if pathInsideRepo(repo, filepath.Join(repo, "missing.js")) {
		t.Errorf("pathInsideRepo(missing) = true, want false")
	}
	if err := os.Symlink(filepath.Join(base, "out.js"), filepath.Join(repo, "link.js")); err == nil {
		if pathInsideRepo(repo, filepath.Join(repo, "link.js")) {
			t.Errorf("pathInsideRepo(symlink escape) = true, want false")
		}
	}
}

func TestFindExportedDecl_forms(t *testing.T) {
	module := filepath.Join(t.TempDir(), "mod.js")
	content := `export const version = 1;
export function helper() {}
export async function fetchAll(u) {}
export class Widget {}
export { helper };
export { helper as renamed };
export default function makeWidget() {}
module.exports.named = function named() {};
exports.alsoNamed = 2;
`
	if err := os.WriteFile(module, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		source   string
		wantLine int
	}{
		{"version", 1},
		{"helper", 2},
		{"fetchAll", 3},
		{"Widget", 4},
		{"renamed", 6},
		{"makeWidget", 7},
		{"named", 8},
		{"alsoNamed", 9},
		{sourceDefault, 7},
		{"absent", 0},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			line, decl, ok := findExportedDecl(module, tc.source)
			if tc.wantLine == 0 {
				if ok {
					t.Fatalf("findExportedDecl(%q) = line %d, want none", tc.source, line)
				}
				return
			}
			if !ok || line != tc.wantLine {
				t.Fatalf("findExportedDecl(%q) = (%d, %q, %v), want line %d", tc.source, line, decl, ok, tc.wantLine)
			}
		})
	}

	t.Run("re-export list is skipped", func(t *testing.T) {
		reexport := filepath.Join(t.TempDir(), "re.js")
		if err := os.WriteFile(reexport, []byte("export { sortTasks } from \"./tasks.js\";\nconst sortTasks = 1;\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, _, ok := findExportedDecl(reexport, "sortTasks"); ok {
			t.Fatalf("re-export line must not match")
		}
	})

	t.Run("whole-object module.exports matches default", func(t *testing.T) {
		cjs := filepath.Join(t.TempDir(), "cjs.js")
		if err := os.WriteFile(cjs, []byte("module.exports = { key: 1 };\n"), 0644); err != nil {
			t.Fatal(err)
		}
		line, _, ok := findExportedDecl(cjs, sourceDefault)
		if !ok || line != 1 {
			t.Fatalf("findExportedDecl(default) = (%d, %v), want line 1", line, ok)
		}
	})

	t.Run("multi-line export list falls back", func(t *testing.T) {
		multi := filepath.Join(t.TempDir(), "multi.js")
		if err := os.WriteFile(multi, []byte("export {\n  multi,\n};\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, _, ok := findExportedDecl(multi, "multi"); ok {
			t.Fatalf("multi-line export list must not match")
		}
	})
}

// TestResolveSymbols_importedName_resolvesInItsModule is the core issue #19
// case: a hunk editing `import { compareTaskIds } from "./discover.mjs";`
// must resolve compareTaskIds in discover.mjs, not to an unrelated
// same-named declaration that plain git grep would return first.
func TestResolveSymbols_importedName_resolvesInItsModule(t *testing.T) {
	ctx := context.Background()
	dir := testutil.TempGitDir(t)
	initGitRepo(t, dir)
	writeRepoFile(t, dir, "src/discover.mjs", "// Sorts task ids.\nexport function compareTaskIds(a, b) {\n  return a - b;\n}\n")
	gitAdd(t, dir, "src/discover.mjs")
	writeDistractor(t, dir, "compareTaskIds")

	hunk := "+import { compareTaskIds } from \"./discover.mjs\";\n+  list.sort(compareTaskIds);\n"
	defs, err := New().ResolveSymbols(ctx, dir, "src/list.mjs", hunk, rag.ResolveOptions{MaxDefinitions: 5})
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition; got %+v", defs)
	}
	d := defs[0]
	if d.Symbol != "compareTaskIds" || d.File != "src/discover.mjs" || d.Line != 2 {
		t.Errorf("definition = %+v, want compareTaskIds at src/discover.mjs:2", d)
	}
	if !strings.Contains(d.Signature, "function compareTaskIds") {
		t.Errorf("Signature = %q, want function compareTaskIds", d.Signature)
	}
	if d.Docstring != "Sorts task ids." {
		t.Errorf("Docstring = %q, want the discover.mjs doc comment", d.Docstring)
	}
}

// TestResolveSymbols_importFromEditedFileBlock resolves an import-bound name
// through the import block of the edited file on disk when the hunk itself
// does not touch the import line.
func TestResolveSymbols_importFromEditedFileBlock(t *testing.T) {
	ctx := context.Background()
	dir := testutil.TempGitDir(t)
	initGitRepo(t, dir)
	writeRepoFile(t, dir, "src/discover.mjs", "// Sorts task ids.\nexport function compareTaskIds(a, b) {\n  return a - b;\n}\n")
	gitAdd(t, dir, "src/discover.mjs")
	writeRepoFile(t, dir, "src/list.mjs", "import { compareTaskIds } from \"./discover.mjs\";\n\nconst order = compareTaskIds(a, b);\n")
	gitAdd(t, dir, "src/list.mjs")
	writeDistractor(t, dir, "compareTaskIds")

	defs, err := New().ResolveSymbols(ctx, dir, "src/list.mjs", "+const order = compareTaskIds(a, b);\n", rag.ResolveOptions{MaxDefinitions: 5})
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition; got %+v", defs)
	}
	if d := defs[0]; d.File != "src/discover.mjs" || d.Line != 2 {
		t.Errorf("definition = %+v, want compareTaskIds at src/discover.mjs:2", d)
	}
}

func TestResolveSymbols_aliasedImport_resolvesSourceExport(t *testing.T) {
	ctx := context.Background()
	dir := testutil.TempGitDir(t)
	initGitRepo(t, dir)
	writeRepoFile(t, dir, "src/util.js", "// Sorts tasks by id.\nexport function sortTasks(list) {\n  return list.slice();\n}\n")
	gitAdd(t, dir, "src/util.js")
	writeDistractor(t, dir, "order")

	hunk := "+import { sortTasks as order } from \"./util.js\";\n+order(items);\n"
	defs, err := New().ResolveSymbols(ctx, dir, "src/main.js", hunk, rag.ResolveOptions{MaxDefinitions: 5})
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition; got %+v", defs)
	}
	d := defs[0]
	if d.Symbol != "order" || d.File != "src/util.js" || d.Line != 2 {
		t.Errorf("definition = %+v, want order (sortTasks) at src/util.js:2", d)
	}
}

func TestResolveSymbols_defaultImport_resolvesExportDefault(t *testing.T) {
	ctx := context.Background()
	dir := testutil.TempGitDir(t)
	initGitRepo(t, dir)
	writeRepoFile(t, dir, "src/loader.mjs", "// Creates a loader.\nexport default function makeLoader() {\n  return {};\n}\n")
	gitAdd(t, dir, "src/loader.mjs")
	writeDistractor(t, dir, "makeLoader")

	hunk := "+import makeLoader from \"./loader.mjs\";\n+const l = makeLoader();\n"
	defs, err := New().ResolveSymbols(ctx, dir, "src/app.mjs", hunk, rag.ResolveOptions{MaxDefinitions: 5})
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition; got %+v", defs)
	}
	d := defs[0]
	if d.Symbol != "makeLoader" || d.File != "src/loader.mjs" || d.Line != 2 {
		t.Errorf("definition = %+v, want makeLoader at src/loader.mjs:2", d)
	}
	if !strings.Contains(d.Signature, "export default function makeLoader") {
		t.Errorf("Signature = %q, want export default function makeLoader", d.Signature)
	}
}

func TestResolveSymbols_requireDestructuring_resolvesCjsExport(t *testing.T) {
	ctx := context.Background()
	dir := testutil.TempGitDir(t)
	initGitRepo(t, dir)
	writeRepoFile(t, dir, "lib/tasks.cjs", "// Appends a task.\nexports.appendTask = function appendTask(list, id) {\n  list.push(id);\n};\n")
	gitAdd(t, dir, "lib/tasks.cjs")
	writeDistractor(t, dir, "appendTask")

	hunk := "+const { appendTask } = require(\"./tasks.cjs\");\n+appendTask(list, id);\n"
	defs, err := New().ResolveSymbols(ctx, dir, "lib/run.cjs", hunk, rag.ResolveOptions{MaxDefinitions: 5})
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition; got %+v", defs)
	}
	d := defs[0]
	if d.Symbol != "appendTask" || d.File != "lib/tasks.cjs" || d.Line != 2 {
		t.Errorf("definition = %+v, want appendTask at lib/tasks.cjs:2", d)
	}
}

func TestResolveSymbols_directoryImport_resolvesIndex(t *testing.T) {
	ctx := context.Background()
	dir := testutil.TempGitDir(t)
	initGitRepo(t, dir)
	writeRepoFile(t, dir, "src/pkg/index.js", "// Formats a label.\nexport function fmt(s) {\n  return s.trim();\n}\n")
	gitAdd(t, dir, "src/pkg/index.js")
	writeDistractor(t, dir, "fmt")

	hunk := "+import { fmt } from \"./pkg\";\n+fmt(label);\n"
	defs, err := New().ResolveSymbols(ctx, dir, "src/app.js", hunk, rag.ResolveOptions{MaxDefinitions: 5})
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition; got %+v", defs)
	}
	if d := defs[0]; d.File != "src/pkg/index.js" || d.Line != 2 {
		t.Errorf("definition = %+v, want fmt at src/pkg/index.js:2", d)
	}
}

func TestResolveSymbols_extensionlessImport_resolvesWithExtension(t *testing.T) {
	ctx := context.Background()
	dir := testutil.TempGitDir(t)
	initGitRepo(t, dir)
	writeRepoFile(t, dir, "src/util.ts", "// Parses JSON safely.\nexport function fromJson(s) {\n  return JSON.parse(s);\n}\n")
	gitAdd(t, dir, "src/util.ts")
	writeDistractor(t, dir, "fromJson")

	hunk := "+import { fromJson } from \"./util\";\n+fromJson(text);\n"
	defs, err := New().ResolveSymbols(ctx, dir, "src/app.ts", hunk, rag.ResolveOptions{MaxDefinitions: 5})
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition; got %+v", defs)
	}
	if d := defs[0]; d.File != "src/util.ts" || d.Line != 2 {
		t.Errorf("definition = %+v, want fromJson at src/util.ts:2", d)
	}
}

// TestResolveSymbols_bareSpecifier_fallsBackToGitGrep keeps bare package and
// node: built-in specifiers on the plain git grep path.
func TestResolveSymbols_bareSpecifier_fallsBackToGitGrep(t *testing.T) {
	ctx := context.Background()
	dir := testutil.TempGitDir(t)
	initGitRepo(t, dir)
	writeRepoFile(t, dir, "src/util.js", "// Sorts tasks by id.\nexport function sortTasks(list) {\n  return list.slice();\n}\n")
	gitAdd(t, dir, "src/util.js")

	hunk := "+import { sortTasks } from \"task-utils\";\n+import { existsSync } from \"node:fs\";\n+sortTasks(items);\n"
	defs, err := New().ResolveSymbols(ctx, dir, "src/app.js", hunk, rag.ResolveOptions{MaxDefinitions: 5})
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition; got %+v", defs)
	}
	d := defs[0]
	if d.Symbol != "sortTasks" || d.File != "src/util.js" || d.Line != 2 {
		t.Errorf("definition = %+v, want sortTasks via git grep at src/util.js:2", d)
	}
}

// TestResolveSymbols_moduleLookupMisses_fallBackToGitGrep covers the two
// remaining fallback triggers: a missing module file and a module without a
// matching export.
func TestResolveSymbols_moduleLookupMisses_fallBackToGitGrep(t *testing.T) {
	ctx := context.Background()
	dir := testutil.TempGitDir(t)
	initGitRepo(t, dir)
	writeRepoFile(t, dir, "src/util.js", "// Sorts tasks by id.\nexport function sortTasks(list) {\n  return list.slice();\n}\n")
	gitAdd(t, dir, "src/util.js")
	writeRepoFile(t, dir, "src/empty.js", "export const other = 1;\n")
	gitAdd(t, dir, "src/empty.js")

	cases := []struct {
		name string
		hunk string
	}{
		{"missing module file", "+import { sortTasks } from \"./missing.js\";\n+sortTasks(items);\n"},
		{"no matching export", "+import { sortTasks } from \"./empty.js\";\n+sortTasks(items);\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defs, err := New().ResolveSymbols(ctx, dir, "src/app.js", tc.hunk, rag.ResolveOptions{MaxDefinitions: 5})
			if err != nil {
				t.Fatalf("ResolveSymbols: %v", err)
			}
			if len(defs) != 1 {
				t.Fatalf("expected 1 definition; got %+v", defs)
			}
			if d := defs[0]; d.Symbol != "sortTasks" || d.File != "src/util.js" || d.Line != 2 {
				t.Errorf("definition = %+v, want sortTasks via git grep at src/util.js:2", d)
			}
		})
	}
}

// TestResolveSymbols_escapingSpecifier_isNotRead verifies that a relative
// specifier leaving the repository — lexically or through a symlink — is
// never read: the symbol simply falls back to git grep and finds nothing.
func TestResolveSymbols_escapingSpecifier_isNotRead(t *testing.T) {
	ctx := context.Background()
	dir := testutil.TempGitDir(t)
	initGitRepo(t, dir)
	outside := filepath.Join(t.TempDir(), "secret.js")
	if err := os.WriteFile(outside, []byte("export function secret() {\n  return 42;\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("dot-dot escape", func(t *testing.T) {
		hunk := "+import { secret } from \"../../secret.js\";\n+secret(name);\n"
		defs, err := New().ResolveSymbols(ctx, dir, "src/app.mjs", hunk, rag.ResolveOptions{MaxDefinitions: 5})
		if err != nil {
			t.Fatalf("ResolveSymbols: %v", err)
		}
		if len(defs) != 0 {
			t.Fatalf("escaped module must not resolve; got %+v", defs)
		}
	})

	t.Run("symlink escape", func(t *testing.T) {
		writeRepoFile(t, dir, "src/keep.txt", "tracked so src exists\n")
		if err := os.Symlink(outside, filepath.Join(dir, "src", "direct.js")); err != nil {
			t.Skip("symlinks unavailable:", err)
		}
		hunk := "+import { secret } from \"./direct.js\";\n+secret(name);\n"
		defs, err := New().ResolveSymbols(ctx, dir, "src/app.mjs", hunk, rag.ResolveOptions{MaxDefinitions: 5})
		if err != nil {
			t.Fatalf("ResolveSymbols: %v", err)
		}
		if len(defs) != 0 {
			t.Fatalf("symlinked module outside the repo must not resolve; got %+v", defs)
		}
	})
}

// writeRepoFile writes content to rel under dir, creating parent directories.
func writeRepoFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// writeDistractor commits an unrelated declaration of name in alpha/<name>.js.
// git grep returns matches in index (path) order, so "alpha/..." sorts before
// the src/ and lib/ files used in these tests and would be returned first by
// the plain git grep fallback — a definition that resolves elsewhere proves
// the module-path lookup ran.
func writeDistractor(t *testing.T, dir, name string) {
	t.Helper()
	writeRepoFile(t, dir, "alpha/"+name+".js", "function "+name+"(x) {\n  return x;\n}\n")
	gitAdd(t, dir, "alpha/"+name+".js")
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
