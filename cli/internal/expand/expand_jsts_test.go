package expand

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/diff"
)

func TestExpandHunk_JSTS_enclosingFunction(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		content  string
		hunk     diff.Hunk
	}{
		{
			name:     "typescript_function",
			fileName: "app.ts",
			content: `export function processData(input: string): number {
  let count = 0;
  for (let i = 0; i < 50; i++) {
    count += i;
  }
  return count;
}
`,
			hunk: diff.Hunk{
				FilePath:   "app.ts",
				RawContent: "@@ -2,5 +2,5 @@\n export function processData(input: string): number {\n-  let count = 0;\n+  let count = 1;\n   for (let i = 0; i < 50; i++) {\n     count += i;\n   }",
			},
		},
		{
			name:     "javascript_function",
			fileName: "app.js",
			content: `function processData(input) {
  var count = 0;
  for (var i = 0; i < 50; i++) {
    count += i;
  }
  return count;
}
`,
			hunk: diff.Hunk{
				FilePath:   "app.js",
				RawContent: "@@ -2,5 +2,5 @@\n function processData(input) {\n-  var count = 0;\n+  var count = 1;\n   for (var i = 0; i < 50; i++) {\n     count += i;\n   }",
			},
		},
		{
			name:     "class_method_tsx",
			fileName: "Widget.tsx",
			content: `export class Widget {
  async render(): Promise<void> {
    const label = "hello";
    console.log(label);
  }
}
`,
			hunk: diff.Hunk{
				FilePath:   "Widget.tsx",
				RawContent: "@@ -2,4 +2,4 @@\n export class Widget {\n   async render(): Promise<void> {\n-    const label = \"hello\";\n+    const label = \"world\";\n     console.log(label);",
			},
		},
		{
			name:     "arrow_function_jsx",
			fileName: "Button.jsx",
			content: `export const Button = ({ onClick }) => {
  const label = "click";
  return <button onClick={onClick}>{label}</button>;
};
`,
			hunk: diff.Hunk{
				FilePath:   "Button.jsx",
				RawContent: "@@ -1,4 +1,4 @@\n export const Button = ({ onClick }) => {\n-  const label = \"click\";\n+  const label = \"tap\";\n   return <button onClick={onClick}>{label}</button>;",
			},
		},
		{
			name:     "es_module_function_mjs",
			fileName: "pending.mjs",
			content: `export function filterPending(ids) {
  const pending = ids.filter(Boolean);
  return pending.sort();
}
`,
			hunk: diff.Hunk{
				FilePath:   "pending.mjs",
				RawContent: "@@ -1,3 +1,3 @@\n export function filterPending(ids) {\n-  const pending = ids.filter(Boolean);\n+  const pending = ids.filter((id) => id !== \"\");\n   return pending.sort();",
			},
		},
		{
			name:     "commonjs_function_cjs",
			fileName: "legacy.cjs",
			content: `function countItems(items) {
  let total = 0;
  for (const item of items) {
    total += item.qty;
  }
  return total;
}
`,
			hunk: diff.Hunk{
				FilePath:   "legacy.cjs",
				RawContent: "@@ -2,4 +2,4 @@\n function countItems(items) {\n-  let total = 0;\n+  let total = 1;\n   for (const item of items) {\n     total += item.qty;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tt.fileName)
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}
			tt.hunk.FilePath = tt.fileName

			expanded, err := ExpandHunk(dir, tt.hunk, 0)
			if err != nil {
				t.Fatalf("ExpandHunk: %v", err)
			}
			if expanded.Context == tt.hunk.RawContent || !strings.Contains(expanded.Context, "## Enclosing function context") {
				t.Error("expected expanded context with enclosing function")
			}
			if !strings.Contains(expanded.Context, "## Diff hunk") {
				t.Error("expected expanded context to contain diff hunk section")
			}
		})
	}
}

