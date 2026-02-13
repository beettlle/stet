// Package run implements the start and finish session flows: worktree creation,
// session persistence, review pipeline (diff, scope, Ollama or dry-run), and
// cleanup. Used by the CLI and by tests.
package run

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"stet/cli/internal/diff"
	"stet/cli/internal/expand"
	"stet/cli/internal/findings"
	"stet/cli/internal/git"
	"stet/cli/internal/history"
	"stet/cli/internal/hunkid"
	"stet/cli/internal/ollama"
	"stet/cli/internal/prompt"
	"stet/cli/internal/review"
	"stet/cli/internal/rules"
	"stet/cli/internal/scope"
	"stet/cli/internal/session"
	"stet/cli/internal/tokens"
	"stet/cli/internal/version"
)

// ErrNoSession indicates there is no active session (e.g. finish without start).
var ErrNoSession = errors.New("no active session; run stet start first")

// ErrDirtyWorktree indicates the working tree has uncommitted changes.
var ErrDirtyWorktree = errors.New("working tree has uncommitted changes; commit or stash before starting")

const (
	dryRunMessage         = "Dry-run placeholder (CI)"
	_defaultOllamaTimeout = 5 * time.Minute
)

// generateSessionID returns a new random session ID (32 hex chars). Used for
// session identity and for the git note on finish.
func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("stet-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// writeStreamLine writes one JSON object and a newline to w. Used for NDJSON streaming.
func writeStreamLine(w io.Writer, obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		return err
	}
	return nil
}

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
			ID:         id,
			File:       file,
			Line:      1,
			Severity:  findings.SeverityInfo,
			Category:  findings.CategoryMaintainability,
			Confidence: 1.0,
			Message:   dryRunMessage,
		})
	}
	return out
}

// hunkLineRange is the line range (1-based, inclusive) for a reviewed hunk in the new file.
type hunkLineRange struct {
	file       string
	start, end int
}

// addressedFindingIDs returns IDs of existing findings that lie inside a reviewed hunk
// and are not in newFindingIDSet (i.e. re-review did not report them, so they are considered addressed).
// Finding location uses f.Line, or f.Range.Start when Range is set.
func addressedFindingIDs(hunks []diff.Hunk, existingFindings []findings.Finding, newFindingIDSet map[string]struct{}) []string {
	var ranges []hunkLineRange
	for _, h := range hunks {
		start, end, ok := expand.HunkLineRange(h)
		if !ok {
			continue
		}
		ranges = append(ranges, hunkLineRange{file: h.FilePath, start: start, end: end})
	}
	inReviewedHunk := func(file string, line int) bool {
		for _, r := range ranges {
			if r.file == file && line >= r.start && line <= r.end {
				return true
			}
		}
		return false
	}
	var addressed []string
	seen := make(map[string]struct{})
	for _, f := range existingFindings {
		if f.ID == "" {
			continue
		}
		if _, inNew := newFindingIDSet[f.ID]; inNew {
			continue
		}
		line := f.Line
		if f.Range != nil {
			line = f.Range.Start
		}
		if inReviewedHunk(f.File, line) {
			if _, ok := seen[f.ID]; !ok {
				seen[f.ID] = struct{}{}
				addressed = append(addressed, f.ID)
			}
		}
	}
	return addressed
}

// StartOptions configures Start. All fields are required except Ref (default "HEAD" by caller).
// DryRun skips the LLM and injects canned findings. Model and OllamaBaseURL are used when DryRun is false.
// ContextLimit and WarnThreshold are used for token estimation warnings (Phase 3.2); zero values disable the warning.
// Temperature and NumCtx are passed to Ollama /api/generate options.
// Verbose, when true, prints progress to stderr (worktree, partition summary, per-hunk).
// AllowDirty, when true, skips the clean worktree check and proceeds with a warning.
// StreamOut, when non-nil, receives NDJSON events (progress, finding, done) one per line.
type StartOptions struct {
	RepoRoot      string
	StateDir      string
	WorktreeRoot  string
	Ref           string
	DryRun        bool
	AllowDirty    bool
	Model         string
	OllamaBaseURL string
	ContextLimit  int
	WarnThreshold float64
	Timeout       time.Duration
	Temperature   float64
	NumCtx        int
	Verbose                 bool
	StreamOut               io.Writer
	RAGSymbolMaxDefinitions int
	RAGSymbolMaxTokens      int
}

// FinishOptions configures Finish.
type FinishOptions struct {
	RepoRoot     string
	StateDir     string
	WorktreeRoot string
}

