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
