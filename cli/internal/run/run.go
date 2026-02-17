// Package run implements the start and finish session flows: worktree creation,
// session persistence, review pipeline (diff, scope, Ollama or dry-run), and
// cleanup. Used by the CLI and by tests.
//
// Run and applyAutoDismiss mutate session state (including DismissedIDs). Callers
// must not invoke Run concurrently for the same session/state dir.
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
	"strings"
	"time"

	"stet/cli/internal/config"
	"stet/cli/internal/diff"
	"stet/cli/internal/erruser"
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
	"stet/cli/internal/trace"
	"stet/cli/internal/version"
)

// ErrNoSession indicates there is no active session (e.g. finish without start).
var ErrNoSession = errors.New("no active session; run stet start first")

// captureUsage returns false when STET_CAPTURE_USAGE is 0/false/no/off (case-insensitive); otherwise true.
func captureUsage() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("STET_CAPTURE_USAGE")))
	switch v {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

// ErrDirtyWorktree indicates the working tree has uncommitted changes.
var ErrDirtyWorktree = errors.New("working tree has uncommitted changes; commit or stash before starting")

const (
	dryRunMessage            = "Dry-run placeholder (CI)"
	_defaultOllamaTimeout    = 5 * time.Minute
	maxPromptContextStoreLen = 4096
)

// truncateForPromptContext truncates s to maxLen and appends "\n[truncated]" if truncated.
func truncateForPromptContext(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n[truncated]"
}

// runPromptShadows converts the session's PromptShadows to []prompt.Shadow for injection.
func runPromptShadows(s *session.Session) []prompt.Shadow {
	if s == nil || len(s.PromptShadows) == 0 {
		return nil
	}
	out := make([]prompt.Shadow, len(s.PromptShadows))
	for i := range s.PromptShadows {
		out[i] = prompt.Shadow{
			FindingID:     s.PromptShadows[i].FindingID,
			PromptContext: s.PromptShadows[i].PromptContext,
		}
	}
	return out
}

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

// tryWriteStreamLine calls writeStreamLine and logs to stderr on error. Used for best-effort NDJSON streaming.
func tryWriteStreamLine(w io.Writer, obj interface{}) {
	if err := writeStreamLine(w, obj); err != nil {
		fmt.Fprintf(os.Stderr, "stet: stream write: %v\n", err)
	}
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
			Line:       1,
			Severity:   findings.SeverityInfo,
			Category:   findings.CategoryMaintainability,
			Confidence: 1.0,
			Message:    dryRunMessage,
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

// applyAutoDismiss updates s.DismissedIDs for findings in toReview that are not in newFindingIDSet
// and appends a history record when any are addressed. Caller must save the session after.
// runConfig is attached to the history record when non-nil.
func applyAutoDismiss(s *session.Session, toReview []diff.Hunk, newFindingIDSet map[string]struct{}, headSHA, stateDir string, runConfig *history.RunConfigSnapshot) error {
	if stateDir == "" {
		return erruser.New("Could not record review history: state directory is required.", nil)
	}
	if s == nil || len(s.Findings) == 0 {
		return nil
	}
	addressed := addressedFindingIDs(toReview, s.Findings, newFindingIDSet)
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
	if len(addressed) == 0 {
		return nil
	}
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
		RunConfig: runConfig,
	}
	if err := history.Append(stateDir, rec, history.DefaultMaxRecords); err != nil {
		return erruser.New("Could not record review history.", err)
	}
	return nil
}

// dismissedLocation is file + line range (1-based, inclusive) for a dismissed finding.
type dismissedLocation struct {
	file                string
	startLine, endLine int
}

// filterHunksWithDismissedFindings returns hunks from toReview that do not contain
// any dismissed finding. When forceFullReview is true, returns toReview unchanged.
func filterHunksWithDismissedFindings(toReview []diff.Hunk, s *session.Session, forceFullReview bool) (filtered []diff.Hunk, skipped int) {
	if forceFullReview || s == nil || len(s.DismissedIDs) == 0 {
		return toReview, 0
	}
	findingByID := make(map[string]findings.Finding, len(s.Findings))
	for _, f := range s.Findings {
		if f.ID != "" {
			findingByID[f.ID] = f
		}
	}
	var locs []dismissedLocation
	for _, id := range s.DismissedIDs {
		f, ok := findingByID[id]
		if !ok {
			continue
		}
		start, end := f.Line, f.Line
		if f.Range != nil {
			start, end = f.Range.Start, f.Range.End
		}
		locs = append(locs, dismissedLocation{file: f.File, startLine: start, endLine: end})
	}
	for _, h := range toReview {
		hStart, hEnd, ok := expand.HunkLineRange(h)
		if !ok {
			filtered = append(filtered, h)
			continue
		}
		hasDismissed := false
		for _, loc := range locs {
			if loc.file == h.FilePath && loc.startLine <= hEnd && loc.endLine >= hStart {
				hasDismissed = true
				break
			}
		}
		if hasDismissed {
			skipped++
		} else {
			filtered = append(filtered, h)
		}
	}
	return filtered, skipped
}

// StartOptions configures Start. All fields are required except Ref (default "HEAD" by caller).
// DryRun skips the LLM and injects canned findings. Model and OllamaBaseURL are used when DryRun is false.
// ContextLimit and WarnThreshold are used for token estimation warnings (Phase 3.2); zero values disable the warning.
// Temperature and NumCtx are passed to Ollama /api/generate options.
// Verbose, when true, prints progress to stderr (worktree, partition summary, per-hunk).
// AllowDirty, when true, skips the clean worktree check and proceeds with a warning.
// StreamOut, when non-nil, receives NDJSON events (progress, finding, done) one per line.
// PersistStrictness, PersistRAGSymbolMaxDefinitions, PersistRAGSymbolMaxTokens: when non-nil,
// the value is stored in the session so stet run uses it when the corresponding flag is not set.
type StartOptions struct {
	RepoRoot                string
	StateDir                string
	WorktreeRoot            string
	Ref                     string
	DryRun                  bool
	AllowDirty              bool
	Model                   string
	OllamaBaseURL           string
	ContextLimit            int
	WarnThreshold           float64
	Timeout                 time.Duration
	Temperature             float64
	NumCtx                  int
	Verbose                 bool
	StreamOut               io.Writer
	RAGSymbolMaxDefinitions int
	RAGSymbolMaxTokens      int
	// MinConfidenceKeep and MinConfidenceMaintainability are abstention thresholds (0,0 = use 0.8, 0.9).
	// ApplyFPKillList nil = apply FP kill list (true); set to false for strict+ presets.
	MinConfidenceKeep            float64
	MinConfidenceMaintainability float64
	ApplyFPKillList              *bool
	// Nitpicky enables convention- and typo-aware review; when true, FP kill list is not applied.
	Nitpicky bool
	// Session-persisted options (from stet start flags); when set, stored in session.
	PersistStrictness              *string
	PersistRAGSymbolMaxDefinitions *int
	PersistRAGSymbolMaxTokens      *int
	PersistNitpicky                *bool
	// TraceOut, when non-nil, receives internal trace output (partition, hunks, rules, RAG, LLM I/O). Used when --trace is set.
	TraceOut io.Writer
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
	RepoRoot                     string
	StateDir                     string
	DryRun                       bool
	Model                        string
	OllamaBaseURL                string
	ContextLimit                 int
	WarnThreshold                float64
	Timeout                      time.Duration
	Temperature                  float64
	NumCtx                       int
	Verbose                      bool
	StreamOut                    io.Writer
	RAGSymbolMaxDefinitions      int
	RAGSymbolMaxTokens           int
	MinConfidenceKeep            float64
	MinConfidenceMaintainability float64
	ApplyFPKillList              *bool
	Nitpicky                     bool
	// TraceOut, when non-nil, receives internal trace output. Used when --trace is set.
	TraceOut io.Writer
	// ForceFullReview, when true, passes empty lastReviewedAt to Partition so all hunks are to-review (used by rerun).
	ForceFullReview bool
	// ReplaceFindings, when true, replaces session findings with only this run's results; when false, merges (append + auto-dismiss).
	ReplaceFindings bool
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
		return erruser.New("Start failed: repository root and state directory are required.", nil)
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
		return err
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
		return err
	}
	defer release()

	sha, err := git.RevParse(opts.RepoRoot, ref)
	if err != nil {
		return erruser.New("Could not resolve baseline ref.", err)
	}

	headSHA, err := git.RevParse(opts.RepoRoot, "HEAD")
	if err != nil {
		return erruser.New("Could not resolve current commit (HEAD).", err)
	}

	// When baseline equals HEAD, there is nothing to review; skip worktree and Ollama.
	if sha == headSHA {
		s := session.Session{
			SessionID:      generateSessionID(),
			BaselineRef:    sha,
			LastReviewedAt: headSHA,
			DismissedIDs:   nil,
		}
		if opts.PersistStrictness != nil {
			s.Strictness = *opts.PersistStrictness
		}
		if opts.PersistRAGSymbolMaxDefinitions != nil {
			v := *opts.PersistRAGSymbolMaxDefinitions
			s.RAGSymbolMaxDefinitions = &v
		}
		if opts.PersistRAGSymbolMaxTokens != nil {
			v := *opts.PersistRAGSymbolMaxTokens
			s.RAGSymbolMaxTokens = &v
		}
		if opts.PersistNitpicky != nil {
			s.Nitpicky = opts.PersistNitpicky
		}
		if err := session.Save(opts.StateDir, &s); err != nil {
			return err
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
			return err
		}
	}

	worktreePath, err := git.Create(opts.RepoRoot, opts.WorktreeRoot, ref)
	if err != nil {
		return err
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
	if opts.PersistStrictness != nil {
		s.Strictness = *opts.PersistStrictness
	}
	if opts.PersistRAGSymbolMaxDefinitions != nil {
		v := *opts.PersistRAGSymbolMaxDefinitions
		s.RAGSymbolMaxDefinitions = &v
	}
	if opts.PersistRAGSymbolMaxTokens != nil {
		v := *opts.PersistRAGSymbolMaxTokens
		s.RAGSymbolMaxTokens = &v
	}
	if opts.PersistNitpicky != nil {
		s.Nitpicky = opts.PersistNitpicky
	}
	if err := session.Save(opts.StateDir, &s); err != nil {
		return err
	}

	part, err := scope.Partition(ctx, opts.RepoRoot, sha, headSHA, "", nil)
	if err != nil {
		return erruser.New("Could not compute hunks to review.", err)
	}
	tr := trace.New(opts.TraceOut)
	if tr.Enabled() {
		tr.Section("Partition")
		tr.Printf("baseline=%s head=%s last_reviewed_at=\n", sha, headSHA)
		approvedN := 0
		if part.Approved != nil {
			approvedN = len(part.Approved)
		}
		tr.Printf("ToReview=%d Approved=%d\n", len(part.ToReview), approvedN)
		tr.Section("AGENTS.md")
		tr.Printf("AGENTS.md: not used by stet (only .cursor/rules/ used).\n")
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
			tryWriteStreamLine(opts.StreamOut, map[string]string{"type": "progress", "msg": "Nothing to review."})
			tryWriteStreamLine(opts.StreamOut, map[string]string{"type": "done"})
		}
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "Nothing to review.")
		}
		s.LastReviewedAt = headSHA
		if err := session.Save(opts.StateDir, &s); err != nil {
			return err
		}
		return nil
	}

	minKeep, minMaint := opts.MinConfidenceKeep, opts.MinConfidenceMaintainability
	if minKeep == 0 && minMaint == 0 {
		minKeep, minMaint = findings.DefaultMinConfidenceKeep, findings.DefaultMinConfidenceMaintainability
	}
	applyFP := true
	if opts.Nitpicky {
		applyFP = false
	} else if opts.ApplyFPKillList != nil {
		applyFP = *opts.ApplyFPKillList
	}

	var collected []findings.Finding
	findingPromptContext := make(map[string]string)
	var sumPrompt, sumCompletion int
	var sumDuration int64
	total := len(part.ToReview)
	if opts.StreamOut != nil {
		tryWriteStreamLine(opts.StreamOut, map[string]interface{}{"type": "progress", "msg": fmt.Sprintf("%d hunks to review", total)})
	}

	effectiveNumCtx := opts.NumCtx
	effectiveContextLimit := opts.ContextLimit
	// Show is called once upfront; model context length is stable for the run.
	if ollamaClient != nil {
		showResult, showErr := ollamaClient.Show(ctx, opts.Model)
		if showErr == nil && showResult != nil && showResult.ContextLength > 0 {
			if showResult.ContextLength > effectiveNumCtx {
				effectiveNumCtx = showResult.ContextLength
			}
			// Use model's larger context when Show returns a value greater than config.
			if effectiveNumCtx > effectiveContextLimit {
				effectiveContextLimit = effectiveNumCtx
			}
			if tr.Enabled() && effectiveNumCtx > opts.NumCtx {
				tr.Section("Context")
				tr.Printf("using model context %d (config num_ctx=%d)\n", effectiveNumCtx, opts.NumCtx)
			}
		}
	}
	// effectiveNumCtx is sent to Ollama; effectiveContextLimit is the single source of
	// truth for token warnings and ReviewHunk RAG budget. Both reflect the model's
	// actual context when Show succeeds; otherwise config values are used.

	if opts.DryRun {
		for i, hunk := range part.ToReview {
			if opts.StreamOut != nil {
				tryWriteStreamLine(opts.StreamOut, map[string]interface{}{"type": "progress", "msg": fmt.Sprintf("Reviewing hunk %d/%d: %s", i+1, total, hunk.FilePath)})
			}
			batch := cannedFindingsForHunks([]diff.Hunk{hunk})
			batch = findings.FilterAbstention(batch, minKeep, minMaint)
			if applyFP {
				batch = findings.FilterFPKillList(batch)
			}
			findings.SetCursorURIs(opts.RepoRoot, batch)
			ctx := truncateForPromptContext(hunk.RawContent, maxPromptContextStoreLen)
			for _, f := range batch {
				if f.ID != "" {
					findingPromptContext[f.ID] = ctx
				}
				if opts.StreamOut != nil {
					tryWriteStreamLine(opts.StreamOut, map[string]interface{}{"type": "finding", "data": f})
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
		if effectiveContextLimit > 0 && opts.WarnThreshold > 0 {
			systemPrompt, err := prompt.SystemPrompt(opts.StateDir)
			if err != nil {
				return err
			}
			systemPrompt = prompt.InjectUserIntent(systemPrompt, branch, commitMsg)
			if opts.Nitpicky {
				systemPrompt = prompt.AppendNitpickyInstructions(systemPrompt)
			}
			maxPromptTokens := 0
			for _, h := range part.ToReview {
				userPrompt := prompt.UserPrompt(h)
				n := tokens.Estimate(systemPrompt + "\n" + userPrompt)
				if n > maxPromptTokens {
					maxPromptTokens = n
				}
			}
			if w := tokens.WarnIfOver(maxPromptTokens, tokens.DefaultResponseReserve, effectiveContextLimit, opts.WarnThreshold); w != "" {
				fmt.Fprintln(os.Stderr, w)
			}
		}
		genOpts := &ollama.GenerateOptions{Temperature: opts.Temperature, NumCtx: effectiveNumCtx}
		rulesLoader := rules.NewLoader(opts.RepoRoot)
		for i, hunk := range part.ToReview {
			if opts.StreamOut != nil {
				tryWriteStreamLine(opts.StreamOut, map[string]interface{}{"type": "progress", "msg": fmt.Sprintf("Reviewing hunk %d/%d: %s", i+1, total, hunk.FilePath)})
			}
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "Reviewing hunk %d/%d: %s\n", i+1, total, hunk.FilePath)
			}
			if tr.Enabled() {
				tr.Section("Hunk " + fmt.Sprintf("%d/%d", i+1, total) + ": " + hunk.FilePath)
				tr.Printf("strict_id=%s semantic_id=%s\n", hunkid.StrictHunkID(hunk.FilePath, hunk.RawContent), hunkid.SemanticHunkID(hunk.FilePath, hunk.RawContent))
			}
			cursorRules := rulesLoader.RulesForFile(hunk.FilePath)
			list, usage, err := review.ReviewHunk(ctx, ollamaClient, opts.Model, opts.StateDir, hunk, genOpts, userIntent, cursorRules, opts.RepoRoot, effectiveContextLimit, opts.RAGSymbolMaxDefinitions, opts.RAGSymbolMaxTokens, runPromptShadows(&s), opts.Nitpicky, tr)
			if err != nil {
				return erruser.New("Review failed for "+hunk.FilePath+".", err)
			}
			if usage != nil {
				sumPrompt += usage.PromptEvalCount
				sumCompletion += usage.EvalCount
				sumDuration += usage.EvalDurationNs
			}
			batch := findings.FilterAbstention(list, minKeep, minMaint)
			if tr.Enabled() {
				tr.Section("Post-filters")
				tr.Printf("Abstention: %d -> %d\n", len(list), len(batch))
			}
			if applyFP {
				beforeFP := len(batch)
				batch = findings.FilterFPKillList(batch)
				if tr.Enabled() {
					tr.Printf("FP kill list: %d -> %d\n", beforeFP, len(batch))
				}
			}
			findings.SetCursorURIs(opts.RepoRoot, batch)
			hunkCtx := truncateForPromptContext(hunk.RawContent, maxPromptContextStoreLen)
			for _, f := range batch {
				if f.ID != "" {
					findingPromptContext[f.ID] = hunkCtx
				}
				if opts.StreamOut != nil {
					tryWriteStreamLine(opts.StreamOut, map[string]interface{}{"type": "finding", "data": f})
				}
				collected = append(collected, f)
			}
		}
	}
	if opts.StreamOut != nil {
		tryWriteStreamLine(opts.StreamOut, map[string]string{"type": "done"})
	}
	// Auto-dismiss: previous findings in reviewed hunks not in collected are considered addressed.
	// applyAutoDismiss runs before updating s.Findings; it needs existing findings to compute addressed IDs.
	collectedIDSet := make(map[string]struct{}, len(collected))
	for _, f := range collected {
		if f.ID != "" {
			collectedIDSet[f.ID] = struct{}{}
		}
	}
	runConfig := history.NewRunConfigSnapshot(opts.Model, s.Strictness, opts.RAGSymbolMaxDefinitions, opts.RAGSymbolMaxTokens, opts.Nitpicky)
	if err := applyAutoDismiss(&s, part.ToReview, collectedIDSet, headSHA, opts.StateDir, runConfig); err != nil {
		return err
	}
	s.Findings = collected
	s.FindingPromptContext = findingPromptContext
	s.LastReviewedAt = headSHA
	if captureUsage() {
		s.LastRunPromptTokens = int64(sumPrompt)
		s.LastRunCompletionTokens = int64(sumCompletion)
		s.LastRunEvalDurationNs = sumDuration
	}
	if err := session.Save(opts.StateDir, &s); err != nil {
		return err
	}
	return nil
}

