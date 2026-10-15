package git

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEnsureWorktreeRootIgnored_createsIgnoreFile(t *testing.T) {
	repo := initRepo(t)
	root := filepath.Join(repo, ".review", "worktrees")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ensureWorktreeRootIgnored(repo, root); err != nil {
		t.Fatalf("ensureWorktreeRootIgnored: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("read ignore file: %v", err)
	}
	if !strings.Contains(string(data), "*") || !strings.Contains(string(data), "stet") {
		t.Errorf("ignore file should contain a stet comment and '*', got %q", string(data))
	}

	// A second call must treat the existing file as success (os.ErrExist path)
	// and leave its content untouched.
	if err := ensureWorktreeRootIgnored(repo, root); err != nil {
		t.Fatalf("second ensureWorktreeRootIgnored: %v", err)
	}
	data2, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("re-read ignore file: %v", err)
	}
	if !bytes.Equal(data, data2) {
		t.Errorf("ignore file changed on second call: %q -> %q", string(data), string(data2))
	}
}

func TestEnsureWorktreeRootIgnored_existingFileUntouched(t *testing.T) {
	repo := initRepo(t)
	root := filepath.Join(repo, ".review", "worktrees")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	existing := "# my own rules\nbuild/\n"
	ignorePath := filepath.Join(root, ".gitignore")
	if err := os.WriteFile(ignorePath, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ensureWorktreeRootIgnored(repo, root); err != nil {
		t.Fatalf("ensureWorktreeRootIgnored: %v", err)
	}
	data, err := os.ReadFile(ignorePath)
	if err != nil {
		t.Fatalf("read ignore file: %v", err)
	}
	if string(data) != existing {
		t.Errorf("existing .gitignore modified:\nwant %q\ngot  %q", existing, string(data))
	}
}

func TestEnsureWorktreeRootIgnored_outsideRepoSkipped(t *testing.T) {
	repo := initRepo(t)
	outside := t.TempDir() // outside the repo tree
	if err := ensureWorktreeRootIgnored(repo, outside); err != nil {
		t.Fatalf("ensureWorktreeRootIgnored outside repo: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, ".gitignore")); !os.IsNotExist(err) {
		t.Errorf(".gitignore should not be written outside the repo: %v", err)
	}
}

func TestEnsureWorktreeRootIgnored_emptyArgsNoop(t *testing.T) {
	repo := initRepo(t)
	if err := ensureWorktreeRootIgnored("", repo); err != nil {
		t.Errorf("empty repoRoot should be a no-op, got %v", err)
	}
	if err := ensureWorktreeRootIgnored(repo, ""); err != nil {
		t.Errorf("empty root should be a no-op, got %v", err)
	}
}

// TestEnsureWorktreeRootIgnored_repoRootItselfSkipped guards against dropping a
// `*` .gitignore on the repository root when worktree_root is misconfigured to
// the repo root itself: only proper subdirectories get an ignore file.
func TestEnsureWorktreeRootIgnored_repoRootItselfSkipped(t *testing.T) {
	repo := initRepo(t)
	if err := ensureWorktreeRootIgnored(repo, repo); err != nil {
		t.Fatalf("ensureWorktreeRootIgnored(repo, repo): %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".gitignore")); !os.IsNotExist(err) {
		t.Errorf(".gitignore must not be written to the repo root itself: %v", err)
	}
}

func TestEnsureWorktreeRootIgnored_readOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("read-only directory semantics differ on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root; directory write permissions are not enforced")
	}
	repo := initRepo(t)
	root := filepath.Join(repo, ".review", "worktrees")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0755) })

	err := ensureWorktreeRootIgnored(repo, root)
	if err == nil {
		t.Fatal("expected error writing into read-only directory")
	}
	if !strings.Contains(err.Error(), "could not write") {
		t.Errorf("error should mention the write failure, got %v", err)
	}
}

// TestCreate_WarnsWhenIgnoreWriteFails verifies that a failure to write the
// worktree-root ignore file only produces a warning on stderr and does not
// abort before worktree creation is attempted (the read-only root also makes
// git worktree add fail, so Create returns an error from that step, not from
// the ignore write).
func TestCreate_WarnsWhenIgnoreWriteFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("read-only directory semantics differ on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root; directory write permissions are not enforced")
	}
	repo := initRepo(t)
	wtRoot := filepath.Join(repo, ".review", "worktrees")
	if err := os.MkdirAll(wtRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(wtRoot, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(wtRoot, 0755) })

	// Capture stderr while Create runs (no t.Parallel: os.Stderr is global).
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	_, createErr := Create(repo, wtRoot, "HEAD")
	os.Stderr = origStderr
	if closeErr := w.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	var stderr bytes.Buffer
	if _, err := io.Copy(&stderr, r); err != nil {
		t.Fatal(err)
	}
	if closeErr := r.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}

	if createErr == nil {
		t.Fatal("expected Create to fail: read-only root blocks git worktree add")
	}
	if strings.Contains(createErr.Error(), ".gitignore") {
		t.Errorf("Create error should come from worktree add, not the ignore write: %v", createErr)
	}
	if !strings.Contains(stderr.String(), "warning: could not write") {
		t.Errorf("stderr should contain ignore write warning, got %q", stderr.String())
	}
}

// TestCreate_WorktreeRootNotStaged is the issue regression test: in a repo with
// no ignore rules, Create with the default root followed by git add -A must not
// stage the nested checkout (gitlink, mode 160000) or anything under it.
func TestCreate_WorktreeRootNotStaged(t *testing.T) {
	repo := initRepo(t)
	wtRoot := filepath.Join(repo, ".review", "worktrees")
	path, err := Create(repo, wtRoot, "HEAD")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// The nested worktree must actually exist for the regression to be real.
	if _, err := os.Stat(filepath.Join(path, "f1.txt")); err != nil {
		t.Fatalf("worktree not checked out at %s: %v", path, err)
	}

	run(t, repo, "git", "add", "-A")
	listed := runOut(t, repo, "git", "ls-files", "-s")
	for _, line := range strings.Split(listed, "\n") {
		if strings.Contains(line, "160000") {
			t.Errorf("gitlink staged despite worktree ignore file: %q", line)
		}
		if strings.Contains(line, ".review") {
			t.Errorf("review path staged despite worktree ignore file: %q", line)
		}
	}
}
