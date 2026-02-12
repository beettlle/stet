package run

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/git"
	"stet/cli/internal/session"
)

func initRepo(t *testing.T) string {
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

func TestStart_createsSessionAndWorktree(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
	}
	err := Start(ctx, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if s.BaselineRef == "" {
		t.Error("session.BaselineRef: want non-empty after Start")
	}
	list, err := git.List(repo)
	if err != nil {
		t.Fatalf("List worktrees: %v", err)
	}
	wantPath, err := git.PathForRef(repo, "", s.BaselineRef)
	if err != nil {
		t.Fatalf("PathForRef: %v", err)
	}
	found := false
	for _, w := range list {
		if w.Path == wantPath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("worktree path %q not in list: %+v", wantPath, list)
	}
}

func TestStart_requiresCleanWorktree(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	writeFile(t, repo, "dirty.txt", "x\n")
	opts := StartOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: "HEAD"}
	err := Start(ctx, opts)
	if err == nil {
		t.Fatal("Start with dirty worktree: expected error")
	}
	if !errors.Is(err, ErrDirtyWorktree) {
		t.Errorf("Start: got %v, want ErrDirtyWorktree", err)
	}
}

func TestStart_baselineNotAncestor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	mainBranch := runOut(t, repo, "git", "rev-parse", "--abbrev-ref", "HEAD")
	runGit(t, repo, "git", "checkout", "--orphan", "orphan")
	writeFile(t, repo, "orphan.txt", "x\n")
	runGit(t, repo, "git", "add", "orphan.txt")
	runGit(t, repo, "git", "commit", "-m", "orphan")
	orphanSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	runGit(t, repo, "git", "checkout", mainBranch)
	opts := StartOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: orphanSHA}
	err := Start(ctx, opts)
	if err == nil {
		t.Fatal("Start with non-ancestor baseline: expected error")
	}
	if !errors.Is(err, git.ErrBaselineNotAncestor) {
		t.Errorf("Start: got %v, want ErrBaselineNotAncestor", err)
	}
}

func TestFinish_removesWorktreeAndPersistsState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: "HEAD~1"}
	if err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	sBefore, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	finishOpts := FinishOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: ""}
	if err := Finish(ctx, finishOpts); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	list, err := git.List(repo)
	if err != nil {
		t.Fatalf("List after Finish: %v", err)
	}
	wantPath, _ := git.PathForRef(repo, "", sBefore.BaselineRef)
	for _, w := range list {
		if w.Path == wantPath {
			t.Error("worktree should be gone after Finish")
			break
		}
	}
	sAfter, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session after Finish: %v", err)
	}
	if sAfter.BaselineRef != sBefore.BaselineRef {
		t.Errorf("state persisted: BaselineRef = %q, want %q", sAfter.BaselineRef, sBefore.BaselineRef)
	}
}

func TestFinish_noSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := FinishOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: ""}
	err := Finish(ctx, opts)
	if err == nil {
		t.Fatal("Finish without start: expected error")
	}
	if !errors.Is(err, ErrNoSession) {
		t.Errorf("Finish: got %v, want ErrNoSession", err)
	}
}

func TestStart_worktreeAlreadyExists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: "HEAD~1"}
	if err := Start(ctx, opts); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	err := Start(ctx, opts)
	if err == nil {
		t.Fatal("second Start: expected error")
	}
	if !errors.Is(err, git.ErrWorktreeExists) {
		t.Errorf("second Start: got %v, want ErrWorktreeExists", err)
	}
}

func TestFinish_worktreeAlreadyGone(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: "HEAD~1"}
	if err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	path, err := git.PathForRef(repo, "", s.BaselineRef)
	if err != nil {
		t.Fatalf("PathForRef: %v", err)
	}
	if err := git.Remove(repo, path); err != nil {
		t.Fatalf("Remove worktree manually: %v", err)
	}
	err = Finish(ctx, FinishOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: ""})
	if err != nil {
		t.Fatalf("Finish when worktree already gone: %v", err)
	}
}
