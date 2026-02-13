package scope

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/diff"
)

func initRepoScope(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "git", "init")
	runGit(t, dir, "git", "config", "user.email", "test@stet.local")
	runGit(t, dir, "git", "config", "user.name", "Test")
	writeFileScope(t, dir, ".gitignore", ".review\n")
	runGit(t, dir, "git", "add", ".gitignore")
	runGit(t, dir, "git", "commit", "-m", "gitignore")
	writeFileScope(t, dir, "f1.txt", "a\n")
	runGit(t, dir, "git", "add", "f1.txt")
	runGit(t, dir, "git", "commit", "-m", "c1")
	writeFileScope(t, dir, "f2.txt", "b\n")
	runGit(t, dir, "git", "add", "f2.txt")
	runGit(t, dir, "git", "commit", "-m", "c2")
	return dir
}

// initRepoGoComment creates a repo with three commits: base, then add p.go, then
// comment-only change. So diff(HEAD~2, HEAD~1) and diff(HEAD~2, HEAD) each have one
// hunk; semantic ID matches across the comment-only change (for TestPartition_semanticMatch).
func initRepoGoComment(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "git", "init")
	runGit(t, dir, "git", "config", "user.email", "test@stet.local")
	runGit(t, dir, "git", "config", "user.name", "Test")
	writeFileScope(t, dir, ".gitignore", "")
	runGit(t, dir, "git", "add", ".gitignore")
	runGit(t, dir, "git", "commit", "-m", "base")
	writeFileScope(t, dir, "p.go", "package p\n\nfunc F() {}\n")
	runGit(t, dir, "git", "add", "p.go")
	runGit(t, dir, "git", "commit", "-m", "add p")
	writeFileScope(t, dir, "p.go", "package p\n\nfunc F() {} // comment\n")
	runGit(t, dir, "git", "add", "p.go")
	runGit(t, dir, "git", "commit", "-m", "add comment")
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

func writeFileScope(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestPartition_firstRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepoScope(t)
	// baseline=HEAD~2 (gitignore), head=HEAD (c2): diff has f1 and f2. First run → all to review.
	got, err := Partition(ctx, repo, "HEAD~2", "HEAD", "", nil)
	if err != nil {
		t.Fatalf("Partition: %v", err)
	}
	if len(got.ToReview) != 2 {
		t.Errorf("first run: len(ToReview) = %d, want 2", len(got.ToReview))
	}
	if got.Approved != nil {
		t.Errorf("first run: Approved = %v, want nil", got.Approved)
	}
	paths := make([]string, len(got.ToReview))
	for i := range got.ToReview {
		paths[i] = got.ToReview[i].FilePath
	}
	if paths[0] != "f1.txt" || paths[1] != "f2.txt" {
		t.Errorf("ToReview paths = %v, want [f1.txt f2.txt]", paths)
	}
}

func TestPartition_noNewWork(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepoScope(t)
	// baseline=HEAD~2, head=HEAD~1, lastReviewedAt=HEAD~1 → current = [f1], reviewed = [f1] → all approved.
	got, err := Partition(ctx, repo, "HEAD~2", "HEAD~1", "HEAD~1", nil)
	if err != nil {
		t.Fatalf("Partition: %v", err)
	}
	if len(got.ToReview) != 0 {
		t.Errorf("no new work: len(ToReview) = %d, want 0", len(got.ToReview))
	}
	if len(got.Approved) != 1 {
		t.Errorf("no new work: len(Approved) = %d, want 1", len(got.Approved))
	}
	if len(got.Approved) > 0 && got.Approved[0].FilePath != "f1.txt" {
		t.Errorf("Approved[0].FilePath = %q, want f1.txt", got.Approved[0].FilePath)
	}
}

func TestPartition_incremental(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepoScope(t)
	// baseline=HEAD~2, head=HEAD, lastReviewedAt=HEAD~1 → current = [f1,f2], reviewed = [f1] → f1 approved, f2 to review.
	got, err := Partition(ctx, repo, "HEAD~2", "HEAD", "HEAD~1", nil)
	if err != nil {
		t.Fatalf("Partition: %v", err)
	}
	if len(got.ToReview) != 1 {
		t.Errorf("incremental: len(ToReview) = %d, want 1", len(got.ToReview))
	}
	if len(got.Approved) != 1 {
		t.Errorf("incremental: len(Approved) = %d, want 1", len(got.Approved))
	}
	if len(got.ToReview) > 0 && got.ToReview[0].FilePath != "f2.txt" {
		t.Errorf("ToReview[0].FilePath = %q, want f2.txt", got.ToReview[0].FilePath)
	}
	if len(got.Approved) > 0 && got.Approved[0].FilePath != "f1.txt" {
		t.Errorf("Approved[0].FilePath = %q, want f1.txt", got.Approved[0].FilePath)
	}
}

