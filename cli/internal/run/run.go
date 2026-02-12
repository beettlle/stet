// Package run implements the start and finish session flows: worktree creation,
// session persistence, and cleanup. Used by the CLI and by tests.
package run

import (
	"context"
	"errors"
	"fmt"
	"os"

	"stet/cli/internal/git"
	"stet/cli/internal/session"
)

// ErrNoSession indicates there is no active session (e.g. finish without start).
var ErrNoSession = errors.New("no active session; run stet start first")

// ErrDirtyWorktree indicates the working tree has uncommitted changes.
var ErrDirtyWorktree = errors.New("working tree has uncommitted changes; commit or stash before starting")

// StartOptions configures Start. All fields are required except Ref (default "HEAD" by caller).
type StartOptions struct {
	RepoRoot     string
	StateDir     string
	WorktreeRoot string
	Ref          string
}

// FinishOptions configures Finish.
type FinishOptions struct {
	RepoRoot     string
	StateDir     string
	WorktreeRoot string
}

// Start creates a worktree at the given ref and writes the session. Validates
// clean worktree and that ref is an ancestor of HEAD. Caller should default
// Ref to "HEAD" if unset. Errors are wrapped with %w so callers can use
// errors.Is for session.ErrLocked, git.ErrWorktreeExists, git.ErrBaselineNotAncestor.
func Start(ctx context.Context, opts StartOptions) error {
	if opts.RepoRoot == "" || opts.StateDir == "" {
		return fmt.Errorf("run Start: RepoRoot and StateDir required")
	}
	ref := opts.Ref
	if ref == "" {
		ref = "HEAD"
	}

	clean, err := git.IsClean(opts.RepoRoot)
	if err != nil {
		return fmt.Errorf("start: check worktree: %w", err)
	}
	if !clean {
		return ErrDirtyWorktree
	}

	release, err := session.AcquireLock(opts.StateDir)
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}
	defer release()

	sha, err := git.RevParse(opts.RepoRoot, ref)
	if err != nil {
		return fmt.Errorf("start: resolve ref %q: %w", ref, err)
	}

	_, err = git.Create(opts.RepoRoot, opts.WorktreeRoot, ref)
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}

	s := session.Session{
		BaselineRef:    sha,
		LastReviewedAt: "",
		DismissedIDs:   nil,
	}
	if err := session.Save(opts.StateDir, &s); err != nil {
		return fmt.Errorf("start: save session: %w", err)
	}
	return nil
}

// Finish removes the session's worktree and releases the lock. Session file
// is left on disk (state persisted). If the worktree path no longer exists,
// Finish succeeds without error.
func Finish(ctx context.Context, opts FinishOptions) error {
	if opts.RepoRoot == "" || opts.StateDir == "" {
		return fmt.Errorf("run Finish: RepoRoot and StateDir required")
	}

	s, err := session.Load(opts.StateDir)
	if err != nil {
		return fmt.Errorf("finish: load session: %w", err)
	}
	if s.BaselineRef == "" {
		return ErrNoSession
	}

	release, err := session.AcquireLock(opts.StateDir)
	if err != nil {
		return fmt.Errorf("finish: %w", err)
	}
	defer release()

	path, err := git.PathForRef(opts.RepoRoot, opts.WorktreeRoot, s.BaselineRef)
	if err != nil {
		return fmt.Errorf("finish: worktree path: %w", err)
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("finish: stat worktree: %w", err)
	}

	if err := git.Remove(opts.RepoRoot, path); err != nil {
		return fmt.Errorf("finish: remove worktree: %w", err)
	}
	return nil
}
