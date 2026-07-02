package expand

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/diff"
)

func TestHunkLineRange(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantStart int
		wantEnd   int
		wantOK    bool
	}{
		{"standard", "@@ -1,3 +5,4 @@\n context\n+added", 5, 8, true},
		{"single_line", "@@ -1 +5 @@\n+line", 5, 5, true},
		{"no_count_implied", "@@ -1,2 +10 @@\n x", 10, 10, true},
		{"empty", "", 0, 0, false},
		{"no_header", "just some code", 0, 0, false},
		{"malformed", "@@ -1 +x @@", 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hunk := diff.Hunk{RawContent: tt.raw}
			start, end, ok := HunkLineRange(hunk)
			if ok != tt.wantOK || start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("HunkLineRange() = (%d, %d, %v), want (%d, %d, %v)",
					start, end, ok, tt.wantStart, tt.wantEnd, tt.wantOK)
			}
		})
	}
}

func TestExpandHunk_goFile_enclosingFunction(t *testing.T) {
	dir := t.TempDir()
	content := `package pkg

func processData(input string) (int, error) {
	var count int
	for i := 0; i < 50; i++ {
		count += i
	}
	return count, nil
}
`
	path := filepath.Join(dir, "pkg", "foo.go")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Hunk at lines 4-6 in new file (inside processData; var count and for loop)
	hunk := diff.Hunk{
		FilePath:   "pkg/foo.go",
		RawContent: "@@ -3,5 +3,5 @@\n func processData(input string) (int, error) {\n-\tvar count int\n+\tcount := 0\n\tfor i := 0; i < 50; i++ {\n\t\tcount += i\n\t}\n\treturn count, nil",
		Context:    "",
	}

	expanded, err := ExpandHunk(dir, hunk, 0)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if expanded.Context == hunk.RawContent || !strings.Contains(expanded.Context, "## Enclosing function context") {
		t.Error("expected expanded context with enclosing function")
	}
	if !strings.Contains(expanded.Context, "var count int") && !strings.Contains(expanded.Context, "count := 0") {
		t.Error("expected expanded context to contain variable declaration (count)")
	}
	if !strings.Contains(expanded.Context, "func processData") {
		t.Error("expected expanded context to contain function signature")
	}
	if !strings.Contains(expanded.Context, "## Diff hunk") {
		t.Error("expected expanded context to contain diff hunk section")
	}
}

func TestExpandHunk_goFile_noEnclosingFunction(t *testing.T) {
	dir := t.TempDir()
	content := "package pkg\n\nvar x int = 1\n"
	path := filepath.Join(dir, "pkg", "foo.go")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	hunk := diff.Hunk{
		FilePath:   "pkg/foo.go",
		RawContent: "@@ -1,2 +1,2 @@\n package pkg\n\n-var x int = 1\n+var x int = 2\n",
		Context:    "",
	}

	expanded, err := ExpandHunk(dir, hunk, 0)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if strings.Contains(expanded.Context, "## Enclosing function context") {
		t.Error("package-level hunk should not be expanded; no enclosing function")
	}
}

func TestExpandHunk_nonGoFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.ts")
	_ = os.WriteFile(path, []byte("const x = 1"), 0644)

	hunk := diff.Hunk{
		FilePath:   "app.ts",
		RawContent: "@@ -1,1 +1,1 @@\n-const x = 1\n+const x = 2\n",
		Context:    "",
	}

	expanded, err := ExpandHunk(dir, hunk, 0)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if strings.Contains(expanded.Context, "## Enclosing function context") {
		t.Error("non-Go file should not be expanded")
	}
}

