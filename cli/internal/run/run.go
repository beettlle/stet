// Package run implements the start and finish session flows: worktree creation,
// session persistence, review pipeline (diff, scope, Ollama or dry-run), and
// cleanup. Used by the CLI and by tests.
package run

import (
	"context"
	"errors"
	"fmt"
	"os"

	"stet/cli/internal/diff"
	"stet/cli/internal/findings"
	"stet/cli/internal/git"
	"stet/cli/internal/hunkid"
	"stet/cli/internal/ollama"
	"stet/cli/internal/prompt"
	"stet/cli/internal/review"
	"stet/cli/internal/scope"
	"stet/cli/internal/session"
	"stet/cli/internal/tokens"
)

// ErrNoSession indicates there is no active session (e.g. finish without start).
var ErrNoSession = errors.New("no active session; run stet start first")

// ErrDirtyWorktree indicates the working tree has uncommitted changes.
var ErrDirtyWorktree = errors.New("working tree has uncommitted changes; commit or stash before starting")

const dryRunMessage = "Dry-run placeholder (CI)"

// cannedFindingsForHunks returns one deterministic finding per hunk for dry-run
// (CI). IDs are stable via hunkid.StableFindingID.
func cannedFindingsForHunks(hunks []diff.Hunk) []findings.Finding {
	out := make([]findings.Finding, 0, len(hunks))
	for _, h := range hunks {
		file := h.FilePath
		if file == "" {
			file = "unknown"
		}
		id := hunkid.StableFindingID(file, 1, 0, 0, dryRunMessage)
		out = append(out, findings.Finding{
			ID:       id,
			File:     file,
			Line:     1,
			Severity: findings.SeverityInfo,
			Category: findings.CategoryStyle,
			Message:  dryRunMessage,
		})
	}
	return out
}

// StartOptions configures Start. All fields are required except Ref (default "HEAD" by caller).
// DryRun skips the LLM and injects canned findings. Model and OllamaBaseURL are used when DryRun is false.
// ContextLimit and WarnThreshold are used for token estimation warnings (Phase 3.2); zero values disable the warning.
type StartOptions struct {
	RepoRoot       string
	StateDir       string
	WorktreeRoot   string
	Ref            string
	DryRun         bool
	Model          string
	OllamaBaseURL  string
	ContextLimit   int
	WarnThreshold  float64
}

// FinishOptions configures Finish.
type FinishOptions struct {
	RepoRoot     string
	StateDir     string
	WorktreeRoot string
}

// RunOptions configures Run. DryRun skips the LLM and injects canned findings.
// ContextLimit and WarnThreshold are used for token estimation warnings (Phase 3.2); zero values disable the warning.
type RunOptions struct {
	RepoRoot      string
	StateDir      string
	DryRun        bool
	Model         string
	OllamaBaseURL string
	ContextLimit  int
	WarnThreshold float64
}

// Start creates a worktree at the given ref, writes the session, then runs the
// review pipeline: diff baseline..HEAD, partition to to-review hunks, and
// either calls Ollama for each hunk or injects canned findings when DryRun.
// Session is updated with findings and last_reviewed_at = HEAD. Validates
// clean worktree and that ref is an ancestor of HEAD. Caller should default
// Ref to "HEAD" if unset. Errors are wrapped with %w so callers can use
// errors.Is for session.ErrLocked, git.ErrWorktreeExists, git.ErrBaselineNotAncestor, ollama.ErrUnreachable.
func Start(ctx context.Context, opts StartOptions) (err error) {
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

	// Upfront Ollama check when not dry-run so wrong URL fails before creating worktree (Phase 3 remediation).
	if !opts.DryRun {
		client := ollama.NewClient(opts.OllamaBaseURL, nil)
		if _, err := client.Check(ctx, opts.Model); err != nil {
			return fmt.Errorf("start: %w", err)
		}
	}

	sha, err := git.RevParse(opts.RepoRoot, ref)
	if err != nil {
		return fmt.Errorf("start: resolve ref %q: %w", ref, err)
	}

	worktreePath, err := git.Create(opts.RepoRoot, opts.WorktreeRoot, ref)
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}
	defer func() {
		if err != nil {
			_ = git.Remove(opts.RepoRoot, worktreePath)
		}
	}()

	s := session.Session{
		BaselineRef:    sha,
		LastReviewedAt: "",
		DismissedIDs:   nil,
	}
	if err := session.Save(opts.StateDir, &s); err != nil {
		return fmt.Errorf("start: save session: %w", err)
	}

	headSHA, err := git.RevParse(opts.RepoRoot, "HEAD")
	if err != nil {
		return fmt.Errorf("start: resolve HEAD: %w", err)
	}

	part, err := scope.Partition(ctx, opts.RepoRoot, sha, headSHA, "", nil)
	if err != nil {
		return fmt.Errorf("start: partition: %w", err)
	}

	if len(part.ToReview) == 0 {
		s.LastReviewedAt = headSHA
		if err := session.Save(opts.StateDir, &s); err != nil {
			return fmt.Errorf("start: save session: %w", err)
		}
		return nil
	}

	var collected []findings.Finding
	if opts.DryRun {
		collected = cannedFindingsForHunks(part.ToReview)
	} else {
		// Token estimation: warn once if any hunk's prompt would exceed context threshold (Phase 3.2).
		if opts.ContextLimit > 0 && opts.WarnThreshold > 0 {
			systemPrompt, err := prompt.SystemPrompt(opts.StateDir)
			if err != nil {
				return fmt.Errorf("start: system prompt: %w", err)
			}
			maxPromptTokens := 0
			for _, h := range part.ToReview {
				userPrompt := prompt.UserPrompt(h)
				n := tokens.Estimate(systemPrompt + "\n" + userPrompt)
				if n > maxPromptTokens {
					maxPromptTokens = n
				}
			}
			if w := tokens.WarnIfOver(maxPromptTokens, tokens.DefaultResponseReserve, opts.ContextLimit, opts.WarnThreshold); w != "" {
				fmt.Fprintln(os.Stderr, w)
			}
		}
		client := ollama.NewClient(opts.OllamaBaseURL, nil)
		for _, hunk := range part.ToReview {
			list, err := review.ReviewHunk(ctx, client, opts.Model, opts.StateDir, hunk)
			if err != nil {
				return fmt.Errorf("start: review hunk %s: %w", hunk.FilePath, err)
			}
			collected = append(collected, list...)
		}
	}

	s.Findings = collected
	s.LastReviewedAt = headSHA
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