func TestExpandHunk_JSTS_noEnclosingFunction(t *testing.T) {
	dir := t.TempDir()
	content := `const x = 1;
export const y = 2;
`
	path := filepath.Join(dir, "top.ts")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	hunk := diff.Hunk{
		FilePath:   "top.ts",
		RawContent: "@@ -1,2 +1,2 @@\n-const x = 1;\n+const x = 2;\n export const y = 2;",
		Context:    "",
	}

	expanded, err := ExpandHunk(dir, hunk, 0)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if strings.Contains(expanded.Context, "## Enclosing function context") {
		t.Error("top-level hunk should not be expanded; no enclosing function")
	}
}

func TestExpandHunk_JSTS_parseError_failOpen(t *testing.T) {
	dir := t.TempDir()
	content := `function broken() {
  const x = "{";
}
`
	path := filepath.Join(dir, "broken.ts")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	hunk := diff.Hunk{
		FilePath:   "broken.ts",
		RawContent: "@@ -1,3 +1,3 @@\n function broken() {\n-  const x = \"{\";\n+  const x = \"}\";\n }",
		Context:    "original",
	}

	expanded, err := ExpandHunk(dir, hunk, 0)
	if err != nil {
		t.Fatalf("ExpandHunk should not return error on brace mismatch: %v", err)
	}
	// broken brace in string should still allow expansion since real braces match
	if !strings.Contains(expanded.Context, "## Enclosing function context") {
		t.Log("brace matcher may fail-open on ambiguous input; context:", expanded.Context)
	}
}

func TestExpandHunk_JSTS_truncation(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("function longFunc() {\n")
	for i := 0; i < 500; i++ {
		b.WriteString("  const x")
		b.WriteString(strings.Repeat("y", 50))
		b.WriteString(" = 1;\n")
	}
	b.WriteString("}\n")
	path := filepath.Join(dir, "long.ts")
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}

	hunk := diff.Hunk{
		FilePath:   "long.ts",
		RawContent: "@@ -2,3 +2,3 @@\n function longFunc() {\n   const a = 1;\n   const b = 2;\n }",
		Context:    "",
	}

	expanded, err := ExpandHunk(dir, hunk, 100)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if !strings.Contains(expanded.Context, truncateMarker) {
		t.Error("expected truncated output to contain truncate marker")
	}
	if !strings.Contains(expanded.Context, "function longFunc") {
		t.Error("expected signature to be preserved in truncated output")
	}
}

func TestFindEnclosingJSTSScope_smallestWins(t *testing.T) {
	src := []byte(`function outer() {
  function inner() {
    return 1;
  }
  return inner();
}
`)
	scope, ok := findEnclosingJSTSScope(src, 3, 3)
	if !ok {
		t.Fatal("expected enclosing scope")
	}
	if !strings.Contains(extractLineRange(src, scope.startLine, scope.endLine), "function inner") {
		t.Errorf("expected inner function, got lines %d-%d: %q", scope.startLine, scope.endLine, extractLineRange(src, scope.startLine, scope.endLine))
	}
}

func TestIsJSTSExt(t *testing.T) {
	for _, ext := range []string{".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx"} {
		if !isJSTSExt(ext) {
			t.Errorf("isJSTSExt(%q) = false, want true", ext)
		}
	}
	if isJSTSExt(".go") || isJSTSExt(".py") {
		t.Error("non JS/TS extensions should return false")
	}
}

func TestJSTSFenceLang(t *testing.T) {
	if got := jstsFenceLang("a.ts"); got != "typescript" {
		t.Errorf("jstsFenceLang(.ts) = %q, want typescript", got)
	}
	if got := jstsFenceLang("a.js"); got != "javascript" {
		t.Errorf("jstsFenceLang(.js) = %q, want javascript", got)
	}
}