// RunOptions configures Run. DryRun skips the LLM and injects canned findings.
// ContextLimit and WarnThreshold are used for token estimation warnings (Phase 3.2); zero values disable the warning.
// Temperature and NumCtx are passed to Ollama /api/generate options.
// Verbose, when true, prints progress to stderr (partition summary, per-hunk).
// RunOptions does not include WorktreeRoot because Run does not create or remove worktrees (only Finish does).
// StreamOut, when non-nil, receives NDJSON events (progress, finding, done) one per line.
type RunOptions struct {
	RepoRoot      string
	StateDir      string
	DryRun        bool
	Model         string
	OllamaBaseURL string
	ContextLimit  int
	WarnThreshold float64
	Timeout       time.Duration
	Temperature   float64
	NumCtx        int
	Verbose                 bool
	StreamOut               io.Writer
	RAGSymbolMaxDefinitions int
	RAGSymbolMaxTokens      int
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
	if opts.RAGSymbolMaxDefinitions < 0 {
		opts.RAGSymbolMaxDefinitions = 0
	}
	if opts.RAGSymbolMaxTokens < 0 {
		opts.RAGSymbolMaxTokens = 0
	}

	clean, err := git.IsClean(opts.RepoRoot)
	if err != nil {
		return fmt.Errorf("start: check worktree: %w", err)
	}
	if !clean {
		if opts.AllowDirty {
			fmt.Fprintln(os.Stderr, "Warning: proceeding with uncommitted changes; review scope may be unclear")
		} else {
			return ErrDirtyWorktree
		}
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

	headSHA, err := git.RevParse(opts.RepoRoot, "HEAD")
	if err != nil {
		return fmt.Errorf("start: resolve HEAD: %w", err)
	}

	// When baseline equals HEAD, there is nothing to review; skip worktree and Ollama.
	if sha == headSHA {
		s := session.Session{
			SessionID:      generateSessionID(),
			BaselineRef:    sha,
			LastReviewedAt: headSHA,
			DismissedIDs:   nil,
		}
		if err := session.Save(opts.StateDir, &s); err != nil {
			return fmt.Errorf("start: save session: %w", err)
		}
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "Nothing to review (baseline is HEAD).")
		}
		return nil
	}

	// Upfront Ollama check when not dry-run so wrong URL fails before creating worktree (Phase 3 remediation).
	// Reuse the same client (with configurable timeout) for the check and the review loop.
	var ollamaClient *ollama.Client
	if !opts.DryRun {
		timeout := opts.Timeout
		if timeout == 0 {
			timeout = _defaultOllamaTimeout
		}
		ollamaClient = ollama.NewClient(opts.OllamaBaseURL, &http.Client{Timeout: timeout})
		if _, err := ollamaClient.Check(ctx, opts.Model); err != nil {
			return fmt.Errorf("start: %w", err)
		}
	}

	worktreePath, err := git.Create(opts.RepoRoot, opts.WorktreeRoot, ref)
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "Worktree created at %s\n", worktreePath)
	}
	// Remove worktree on any error so we do not leave it behind and pollute the repo.
	defer func() {
		if err != nil {
			if removeErr := git.Remove(opts.RepoRoot, worktreePath); removeErr != nil {
				fmt.Fprintf(os.Stderr, "warning: cleanup worktree: %v\n", removeErr)
			}
		}
	}()

	s := session.Session{
		SessionID:      generateSessionID(),
		BaselineRef:    sha,
		LastReviewedAt: "",
		DismissedIDs:   nil,
	}
	if err := session.Save(opts.StateDir, &s); err != nil {
		return fmt.Errorf("start: save session: %w", err)
	}

	part, err := scope.Partition(ctx, opts.RepoRoot, sha, headSHA, "", nil)
	if err != nil {
		return fmt.Errorf("start: partition: %w", err)
	}
	if opts.Verbose {
		approvedN := 0
		if part.Approved != nil {
			approvedN = len(part.Approved)
		}
		fmt.Fprintf(os.Stderr, "%d hunks to review, %d already approved\n", len(part.ToReview), approvedN)
	}

	if len(part.ToReview) == 0 {
		if opts.StreamOut != nil {
			_ = writeStreamLine(opts.StreamOut, map[string]string{"type": "progress", "msg": "Nothing to review."})
			_ = writeStreamLine(opts.StreamOut, map[string]string{"type": "done"})
		}
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "Nothing to review.")
		}
		s.LastReviewedAt = headSHA
		if err := session.Save(opts.StateDir, &s); err != nil {
			return fmt.Errorf("start: save session: %w", err)
		}
		return nil
	}

	var collected []findings.Finding
	total := len(part.ToReview)
	if opts.StreamOut != nil {
		_ = writeStreamLine(opts.StreamOut, map[string]interface{}{"type": "progress", "msg": fmt.Sprintf("%d hunks to review", total)})
	}

	if opts.DryRun {
		for i, hunk := range part.ToReview {
			if opts.StreamOut != nil {
				_ = writeStreamLine(opts.StreamOut, map[string]interface{}{"type": "progress", "msg": fmt.Sprintf("Reviewing hunk %d/%d: %s", i+1, total, hunk.FilePath)})
			}
			batch := cannedFindingsForHunks([]diff.Hunk{hunk})
			batch = findings.FilterAbstention(batch)
			batch = findings.FilterFPKillList(batch)
			findings.SetCursorURIs(opts.RepoRoot, batch)
			for _, f := range batch {
				if opts.StreamOut != nil {
					_ = writeStreamLine(opts.StreamOut, map[string]interface{}{"type": "finding", "data": f})
				}
				collected = append(collected, f)
			}
		}
	} else {
		branch, commitMsg, intentErr := git.UserIntent(opts.RepoRoot)
		if intentErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not retrieve Git intent (branch/commit): %v; using placeholder\n", intentErr)
		}
		userIntent := &prompt.UserIntent{Branch: branch, CommitMsg: commitMsg}
		// Token estimation: warn once if any hunk's prompt would exceed context threshold (Phase 3.2).
		if opts.ContextLimit > 0 && opts.WarnThreshold > 0 {
			systemPrompt, err := prompt.SystemPrompt(opts.StateDir)
			if err != nil {
				return fmt.Errorf("start: system prompt: %w", err)
			}
			systemPrompt = prompt.InjectUserIntent(systemPrompt, branch, commitMsg)
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
		genOpts := &ollama.GenerateOptions{Temperature: opts.Temperature, NumCtx: opts.NumCtx}
		cursorRules, _ := rules.LoadRules(filepath.Join(opts.RepoRoot, ".cursor", "rules"))
		for i, hunk := range part.ToReview {
			if opts.StreamOut != nil {
				_ = writeStreamLine(opts.StreamOut, map[string]interface{}{"type": "progress", "msg": fmt.Sprintf("Reviewing hunk %d/%d: %s", i+1, total, hunk.FilePath)})
			}
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "Reviewing hunk %d/%d: %s\n", i+1, total, hunk.FilePath)
			}
			list, err := review.ReviewHunk(ctx, ollamaClient, opts.Model, opts.StateDir, hunk, genOpts, userIntent, cursorRules, opts.RepoRoot, opts.ContextLimit, opts.RAGSymbolMaxDefinitions, opts.RAGSymbolMaxTokens)
			if err != nil {
				return fmt.Errorf("start: review hunk %s: %w", hunk.FilePath, err)
			}
			batch := findings.FilterAbstention(list)
			batch = findings.FilterFPKillList(batch)
			findings.SetCursorURIs(opts.RepoRoot, batch)
			for _, f := range batch {
				if opts.StreamOut != nil {
					_ = writeStreamLine(opts.StreamOut, map[string]interface{}{"type": "finding", "data": f})
				}
				collected = append(collected, f)
			}
		}
	}
	if opts.StreamOut != nil {
		_ = writeStreamLine(opts.StreamOut, map[string]string{"type": "done"})
	}
	// Auto-dismiss: previous findings in reviewed hunks not in collected are considered addressed.
	if len(s.Findings) > 0 {
		collectedIDSet := make(map[string]struct{}, len(collected))
		for _, f := range collected {
			if f.ID != "" {
				collectedIDSet[f.ID] = struct{}{}
			}
		}
		addressed := addressedFindingIDs(part.ToReview, s.Findings, collectedIDSet)
		dismissedSet := make(map[string]struct{}, len(s.DismissedIDs))
		for _, id := range s.DismissedIDs {
			dismissedSet[id] = struct{}{}
		}
		for _, id := range addressed {
			if _, ok := dismissedSet[id]; !ok {
				dismissedSet[id] = struct{}{}
				s.DismissedIDs = append(s.DismissedIDs, id)
			}
		}
		if len(addressed) > 0 {
			dismissals := make([]history.Dismissal, len(addressed))
			for i, id := range addressed {
				dismissals[i] = history.Dismissal{FindingID: id, Reason: history.ReasonAlreadyCorrect}
			}
			diffRef := headSHA
			if diffRef == "" {
				diffRef = s.BaselineRef
			}
			rec := history.Record{
				DiffRef:      diffRef,
				ReviewOutput: s.Findings,
				UserAction: history.UserAction{
					DismissedIDs: addressed,
					Dismissals:   dismissals,
				},
			}
			if err := history.Append(opts.StateDir, rec, history.DefaultMaxRecords); err != nil {
				return fmt.Errorf("start: append history: %w", err)
			}
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

	if _, err := os.Stat(path); err == nil {
		if err := git.Remove(opts.RepoRoot, path); err != nil {
			return fmt.Errorf("finish: remove worktree: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("finish: stat worktree: %w", err)
	}

	headSHA, err := git.RevParse(opts.RepoRoot, "HEAD")
	if err != nil {
		return fmt.Errorf("finish: resolve HEAD: %w", err)
	}
	baselineSHA, err := git.RevParse(opts.RepoRoot, s.BaselineRef)
	if err != nil {
		return fmt.Errorf("finish: resolve baseline: %w", err)
	}
	sessionID := s.SessionID
	if sessionID == "" {
		sessionID = generateSessionID()
	}
	if len(s.Findings) > 0 {
		diffRef := s.LastReviewedAt
		if diffRef == "" {
			diffRef = s.BaselineRef
		}
		rec := history.Record{
			DiffRef:      diffRef,
			ReviewOutput: s.Findings,
			UserAction: history.UserAction{
				DismissedIDs: s.DismissedIDs,
				FinishedAt:  time.Now().UTC().Format(time.RFC3339),
			},
		}
		if err := history.Append(opts.StateDir, rec, history.DefaultMaxRecords); err != nil {
			return fmt.Errorf("finish: append history: %w", err)
		}
	}
	notePayload := struct {
		SessionID       string `json:"session_id"`
		BaselineSHA     string `json:"baseline_sha"`
		HeadSHA         string `json:"head_sha"`
		FindingsCount   int    `json:"findings_count"`
		DismissalsCount int    `json:"dismissals_count"`
		ToolVersion     string `json:"tool_version"`
		FinishedAt      string `json:"finished_at"`
	}{
		SessionID:       sessionID,
		BaselineSHA:     baselineSHA,
		HeadSHA:         headSHA,
		FindingsCount:   len(s.Findings),
		DismissalsCount: len(s.DismissedIDs),
		ToolVersion:     version.Version,
		FinishedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	noteJSON, err := json.Marshal(notePayload)
	if err != nil {
		return fmt.Errorf("finish: marshal note: %w", err)
	}
	if err := git.AddNote(opts.RepoRoot, git.NotesRefStet, headSHA, string(noteJSON)); err != nil {
		return fmt.Errorf("finish: write git note: %w", err)
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
	if opts.RAGSymbolMaxDefinitions < 0 {
		opts.RAGSymbolMaxDefinitions = 0
	}
	if opts.RAGSymbolMaxTokens < 0 {
		opts.RAGSymbolMaxTokens = 0
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
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "%d hunks to review\n", len(part.ToReview))
	}

	if len(part.ToReview) == 0 {
		if opts.StreamOut != nil {
			_ = writeStreamLine(opts.StreamOut, map[string]string{"type": "progress", "msg": "Nothing to review."})
			_ = writeStreamLine(opts.StreamOut, map[string]string{"type": "done"})
		}
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "Nothing to review.")
		}
		s.LastReviewedAt = headSHA
		return session.Save(opts.StateDir, &s)
	}

	var newFindings []findings.Finding
	total := len(part.ToReview)
	if opts.StreamOut != nil {
		_ = writeStreamLine(opts.StreamOut, map[string]interface{}{"type": "progress", "msg": fmt.Sprintf("%d hunks to review", total)})
	}

	if opts.DryRun {
		for i, hunk := range part.ToReview {
			if opts.StreamOut != nil {
				_ = writeStreamLine(opts.StreamOut, map[string]interface{}{"type": "progress", "msg": fmt.Sprintf("Reviewing hunk %d/%d: %s", i+1, total, hunk.FilePath)})
			}
			batch := cannedFindingsForHunks([]diff.Hunk{hunk})
			batch = findings.FilterAbstention(batch)
			batch = findings.FilterFPKillList(batch)
			findings.SetCursorURIs(opts.RepoRoot, batch)
			for _, f := range batch {
				if opts.StreamOut != nil {
					_ = writeStreamLine(opts.StreamOut, map[string]interface{}{"type": "finding", "data": f})
				}
				newFindings = append(newFindings, f)
			}
		}
	} else {
		timeout := opts.Timeout
		if timeout == 0 {
			timeout = _defaultOllamaTimeout
		}
		client := ollama.NewClient(opts.OllamaBaseURL, &http.Client{Timeout: timeout})
		// Upfront Ollama check so wrong URL fails before review loop (Phase 3 remediation).
		if _, err := client.Check(ctx, opts.Model); err != nil {
			return fmt.Errorf("run: %w", err)
		}
		branch, commitMsg, intentErr := git.UserIntent(opts.RepoRoot)
		if intentErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not retrieve Git intent (branch/commit): %v; using placeholder\n", intentErr)
		}
		userIntent := &prompt.UserIntent{Branch: branch, CommitMsg: commitMsg}
		// Token estimation: warn once if any hunk's prompt would exceed context threshold (Phase 3.2).
		if opts.ContextLimit > 0 && opts.WarnThreshold > 0 {
			systemPrompt, err := prompt.SystemPrompt(opts.StateDir)
			if err != nil {
				return fmt.Errorf("run: system prompt: %w", err)
			}
			systemPrompt = prompt.InjectUserIntent(systemPrompt, branch, commitMsg)
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
		genOpts := &ollama.GenerateOptions{Temperature: opts.Temperature, NumCtx: opts.NumCtx}
		cursorRules, _ := rules.LoadRules(filepath.Join(opts.RepoRoot, ".cursor", "rules"))
		for i, hunk := range part.ToReview {
			if opts.StreamOut != nil {
				_ = writeStreamLine(opts.StreamOut, map[string]interface{}{"type": "progress", "msg": fmt.Sprintf("Reviewing hunk %d/%d: %s", i+1, total, hunk.FilePath)})
			}
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "Reviewing hunk %d/%d: %s\n", i+1, total, hunk.FilePath)
			}
			list, err := review.ReviewHunk(ctx, client, opts.Model, opts.StateDir, hunk, genOpts, userIntent, cursorRules, opts.RepoRoot, opts.ContextLimit, opts.RAGSymbolMaxDefinitions, opts.RAGSymbolMaxTokens)
			if err != nil {
				return fmt.Errorf("run: review hunk %s: %w", hunk.FilePath, err)
			}
			batch := findings.FilterAbstention(list)
			batch = findings.FilterFPKillList(batch)
			findings.SetCursorURIs(opts.RepoRoot, batch)
			for _, f := range batch {
				if opts.StreamOut != nil {
					_ = writeStreamLine(opts.StreamOut, map[string]interface{}{"type": "finding", "data": f})
				}
				newFindings = append(newFindings, f)
			}
		}
	}
	if opts.StreamOut != nil {
		_ = writeStreamLine(opts.StreamOut, map[string]string{"type": "done"})
	}
	// Auto-dismiss: findings in reviewed hunks that were not re-reported are considered addressed.
	newFindingIDSet := make(map[string]struct{}, len(newFindings))
	for _, f := range newFindings {
		if f.ID != "" {
			newFindingIDSet[f.ID] = struct{}{}
		}
	}
	addressed := addressedFindingIDs(part.ToReview, s.Findings, newFindingIDSet)
	dismissedSet := make(map[string]struct{}, len(s.DismissedIDs))
	for _, id := range s.DismissedIDs {
		dismissedSet[id] = struct{}{}
	}
	for _, id := range addressed {
		if _, ok := dismissedSet[id]; !ok {
			dismissedSet[id] = struct{}{}
			s.DismissedIDs = append(s.DismissedIDs, id)
		}
	}
	if len(addressed) > 0 {
		dismissals := make([]history.Dismissal, len(addressed))
		for i, id := range addressed {
			dismissals[i] = history.Dismissal{FindingID: id, Reason: history.ReasonAlreadyCorrect}
		}
		diffRef := headSHA
		if diffRef == "" {
			diffRef = s.BaselineRef
		}
		rec := history.Record{
			DiffRef:      diffRef,
			ReviewOutput: s.Findings,
			UserAction: history.UserAction{
				DismissedIDs: addressed,
				Dismissals:   dismissals,
			},
		}
		if err := history.Append(opts.StateDir, rec, history.DefaultMaxRecords); err != nil {
			return fmt.Errorf("run: append history: %w", err)
		}
	}
	s.Findings = append(s.Findings, newFindings...)
	s.LastReviewedAt = headSHA
	if err := session.Save(opts.StateDir, &s); err != nil {
		return fmt.Errorf("run: save session: %w", err)
	}
	return nil
}