// Run loads the session, partitions baseline..HEAD into to-review hunks (using
// last_reviewed_at for incremental), runs review for each to-review hunk (or
// canned findings when DryRun), merges new findings into the session, and
// updates last_reviewed_at = HEAD. Returns ErrNoSession if there is no active
// session. On Ollama unreachable, returns an error that wraps ollama.ErrUnreachable.
func Run(ctx context.Context, opts RunOptions) error {
	if opts.RepoRoot == "" || opts.StateDir == "" {
		return fmt.Errorf("run Run: RepoRoot and StateDir required")
	}

	s, err := session.Load(opts.StateDir)
	if err != nil {
		return fmt.Errorf("run: load session: %w", err)
	}
	if s.BaselineRef == "" {
		return ErrNoSession
	}

	headSHA, err := git.RevParse(opts.RepoRoot, "HEAD")
	if err != nil {
		return fmt.Errorf("run: resolve HEAD: %w", err)
	}

	part, err := scope.Partition(ctx, opts.RepoRoot, s.BaselineRef, headSHA, s.LastReviewedAt, nil)
	if err != nil {
		return fmt.Errorf("run: partition: %w", err)
	}

	if len(part.ToReview) == 0 {
		s.LastReviewedAt = headSHA
		return session.Save(opts.StateDir, &s)
	}

	var newFindings []findings.Finding
	if opts.DryRun {
		newFindings = cannedFindingsForHunks(part.ToReview)
	} else {
		// Upfront Ollama check so wrong URL fails before review loop (Phase 3 remediation).
		client := ollama.NewClient(opts.OllamaBaseURL, nil)
		if _, err := client.Check(ctx, opts.Model); err != nil {
			return fmt.Errorf("run: %w", err)
		}
		// Token estimation: warn once if any hunk's prompt would exceed context threshold (Phase 3.2).
		if opts.ContextLimit > 0 && opts.WarnThreshold > 0 {
			systemPrompt, err := prompt.SystemPrompt(opts.StateDir)
			if err != nil {
				return fmt.Errorf("run: system prompt: %w", err)
			}
			maxPromptTokens := 0
			for _, h := range part.ToReview {
				userPrompt := prompt.UserPrompt(h)
				n := tokens.Estimate(systemPrompt + "\n" + userPrompt)
				if n > maxPromptTokens {
					maxPromptTokens = n
				}
			}
			if w := tokens.WarnIfOver(maxPromptTokens, tokens.DefaultResponseReserve, opts.ContextLimit, opts.WarnThreshold); w != "" {
				fmt.Fprintln(os.Stderr, w)
			}
		}
		for _, hunk := range part.ToReview {
			list, err := review.ReviewHunk(ctx, client, opts.Model, opts.StateDir, hunk)
			if err != nil {
				return fmt.Errorf("run: review hunk %s: %w", hunk.FilePath, err)
			}
			newFindings = append(newFindings, list...)
		}
	}

	s.Findings = append(s.Findings, newFindings...)
	s.LastReviewedAt = headSHA
	if err := session.Save(opts.StateDir, &s); err != nil {
		return fmt.Errorf("run: save session: %w", err)
	}
	return nil
}
