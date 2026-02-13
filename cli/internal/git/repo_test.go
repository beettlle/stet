package git

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestRepoRoot_fromRoot(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	got, err := RepoRoot(repo)
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		t.Fatal(err)
	}
	if got != absRepo {
		t.Errorf("RepoRoot(%q) = %q, want %q", repo, got, absRepo)
	}
}

func TestRepoRoot_fromSubdir(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	subdir := filepath.Join(repo, "sub", "dir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	got, err := RepoRoot(subdir)
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		t.Fatal(err)
	}
	if got != absRepo {
		t.Errorf("RepoRoot(subdir) = %q, want %q", got, absRepo)
	}
}

func TestRepoRoot_notARepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := RepoRoot(dir)
	if err == nil {
		t.Fatal("RepoRoot(non-repo): expected error")
	}
}

func TestIsClean_clean(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	ok, err := IsClean(repo)
	if err != nil {
		t.Fatalf("IsClean: %v", err)
	}
	if !ok {
		t.Error("IsClean after initRepo: want true, got false")
	}
}

func TestIsClean_dirtyUncommitted(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	writeFile(t, repo, "dirty.txt", "x\n")
	ok, err := IsClean(repo)
	if err != nil {
		t.Fatalf("IsClean: %v", err)
	}
	if ok {
		t.Error("IsClean with uncommitted file: want false, got true")
	}
}

func TestIsClean_staged(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	writeFile(t, repo, "staged.txt", "y\n")
	run(t, repo, "git", "add", "staged.txt")
	ok, err := IsClean(repo)
	if err != nil {
		t.Fatalf("IsClean: %v", err)
	}
	if ok {
		t.Error("IsClean with staged file: want false, got true")
	}
}

func TestRevParse_head(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	sha, err := RevParse(repo, "HEAD")
	if err != nil {
		t.Fatalf("RevParse HEAD: %v", err)
	}
	shaRegex := regexp.MustCompile("^[0-9a-f]{40}$")
	if !shaRegex.MatchString(sha) {
		t.Errorf("RevParse HEAD = %q, want 40-char hex SHA", sha)
	}
}

func TestRevParse_headTilde1(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	sha, err := RevParse(repo, "HEAD~1")
	if err != nil {
		t.Fatalf("RevParse HEAD~1: %v", err)
	}
	shaRegex := regexp.MustCompile("^[0-9a-f]{40}$")
	if !shaRegex.MatchString(sha) {
		t.Errorf("RevParse HEAD~1 = %q, want 40-char hex SHA", sha)
	}
}

func TestRevParse_invalidRef(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	_, err := RevParse(repo, "not-a-ref-at-all")
	if err == nil {
		t.Fatal("RevParse(invalid): expected error")
	}
}

func TestUserIntent_returnsBranchAndCommit(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	// initRepo creates commits "gitignore", "c1", "c2"; HEAD is at "c2"
	branch, commitMsg, err := UserIntent(repo)
	if err != nil {
		t.Fatalf("UserIntent: %v", err)
	}
	// Branch is typically "main" or "master" depending on git config
	if branch != "main" && branch != "master" {
		t.Errorf("UserIntent branch = %q, want main or master", branch)
	}
	if commitMsg != "c2" {
		t.Errorf("UserIntent commitMsg = %q, want c2", commitMsg)
	}
}

func TestUserIntent_customCommitMessage(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	writeFile(t, repo, "f3.txt", "c\n")
	run(t, repo, "git", "add", "f3.txt")
	run(t, repo, "git", "commit", "-m", "Refactor: formatting only")
	branch, commitMsg, err := UserIntent(repo)
	if err != nil {
		t.Fatalf("UserIntent: %v", err)
	}
	if branch == "" {
		t.Error("UserIntent branch: want non-empty")
	}
	if commitMsg != "Refactor: formatting only" {
		t.Errorf("UserIntent commitMsg = %q, want \"Refactor: formatting only\"", commitMsg)
	}
}

func TestUserIntent_detachedHEAD(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	sha := runOut(t, repo, "git", "rev-parse", "HEAD~1")
	run(t, repo, "git", "checkout", sha)
	branch, commitMsg, err := UserIntent(repo)
	if err != nil {
		t.Fatalf("UserIntent: %v", err)
	}
	if branch != "HEAD" {
		t.Errorf("UserIntent branch (detached) = %q, want HEAD", branch)
	}
	if commitMsg != "c1" {
		t.Errorf("UserIntent commitMsg = %q, want c1 (HEAD~1)", commitMsg)
	}
}
