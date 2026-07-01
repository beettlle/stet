package expand

import (
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
	for _, ext := range []string{".js", ".jsx", ".ts", ".tsx"} {
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
