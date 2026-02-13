package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"stet/cli/internal/config"
	"stet/cli/internal/findings"
	"stet/cli/internal/git"
	"stet/cli/internal/history"
	"stet/cli/internal/ollama"
	"stet/cli/internal/run"
	"stet/cli/internal/session"

	_ "stet/cli/internal/rag/go" // register Go resolver for RAG symbol lookup
)

// errExit is an error that carries an exit code for the CLI. Use errors.As to detect it.
type errExit int

func (e errExit) Error() string {
	return "exit " + strconv.Itoa(int(e))
}

// findingsOut is the writer for findings JSON on success. Tests may replace it to capture output.
var findingsOut io.Writer = os.Stdout

// errHintOut is the writer for recovery hints on start failure. Tests may replace it to capture output.
var errHintOut io.Writer = os.Stderr

// activeFindings loads session from stateDir and returns findings not in DismissedIDs.
func activeFindings(stateDir string) ([]findings.Finding, error) {
	s, err := session.Load(stateDir)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	dismissed := make(map[string]struct{}, len(s.DismissedIDs))
	for _, id := range s.DismissedIDs {
		dismissed[id] = struct{}{}
	}
	active := make([]findings.Finding, 0, len(s.Findings))
	for _, f := range s.Findings {
		if _, ok := dismissed[f.ID]; !ok {
			active = append(active, f)
		}
	}
	return active, nil
}

// writeFindingsJSON writes {"findings": [...]} to w. Uses activeFindings so dismissed are excluded.
func writeFindingsJSON(w io.Writer, stateDir string) error {
	active, err := activeFindings(stateDir)
	if err != nil {
		return fmt.Errorf("write findings: %w", err)
	}
	payload := struct {
		Findings []findings.Finding `json:"findings"`
	}{Findings: active}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("write findings: marshal: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write findings: %w", err)
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		return fmt.Errorf("write findings: %w", err)
	}
	return nil
}

// writeFindingsHuman writes a human-readable summary to w: one line per finding (file:line  severity  message), then a summary line.
func writeFindingsHuman(w io.Writer, stateDir string) error {
	active, err := activeFindings(stateDir)
	if err != nil {
		return fmt.Errorf("write findings: %w", err)
	}
	for _, f := range active {
		line := f.Line
		if f.Range != nil {
			line = f.Range.Start
		}
		if _, err := fmt.Fprintf(w, "%s:%d  %s  %s\n", f.File, line, f.Severity, f.Message); err != nil {
			return fmt.Errorf("write findings: %w", err)
		}
	}
	n := len(active)
	if n != 1 {
		if _, err := fmt.Fprintf(w, "%d finding(s).\n", n); err != nil {
			return fmt.Errorf("write findings: %w", err)
		}
	} else {
		if _, err := fmt.Fprintln(w, "1 finding."); err != nil {
			return fmt.Errorf("write findings: %w", err)
		}
	}
	return nil
}

// writeFindingsWithIDs writes one line per active finding: id  file:line  severity  message.
// Used by status --ids and list commands.
func writeFindingsWithIDs(w io.Writer, stateDir string) error {
	active, err := activeFindings(stateDir)
	if err != nil {
		return fmt.Errorf("write findings with IDs: %w", err)
	}
	for _, f := range active {
		line := f.Line
		if f.Range != nil {
			line = f.Range.Start
		}
		if _, err := fmt.Fprintf(w, "%s  %s:%d  %s  %s\n", f.ID, f.File, line, f.Severity, f.Message); err != nil {
			return fmt.Errorf("write findings with IDs: %w", err)
		}
	}
	return nil
}

func main() {
	os.Exit(Run())
}

// Run is the entry point for the CLI. It is exported for testing so that
// main.go can meet per-file coverage requirements.
func Run() int {
	return runCLI(os.Args[1:])
}