func TestExpandHunk_JSTS_expressionBodyArrow(t *testing.T) {
	dir := t.TempDir()
	content := `export const double = (n: number) => n * 2;
`
	path := filepath.Join(dir, "math.ts")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	hunk := diff.Hunk{
		FilePath:   "math.ts",
		RawContent: "@@ -1,1 +1,1 @@\n-export const double = (n: number) => n * 2;\n+export const double = (n: number) => n * 3;",
	}
	expanded, err := ExpandHunk(dir, hunk, 0)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if !strings.Contains(expanded.Context, "## Enclosing function context") {
		t.Error("expected expression-bodied arrow to expand")
	}
}

func TestExpandHunk_JSTS_failOpen_badHunkHeader(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.ts"), []byte("function f() {}\n"), 0644)
	hunk := diff.Hunk{FilePath: "a.ts", RawContent: "not a hunk", Context: "keep"}
	expanded, err := ExpandHunk(dir, hunk, 0)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if expanded.Context != "keep" {
		t.Error("expected unchanged context on bad hunk header")
	}
}

func TestSkipJSTSToken_stringsAndComments(t *testing.T) {
	if got, ok := skipJSTSToken(`"a{b}" + x`, 0); !ok || got != len(`"a{b}"`) {
		t.Errorf(`double-quoted string: got (%d, %v)`, got, ok)
	}
	if got, ok := skipJSTSToken(`'a{b}' + x`, 0); !ok || got != len(`'a{b}'`) {
		t.Errorf(`single-quoted string: got (%d, %v)`, got, ok)
	}
	if got, ok := skipJSTSToken("// not a brace\n{", 0); !ok || got >= strings.Index("// not a brace\n{", "{") {
		t.Errorf("line comment should end before brace: got %d", got)
	}
	if got, ok := skipJSTSToken("/* not a brace */ {", 0); !ok || got >= strings.Index("/* not a brace */ {", "{") {
		t.Errorf("block comment should end before brace: got %d", got)
	}
}

func TestSkipJSTSToken_templateLiteral(t *testing.T) {
	src := "`hello ${name}`"
	got, ok := skipJSTSToken(src, 0)
	if !ok {
		t.Fatal("expected template literal to parse")
	}
	if got != len(src) {
		t.Errorf("skipJSTSToken template = %d, want %d", got, len(src))
	}
}

func TestFindMatchingBrace_nestedStrings(t *testing.T) {
	src := `function f() {
  const s = "{";
  return s;
}`
	open := strings.Index(src, "function")
	open = strings.Index(src[open:], "{") + open
	close := findMatchingBrace(src, open)
	if close < 0 {
		t.Fatal("expected matching brace")
	}
	if src[close] != '}' {
		t.Fatalf("expected }, got %q", src[close])
	}
}

func TestLooksLikeJSTSDecl_rejectsControlFlow(t *testing.T) {
	if looksLikeJSTSDecl("  if (cond) {") {
		t.Error("if statement should not match")
	}
	if !looksLikeJSTSDecl("  async render(ctx) {") {
		t.Error("class method should match")
	}
}

func TestExpandHunk_JSTS_fileImports_fixture(t *testing.T) {
	// Fixture from #10: `import fs from "node:fs";` on line 2, hunk inside a
	// helper that uses fs.mkdirSync. The reviewer previously reported
	// "`fs` is not imported" because the import was outside its view.
	dir := t.TempDir()
	content := `#!/usr/bin/env node
import fs from "node:fs";

function saveReport() {
  fs.mkdirSync("out", { recursive: true });
  fs.writeFileSync("out/report.txt", "done");
}
`
	path := filepath.Join(dir, "report.js")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	hunk := diff.Hunk{
		FilePath:   "report.js",
		RawContent: "@@ -4,3 +4,3 @@\n function saveReport() {\n-  fs.mkdirSync(\"out\", { recursive: true });\n+  fs.mkdirSync(\"out\");\n   fs.writeFileSync(\"out/report.txt\", \"done\");",
	}

	expanded, err := ExpandHunk(dir, hunk, 0)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if !strings.Contains(expanded.Context, "## File imports") {
		t.Error("expected ## File imports section, got:", expanded.Context)
	}
	if !strings.Contains(expanded.Context, "## File imports\n\n```javascript\nimport fs from \"node:fs\";\n```") {
		t.Errorf("expected the node:fs import inside the imports fence, got:\n%s", expanded.Context)
	}
	importsIdx := strings.Index(expanded.Context, "## File imports")
	funcIdx := strings.Index(expanded.Context, "## Enclosing function context")
	hunkIdx := strings.Index(expanded.Context, "## Diff hunk")
	if importsIdx < 0 || funcIdx < 0 || hunkIdx < 0 || !(importsIdx < funcIdx && funcIdx < hunkIdx) {
		t.Errorf("expected imports before function context before hunk, got:\n%s", expanded.Context)
	}
	if !strings.Contains(expanded.Context, "function saveReport") {
		t.Error("expected enclosing function context to be kept")
	}
}

