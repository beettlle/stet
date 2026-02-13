// Package git provides worktree lifecycle operations: create, list, and remove
// read-only worktrees at a given ref. Uses exec git for compatibility.
package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrWorktreeExists indicates a worktree already exists at the target path.
var ErrWorktreeExists = errors.New("worktree already exists; finish or cleanup current review first")

// ErrBaselineNotAncestor indicates the baseline ref is not an ancestor of HEAD.
var ErrBaselineNotAncestor = errors.New("baseline ref is not an ancestor of HEAD")

// WorktreeInfo holds parsed output from git worktree list.
type WorktreeInfo struct {
	Path   string // absolute path to worktree
	HEAD   string // SHA or ref at HEAD
	Branch string // branch name if not detached; empty if detached
}

// PathForRef resolves ref to a short SHA and derives the worktree path.
// If worktreeRoot is empty, uses repoRoot/.review/worktrees/stet-<short-sha>.
// Otherwise uses worktreeRoot/stet-<short-sha>. Returns absolute path.
func PathForRef(repoRoot, worktreeRoot, ref string) (string, error) {
	sha, err := revParseShort(repoRoot, ref, 12)
	if err != nil {
		return "", fmt.Errorf("resolve ref %q: %w", ref, err)
	}
	var base string
	if worktreeRoot != "" {
		base = worktreeRoot
	} else {
		base = filepath.Join(repoRoot, ".review", "worktrees")
	}
	p := filepath.Join(base, "stet-"+sha)
	return filepath.Abs(p)
}

// revParseShort runs git rev-parse --short=N and returns the SHA.
func revParseShort(repoRoot, ref string, n int) (string, error) {
	cmd := exec.Command("git", "rev-parse", fmt.Sprintf("--short=%d", n), ref)
	cmd.Dir = repoRoot
	cmd.Env = minimalEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// IsAncestor returns true if ancestor is an ancestor of descendant in the repo.
func IsAncestor(repoRoot, ancestor, descendant string) (bool, error) {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", ancestor, descendant)
	cmd.Dir = repoRoot
	cmd.Env = minimalEnv()
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git merge-base --is-ancestor: %w", err)
}

// Create creates a read-only worktree at the given ref. Path is derived via PathForRef.
// Returns ErrWorktreeExists if a worktree already exists at the target path.
// Returns ErrBaselineNotAncestor if ref is not an ancestor of HEAD.
func Create(repoRoot, worktreeRoot, ref string) (path string, err error) {
	if ok, err := IsAncestor(repoRoot, ref, "HEAD"); err != nil {
		return "", fmt.Errorf("validate baseline: %w", err)
	} else if !ok {
		return "", ErrBaselineNotAncestor
	}

	path, err = PathForRef(repoRoot, worktreeRoot, ref)
	if err != nil {
		return "", err
	}

	// Check if path or worktree already exists before adding.
	if _, err := os.Stat(path); err == nil {
		return path, fmt.Errorf("worktree already exists at %s: %w", path, ErrWorktreeExists)
	}

	// Ensure parent dir exists for worktree add.
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return "", fmt.Errorf("create worktree parent dir: %w", err)
	}

	cmd := exec.Command("git", "worktree", "add", path, ref)
	cmd.Dir = repoRoot
	cmd.Env = minimalEnv()
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		msg := strings.TrimSpace(string(out))
		if isWorktreeExistsError(msg, path) {
			return path, fmt.Errorf("worktree already exists at %s: %w", path, ErrWorktreeExists)
		}
		return "", fmt.Errorf("git worktree add: %w: %s", runErr, msg)
	}

	return path, nil
}

// List returns all worktrees for the repo.
func List(repoRoot string) ([]WorktreeInfo, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoRoot
	cmd.Env = minimalEnv()
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	return parseWorktreeList(string(out))
}

func parseWorktreeList(s string) ([]WorktreeInfo, error) {
	var list []WorktreeInfo
	var cur WorktreeInfo
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			if cur.Path != "" {
				list = append(list, cur)
			}
			cur = WorktreeInfo{Path: strings.TrimPrefix(line, "worktree ")}
			continue
		}
		if strings.HasPrefix(line, "HEAD ") {
			cur.HEAD = strings.TrimPrefix(line, "HEAD ")
		} else if strings.HasPrefix(line, "branch ") {
			cur.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}
	if cur.Path != "" {
		list = append(list, cur)
	}
	return list, nil
}

// Remove removes the worktree at path.
func Remove(repoRoot, path string) error {
	cmd := exec.Command("git", "worktree", "remove", path)
	cmd.Dir = repoRoot
	cmd.Env = minimalEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func isWorktreeExistsError(msg, path string) bool {
	lc := strings.ToLower(msg)
	return strings.Contains(lc, "already checked out") ||
		strings.Contains(lc, "already exists") ||
		(strings.Contains(msg, path) && strings.Contains(lc, "fatal:"))
}

func minimalEnv() []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_PAGER=cat", // prevent pager; subprocess output is captured
	}
	if home := os.Getenv("HOME"); home != "" {
		env = append(env, "HOME="+home)
	} else if runtime.GOOS == "windows" {
		if profile := os.Getenv("USERPROFILE"); profile != "" {
			env = append(env, "HOME="+profile)
		}
	}
	return env
}

// MinimalEnv returns the environment used for git subprocesses. Exported for tests
// so callers can assert HOME is included when set (e.g. to avoid "Author identity unknown").
func MinimalEnv() []string {
	return minimalEnv()
}
