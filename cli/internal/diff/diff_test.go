package diff

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// testRepo holds a temp repo directory and the SHA of each commit so tests
// reference explicit SHAs instead of brittle relative refs like HEAD~1.
type testRepo struct {
	dir     string
	initSHA string // .gitignore commit
	c1SHA   string // f1.txt commit
	c2SHA   string // f2.txt commit
}

func gitHEAD(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func initRepoDiff(t *testing.T) testRepo {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "git", "init")
	runGit(t, dir, "git", "config", "user.email", "test@stet.local")
	runGit(t, dir, "git", "config", "user.name", "Test")
	writeFile(t, dir, ".gitignore", ".review\n")
	runGit(t, dir, "git", "add", ".gitignore")
	runGit(t, dir, "git", "commit", "-m", "init")
	initSHA := gitHEAD(t, dir)
	writeFile(t, dir, "f1.txt", "a\n")
	runGit(t, dir, "git", "add", "f1.txt")
	runGit(t, dir, "git", "commit", "-m", "add f1")
	c1SHA := gitHEAD(t, dir)
	writeFile(t, dir, "f2.txt", "b\n")
	runGit(t, dir, "git", "add", "f2.txt")
	runGit(t, dir, "git", "commit", "-m", "add f2")
	c2SHA := gitHEAD(t, dir)
	return testRepo{dir: dir, initSHA: initSHA, c1SHA: c1SHA, c2SHA: c2SHA}
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
	fix := initRepoDiff(t)
	// c1 has f1.txt, c2 adds f2.txt. Diff c1..c2 is f2.txt only.
	hunks, err := Hunks(ctx, fix.dir, fix.c1SHA, fix.c2SHA, nil)
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
	fix := initRepoDiff(t)
	hunks, err := Hunks(ctx, fix.dir, fix.c2SHA, fix.c2SHA, nil)
	if err != nil {
		t.Fatalf("Hunks: %v", err)
	}
	if len(hunks) != 0 {
		t.Errorf("Hunks(same ref) = %d hunks, want 0", len(hunks))
	}
	if hunks != nil {
		t.Errorf("Hunks(same ref) = %v, want nil slice", hunks)
	}
}

func TestHunks_excludeGenerated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fix := initRepoDiff(t)
	beforeSHA := fix.c2SHA
	writeFile(t, fix.dir, "bar.go", "package p\n\nfunc Bar() {}\n")
	writeFile(t, fix.dir, "gen.pb.go", "package p\n// generated\n")
	runGit(t, fix.dir, "git", "add", "bar.go", "gen.pb.go")
	runGit(t, fix.dir, "git", "commit", "-m", "add bar and gen")
	afterSHA := gitHEAD(t, fix.dir)
	// Diff beforeSHA..afterSHA: bar.go and gen.pb.go. Default exclusions drop gen.pb.go.
	hunks, err := Hunks(ctx, fix.dir, beforeSHA, afterSHA, nil)
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
	fix := initRepoDiff(t)
	// Diff c1..c2 is f2.txt. Override to exclude *.txt.
	opts := &Options{ExcludePatterns: []string{"*.txt"}}
	hunks, err := Hunks(ctx, fix.dir, fix.c1SHA, fix.c2SHA, opts)
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
	fix := initRepoDiff(t)
	// Empty ExcludePatterns means no exclusions (override defaults with empty list = no filter).
	opts := &Options{ExcludePatterns: []string{}}
	hunks, err := Hunks(ctx, fix.dir, fix.c1SHA, fix.c2SHA, opts)
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