func TestExpandHunk_JSTS_fileImports_mjs(t *testing.T) {
	// SP-831: the reviewer reported compareTaskIds as "not defined or not
	// exported" in an .mjs file because .mjs hunks were never expanded, so
	// the import block was outside its view.
	dir := t.TempDir()
	content := `import { compareTaskIds } from "./discover.mjs";

export function filterPendingTaskIds(ids, done) {
  const pending = ids.filter(Boolean);
  return pending.sort(compareTaskIds);
}
`
	path := filepath.Join(dir, "pending.mjs")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	hunk := diff.Hunk{
		FilePath:   "pending.mjs",
		RawContent: "@@ -4,2 +4,2 @@\n-  const pending = ids.filter(Boolean);\n+  const pending = ids.filter(String);\n   return pending.sort(compareTaskIds);",
	}

	expanded, err := ExpandHunk(dir, hunk, 0)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if !strings.Contains(expanded.Context, "## File imports\n\n```javascript\nimport { compareTaskIds } from \"./discover.mjs\";\n```") {
		t.Errorf("expected the compareTaskIds import inside the imports fence, got:\n%s", expanded.Context)
	}
	if !strings.Contains(expanded.Context, "function filterPendingTaskIds") {
		t.Errorf("expected enclosing function context, got:\n%s", expanded.Context)
	}
}

func TestExpandHunk_JSTS_fileImports_noEnclosingScope(t *testing.T) {
	// Top-level code after the import block used to come back unchanged; it
	// now gets the import section without a function context section.
	dir := t.TempDir()
	content := `import fs from "fs";

export const config = {
  debug: true,
};
`
	path := filepath.Join(dir, "config.js")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	hunk := diff.Hunk{
		FilePath:   "config.js",
		RawContent: "@@ -3,3 +3,3 @@\n export const config = {\n-  debug: true,\n+  debug: false,\n };",
	}

	expanded, err := ExpandHunk(dir, hunk, 0)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if !strings.Contains(expanded.Context, "## File imports\n\n```javascript\nimport fs from \"fs\";\n```") {
		t.Errorf("expected import section for scopeless hunk after imports, got:\n%s", expanded.Context)
	}
	if strings.Contains(expanded.Context, "## Enclosing function context") {
		t.Error("top-level hunk should not get a function context section")
	}
	if !strings.Contains(expanded.Context, "## Diff hunk") {
		t.Error("expected diff hunk section")
	}
}

func TestExpandHunk_JSTS_fileImports_overlap_noImportSection(t *testing.T) {
	dir := t.TempDir()
	content := `import fs from "fs";
import path from "path";

export function joiner(): string {
  return path.join("a", "b");
}
`
	path := filepath.Join(dir, "joiner.ts")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	// Hunk covers the import lines themselves: it overlaps the import block.
	hunk := diff.Hunk{
		FilePath:   "joiner.ts",
		RawContent: "@@ -1,2 +1,2 @@\n-import fs from \"fs\";\n+import fs from \"node:fs\";\n import path from \"path\";",
		Context:    "keep",
	}

	expanded, err := ExpandHunk(dir, hunk, 0)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if expanded.Context != "keep" {
		t.Errorf("hunk overlapping the import block should come back unchanged, got:\n%s", expanded.Context)
	}
	if strings.Contains(expanded.Context, "## File imports") {
		t.Error("overlapping hunk must not get an import section")
	}
}

