package expand

import (
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
