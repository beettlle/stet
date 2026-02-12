// Package git (repo.go) provides repository discovery and status helpers.
package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
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
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
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
		return false, fmt.Errorf("git status --porcelain: %w", err)
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
		return "", fmt.Errorf("git rev-parse %q: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}