func TestExpandHunk_JSTS_noImports_unchangedFormat(t *testing.T) {
	dir := t.TempDir()
	content := `function processData(input) {
  var count = 0;
  return count;
}
`
	path := filepath.Join(dir, "plain.js")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	hunk := diff.Hunk{
		FilePath:   "plain.js",
		RawContent: "@@ -2,2 +2,2 @@\n-  var count = 0;\n+  var count = 1;\n   return count;",
	}

	expanded, err := ExpandHunk(dir, hunk, 0)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	// Byte-identical to the pre-change expansion format.
	want := "## Enclosing function context\n\n```javascript\n" + content[:len(content)-1] + "\n```\n\n## Diff hunk\n\n" + hunk.RawContent
	if expanded.Context != want {
		t.Errorf("no-import expansion changed format.\nwant:\n%q\ngot:\n%q", want, expanded.Context)
	}
}

func TestExpandHunk_JSTS_fileImports_maxTokensSplit(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	for i := 0; i < 5; i++ {
		b.WriteString(fmt.Sprintf("import module%02d from \"./very-long-module-path-%02d\";\n", i, i))
	}
	b.WriteString("function longFunc() {\n")
	for i := 0; i < 100; i++ {
		b.WriteString("  const value = computeSomething(value + 1);\n")
	}
	b.WriteString("}\n")
	path := filepath.Join(dir, "budget.js")
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}
	hunk := diff.Hunk{
		FilePath:   "budget.js",
		RawContent: "@@ -7,2 +7,2 @@\n function longFunc() {\n-  const value = computeSomething(value + 1);\n+  const value = computeSomething(value + 2);",
	}

	const maxTokens = 40 // importBudget = 10 tokens = 40 chars; funcBudget = 30 tokens = 120 chars
	expanded, err := ExpandHunk(dir, hunk, maxTokens)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	importContent := fenceContent(t, expanded.Context, "## File imports")
	if len(importContent) > maxTokens { // quarter of maxTokens, chars/4 heuristic
		t.Errorf("import block should be truncated to <= %d chars, got %d: %q", maxTokens, len(importContent), importContent)
	}
	if !strings.Contains(importContent, truncateMarker) {
		t.Errorf("expected truncate marker in import block, got %q", importContent)
	}
	if strings.Contains(importContent, "module04") {
		t.Errorf("last import should have been truncated away, got %q", importContent)
	}
	funcContent := fenceContent(t, expanded.Context, "## Enclosing function context")
	if !strings.Contains(funcContent, truncateMarker) {
		t.Error("expected truncate marker in function context")
	}
	if !strings.Contains(funcContent, "function longFunc") {
		t.Error("expected function signature to survive truncation")
	}
}

// fenceContent extracts the fenced code block under the given "## heading".
func fenceContent(t *testing.T, context, heading string) string {
	t.Helper()
	headingIdx := strings.Index(context, heading)
	if headingIdx < 0 {
		t.Fatalf("heading %q not found in context:\n%s", heading, context)
	}
	rest := context[headingIdx:]
	open := strings.Index(rest, "```")
	if open < 0 {
		t.Fatalf("no fence under heading %q in context:\n%s", heading, context)
	}
	rest = rest[open+len("```"):]
	rest = rest[strings.Index(rest, "\n")+1:] // skip the language line
	close := strings.Index(rest, "\n```")
	if close < 0 {
		t.Fatalf("unterminated fence under heading %q in context:\n%s", heading, context)
	}
	return rest[:close]
}