func TestExpandHunk_truncation(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("package pkg\n\nfunc longFunc() {\n")
	for i := 0; i < 500; i++ {
		b.WriteString("\t_ = ")
		b.WriteString(strings.Repeat("x", 50))
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	path := filepath.Join(dir, "pkg", "long.go")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}

	// Hunk at lines 4-6 in new file (inside longFunc body)
	hunk := diff.Hunk{
		FilePath:   "pkg/long.go",
		RawContent: "@@ -3,5 +3,5 @@\n func longFunc() {\n\t_ = xxx\n\t_ = yyy\n }",
		Context:    "",
	}

	expanded, err := ExpandHunk(dir, hunk, 100)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if !strings.Contains(expanded.Context, truncateMarker) {
		t.Error("expected truncated output to contain truncate marker")
	}
	if !strings.Contains(expanded.Context, "func longFunc") {
		t.Error("expected signature to be preserved in truncated output")
	}
}

func TestExpandHunk_fileNotFound(t *testing.T) {
	dir := t.TempDir()
	hunk := diff.Hunk{
		FilePath:   "nonexistent/pkg/foo.go",
		RawContent: "@@ -1,3 +5,4 @@\n context\n+added",
		Context:    "original",
	}

	expanded, err := ExpandHunk(dir, hunk, 0)
	if err != nil {
		t.Fatalf("ExpandHunk should not return error on missing file: %v", err)
	}
	if strings.Contains(expanded.Context, "## Enclosing function context") {
		t.Error("expected unchanged hunk when file not found (fail open)")
	}
}

func TestExpandHunk_emptyRepoRoot(t *testing.T) {
	hunk := diff.Hunk{
		FilePath:   "pkg/foo.go",
		RawContent: "@@ -1,3 +5,4 @@\n+code",
		Context:    "original",
	}
	expanded, err := ExpandHunk("", hunk, 4096)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if strings.Contains(expanded.Context, "## Enclosing function context") {
		t.Error("expected unchanged when repoRoot empty")
	}
}

func TestExpandHunk_badLineRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pkg", "foo.go")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(path, []byte("package pkg\nfunc f() {}\n"), 0644)

	hunk := diff.Hunk{
		FilePath:   "pkg/foo.go",
		RawContent: "no valid hunk header here",
		Context:    "original",
	}
	expanded, err := ExpandHunk(dir, hunk, 4096)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if strings.Contains(expanded.Context, "## Enclosing function context") {
		t.Error("expected unchanged when line range cannot be parsed")
	}
}

func TestExpandHunk_pathTraversal_rejected(t *testing.T) {
	dir := t.TempDir()
	// Create a valid file inside repo
	path := filepath.Join(dir, "pkg", "foo.go")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	content := "package pkg\n\nfunc f() {}\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// FilePath attempts to escape repo
	hunk := diff.Hunk{
		FilePath:   filepath.Join("pkg", "..", "..", "..", "bar.go"),
		RawContent: "@@ -1,3 +1,3 @@\n package pkg\n\n func f() {}",
		Context:    "",
	}

	expanded, err := ExpandHunk(dir, hunk, 0)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if strings.Contains(expanded.Context, "## Enclosing function context") {
		t.Error("path traversal must be rejected; expected no expansion")
	}
}

func TestExpandHunk_largeFile_skipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pkg", "large.go")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	// Create file > maxExpandFileSize (1 MiB)
	data := make([]byte, maxExpandFileSize+1)
	copy(data, []byte("package pkg\n\nfunc f() {\n}\n"))
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	hunk := diff.Hunk{
		FilePath:   "pkg/large.go",
		RawContent: "@@ -1,4 +1,4 @@\n package pkg\n\n func f() {\n }",
		Context:    "",
	}

	expanded, err := ExpandHunk(dir, hunk, 0)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if strings.Contains(expanded.Context, "## Enclosing function context") {
		t.Error("large file must be skipped; expected no expansion")
	}
}