func TestPartition_emptyDiff(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepoScope(t)
	got, err := Partition(ctx, repo, "HEAD", "HEAD", "", nil)
	if err != nil {
		t.Fatalf("Partition: %v", err)
	}
	if got.ToReview != nil {
		t.Errorf("empty diff: ToReview = %v, want nil", got.ToReview)
	}
	if got.Approved != nil {
		t.Errorf("empty diff: Approved = %v, want nil", got.Approved)
	}
}

func TestPartition_emptyDiffWithLastReviewed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepoScope(t)
	got, err := Partition(ctx, repo, "HEAD", "HEAD", "HEAD", nil)
	if err != nil {
		t.Fatalf("Partition: %v", err)
	}
	if got.ToReview != nil {
		t.Errorf("empty diff: ToReview = %v, want nil", got.ToReview)
	}
	if got.Approved != nil {
		t.Errorf("empty diff: Approved = %v, want nil", got.Approved)
	}
}

func TestPartition_invalidLastReviewedAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepoScope(t)
	_, err := Partition(ctx, repo, "HEAD~2", "HEAD", "invalid-ref-no-such-commit", nil)
	if err == nil {
		t.Fatal("Partition with invalid lastReviewedAt: expected error")
	}
	if !strings.Contains(err.Error(), "diff") {
		t.Errorf("error = %v, want mention of diff", err)
	}
}

func TestPartition_semanticMatch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepoGoComment(t)
	// baseline=HEAD~2, lastReviewedAt=HEAD~1, head=HEAD. Reviewed = [original p.go hunk],
	// current = [p.go hunk with comment]. Semantic ID same → approved.
	got, err := Partition(ctx, repo, "HEAD~2", "HEAD", "HEAD~1", nil)
	if err != nil {
		t.Fatalf("Partition: %v", err)
	}
	if len(got.ToReview) != 0 {
		t.Errorf("semantic match: len(ToReview) = %d, want 0 (comment-only change should be approved)", len(got.ToReview))
	}
	if len(got.Approved) != 1 {
		t.Errorf("semantic match: len(Approved) = %d, want 1", len(got.Approved))
	}
	if len(got.Approved) > 0 && got.Approved[0].FilePath != "p.go" {
		t.Errorf("Approved[0].FilePath = %q, want p.go", got.Approved[0].FilePath)
	}
}

func TestPartition_diffErrorPropagated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, err := Partition(ctx, "", "HEAD~2", "HEAD", "", nil)
	if err == nil {
		t.Fatal("Partition with empty repoRoot: expected error")
	}
	// scope wraps diff errors with user message "Could not compute diff."
	if !strings.Contains(err.Error(), "diff") {
		t.Errorf("error = %v, want mention of diff", err)
	}
}

func TestPartition_headEqualsLastReviewedAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepoScope(t)
	// head=HEAD, lastReviewedAt=HEAD → current == reviewed → all approved, none to review.
	got, err := Partition(ctx, repo, "HEAD~2", "HEAD", "HEAD", nil)
	if err != nil {
		t.Fatalf("Partition: %v", err)
	}
	if len(got.ToReview) != 0 {
		t.Errorf("head==lastReviewedAt: len(ToReview) = %d, want 0", len(got.ToReview))
	}
	if len(got.Approved) != 2 {
		t.Errorf("head==lastReviewedAt: len(Approved) = %d, want 2", len(got.Approved))
	}
}

func TestPartition_strictMatch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepoScope(t)
	// Unchanged hunk: strict ID matches. baseline..HEAD has f1 and f2; baseline..HEAD~1 has f1.
	// So f1 is in reviewed set with same content at HEAD. Partition(HEAD~2, HEAD, HEAD~1): current
	// has f1 (unchanged) and f2 (new). f1 strict match → approved, f2 → to review. We already
	// covered this in TestPartition_incremental (f1 approved = strict match). Add explicit
	// check that approved hunk content matches reviewed.
	got, err := Partition(ctx, repo, "HEAD~2", "HEAD", "HEAD~1", nil)
	if err != nil {
		t.Fatalf("Partition: %v", err)
	}
	// f1 is approved (strict match: same content at HEAD as at HEAD~1 for that file).
	reviewed, _ := diff.Hunks(ctx, repo, "HEAD~2", "HEAD~1", nil)
	if len(reviewed) != 1 {
		t.Fatalf("setup: expected 1 reviewed hunk, got %d", len(reviewed))
	}
	if len(got.Approved) != 1 {
		t.Fatalf("expected 1 approved, got %d", len(got.Approved))
	}
	if got.Approved[0].RawContent != reviewed[0].RawContent {
		t.Error("approved hunk RawContent should match reviewed (strict match)")
	}
}