// Finish removes the session's worktree and releases the lock. Session file
// is left on disk (state persisted). If the worktree path no longer exists,
// Finish succeeds without error.
func Finish(ctx context.Context, opts FinishOptions) error {
	if opts.RepoRoot == "" || opts.StateDir == "" {
		return erruser.New("Finish failed: repository root and state directory are required.", nil)
	}

	s, err := session.Load(opts.StateDir)
	if err != nil {
		return err
	}
	if s.BaselineRef == "" {
		return ErrNoSession
	}

	release, err := session.AcquireLock(opts.StateDir)
	if err != nil {
		return err
	}
	defer release()

	path, err := git.PathForRef(opts.RepoRoot, opts.WorktreeRoot, s.BaselineRef)
	if err != nil {
		return erruser.New("Could not determine worktree path.", err)
	}

	if _, err := os.Stat(path); err == nil {
		if err := git.Remove(opts.RepoRoot, path); err != nil {
			return erruser.New("Could not remove review worktree.", err)
		}
	} else if !os.IsNotExist(err) {
		return erruser.New("Could not access worktree directory.", err)
	}

	headSHA, err := git.RevParse(opts.RepoRoot, "HEAD")
	if err != nil {
		return erruser.New("Could not resolve current commit (HEAD).", err)
	}
	baselineSHA, err := git.RevParse(opts.RepoRoot, s.BaselineRef)
	if err != nil {
		return erruser.New("Could not resolve baseline commit.", err)
	}
	// Scope fields default to zero when diff.Hunks fails; only assign when hErr == nil. All five CountHunkScope return values are used in notePayload below.
	var hunksReviewed, linesAdded, linesRemoved, charsAdded, charsDeleted, charsReviewed int
	if hunks, hErr := diff.Hunks(ctx, opts.RepoRoot, baselineSHA, headSHA, nil); hErr == nil {
		hunksReviewed = len(hunks)
		linesAdded, linesRemoved, charsAdded, charsDeleted, charsReviewed = diff.CountHunkScope(hunks)
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
		var runConfig *history.RunConfigSnapshot
		if cfg, err := config.Load(ctx, config.LoadOptions{RepoRoot: opts.RepoRoot}); err == nil {
			runConfig = history.NewRunConfigSnapshot(cfg.Model, cfg.Strictness, cfg.RAGSymbolMaxDefinitions, cfg.RAGSymbolMaxTokens, cfg.Nitpicky)
		}
		rec := history.Record{
			DiffRef:      diffRef,
			ReviewOutput: s.Findings,
			UserAction: history.UserAction{
				DismissedIDs: s.DismissedIDs,
				FinishedAt:   time.Now().UTC().Format(time.RFC3339),
			},
			RunConfig: runConfig,
		}
		if s.LastRunPromptTokens > 0 || s.LastRunCompletionTokens > 0 || s.LastRunEvalDurationNs > 0 {
			rec.PromptTokens = &s.LastRunPromptTokens
			rec.CompletionTokens = &s.LastRunCompletionTokens
			rec.EvalDurationNs = &s.LastRunEvalDurationNs
		}
		if err := history.Append(opts.StateDir, rec, history.DefaultMaxRecords); err != nil {
			return erruser.New("Could not record review history.", err)
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
		// Scope (zero when diff.Hunks failed)
		HunksReviewed  int    `json:"hunks_reviewed"`
		LinesAdded      int    `json:"lines_added"`
		LinesRemoved    int    `json:"lines_removed"`
		CharsAdded      int    `json:"chars_added"`
		CharsDeleted    int    `json:"chars_deleted"`
		CharsReviewed   int    `json:"chars_reviewed"`
	}{
		SessionID:       sessionID,
		BaselineSHA:     baselineSHA,
		HeadSHA:         headSHA,
		FindingsCount:   len(s.Findings),
		DismissalsCount: len(s.DismissedIDs),
		ToolVersion:     version.String(),
		FinishedAt:      time.Now().UTC().Format(time.RFC3339),
		// Scope (zero when diff.Hunks failed)
		HunksReviewed:  hunksReviewed,
		LinesAdded:      linesAdded,
		LinesRemoved:    linesRemoved,
		CharsAdded:      charsAdded,
		CharsDeleted:    charsDeleted,
		CharsReviewed:   charsReviewed,
	}
	noteJSON, err := json.Marshal(notePayload)
	if err != nil {
		return erruser.New("Could not write Git note for this review.", err)
	}
	if err := git.AddNote(opts.RepoRoot, git.NotesRefStet, headSHA, string(noteJSON)); err != nil {
		return erruser.New("Could not write Git note for this review.", err)
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
		return erruser.New("Run failed: repository root and state directory are required.", nil)
	}
	if opts.RAGSymbolMaxDefinitions < 0 {
		opts.RAGSymbolMaxDefinitions = 0
	}
	if opts.RAGSymbolMaxTokens < 0 {
		opts.RAGSymbolMaxTokens = 0
	}

	s, err := session.Load(opts.StateDir)
	if err != nil {
		return err
	}
	if s.BaselineRef == "" {
		return ErrNoSession
	}

	headSHA, err := git.RevParse(opts.RepoRoot, "HEAD")
	if err != nil {
		return erruser.New("Could not resolve current commit (HEAD).", err)
	}

	lastReviewedAt := s.LastReviewedAt
	if opts.ForceFullReview {
		lastReviewedAt = ""
	}
	part, err := scope.Partition(ctx, opts.RepoRoot, s.BaselineRef, headSHA, lastReviewedAt, nil)
	if err != nil {
		return erruser.New("Could not compute hunks to review.", err)
	}
	trRun := trace.New(opts.TraceOut)
	if trRun.Enabled() {
		trRun.Section("Partition")
		trRun.Printf("baseline=%s head=%s last_reviewed_at=%s\n", s.BaselineRef, headSHA, lastReviewedAt)
		approvedN := 0
		if part.Approved != nil {
			approvedN = len(part.Approved)
		}
		trRun.Printf("ToReview=%d Approved=%d\n", len(part.ToReview), approvedN)
		trRun.Section("AGENTS.md")
		trRun.Printf("AGENTS.md: not used by stet (only .cursor/rules/ used).\n")
	}
	toReview := part.ToReview
	skippedDismissed := 0
	if !opts.ForceFullReview && len(s.DismissedIDs) > 0 {
		toReview, skippedDismissed = filterHunksWithDismissedFindings(part.ToReview, &s, opts.ForceFullReview)
		if trRun.Enabled() && skippedDismissed > 0 {
			trRun.Printf("SkippedDismissed=%d\n", skippedDismissed)
		}
	}
	if opts.Verbose {
		if skippedDismissed > 0 {
			fmt.Fprintf(os.Stderr, "%d hunks to review, %d skipped (dismissed)\n", len(toReview), skippedDismissed)
		} else {
			fmt.Fprintf(os.Stderr, "%d hunks to review\n", len(toReview))
		}
	}

	if len(toReview) == 0 {
		if opts.StreamOut != nil {
			tryWriteStreamLine(opts.StreamOut, map[string]string{"type": "progress", "msg": "Nothing to review."})
			tryWriteStreamLine(opts.StreamOut, map[string]string{"type": "done"})
		}
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "Nothing to review.")
		}
		s.LastReviewedAt = headSHA
		return session.Save(opts.StateDir, &s)
	}

	var sumPrompt, sumCompletion int
	var sumDuration int64
	if s.FindingPromptContext == nil {
		s.FindingPromptContext = make(map[string]string)
	}
	minKeep, minMaint := opts.MinConfidenceKeep, opts.MinConfidenceMaintainability
	if minKeep == 0 && minMaint == 0 {
		minKeep, minMaint = findings.DefaultMinConfidenceKeep, findings.DefaultMinConfidenceMaintainability
	}
	applyFP := true
	if opts.Nitpicky {
		applyFP = false
	} else if opts.ApplyFPKillList != nil {
		applyFP = *opts.ApplyFPKillList
	}

	var newFindings []findings.Finding
	total := len(toReview)
	if opts.StreamOut != nil {
		tryWriteStreamLine(opts.StreamOut, map[string]interface{}{"type": "progress", "msg": fmt.Sprintf("%d hunks to review", total)})
	}

	if opts.DryRun {
		for i, hunk := range toReview {
			if opts.StreamOut != nil {
				tryWriteStreamLine(opts.StreamOut, map[string]interface{}{"type": "progress", "msg": fmt.Sprintf("Reviewing hunk %d/%d: %s", i+1, total, hunk.FilePath)})
			}
			batch := cannedFindingsForHunks([]diff.Hunk{hunk})
			batch = findings.FilterAbstention(batch, minKeep, minMaint)
			if applyFP {
				batch = findings.FilterFPKillList(batch)
			}
			findings.SetCursorURIs(opts.RepoRoot, batch)
			hunkCtx := truncateForPromptContext(hunk.RawContent, maxPromptContextStoreLen)
			for _, f := range batch {
				if f.ID != "" {
					s.FindingPromptContext[f.ID] = hunkCtx
				}
				if opts.StreamOut != nil {
					tryWriteStreamLine(opts.StreamOut, map[string]interface{}{"type": "finding", "data": f})
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
			return err
		}
		effectiveNumCtx := opts.NumCtx
		effectiveContextLimit := opts.ContextLimit
		// Show is called once upfront; model context length is stable for the run.
		showResult, showErr := client.Show(ctx, opts.Model)
		if showErr == nil && showResult != nil && showResult.ContextLength > 0 {
			if showResult.ContextLength > effectiveNumCtx {
				effectiveNumCtx = showResult.ContextLength
			}
			// Use model's larger context when Show returns a value greater than config.
			if effectiveNumCtx > effectiveContextLimit {
				effectiveContextLimit = effectiveNumCtx
			}
			if trRun.Enabled() && effectiveNumCtx > opts.NumCtx {
				trRun.Section("Context")
				trRun.Printf("using model context %d (config num_ctx=%d)\n", effectiveNumCtx, opts.NumCtx)
			}
		}
		// effectiveNumCtx is sent to Ollama; effectiveContextLimit is the single source of
		// truth for token warnings and ReviewHunk RAG budget. Both reflect the model's
		// actual context when Show succeeds; otherwise config values are used.
		branch, commitMsg, intentErr := git.UserIntent(opts.RepoRoot)
		if intentErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not retrieve Git intent (branch/commit): %v; using placeholder\n", intentErr)
		}
		userIntent := &prompt.UserIntent{Branch: branch, CommitMsg: commitMsg}
		// Token estimation: warn once if any hunk's prompt would exceed context threshold (Phase 3.2).
		if effectiveContextLimit > 0 && opts.WarnThreshold > 0 {
			systemPrompt, err := prompt.SystemPrompt(opts.StateDir)
			if err != nil {
				return err
			}
			systemPrompt = prompt.InjectUserIntent(systemPrompt, branch, commitMsg)
			if opts.Nitpicky {
				systemPrompt = prompt.AppendNitpickyInstructions(systemPrompt)
			}
			maxPromptTokens := 0
			for _, h := range toReview {
				userPrompt := prompt.UserPrompt(h)
				n := tokens.Estimate(systemPrompt + "\n" + userPrompt)
				if n > maxPromptTokens {
					maxPromptTokens = n
				}
			}
			if w := tokens.WarnIfOver(maxPromptTokens, tokens.DefaultResponseReserve, effectiveContextLimit, opts.WarnThreshold); w != "" {
				fmt.Fprintln(os.Stderr, w)
			}
		}
		genOpts := &ollama.GenerateOptions{Temperature: opts.Temperature, NumCtx: effectiveNumCtx}
		rulesLoader := rules.NewLoader(opts.RepoRoot)
		for i, hunk := range toReview {
			if opts.StreamOut != nil {
				tryWriteStreamLine(opts.StreamOut, map[string]interface{}{"type": "progress", "msg": fmt.Sprintf("Reviewing hunk %d/%d: %s", i+1, total, hunk.FilePath)})
			}
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "Reviewing hunk %d/%d: %s\n", i+1, total, hunk.FilePath)
			}
			if trRun.Enabled() {
				trRun.Section("Hunk " + fmt.Sprintf("%d/%d", i+1, total) + ": " + hunk.FilePath)
				trRun.Printf("strict_id=%s semantic_id=%s\n", hunkid.StrictHunkID(hunk.FilePath, hunk.RawContent), hunkid.SemanticHunkID(hunk.FilePath, hunk.RawContent))
			}
			cursorRules := rulesLoader.RulesForFile(hunk.FilePath)
			list, usage, err := review.ReviewHunk(ctx, client, opts.Model, opts.StateDir, hunk, genOpts, userIntent, cursorRules, opts.RepoRoot, effectiveContextLimit, opts.RAGSymbolMaxDefinitions, opts.RAGSymbolMaxTokens, runPromptShadows(&s), opts.Nitpicky, trRun)
			if err != nil {
				return erruser.New("Review failed for "+hunk.FilePath+".", err)
			}
			if usage != nil {
				sumPrompt += usage.PromptEvalCount
				sumCompletion += usage.EvalCount
				sumDuration += usage.EvalDurationNs
			}
			batch := findings.FilterAbstention(list, minKeep, minMaint)
			if trRun.Enabled() {
				trRun.Section("Post-filters")
				trRun.Printf("Abstention: %d -> %d\n", len(list), len(batch))
			}
			if applyFP {
				beforeFP := len(batch)
				batch = findings.FilterFPKillList(batch)
				if trRun.Enabled() {
					trRun.Printf("FP kill list: %d -> %d\n", beforeFP, len(batch))
				}
			}
			findings.SetCursorURIs(opts.RepoRoot, batch)
			hunkCtx := truncateForPromptContext(hunk.RawContent, maxPromptContextStoreLen)
			for _, f := range batch {
				if f.ID != "" {
					s.FindingPromptContext[f.ID] = hunkCtx
				}
				if opts.StreamOut != nil {
					tryWriteStreamLine(opts.StreamOut, map[string]interface{}{"type": "finding", "data": f})
				}
				newFindings = append(newFindings, f)
			}
		}
	}
	if opts.StreamOut != nil {
		tryWriteStreamLine(opts.StreamOut, map[string]string{"type": "done"})
	}
	if captureUsage() {
		s.LastRunPromptTokens = int64(sumPrompt)
		s.LastRunCompletionTokens = int64(sumCompletion)
		s.LastRunEvalDurationNs = sumDuration
	}

	// Replace: clear state. Merge: auto-dismiss then append; applyAutoDismiss uses existing s.Findings.
	if opts.ReplaceFindings {
		s.Findings = newFindings
		// Keep only prompt context for the new findings.
		newContext := make(map[string]string, len(newFindings))
		for _, f := range newFindings {
			if f.ID != "" && s.FindingPromptContext[f.ID] != "" {
				newContext[f.ID] = s.FindingPromptContext[f.ID]
			}
		}
		s.FindingPromptContext = newContext
		s.DismissedIDs = nil
		// Record replace run for audit trail.
		diffRef := headSHA
		if diffRef == "" {
			diffRef = s.BaselineRef
		}
		runConfig := history.NewRunConfigSnapshot(opts.Model, s.Strictness, opts.RAGSymbolMaxDefinitions, opts.RAGSymbolMaxTokens, opts.Nitpicky)
		rec := history.Record{
			DiffRef:      diffRef,
			ReviewOutput: newFindings,
			UserAction:   history.UserAction{ReplaceFindings: true},
			RunConfig:    runConfig,
		}
		if err := history.Append(opts.StateDir, rec, history.DefaultMaxRecords); err != nil {
			return erruser.New("Could not record review history.", err)
		}
	} else {
		// Auto-dismiss: findings in reviewed hunks that were not re-reported are considered addressed.
		newFindingIDSet := make(map[string]struct{}, len(newFindings))
		for _, f := range newFindings {
			if f.ID != "" {
				newFindingIDSet[f.ID] = struct{}{}
			}
		}
		runConfig := history.NewRunConfigSnapshot(opts.Model, s.Strictness, opts.RAGSymbolMaxDefinitions, opts.RAGSymbolMaxTokens, opts.Nitpicky)
		if err := applyAutoDismiss(&s, toReview, newFindingIDSet, headSHA, opts.StateDir, runConfig); err != nil {
			return err
		}
		s.Findings = append(s.Findings, newFindings...)
	}

	s.LastReviewedAt = headSHA
	if err := session.Save(opts.StateDir, &s); err != nil {
		return err
	}
	return nil
}