func TestEnclosingFuncName(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		startLine int
		endLine   int
		wantName  string
		wantOK    bool
	}{
		{
			name: "inside_function",
			content: `package pkg

func processData(input string) (int, error) {
	var count int
	return count, nil
}
`,
			startLine: 4,
			endLine:   5,
			wantName:  "processData",
			wantOK:    true,
		},
		{
			name: "inside_method",
			content: `package pkg

type S struct{}

func (s *S) DoWork() int {
	return 42
}
`,
			startLine: 5,
			endLine:   6,
			wantName:  "(*S).DoWork",
			wantOK:    true,
		},
		{
			name: "method_value_receiver",
			content: `package pkg

type T struct{}

func (t T) Foo() {}
`,
			startLine: 5,
			endLine:   5,
			wantName:  "(T).Foo",
			wantOK:    true,
		},
		{
			name: "file_level_no_function",
			content: `package pkg

var x int = 1
`,
			startLine: 1,
			endLine:   3,
			wantName:  "",
			wantOK:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "pkg", "code.go")
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}
			gotName, gotOK := EnclosingFuncName(dir, "pkg/code.go", tt.startLine, tt.endLine)
			if gotOK != tt.wantOK || gotName != tt.wantName {
				t.Errorf("EnclosingFuncName() = (%q, %v), want (%q, %v)", gotName, gotOK, tt.wantName, tt.wantOK)
			}
		})
	}
}

func TestFindEnclosingFunc_nilBody_skipped(t *testing.T) {
	// A FuncDecl with Body == nil (forward/cgo declaration) must be skipped.
	fset := token.NewFileSet()
	file := fset.AddFile("test.go", fset.Base(), 100)
	_ = file // registered in fset for position tracking

	decl := &ast.FuncDecl{
		Name: ast.NewIdent("ExternalFunc"),
		Type: &ast.FuncType{Params: &ast.FieldList{}},
		Body: nil, // forward declaration, no body
	}
	astFile := &ast.File{
		Name:  ast.NewIdent("pkg"),
		Decls: []ast.Decl{decl},
	}
	result := findEnclosingFunc(fset, astFile, 1, 1)
	if result != nil {
		t.Errorf("findEnclosingFunc returned non-nil for nil-body FuncDecl; want nil")
	}
}

func TestEnclosingFuncName_nonGo_returnsFalse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.ts")
	_ = os.WriteFile(path, []byte("export function foo() {}"), 0644)
	name, ok := EnclosingFuncName(dir, "app.ts", 1, 1)
	if ok || name != "" {
		t.Errorf("EnclosingFuncName(non-Go) = (%q, %v), want (\"\", false)", name, ok)
	}
}

func TestEnclosingFuncName_invalidPath_returnsFalse(t *testing.T) {
	dir := t.TempDir()
	name, ok := EnclosingFuncName(dir, "nonexistent/foo.go", 1, 1)
	if ok || name != "" {
		t.Errorf("EnclosingFuncName(invalid path) = (%q, %v), want (\"\", false)", name, ok)
	}
}

func TestEnclosingFuncName_emptyRepoRoot_returnsFalse(t *testing.T) {
	name, ok := EnclosingFuncName("", "pkg/foo.go", 1, 1)
	if ok || name != "" {
		t.Errorf("EnclosingFuncName(empty repo) = (%q, %v), want (\"\", false)", name, ok)
	}
}

