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

func TestHunks_emptyRepoRoot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, err := Hunks(ctx, "", "HEAD~1", "HEAD", nil)
	if err == nil {
		t.Fatal("Hunks with empty repoRoot: expected error")
	}
	if !strings.Contains(err.Error(), "repoRoot required") {
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
