// Package git (repo.go) provides repository discovery and status helpers.
package git

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"

	"stet/cli/internal/erruser"
)

// RepoRoot returns the absolute path of the git repository root containing dir.
// Runs "git rev-parse --show-toplevel" with Dir=dir. Returns error if dir is
// not inside a git repository.
func RepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	cmd.Env = minimalEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", erruser.New("This directory is not inside a Git repository.", err)
	}
	root := strings.TrimSpace(string(out))
	return filepath.Abs(root)
}

// IsClean reports whether the working tree at repoRoot has no uncommitted
// changes. Runs "git status --porcelain"; true only if output is empty.
// Returns error only on command failure.
func IsClean(repoRoot string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	cmd.Env = minimalEnv()
	out, err := cmd.Output()
	if err != nil {
		return false, erruser.New("Could not check working tree status.", err)
	}
	return len(strings.TrimSpace(string(out))) == 0, nil
}

// RefExists reports whether the given ref exists in the repository at repoRoot.
// Runs "git rev-parse ref"; returns true if the ref exists (exit 0), false if it
// does not exist (exit 128 in a valid repo), or an error for other failures
// (e.g. repoRoot is not a git repository).
func RefExists(repoRoot, ref string) (bool, error) {
	if repoRoot == "" || ref == "" {
		return false, erruser.New("RefExists: repo root and ref required", nil)
	}
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = repoRoot
	cmd.Env = minimalEnv()
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
		if strings.Contains(string(out), "not a git repository") {
			return false, erruser.New("This directory is not inside a Git repository.", err)
		}
		return false, nil
	}
	return false, erruser.New("Could not check if ref exists.", err)
}

// RevParse resolves ref to a full SHA in the repository at repoRoot.
// Returns the 40-character commit SHA, or error if ref is invalid.
func RevParse(repoRoot, ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = repoRoot
	cmd.Env = minimalEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", erruser.New("Invalid ref or commit.", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// UserIntent returns the current branch name and the last commit message at HEAD.
// Branch is from "git rev-parse --abbrev-ref HEAD" (returns "HEAD" when detached).
// CommitMsg is from "git log -1 --format=%B HEAD". Both are trimmed.
func UserIntent(repoRoot string) (branch, commitMsg string, err error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoRoot
	cmd.Env = minimalEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", "", erruser.New("Could not read branch or commit message.", err)
	}
	branch = strings.TrimSpace(string(out))

	cmd = exec.Command("git", "log", "-1", "--format=%B", "HEAD")
	cmd.Dir = repoRoot
	cmd.Env = minimalEnv()
	out, err = cmd.Output()
	if err != nil {
		return "", "", erruser.New("Could not read branch or commit message.", err)
	}
	commitMsg = strings.TrimSpace(string(out))

	return branch, commitMsg, nil
}

// UncommittedDiff returns the unified diff of uncommitted changes at repoRoot.
// If stagedOnly is true, returns only staged changes (git diff --cached).
// Otherwise returns staged plus unstaged (git diff HEAD). Uses --no-color.
// Returns empty string and nil error when there are no changes.
func UncommittedDiff(ctx context.Context, repoRoot string, stagedOnly bool) (string, error) {
	args := []string{"diff", "--no-color"}
	if stagedOnly {
		args = append(args, "--cached")
	} else {
		args = append(args, "HEAD")
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoRoot
	cmd.Env = minimalEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", erruser.New("Could not get uncommitted diff.", err)
	}
	return strings.TrimSpace(string(out)), nil
}
