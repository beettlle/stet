package diff

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initRepoDiff(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "git", "init")
	runGit(t, dir, "git", "config", "user.email", "test@stet.local")
	runGit(t, dir, "git", "config", "user.name", "Test")
	writeFile(t, dir, ".gitignore", ".review\n")
	runGit(t, dir, "git", "add", ".gitignore")
	runGit(t, dir, "git", "commit", "-m", "gitignore")
	writeFile(t, dir, "f1.txt", "a\n")
	runGit(t, dir, "git", "add", "f1.txt")
	runGit(t, dir, "git", "commit", "-m", "c1")
	writeFile(t, dir, "f2.txt", "b\n")
	runGit(t, dir, "git", "add", "f2.txt")
	runGit(t, dir, "git", "commit", "-m", "c2")
	return dir
}

func runGit(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestFilterByPatterns(t *testing.T) {
	t.Parallel()
	hunks := []Hunk{
		{FilePath: "foo.go", RawContent: "@@ 1,1 @@\n", Context: "@@ 1,1 @@\n"},
		{FilePath: "gen.pb.go", RawContent: "@@ 1,1 @@\n", Context: "@@ 1,1 @@\n"},
		{FilePath: "sub/bar.go", RawContent: "@@ 1,1 @@\n", Context: "@@ 1,1 @@\n"},
		{FilePath: "vendor/pkg/x.go", RawContent: "@@ 1,1 @@\n", Context: "@@ 1,1 @@\n"},
	}
	patterns := []string{"*.pb.go", "vendor/*", "vendor/**/*"}
	got := filterByPatterns(hunks, patterns)
	if len(got) != 2 {
		t.Fatalf("filterByPatterns: got %d hunks, want 2 (foo.go and sub/bar.go)", len(got))
	}
	paths := make([]string, len(got))
	for i := range got {
		paths[i] = got[i].FilePath
	}
	if paths[0] != "foo.go" || paths[1] != "sub/bar.go" {
		t.Errorf("filtered paths = %v, want [foo.go sub/bar.go]", paths)
	}
}

func TestFilterByPatterns_emptyPatterns(t *testing.T) {
	t.Parallel()
	hunks := []Hunk{
		{FilePath: "gen.pb.go", RawContent: "x", Context: "x"},
	}
	got := filterByPatterns(hunks, nil)
	if len(got) != 1 {
		t.Errorf("filterByPatterns with nil patterns: got %d hunks, want 1", len(got))
	}
	got = filterByPatterns(hunks, []string{})
	if len(got) != 1 {
		t.Errorf("filterByPatterns with empty patterns: got %d hunks, want 1", len(got))
	}
}

func TestFilterByPatterns_malformedPattern_skipsPatternDoesNotExclude(t *testing.T) {
	t.Parallel()
	hunks := []Hunk{
		{FilePath: "foo.go", RawContent: "x", Context: "x"},
	}
	// filepath.Match("[", "foo.go") returns an error; malformed pattern must be skipped, hunk kept.
	got := filterByPatterns(hunks, []string{"["})
	if len(got) != 1 {
		t.Errorf("filterByPatterns with malformed pattern: got %d hunks, want 1 (pattern skipped)", len(got))
	}
	if len(got) > 0 && got[0].FilePath != "foo.go" {
		t.Errorf("got file %q, want foo.go", got[0].FilePath)
	}
}

func TestHunks_emptyRepoRoot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, err := Hunks(ctx, "", "HEAD~1", "HEAD", nil)
	if err == nil {
		t.Fatal("Hunks with empty repoRoot: expected error")
	}
	if !strings.Contains(err.Error(), "Repository root") && !strings.Contains(err.Error(), "required") {
		t.Errorf("Hunks error = %v", err)
	}
}

func TestHunks_integration_twoCommits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepoDiff(t)
	// HEAD~1 = c1 (f1 only), HEAD = c2 (f1 + f2). Diff is f2.txt.
	hunks, err := Hunks(ctx, repo, "HEAD~1", "HEAD", nil)
	if err != nil {
		t.Fatalf("Hunks: %v", err)
	}
	if len(hunks) != 1 {
		t.Fatalf("len(hunks) = %d, want 1", len(hunks))
	}
	if hunks[0].FilePath != "f2.txt" {
		t.Errorf("FilePath = %q, want f2.txt", hunks[0].FilePath)
	}
	if hunks[0].RawContent == "" {
		t.Error("RawContent empty")
	}
	if hunks[0].Context != hunks[0].RawContent {
		t.Error("Context should equal RawContent")
	}
}

func TestHunks_emptyDiff(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepoDiff(t)
	hunks, err := Hunks(ctx, repo, "HEAD", "HEAD", nil)
	if err != nil {
		t.Fatalf("Hunks: %v", err)
	}
	if len(hunks) != 0 {
		t.Errorf("Hunks(HEAD, HEAD) = %d hunks, want 0", len(hunks))
	}
	if hunks != nil {
		t.Errorf("Hunks(HEAD, HEAD) = %v, want nil slice", hunks)
	}
}