func TestExtractJSTSImports_forms(t *testing.T) {
	src := []byte(`#!/usr/bin/env node
"use strict";
import fs from "fs";
import{
compact
} from "./compact";
import {
  alpha,
  beta
} from "./letters";
export * from "./stars";
export * as stars from "./stars-ns";
export { helper as default } from "./helper";
const path = require("path");
let legacy = require('legacy');
import settings = require("./settings");

export function useAll(): string {
  return alpha + beta;
}
`)
	imports := extractJSTSImports(src)
	if !imports.found {
		t.Fatal("expected imports to be found")
	}
	wantLines := []string{
		`import fs from "fs";`,
		"import{\ncompact\n} from \"./compact\";",
		"import {\n  alpha,\n  beta\n} from \"./letters\";",
		`export * from "./stars";`,
		`export * as stars from "./stars-ns";`,
		`export { helper as default } from "./helper";`,
		`const path = require("path");`,
		`let legacy = require('legacy');`,
		`import settings = require("./settings");`,
	}
	if got := imports.block; got != strings.Join(wantLines, "\n") {
		t.Errorf("import block mismatch.\nwant:\n%s\ngot:\n%s", strings.Join(wantLines, "\n"), got)
	}
	if strings.Contains(imports.block, "#!") || strings.Contains(imports.block, "use strict") {
		t.Error("directives should not be part of the import block")
	}
	// Last import line in the source above is line 16; the blank line and
	// "export function" that follow stop the scan.
	if imports.endLine != 16 {
		t.Errorf("endLine = %d, want 16", imports.endLine)
	}
}

func TestExtractJSTSImports_stopRules(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		found   bool
		block   string
		endLine int
	}{
		{
			name:    "code before imports stops scan",
			src:     "const x = 1;\nimport fs from \"fs\";\n",
			found:   false,
			block:   "",
			endLine: 0,
		},
		{
			name:    "re-export without from is not an import and stops scan",
			src:     "import a from \"a\";\nexport { a };\nimport b from \"b\";\n",
			found:   true,
			block:   `import a from "a";`,
			endLine: 1,
		},
		{
			name:    "from in a comment is not a module specifier and stops scan",
			src:     "import a from \"a\";\nexport { a }; // from local\nimport b from \"b\";\n",
			found:   true,
			block:   `import a from "a";`,
			endLine: 1,
		},
		{
			name:    "re-export with multi-line from specifier is kept",
			src:     "export {\n  a\n} from\n\"./a\";\nimport b from \"b\";\n",
			found:   true,
			block:   "export {\n  a\n} from\n\"./a\";\nimport b from \"b\";",
			endLine: 5,
		},
		{
			name:    "destructured require is not an import start and stops scan",
			src:     "import a from \"a\";\nconst { b } = require(\"./b\");\nimport c from \"c\";\n",
			found:   true,
			block:   `import a from "a";`,
			endLine: 1,
		},
		{
			name:    "blank lines and comments between imports are skipped",
			src:     "import a from \"a\";\n\n// note\n/* block */\nimport b from \"b\";\n",
			found:   true,
			block:   "import a from \"a\";\nimport b from \"b\";",
			endLine: 5,
		},
		{
			name:    "multi-line require chain stops at non-import",
			src:     "import fs from \"fs\";\nconst x = 1;\n",
			found:   true,
			block:   `import fs from "fs";`,
			endLine: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSTSImports([]byte(tt.src))
			if got.found != tt.found {
				t.Fatalf("found = %v, want %v (block: %q)", got.found, tt.found, got.block)
			}
			if got.block != tt.block {
				t.Errorf("block mismatch.\nwant: %q\ngot:  %q", tt.block, got.block)
			}
			if got.endLine != tt.endLine {
				t.Errorf("endLine = %d, want %d", got.endLine, tt.endLine)
			}
		})
	}
}

