// Package git (repo.go) provides repository discovery and status helpers.
package git

import (
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