func TestExpandHunk_Python_enclosingFunction(t *testing.T) {
	tests := []struct {
		name    string
		content string
		hunk    diff.Hunk
	}{
		{
			name: "module_function",
			content: `def process_data(input_str: str) -> int:
    count = 0
    for i in range(50):
        count += i
    return count
`,
			hunk: diff.Hunk{
				FilePath:   "pkg/foo.py",
				RawContent: "@@ -2,3 +2,3 @@\n-    count = 0\n+    count = 1\n     for i in range(50):",
			},
		},
		{
			name: "class_method",
			content: `class Widget:
    async def render(self) -> None:
        label = "hello"
        print(label)
`,
			hunk: diff.Hunk{
				FilePath:   "Widget.py",
				RawContent: "@@ -3,2 +3,2 @@\n-        label = \"hello\"\n+        label = \"world\"\n         print(label)",
			},
		},
		{
			name: "decorated_method",
			content: `@property
def name(self) -> str:
    return self._name
`,
			hunk: diff.Hunk{
				FilePath:   "model.py",
				RawContent: "@@ -3,1 +3,1 @@\n-    return self._name\n+    return self._label",
			},
		},
		{
			name: "multiline_signature",
			content: `def long_sig(
    a: int,
    b: int,
) -> int:
    return a + b
`,
			hunk: diff.Hunk{
				FilePath:   "math.py",
				RawContent: "@@ -5,1 +5,1 @@\n-    return a + b\n+    return a * b",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, filepath.FromSlash(tt.hunk.FilePath))
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			expanded, err := ExpandHunk(dir, tt.hunk, 0)
			if err != nil {
				t.Fatalf("ExpandHunk: %v", err)
			}
			if expanded.Context == tt.hunk.RawContent || !strings.Contains(expanded.Context, "## Enclosing function context") {
				t.Error("expected expanded context with enclosing function")
			}
			if !strings.Contains(expanded.Context, "```python") {
				t.Error("expected python code fence")
			}
			if !strings.Contains(expanded.Context, "## Diff hunk") {
				t.Error("expected expanded context to contain diff hunk section")
			}
		})
	}
}

func TestExpandHunk_Python_noEnclosingFunction(t *testing.T) {
	dir := t.TempDir()
	content := `x = 1
y = 2
`
	path := filepath.Join(dir, "top.py")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	hunk := diff.Hunk{
		FilePath:   "top.py",
		RawContent: "@@ -1,2 +1,2 @@\n-x = 1\n+x = 2\n y = 2",
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

func TestExpandHunk_Python_truncation(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("def long_func():\n")
	for i := 0; i < 500; i++ {
		b.WriteString("    x")
		b.WriteString(strings.Repeat("y", 50))
		b.WriteString(" = 1\n")
	}
	b.WriteString("\n")
	path := filepath.Join(dir, "long.py")
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}

	hunk := diff.Hunk{
		FilePath:   "long.py",
		RawContent: "@@ -2,3 +2,3 @@\n def long_func():\n     a = 1\n     b = 2",
		Context:    "",
	}

	expanded, err := ExpandHunk(dir, hunk, 100)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if !strings.Contains(expanded.Context, truncateMarker) {
		t.Error("expected truncated output to contain truncate marker")
	}
	if !strings.Contains(expanded.Context, "def long_func") {
		t.Error("expected signature to be preserved in truncated output")
	}
}

func TestExpandHunk_Python_failOpen_badHunkHeader(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.py"), []byte("def f():\n    pass\n"), 0644)
	hunk := diff.Hunk{FilePath: "a.py", RawContent: "not a hunk", Context: "keep"}
	expanded, err := ExpandHunk(dir, hunk, 0)
	if err != nil {
		t.Fatalf("ExpandHunk: %v", err)
	}
	if expanded.Context != "keep" {
		t.Error("expected unchanged context on bad hunk header")
	}
}

func TestFindEnclosingPythonScope_smallestWins(t *testing.T) {
	src := []byte(`def outer():
    def inner():
        return 1
    return inner()
`)
	scope, ok := findEnclosingPythonScope(src, 3, 3)
	if !ok {
		t.Fatal("expected enclosing scope")
	}
	if !strings.Contains(extractLineRange(src, scope.startLine, scope.endLine), "def inner") {
		t.Errorf("expected inner function, got lines %d-%d: %q", scope.startLine, scope.endLine, extractLineRange(src, scope.startLine, scope.endLine))
	}
}

func TestLooksLikePythonDef_rejectsClass(t *testing.T) {
	if looksLikePythonDef("class Foo:") {
		t.Error("class declaration should not match")
	}
	if !looksLikePythonDef("    def bar(self):") {
		t.Error("indented method should match")
	}
}
