package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// worktreeIgnoreContent is written to a worktree root inside the repository so
// nested review checkouts never show up in git status and are never staged by
// "git add -A" (a nested checkout would otherwise be staged as a gitlink).
// Ignoring "*" also hides the .gitignore file itself.
const worktreeIgnoreContent = "# Created by stet so review worktrees are never staged.\n*\n"

// ensureWorktreeRootIgnored ensures <root>/.gitignore exists when root is inside
// repoRoot. The file ignores everything in the worktree root, so review
// worktrees never appear in git status or get staged. An existing .gitignore is
// never overwritten, and roots outside repoRoot (e.g. a worktree_root config
// pointing elsewhere) are left untouched.
//
// The returned error is meant to be surfaced as a warning by callers: worktree
// creation must still succeed when the file cannot be written.
func ensureWorktreeRootIgnored(repoRoot, root string) error {
	if repoRoot == "" || root == "" || !pathInsideRepo(repoRoot, root) {
		return nil
	}
	ignorePath := filepath.Join(root, ".gitignore")
	f, err := os.OpenFile(ignorePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return fmt.Errorf("could not write %s: %w", ignorePath, err)
	}
	if _, err := f.WriteString(worktreeIgnoreContent); err != nil {
		f.Close()
		return fmt.Errorf("could not write %s: %w", ignorePath, err)
	}
	return f.Close()
}

// pathInsideRepo reports whether path is inside repoRoot. Both sides become
// cleaned absolute paths with symlinks resolved the same way, so repos in
// symlinked locations (e.g. macOS /var -> /private/var temp dirs) compare equal.
func pathInsideRepo(repoRoot, path string) bool {
	absRepo, err := filepath.Abs(repoRoot)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(resolveIfExists(absRepo), resolveIfExists(absPath))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// resolveIfExists resolves symlinks in an existing path. If the path (or any of
// its components) cannot be resolved, the cleaned absolute path is returned.
func resolveIfExists(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}