func runCLI(args []string) int {
	rootCmd := &cobra.Command{
		Use:   "stet",
		Short: "Local-first code review tool",
	}
	rootCmd.AddCommand(newStartCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newFinishCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newDismissCmd())
	rootCmd.AddCommand(newOptimizeCmd())
	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.SilenceUsage = true
	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		var exitErr errExit
		if errors.As(err, &exitErr) {
			return int(exitErr)
		}
		return 1
	}
	return 0
}

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [ref]",
		Short: "Start a review session at the given ref (default HEAD)",
		RunE:  runStart,
	}
	cmd.Flags().Bool("dry-run", false, "Skip LLM; inject canned findings for CI")
	cmd.Flags().BoolP("quiet", "q", false, "Suppress progress (use for scripts and IDE integration)")
	cmd.Flags().String("output", "human", "Output format: human (default) or json")
	cmd.Flags().Bool("json", false, "Emit findings as JSON to stdout (same as --output=json)")
	cmd.Flags().Bool("stream", false, "Emit progress and findings as NDJSON (one event per line); requires --output=json")
	cmd.Flags().Bool("allow-dirty", false, "Proceed with uncommitted changes (warns)")
	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	ref := "HEAD"
	if len(args) > 0 {
		ref = args[0]
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	quiet, _ := cmd.Flags().GetBool("quiet")
	output, _ := cmd.Flags().GetString("output")
	outputJSON, _ := cmd.Flags().GetBool("json")
	if outputJSON {
		output = "json"
	}
	if output != "human" && output != "json" {
		return fmt.Errorf("start: invalid output %q; use human or json", output)
	}
	stream, _ := cmd.Flags().GetBool("stream")
	if stream && output != "json" {
		return fmt.Errorf("start: --stream requires --output=json or --json")
	}
	verbose := !quiet
	if stream || output == "json" {
		verbose = false
	}
	allowDirty, _ := cmd.Flags().GetBool("allow-dirty")
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("start: not a git repository: %w", err)
	}
	cfg, err := config.Load(context.Background(), config.LoadOptions{RepoRoot: repoRoot})
	if err != nil {
		return fmt.Errorf("start: load config: %w", err)
	}
	stateDir := cfg.EffectiveStateDir(repoRoot)
	opts := run.StartOptions{
		RepoRoot:      repoRoot,
		StateDir:      stateDir,
		WorktreeRoot:  cfg.WorktreeRoot,
		Ref:           ref,
		DryRun:        dryRun,
		AllowDirty:    allowDirty,
		Model:         cfg.Model,
		OllamaBaseURL: cfg.OllamaBaseURL,
		ContextLimit:  cfg.ContextLimit,
		WarnThreshold: cfg.WarnThreshold,
		Timeout:       cfg.Timeout,
		Temperature:   cfg.Temperature,
		NumCtx:                  cfg.NumCtx,
		Verbose:                 verbose,
		StreamOut:               nil,
		RAGSymbolMaxDefinitions: cfg.RAGSymbolMaxDefinitions,
		RAGSymbolMaxTokens:      cfg.RAGSymbolMaxTokens,
	}
	if stream {
		opts.StreamOut = findingsOut
	}
	if err := run.Start(cmd.Context(), opts); err != nil {
		if errors.Is(err, ollama.ErrUnreachable) {
			fmt.Fprintf(os.Stderr, "Ollama unreachable at %s. Is the server running? For local: ollama serve.\n", cfg.OllamaBaseURL)
			fmt.Fprintf(os.Stderr, "Details: %v\n", err)
			return errExit(2)
		}
		if errors.Is(err, run.ErrDirtyWorktree) {
			fmt.Fprintf(errHintOut, "Hint: Commit or stash your changes, then run 'stet start %s' again.\n", ref)
			return err
		}
		if errors.Is(err, git.ErrWorktreeExists) {
			fmt.Fprintf(errHintOut, "Hint: Run 'stet finish' to end the current review and remove the worktree, then run 'stet start %s' again.\n", ref)
			return errors.New("worktree already exists")
		}
		if errors.Is(err, git.ErrBaselineNotAncestor) {
			return errors.New("baseline ref is not an ancestor of HEAD")
		}
		if errors.Is(err, session.ErrLocked) {
			fmt.Fprintf(errHintOut, "Hint: Run 'stet finish' to end the current review, then run 'stet start %s' again.\n", ref)
			return errors.New("finish or cleanup current review first")
		}
		return err
	}
	if stream {
		// Findings already emitted as NDJSON by run.Start
		return nil
	}
	if output == "json" {
		if err := writeFindingsJSON(findingsOut, stateDir); err != nil {
			return fmt.Errorf("start: %w", err)
		}
	} else {
		if err := writeFindingsHuman(findingsOut, stateDir); err != nil {
			return fmt.Errorf("start: %w", err)
		}
	}
	return nil
}

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run incremental review (only to-review hunks)",
		RunE:  runRun,
	}
	cmd.Flags().Bool("dry-run", false, "Skip LLM; inject canned findings for CI")
	cmd.Flags().BoolP("quiet", "q", false, "Suppress progress (use for scripts and IDE integration)")
	cmd.Flags().String("output", "human", "Output format: human (default) or json")
	cmd.Flags().Bool("json", false, "Emit findings as JSON to stdout (same as --output=json)")
	cmd.Flags().Bool("stream", false, "Emit progress and findings as NDJSON (one event per line); requires --output=json")
	return cmd
}

