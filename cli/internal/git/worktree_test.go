package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@stet.local")
	run(t, dir, "git", "config", "user.name", "Test")
	writeFile(t, dir, "f1.txt", "a\n")
	run(t, dir, "git", "add", "f1.txt")
	run(t, dir, "git", "commit", "-m", "c1")
	writeFile(t, dir, "f2.txt", "b\n")
	run(t, dir, "git", "add", "f2.txt")
	run(t, dir, "git", "commit", "-m", "c2")
	return dir
}

func run(t *testing.T, dir, name string, args ...string) {
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

func runOut(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s %v: %v", name, args, err)
	}
	return strings.TrimSpace(string(out))
}

func TestPathForRef(t *testing.T) {
	repo := initRepo(t)
	baseSha := runOut(t, repo, "git", "rev-parse", "--short=12", "HEAD~1")
	shaRegex := regexp.MustCompile("^[0-9a-f]{12}$")
	if !shaRegex.MatchString(baseSha) {
		t.Fatalf("expected 12-char SHA, got %q", baseSha)
	}
	path, err := PathForRef(repo, "", "HEAD~1")
	if err != nil {
		t.Fatalf("PathForRef: %v", err)
	}
	wantBase := filepath.Join(repo, ".review", "worktrees")
	if !strings.HasPrefix(path, wantBase) {
		t.Errorf("path %q should be under %q", path, wantBase)
	}
	if filepath.Base(path) != "stet-"+baseSha {
		t.Errorf("path base %q, want stet-%s", filepath.Base(path), baseSha)
	}
}

func TestPathForRef_customWorktreeRoot(t *testing.T) {
	repo := initRepo(t)
	wtRoot := filepath.Join(repo, "wtroot")
	path, err := PathForRef(repo, wtRoot, "HEAD")
	if err != nil {
		t.Fatalf("PathForRef: %v", err)
	}
	if !strings.HasPrefix(path, wtRoot) {
		t.Errorf("path %q should be under %q", path, wtRoot)
	}
}

func TestIsAncestor(t *testing.T) {
	repo := initRepo(t)
	ancestor, err := IsAncestor(repo, "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("IsAncestor: %v", err)
	}
	if !ancestor {
		t.Error("HEAD~1 should be ancestor of HEAD")
	}
	notAncestor, err := IsAncestor(repo, "HEAD", "HEAD~1")
	if err != nil {
		t.Fatalf("IsAncestor: %v", err)
	}
	if notAncestor {
		t.Error("HEAD should not be ancestor of HEAD~1")
	}
}

func TestCreate(t *testing.T) {
	repo := initRepo(t)
	path, err := Create(repo, "", "HEAD~1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if path == "" || !filepath.IsAbs(path) {
		t.Errorf("Create returned invalid path %q", path)
	}
	base := filepath.Base(path)
	if len(base) < 6 || base[:5] != "stet-" {
		t.Errorf("path base %q should match stet-<sha>", base)
	}
	if _, err := os.Stat(filepath.Join(path, "f1.txt")); err != nil {
		t.Errorf("f1.txt should exist in worktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "f2.txt")); err == nil {
		t.Error("f2.txt should not exist in baseline worktree")
	}
}

func TestCreate_WorktreeAlreadyExists(t *testing.T) {
	repo := initRepo(t)
	_, err := Create(repo, "", "HEAD~1")
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err = Create(repo, "", "HEAD~1")
	if err == nil {
		t.Fatal("second Create should fail")
	}
	if !errors.Is(err, ErrWorktreeExists) {
		t.Errorf("expected ErrWorktreeExists, got %v", err)
	}
}

func TestCreate_BaselineNotAncestor(t *testing.T) {
	repo := initRepo(t)
	mainBranch := runOut(t, repo, "git", "rev-parse", "--abbrev-ref", "HEAD")
	run(t, repo, "git", "checkout", "--orphan", "orphan")
	writeFile(t, repo, "orphan.txt", "x\n")
	run(t, repo, "git", "add", "orphan.txt")
	run(t, repo, "git", "commit", "-m", "orphan")
	orphanSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	run(t, repo, "git", "checkout", mainBranch)
	_, err := Create(repo, "", orphanSHA)
	if err == nil {
		t.Fatal("Create with non-ancestor should fail")
	}
	if !errors.Is(err, ErrBaselineNotAncestor) {
		t.Errorf("expected ErrBaselineNotAncestor, got %v", err)
	}
}

func TestList(t *testing.T) {
	repo := initRepo(t)
	path, err := Create(repo, "", "HEAD~1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	list, err := List(repo)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) < 2 {
		t.Fatalf("expected at least 2 worktrees, got %d", len(list))
	}
	found := false
	for _, w := range list {
		if w.Path == path {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("stet worktree path %q not in list", path)
	}
}

func TestRemove(t *testing.T) {
	repo := initRepo(t)
	path, err := Create(repo, "", "HEAD~1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := Remove(repo, path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	list, err := List(repo)
	if err != nil {
		t.Fatalf("List after Remove: %v", err)
	}
	for _, w := range list {
		if w.Path == path {
			t.Error("worktree should be gone after Remove")
			break
		}
	}
}

func TestParseWorktreeList(t *testing.T) {
	input := "worktree /a/main\nHEAD abc123\nbranch refs/heads/main\n\nworktree /a/stet-abc123\nHEAD abc123\nbare\n\n"
	list, err := parseWorktreeList(input)
	if err != nil {
		t.Fatalf("parseWorktreeList: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(list))
	}
	if list[0].Path != "/a/main" || list[0].HEAD != "abc123" || list[0].Branch != "main" {
		t.Errorf("first: %+v", list[0])
	}
	if list[1].Path != "/a/stet-abc123" || list[1].HEAD != "abc123" {
		t.Errorf("second: %+v", list[1])
	}
}

func TestMinimalEnv_includesHOMEWhenSet(t *testing.T) {
	env := MinimalEnv()
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("HOME not set; cannot assert MinimalEnv includes it")
	}
	want := "HOME=" + home
	var found bool
	for _, e := range env {
		if e == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("MinimalEnv() should contain %q when HOME is set; got %v", want, env)
	}
}