func TestHunks_excludeGenerated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepoDiff(t)
	writeFile(t, repo, "bar.go", "package p\n\nfunc Bar() {}\n")
	writeFile(t, repo, "gen.pb.go", "package p\n// generated\n")
	runGit(t, repo, "git", "add", "bar.go", "gen.pb.go")
	runGit(t, repo, "git", "commit", "-m", "add bar and gen")
	// Diff HEAD~1..HEAD: bar.go and gen.pb.go. Default exclusions drop gen.pb.go.
	hunks, err := Hunks(ctx, repo, "HEAD~1", "HEAD", nil)
	if err != nil {
		t.Fatalf("Hunks: %v", err)
	}
	if len(hunks) != 1 {
		t.Fatalf("len(hunks) = %d, want 1 (gen.pb.go excluded)", len(hunks))
	}
	if hunks[0].FilePath != "bar.go" {
		t.Errorf("FilePath = %q, want bar.go", hunks[0].FilePath)
	}
}

func TestHunks_customExcludePatterns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepoDiff(t)
	// Diff HEAD~1..HEAD is f2.txt. Override to exclude *.txt.
	opts := &Options{ExcludePatterns: []string{"*.txt"}}
	hunks, err := Hunks(ctx, repo, "HEAD~1", "HEAD", opts)
	if err != nil {
		t.Fatalf("Hunks: %v", err)
	}
	if len(hunks) != 0 {
		t.Fatalf("len(hunks) = %d, want 0 (f2.txt excluded by custom pattern)", len(hunks))
	}
}

func TestHunks_customExcludePatternsEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepoDiff(t)
	// Empty ExcludePatterns means no exclusions (override defaults with empty list = no filter).
	opts := &Options{ExcludePatterns: []string{}}
	hunks, err := Hunks(ctx, repo, "HEAD~1", "HEAD", opts)
	if err != nil {
		t.Fatalf("Hunks: %v", err)
	}
	if len(hunks) != 1 {
		t.Fatalf("len(hunks) = %d, want 1 when ExcludePatterns is empty", len(hunks))
	}
}

func TestCountHunkScope_fixture(t *testing.T) {
	t.Parallel()
	const addedLine = "+added"
	const removedLine = "-removed"
	raw := "@@ -1,3 +1,4 @@\n context\n" + addedLine + "\n" + removedLine + "\n"
	hunks := []Hunk{{FilePath: "a.go", RawContent: raw, Context: raw}}
	la, lr, ca, cd, cr := CountHunkScope(hunks)
	if la != 1 || lr != 1 {
		t.Errorf("lines: got added=%d removed=%d, want 1, 1", la, lr)
	}
	wantCharsAdded := len(addedLine) - 1   // exclude '+' prefix
	wantCharsDeleted := len(removedLine) - 1 // exclude '-' prefix
	if ca != wantCharsAdded || cd != wantCharsDeleted {
		t.Errorf("chars: got added=%d deleted=%d, want %d, %d", ca, cd, wantCharsAdded, wantCharsDeleted)
	}
	if cr != len(raw) {
		t.Errorf("chars_reviewed: got %d, want %d", cr, len(raw))
	}
}

func TestCountHunkScope_emptyHunks(t *testing.T) {
	t.Parallel()
	la, lr, ca, cd, cr := CountHunkScope(nil)
	if la != 0 || lr != 0 || ca != 0 || cd != 0 || cr != 0 {
		t.Errorf("empty hunks: got la=%d lr=%d ca=%d cd=%d cr=%d, want all 0", la, lr, ca, cd, cr)
	}
	la, lr, ca, cd, cr = CountHunkScope([]Hunk{})
	if la != 0 || lr != 0 || ca != 0 || cd != 0 || cr != 0 {
		t.Errorf("zero hunks: got la=%d lr=%d ca=%d cd=%d cr=%d, want all 0", la, lr, ca, cd, cr)
	}
}

func TestCountHunkScope_headersOnly(t *testing.T) {
	t.Parallel()
	raw := "--- a/old.go\n+++ b/new.go\n"
	hunks := []Hunk{{FilePath: "new.go", RawContent: raw, Context: raw}}
	la, lr, ca, cd, cr := CountHunkScope(hunks)
	if la != 0 || lr != 0 || ca != 0 || cd != 0 {
		t.Errorf("headers only: got added=%d removed=%d charsAdded=%d charsDeleted=%d, want all 0", la, lr, ca, cd)
	}
	if cr != len(raw) {
		t.Errorf("chars_reviewed: got %d, want %d", cr, len(raw))
	}
}

func TestCountHunkScope_multipleHunks(t *testing.T) {
	t.Parallel()
	const firstLine = "+first"
	const secondLine = "+second"
	hunks := []Hunk{
		{FilePath: "a.go", RawContent: "@@ -1,1 +1,2 @@\n" + firstLine + "\n", Context: ""},
		{FilePath: "b.go", RawContent: "@@ -1,1 +1,2 @@\n" + secondLine + "\n", Context: ""},
	}
	la, lr, ca, cd, cr := CountHunkScope(hunks)
	if la != 2 || lr != 0 {
		t.Errorf("lines: got added=%d removed=%d, want 2, 0", la, lr)
	}
	wantCharsAdded := len(firstLine) - 1 + len(secondLine) - 1 // exclude '+' prefixes
	if ca != wantCharsAdded || cd != 0 {
		t.Errorf("chars: got added=%d deleted=%d, want %d, 0", ca, cd, wantCharsAdded)
	}
	wantCR := len(hunks[0].RawContent) + len(hunks[1].RawContent)
	if cr != wantCR {
		t.Errorf("chars_reviewed: got %d, want %d", cr, wantCR)
	}
}