func runRun(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("run: not a git repository: %w", err)
	}
	cfg, err := config.Load(cmd.Context(), config.LoadOptions{RepoRoot: repoRoot})
	if err != nil {
		return fmt.Errorf("run: load config: %w", err)
	}
	stateDir := cfg.EffectiveStateDir(repoRoot)
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	quiet, _ := cmd.Flags().GetBool("quiet")
	output, _ := cmd.Flags().GetString("output")
	outputJSON, _ := cmd.Flags().GetBool("json")
	if outputJSON {
		output = "json"
	}
	if output != "human" && output != "json" {
		return fmt.Errorf("run: invalid output %q; use human or json", output)
	}
	stream, _ := cmd.Flags().GetBool("stream")
	if stream && output != "json" {
		return fmt.Errorf("run: --stream requires --output=json or --json")
	}
	verbose := !quiet
	if stream || output == "json" {
		verbose = false
	}
	opts := run.RunOptions{
		RepoRoot:      repoRoot,
		StateDir:      stateDir,
		DryRun:        dryRun,
		Model:         cfg.Model,
		OllamaBaseURL: cfg.OllamaBaseURL,
		ContextLimit:  cfg.ContextLimit,
		WarnThreshold: cfg.WarnThreshold,
		Timeout:       cfg.Timeout,
		Temperature:   cfg.Temperature,
		NumCtx:                  cfg.NumCtx,
		Verbose:                 verbose,
		StreamOut:               nil,
		RAGSymbolMaxDefinitions: cfg.RAGSymbolMaxDefinitions,
		RAGSymbolMaxTokens:      cfg.RAGSymbolMaxTokens,
	}
	if stream {
		opts.StreamOut = findingsOut
	}
	if err := run.Run(cmd.Context(), opts); err != nil {
		if errors.Is(err, run.ErrNoSession) {
			fmt.Fprintln(os.Stderr, err.Error())
			return errExit(1)
		}
		if errors.Is(err, ollama.ErrUnreachable) {
			fmt.Fprintf(os.Stderr, "Ollama unreachable at %s. Is the server running? For local: ollama serve.\n", cfg.OllamaBaseURL)
			fmt.Fprintf(os.Stderr, "Details: %v\n", err)
			return errExit(2)
		}
		return err
	}
	if stream {
		// Findings already emitted as NDJSON by run.Run
		return nil
	}
	if output == "json" {
		if err := writeFindingsJSON(findingsOut, stateDir); err != nil {
			return fmt.Errorf("run: %w", err)
		}
	} else {
		if err := writeFindingsHuman(findingsOut, stateDir); err != nil {
			return fmt.Errorf("run: %w", err)
		}
	}
	return nil
}

func newFinishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finish",
		Short: "Finish the review session and remove the worktree",
		RunE:  runFinish,
	}
	return cmd
}