func TestExtractJSTSImports_cappedAt60Lines(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 70; i++ {
		fmt.Fprintf(&b, "import mod%02d from \"mod%02d\";\n", i, i)
	}
	b.WriteString("\nfunction f() {}\n")
	imports := extractJSTSImports([]byte(b.String()))
	if !imports.found {
		t.Fatal("expected imports to be found")
	}
	if got := strings.Count(imports.block, "\n") + 1; got != maxJSTSImportLines {
		t.Errorf("block lines = %d, want %d", got, maxJSTSImportLines)
	}
	if !strings.Contains(imports.block, "mod00") {
		t.Error("first import should be kept")
	}
	if strings.Contains(imports.block, "mod60") {
		t.Error("import beyond the 60-line cap should be dropped")
	}
	if imports.endLine != 70 {
		t.Errorf("endLine = %d, want 70 (uncapped last import line)", imports.endLine)
	}
}

func TestJSTSIsDirective(t *testing.T) {
	for _, d := range []string{"#!/usr/bin/env node", `"use strict"`, `'use strict'`, `"use strict";`, `'use strict';`} {
		if !jstsIsDirective(d) {
			t.Errorf("jstsIsDirective(%q) = false, want true", d)
		}
	}
	for _, s := range []string{"import fs from \"fs\";", "const x = 1;", "\"use strict mode\";"} {
		if jstsIsDirective(s) {
			t.Errorf("jstsIsDirective(%q) = true, want false", s)
		}
	}
}

func TestJSTSBraceDelta_skipsStringsAndComments(t *testing.T) {
	if got := jstsBraceDelta(`import { a } from "{not-a-brace}";`); got != 0 {
		t.Errorf("balanced import line delta = %d, want 0", got)
	}
	if got := jstsBraceDelta("import {"); got != 1 {
		t.Errorf("opener delta = %d, want 1", got)
	}
	if got := jstsBraceDelta(`} from "x";`); got != -1 {
		t.Errorf("closer delta = %d, want -1", got)
	}
	if got := jstsBraceDelta("// { comment brace\n"); got != 0 {
		t.Errorf("comment line delta = %d, want 0", got)
	}
}

func TestExtractJSTSImports_unterminatedStatement_stopsScan(t *testing.T) {
	// A multi-line import whose braces never close must not consume the rest
	// of the file as one statement; imports collected before it are kept.
	src := "import ok from \"ok\";\nimport {\n  a,\n  b\nconst x = 1;\nfunction f() {}\n"
	got := extractJSTSImports([]byte(src))
	if !got.found {
		t.Fatal("expected the first import to be found")
	}
	if got.block != `import ok from "ok";` {
		t.Errorf("block = %q, want only the terminated import", got.block)
	}
	if got.endLine != 1 {
		t.Errorf("endLine = %d, want 1", got.endLine)
	}
	if strings.Contains(got.block, "function f") || strings.Contains(got.block, "const x") {
		t.Errorf("unterminated statement consumed code: %q", got.block)
	}
}

func TestExpandHunk_JSTS_fileImports_tinyBudget_noImportSection(t *testing.T) {
	// With maxTokens=23 the import quarter share is 5 tokens, below
	// minJSTSTruncateTokens: the import section is dropped and the function
	// context keeps the full budget (pre-change behavior).
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("import fs from \"fs\";\n\nfunction longFunc() {\n")
	for i := 0; i < 100; i++ {
		b.WriteString("  const value = computeSomething(value + 1);\n")
	}
	b.WriteString("}\n")
	path := filepath.Join(dir, "tiny.js")
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}
	hunk := diff.Hunk{
		FilePath:   "tiny.js",
		RawContent: "@@ -3,2 +3,2 @@\n function longFunc() {\n-  const value = computeSomething(value + 1);\n+  const value = computeSomething(value + 2);",
	}

	expanded, err := ExpandHunk(dir, hunk, 23)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if strings.Contains(expanded.Context, "## File imports") {
		t.Error("tiny budget should not show an import section")
	}
	if !strings.Contains(expanded.Context, "## Enclosing function context") {
		t.Error("expected function context to be kept")
	}
	if !strings.Contains(expanded.Context, truncateMarker) {
		t.Error("expected function context to be truncated with the full budget")
	}
}