func runFinish(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("finish: %w", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("finish: not a git repository: %w", err)
	}
	cfg, err := config.Load(context.Background(), config.LoadOptions{RepoRoot: repoRoot})
	if err != nil {
		return fmt.Errorf("finish: load config: %w", err)
	}
	stateDir := cfg.EffectiveStateDir(repoRoot)
	opts := run.FinishOptions{
		RepoRoot:     repoRoot,
		StateDir:     stateDir,
		WorktreeRoot: cfg.WorktreeRoot,
	}
	if err := run.Finish(cmd.Context(), opts); err != nil {
		if errors.Is(err, run.ErrNoSession) {
			fmt.Fprintln(os.Stderr, err.Error())
			return errExit(1)
		}
		return err
	}
	return nil
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report session status (baseline, findings, dismissed)",
		RunE:  runStatus,
	}
	cmd.Flags().BoolP("ids", "i", false, "List active finding IDs (for stet dismiss)")
	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("status: not a git repository: %w", err)
	}
	cfg, err := config.Load(context.Background(), config.LoadOptions{RepoRoot: repoRoot})
	if err != nil {
		return fmt.Errorf("status: load config: %w", err)
	}
	stateDir := cfg.EffectiveStateDir(repoRoot)
	s, err := session.Load(stateDir)
	if err != nil {
		return fmt.Errorf("status: load session: %w", err)
	}
	if s.BaselineRef == "" {
		fmt.Fprintln(os.Stderr, "No active session. Run 'stet start' to begin a review.")
		return errExit(1)
	}
	worktreePath, err := git.PathForRef(repoRoot, cfg.WorktreeRoot, s.BaselineRef)
	if err != nil {
		return fmt.Errorf("status: worktree path: %w", err)
	}
	fmt.Fprintf(os.Stdout, "baseline: %s\n", s.BaselineRef)
	fmt.Fprintf(os.Stdout, "last_reviewed_at: %s\n", s.LastReviewedAt)
	fmt.Fprintf(os.Stdout, "worktree: %s\n", worktreePath)
	fmt.Fprintf(os.Stdout, "findings: %d\n", len(s.Findings))
	fmt.Fprintf(os.Stdout, "dismissed: %d\n", len(s.DismissedIDs))
	showIDs, _ := cmd.Flags().GetBool("ids")
	if showIDs {
		active, err := activeFindings(stateDir)
		if err != nil {
			return fmt.Errorf("status: %w", err)
		}
		if len(active) > 0 {
			fmt.Fprintln(os.Stdout, "---")
			if err := writeFindingsWithIDs(os.Stdout, stateDir); err != nil {
				return fmt.Errorf("status: %w", err)
			}
		}
	}
	return nil
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active findings with IDs (for stet dismiss)",
		RunE:  runList,
	}
	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("list: not a git repository: %w", err)
	}
	cfg, err := config.Load(context.Background(), config.LoadOptions{RepoRoot: repoRoot})
	if err != nil {
		return fmt.Errorf("list: load config: %w", err)
	}
	stateDir := cfg.EffectiveStateDir(repoRoot)
	s, err := session.Load(stateDir)
	if err != nil {
		return fmt.Errorf("list: load session: %w", err)
	}
	if s.BaselineRef == "" {
		fmt.Fprintln(os.Stderr, "No active session. Run 'stet start' to begin a review.")
		return errExit(1)
	}
	return writeFindingsWithIDs(os.Stdout, stateDir)
}

func newDismissCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dismiss <id> [reason]",
		Short: "Mark a finding as dismissed so it does not resurface",
		RunE:  runDismiss,
	}
	return cmd
}

// runDismiss adds the finding id to the session's dismissed list so it does not resurface.
// Optional second argument is a dismissal reason (false_positive, already_correct, wrong_suggestion, out_of_scope)
// for the optimizer. Phase 4.5: appends to .review/history.jsonl on dismiss.
func runDismiss(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("dismiss requires a finding id (e.g. stet dismiss <id>)")
	}
	id := strings.TrimSpace(args[0])
	if id == "" {
		return errors.New("dismiss requires a non-empty finding id")
	}
	var reason string
	if len(args) >= 2 {
		reason = strings.TrimSpace(strings.ToLower(args[1]))
		if reason != "" && !history.ValidReason(reason) {
			return fmt.Errorf("dismiss: invalid reason %q; must be one of: false_positive, already_correct, wrong_suggestion, out_of_scope", reason)
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("dismiss: %w", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("dismiss: not a git repository: %w", err)
	}
	cfg, err := config.Load(context.Background(), config.LoadOptions{RepoRoot: repoRoot})
	if err != nil {
		return fmt.Errorf("dismiss: load config: %w", err)
	}
	stateDir := cfg.EffectiveStateDir(repoRoot)
	s, err := session.Load(stateDir)
	if err != nil {
		return fmt.Errorf("dismiss: load session: %w", err)
	}
	if s.BaselineRef == "" {
		fmt.Fprintln(os.Stderr, run.ErrNoSession.Error())
		return errExit(1)
	}
	alreadyDismissed := false
	for _, d := range s.DismissedIDs {
		if d == id {
			alreadyDismissed = true
			break
		}
	}
	if !alreadyDismissed {
		s.DismissedIDs = append(s.DismissedIDs, id)
		if err := session.Save(stateDir, &s); err != nil {
			return fmt.Errorf("dismiss: save session: %w", err)
		}
	}
	diffRef := s.LastReviewedAt
	if diffRef == "" {
		diffRef = s.BaselineRef
	}
	ua := history.UserAction{DismissedIDs: []string{id}}
	if reason != "" {
		ua.Dismissals = []history.Dismissal{{FindingID: id, Reason: reason}}
	}
	rec := history.Record{
		DiffRef:      diffRef,
		ReviewOutput: s.Findings,
		UserAction:   ua,
	}
	if err := history.Append(stateDir, rec, history.DefaultMaxRecords); err != nil {
		return fmt.Errorf("dismiss: append history: %w", err)
	}
	return nil
}

func newOptimizeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "optimize",
		Short: "Run optional DSPy optimizer (reads history.jsonl, writes system_prompt_optimized.txt)",
		RunE:  runOptimize,
	}
	return cmd
}

func runOptimize(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("optimize: %w", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("optimize: not a git repository: %w", err)
	}
	cfg, err := config.Load(context.Background(), config.LoadOptions{RepoRoot: repoRoot})
	if err != nil {
		return fmt.Errorf("optimize: load config: %w", err)
	}
	if cfg.OptimizerScript == "" {
		fmt.Fprintln(os.Stderr, "Optimizer not configured. Set STET_OPTIMIZER_SCRIPT or optimizer_script in config. See docs.")
		return errExit(1)
	}
	stateDir := cfg.EffectiveStateDir(repoRoot)
	parts := strings.Fields(cfg.OptimizerScript)
	if len(parts) == 0 {
		fmt.Fprintln(os.Stderr, "Optimizer not configured. Set STET_OPTIMIZER_SCRIPT or optimizer_script in config. See docs.")
		return errExit(1)
	}
	var execCmd *exec.Cmd
	if len(parts) == 1 {
		execCmd = exec.CommandContext(cmd.Context(), parts[0])
	} else {
		execCmd = exec.CommandContext(cmd.Context(), parts[0], parts[1:]...)
	}
	execCmd.Env = append(os.Environ(), "STET_STATE_DIR="+stateDir)
	execCmd.Dir = repoRoot
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	if err := execCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			if code >= 0 && code <= 255 {
				return errExit(code)
			}
		}
		return errExit(1)
	}
	return nil
}

func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Verify environment (Ollama, Git, models)",
		RunE:  runDoctor,
	}
	return cmd
}

func runDoctor(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("doctor: %w", err)
	}
	repoRoot := ""
	if r, e := git.RepoRoot(cwd); e == nil {
		repoRoot = r
	}
	cfg, err := config.Load(cmd.Context(), config.LoadOptions{RepoRoot: repoRoot})
	if err != nil {
		return fmt.Errorf("doctor: load config: %w", err)
	}
	client := ollama.NewClient(cfg.OllamaBaseURL, nil)
	result, err := client.Check(cmd.Context(), cfg.Model)
	if err != nil {
		if errors.Is(err, ollama.ErrUnreachable) {
			fmt.Fprintf(os.Stderr, "Ollama unreachable at %s. Is the server running? For local: ollama serve.\n", cfg.OllamaBaseURL)
			fmt.Fprintf(os.Stderr, "Details: %v\n", err)
			return errExit(2)
		}
		fmt.Fprintln(os.Stderr, err.Error())
		return errExit(1)
	}
	if !result.ModelPresent {
		fmt.Fprintf(os.Stderr, "Model %q not found. Pull it with: ollama pull %s\n", cfg.Model, cfg.Model)
		return errExit(1)
	}
	fmt.Fprintln(os.Stdout, "Ollama OK")
	return nil
}
