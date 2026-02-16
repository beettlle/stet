package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"stet/cli/internal/config"
	"stet/cli/internal/erruser"
	"stet/cli/internal/findings"
	"stet/cli/internal/git"
	"stet/cli/internal/history"
	"stet/cli/internal/ollama"
	"stet/cli/internal/run"
	"stet/cli/internal/session"
	"stet/cli/internal/version"

	_ "stet/cli/internal/rag/go"     // register Go resolver for RAG symbol lookup
	_ "stet/cli/internal/rag/java"   // register Java resolver
	_ "stet/cli/internal/rag/js"     // register JavaScript/TypeScript resolver
	_ "stet/cli/internal/rag/python" // register Python resolver
	_ "stet/cli/internal/rag/swift"  // register Swift resolver
)

// errExit is an error that carries an exit code for the CLI. Use errors.As to detect it.
type errExit int

func (e errExit) Error() string {
	return "exit " + strconv.Itoa(int(e))
}

// getFindingsOut returns the writer for findings JSON on success. Tests may replace it to capture output.
var defaultGetFindingsOut = func() io.Writer { return os.Stdout }
var getFindingsOut = defaultGetFindingsOut

// findingsWriter returns the writer for findings output, or os.Stdout if getFindingsOut() returns nil.
// It never returns nil; callers may assume a non-nil writer.
func findingsWriter() io.Writer {
	w := getFindingsOut()
	if w == nil {
		return os.Stdout
	}
	return w
}

// errHintOut is the writer for recovery hints on start failure. Tests may replace it to capture output.
var errHintOut io.Writer = os.Stderr

// activeFindings loads session from stateDir and returns findings not in DismissedIDs.
func activeFindings(stateDir string) ([]findings.Finding, error) {
	s, err := session.Load(stateDir)
	if err != nil {
		return nil, erruser.New("Could not load session.", err)
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
		return err
	}
	payload := struct {
		Findings []findings.Finding `json:"findings"`
	}{Findings: active}
	data, err := json.Marshal(payload)
	if err != nil {
		return erruser.New("Could not write findings.", err)
	}
	if _, err := w.Write(data); err != nil {
		return erruser.New("Could not write findings.", err)
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		return erruser.New("Could not write findings.", err)
	}
	return nil
}

// writeFindingsHuman writes a human-readable summary to w: one line per finding (file:line  severity  message), then a summary line.
func writeFindingsHuman(w io.Writer, stateDir string) error {
	active, err := activeFindings(stateDir)
	if err != nil {
		return err
	}
	for _, f := range active {
		line := f.Line
		if f.Range != nil {
			line = f.Range.Start
		}
		if _, err := fmt.Fprintf(w, "%s:%d  %s  %s\n", f.File, line, f.Severity, f.Message); err != nil {
			return erruser.New("Could not write findings.", err)
		}
	}
	n := len(active)
	if n != 1 {
		if _, err := fmt.Fprintf(w, "%d finding(s).\n", n); err != nil {
			return erruser.New("Could not write findings.", err)
		}
	} else {
		if _, err := fmt.Fprintln(w, "1 finding."); err != nil {
			return erruser.New("Could not write findings.", err)
		}
	}
	return nil
}

// writeFindingsWithIDs writes one line per active finding: id  file:line  severity  message.
// Used by status --ids and list commands.
func writeFindingsWithIDs(w io.Writer, stateDir string) error {
	active, err := activeFindings(stateDir)
	if err != nil {
		return err
	}
	for _, f := range active {
		line := f.Line
		if f.Range != nil {
			line = f.Range.Start
		}
		if _, err := fmt.Fprintf(w, "%s  %s:%d  %s  %s\n", findings.ShortID(f.ID), f.File, line, f.Severity, f.Message); err != nil {
			return erruser.New("Could not write findings.", err)
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
		Use:     "stet",
		Short:   "Local-first code review tool",
		Version: version.String(),
	}
	rootCmd.AddCommand(newStartCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newRerunCmd())
	rootCmd.AddCommand(newFinishCmd())
	rootCmd.AddCommand(newCleanupCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newDismissCmd())
	rootCmd.AddCommand(newOptimizeCmd())
	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		var exitErr errExit
		if errors.As(err, &exitErr) {
			return int(exitErr)
		}
		fmt.Fprintln(os.Stderr, err)
		if u := errors.Unwrap(err); u != nil {
			fmt.Fprintf(os.Stderr, "Details: %v\n", u)
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
	cmd.Flags().Int("rag-symbol-max-definitions", 0, "Max symbol definitions to inject (0 = use config); overrides config and env")
	cmd.Flags().Int("rag-symbol-max-tokens", 0, "Max tokens for symbol-definitions block (0 = use config); overrides config and env")
	cmd.Flags().String("strictness", "", "Review strictness preset: strict, default, lenient, strict+, default+, lenient+ (overrides config and env)")
	cmd.Flags().Bool("nitpicky", false, "Enable nitpicky mode: report typos, grammar, style, and convention violations; do not filter those findings")
	cmd.Flags().Bool("trace", false, "Print internal steps to stderr (partition, rules, RAG, prompts, LLM I/O)")
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
		return errors.New("Invalid output format; use human or json.")
	}
	stream, _ := cmd.Flags().GetBool("stream")
	if stream && output != "json" {
		return errors.New("--stream requires --output=json or --json.")
	}
	verbose := !quiet
	if stream || output == "json" {
		verbose = false
	}
	allowDirty, _ := cmd.Flags().GetBool("allow-dirty")
	trace, _ := cmd.Flags().GetBool("trace")
	var traceOut io.Writer
	if trace {
		traceOut = os.Stderr
	}
	cwd, err := os.Getwd()
	if err != nil {
		return erruser.New("Could not determine current directory.", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return err
	}
	overrides := overridesFromFlags(cmd)
	if overrides != nil && overrides.Strictness != nil && *overrides.Strictness != "" {
		if _, _, _, err := findings.ResolveStrictness(*overrides.Strictness); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return errExit(1)
		}
	}
	cfg, err := config.Load(context.Background(), config.LoadOptions{RepoRoot: repoRoot, Overrides: overrides})
	if err != nil {
		return err
	}
	minKeep, minMaint, applyFP, err := findings.ResolveStrictness(cfg.Strictness)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return errExit(1)
	}
	if cfg.Nitpicky {
		applyFP = false
	}
	stateDir := cfg.EffectiveStateDir(repoRoot)
	var persistStrictness *string
	if overrides != nil && overrides.Strictness != nil && *overrides.Strictness != "" {
		persistStrictness = &cfg.Strictness
	}
	var persistRAGDefs *int
	if overrides != nil && overrides.RAGSymbolMaxDefinitions != nil {
		persistRAGDefs = overrides.RAGSymbolMaxDefinitions
	}
	var persistRAGTokens *int
	if overrides != nil && overrides.RAGSymbolMaxTokens != nil {
		persistRAGTokens = overrides.RAGSymbolMaxTokens
	}
	var persistNitpicky *bool
	if overrides != nil && overrides.Nitpicky != nil {
		persistNitpicky = &cfg.Nitpicky
	}
	opts := run.StartOptions{
		RepoRoot:                       repoRoot,
		StateDir:                       stateDir,
		WorktreeRoot:                   cfg.WorktreeRoot,
		Ref:                            ref,
		DryRun:                         dryRun,
		AllowDirty:                     allowDirty,
		Model:                          cfg.Model,
		OllamaBaseURL:                  cfg.OllamaBaseURL,
		ContextLimit:                   cfg.ContextLimit,
		WarnThreshold:                  cfg.WarnThreshold,
		Timeout:                        cfg.Timeout,
		Temperature:                    cfg.Temperature,
		NumCtx:                         cfg.NumCtx,
		Verbose:                        verbose,
		StreamOut:                      nil,
		RAGSymbolMaxDefinitions:        cfg.RAGSymbolMaxDefinitions,
		RAGSymbolMaxTokens:             cfg.RAGSymbolMaxTokens,
		MinConfidenceKeep:              minKeep,
		MinConfidenceMaintainability:   minMaint,
		ApplyFPKillList:                &applyFP,
		Nitpicky:                       cfg.Nitpicky,
		PersistStrictness:              persistStrictness,
		PersistRAGSymbolMaxDefinitions: persistRAGDefs,
		PersistRAGSymbolMaxTokens:      persistRAGTokens,
		PersistNitpicky:                persistNitpicky,
		TraceOut:                       traceOut,
	}
	if stream {
		opts.StreamOut = findingsWriter()
	}
	if err := run.Start(cmd.Context(), opts); err != nil {
		if errors.Is(err, ollama.ErrUnreachable) {
			fmt.Fprintf(os.Stderr, "Ollama unreachable at %s. Is the server running? For local: ollama serve.\n", cfg.OllamaBaseURL)
			fmt.Fprintf(os.Stderr, "Details: %v\n", err)
			return errExit(2)
		}
		if errors.Is(err, ollama.ErrBadRequest) {
			fmt.Fprintf(os.Stderr, "Ollama bad request at %s. %v\n", cfg.OllamaBaseURL, err)
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
	w := findingsWriter()
	if output == "json" {
		if err := writeFindingsJSON(w, stateDir); err != nil {
			return err
		}
	} else {
		if err := writeFindingsHuman(w, stateDir); err != nil {
			return err
		}
	}
	return nil
}

// addRunLikeFlags registers the flags shared by run and rerun (dry-run, quiet, output, json, stream, rag-symbol-*, strictness, nitpicky, trace).
func addRunLikeFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("dry-run", false, "Skip LLM; inject canned findings for CI")
	cmd.Flags().BoolP("quiet", "q", false, "Suppress progress (use for scripts and IDE integration)")
	cmd.Flags().String("output", "human", "Output format: human (default) or json")
	cmd.Flags().Bool("json", false, "Emit findings as JSON to stdout (same as --output=json)")
	cmd.Flags().Bool("stream", false, "Emit progress and findings as NDJSON (one event per line); requires --output=json")
	cmd.Flags().Int("rag-symbol-max-definitions", 0, "Max symbol definitions to inject (0 = use config); overrides config and env")
	cmd.Flags().Int("rag-symbol-max-tokens", 0, "Max tokens for symbol-definitions block (0 = use config); overrides config and env")
	cmd.Flags().String("strictness", "", "Review strictness preset: strict, default, lenient, strict+, default+, lenient+ (overrides config and env)")
	cmd.Flags().Bool("nitpicky", false, "Enable nitpicky mode: report typos, grammar, style, and convention violations; do not filter those findings")
	cmd.Flags().Bool("trace", false, "Print internal steps to stderr (partition, rules, RAG, prompts, LLM I/O)")
}

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run incremental review (only to-review hunks)",
		RunE:  runRun,
	}
	addRunLikeFlags(cmd)
	return cmd
}

// overridesFromFlags returns Overrides for RAG, strictness, and nitpicky when the corresponding flags were set (start/run both define these flags).
func overridesFromFlags(cmd *cobra.Command) *config.Overrides {
	defChanged := cmd.Flags().Lookup("rag-symbol-max-definitions") != nil && cmd.Flags().Lookup("rag-symbol-max-definitions").Changed
	tokChanged := cmd.Flags().Lookup("rag-symbol-max-tokens") != nil && cmd.Flags().Lookup("rag-symbol-max-tokens").Changed
	strictnessChanged := cmd.Flags().Lookup("strictness") != nil && cmd.Flags().Lookup("strictness").Changed
	nitpickyChanged := cmd.Flags().Lookup("nitpicky") != nil && cmd.Flags().Lookup("nitpicky").Changed
	if !defChanged && !tokChanged && !strictnessChanged && !nitpickyChanged {
		return nil
	}
	o := &config.Overrides{}
	if defChanged {
		v, _ := cmd.Flags().GetInt("rag-symbol-max-definitions")
		o.RAGSymbolMaxDefinitions = &v
	}
	if tokChanged {
		v, _ := cmd.Flags().GetInt("rag-symbol-max-tokens")
		o.RAGSymbolMaxTokens = &v
	}
	if strictnessChanged {
		v, _ := cmd.Flags().GetString("strictness")
		o.Strictness = &v
	}
	if nitpickyChanged {
		v, _ := cmd.Flags().GetBool("nitpicky")
		o.Nitpicky = &v
	}
	return o
}

func runRun(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return erruser.New("Could not determine current directory.", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return err
	}
	overrides := overridesFromFlags(cmd)
	if overrides != nil && overrides.Strictness != nil && *overrides.Strictness != "" {
		if _, _, _, err := findings.ResolveStrictness(*overrides.Strictness); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return errExit(1)
		}
	}
	cfg, err := config.Load(cmd.Context(), config.LoadOptions{RepoRoot: repoRoot, Overrides: overrides})
	if err != nil {
		return err
	}
	stateDir := cfg.EffectiveStateDir(repoRoot)
	s, err := session.Load(stateDir)
	if err != nil {
		return err
	}
	// Effective options: flag override > session (from start) > config/env/default.
	effectiveStrictness := cfg.Strictness
	if overrides != nil && overrides.Strictness != nil && *overrides.Strictness != "" {
		effectiveStrictness = *overrides.Strictness
	} else if s.Strictness != "" {
		effectiveStrictness = s.Strictness
	}
	effectiveRAGDefs := cfg.RAGSymbolMaxDefinitions
	if overrides != nil && overrides.RAGSymbolMaxDefinitions != nil {
		effectiveRAGDefs = *overrides.RAGSymbolMaxDefinitions
	} else if s.RAGSymbolMaxDefinitions != nil {
		effectiveRAGDefs = *s.RAGSymbolMaxDefinitions
	}
	effectiveRAGTokens := cfg.RAGSymbolMaxTokens
	if overrides != nil && overrides.RAGSymbolMaxTokens != nil {
		effectiveRAGTokens = *overrides.RAGSymbolMaxTokens
	} else if s.RAGSymbolMaxTokens != nil {
		effectiveRAGTokens = *s.RAGSymbolMaxTokens
	}
	effectiveNitpicky := cfg.Nitpicky
	if overrides != nil && overrides.Nitpicky != nil {
		effectiveNitpicky = *overrides.Nitpicky
	} else if s.Nitpicky != nil {
		effectiveNitpicky = *s.Nitpicky
	}
	minKeep, minMaint, applyFP, err := findings.ResolveStrictness(effectiveStrictness)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return errExit(1)
	}
	if effectiveNitpicky {
		applyFP = false
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	quiet, _ := cmd.Flags().GetBool("quiet")
	output, _ := cmd.Flags().GetString("output")
	outputJSON, _ := cmd.Flags().GetBool("json")
	if outputJSON {
		output = "json"
	}
	if output != "human" && output != "json" {
		return errors.New("Invalid output format; use human or json.")
	}
	stream, _ := cmd.Flags().GetBool("stream")
	if stream && output != "json" {
		return errors.New("--stream requires --output=json or --json.")
	}
	verbose := !quiet
	if stream || output == "json" {
		verbose = false
	}
	trace, _ := cmd.Flags().GetBool("trace")
	var traceOut io.Writer
	if trace {
		traceOut = os.Stderr
	}
	opts := run.RunOptions{
		RepoRoot:                     repoRoot,
		StateDir:                     stateDir,
		DryRun:                       dryRun,
		Model:                        cfg.Model,
		OllamaBaseURL:                cfg.OllamaBaseURL,
		ContextLimit:                 cfg.ContextLimit,
		WarnThreshold:                cfg.WarnThreshold,
		Timeout:                      cfg.Timeout,
		Temperature:                  cfg.Temperature,
		NumCtx:                       cfg.NumCtx,
		Verbose:                      verbose,
		StreamOut:                    nil,
		RAGSymbolMaxDefinitions:      effectiveRAGDefs,
		RAGSymbolMaxTokens:           effectiveRAGTokens,
		MinConfidenceKeep:            minKeep,
		MinConfidenceMaintainability: minMaint,
		ApplyFPKillList:              &applyFP,
		Nitpicky:                     effectiveNitpicky,
		TraceOut:                     traceOut,
	}
	if stream {
		opts.StreamOut = findingsWriter()
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
		if errors.Is(err, ollama.ErrBadRequest) {
			fmt.Fprintf(os.Stderr, "Ollama bad request at %s. %v\n", cfg.OllamaBaseURL, err)
			return errExit(2)
		}
		return err
	}
	if stream {
		// Findings already emitted as NDJSON by run.Run
		return nil
	}
	w := findingsWriter()
	if output == "json" {
		if err := writeFindingsJSON(w, stateDir); err != nil {
			return err
		}
	} else {
		if err := writeFindingsHuman(w, stateDir); err != nil {
			return err
		}
	}
	return nil
}

func newRerunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rerun",
		Short: "Re-run full review (all hunks) with current or overridden parameters",
		Long: `Re-run a full review over all hunks (not just incremental). Requires an active session; run stet start first.

Use --replace to replace session findings with only this run's results (default is to merge new findings with existing).
Other flags (--dry-run, --json, --stream, --rag-symbol-*, --strictness, --nitpicky, --trace) behave like stet run.`,
		RunE: runRerun,
	}
	addRunLikeFlags(cmd)
	cmd.Flags().Bool("replace", false, "Replace session findings with only this run's results; default is to merge new findings with existing")
	return cmd
}

func runRerun(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return erruser.New("Could not determine current directory.", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return err
	}
	overrides := overridesFromFlags(cmd)
	if overrides != nil && overrides.Strictness != nil && *overrides.Strictness != "" {
		if _, _, _, err := findings.ResolveStrictness(*overrides.Strictness); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return errExit(1)
		}
	}
	cfg, err := config.Load(cmd.Context(), config.LoadOptions{RepoRoot: repoRoot, Overrides: overrides})
	if err != nil {
		return err
	}
	stateDir := cfg.EffectiveStateDir(repoRoot)
	s, err := session.Load(stateDir)
	if err != nil {
		return err
	}
	effectiveStrictness := cfg.Strictness
	if overrides != nil && overrides.Strictness != nil && *overrides.Strictness != "" {
		effectiveStrictness = *overrides.Strictness
	} else if s.Strictness != "" {
		effectiveStrictness = s.Strictness
	}
	effectiveRAGDefs := cfg.RAGSymbolMaxDefinitions
	if overrides != nil && overrides.RAGSymbolMaxDefinitions != nil {
		effectiveRAGDefs = *overrides.RAGSymbolMaxDefinitions
	} else if s.RAGSymbolMaxDefinitions != nil {
		effectiveRAGDefs = *s.RAGSymbolMaxDefinitions
	}
	effectiveRAGTokens := cfg.RAGSymbolMaxTokens
	if overrides != nil && overrides.RAGSymbolMaxTokens != nil {
		effectiveRAGTokens = *overrides.RAGSymbolMaxTokens
	} else if s.RAGSymbolMaxTokens != nil {
		effectiveRAGTokens = *s.RAGSymbolMaxTokens
	}
	effectiveNitpicky := cfg.Nitpicky
	if overrides != nil && overrides.Nitpicky != nil {
		effectiveNitpicky = *overrides.Nitpicky
	} else if s.Nitpicky != nil {
		effectiveNitpicky = *s.Nitpicky
	}
	minKeep, minMaint, applyFP, err := findings.ResolveStrictness(effectiveStrictness)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return errExit(1)
	}
	if effectiveNitpicky {
		applyFP = false
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	quiet, _ := cmd.Flags().GetBool("quiet")
	output, _ := cmd.Flags().GetString("output")
	outputJSON, _ := cmd.Flags().GetBool("json")
	if outputJSON {
		output = "json"
	}
	if output != "human" && output != "json" {
		return errors.New("Invalid output format; use human or json.")
	}
	stream, _ := cmd.Flags().GetBool("stream")
	if stream && output != "json" {
		return errors.New("--stream requires --output=json or --json.")
	}
	verbose := !quiet
	if stream || output == "json" {
		verbose = false
	}
	trace, _ := cmd.Flags().GetBool("trace")
	var traceOut io.Writer
	if trace {
		traceOut = os.Stderr
	}
	replace, _ := cmd.Flags().GetBool("replace")
	opts := run.RunOptions{
		RepoRoot:                     repoRoot,
		StateDir:                     stateDir,
		DryRun:                       dryRun,
		Model:                        cfg.Model,
		OllamaBaseURL:                cfg.OllamaBaseURL,
		ContextLimit:                 cfg.ContextLimit,
		WarnThreshold:                cfg.WarnThreshold,
		Timeout:                      cfg.Timeout,
		Temperature:                  cfg.Temperature,
		NumCtx:                       cfg.NumCtx,
		Verbose:                      verbose,
		StreamOut:                    nil,
		RAGSymbolMaxDefinitions:      effectiveRAGDefs,
		RAGSymbolMaxTokens:           effectiveRAGTokens,
		MinConfidenceKeep:            minKeep,
		MinConfidenceMaintainability: minMaint,
		ApplyFPKillList:              &applyFP,
		Nitpicky:                     effectiveNitpicky,
		TraceOut:                     traceOut,
		ForceFullReview:             true,
		ReplaceFindings:             replace,
	}
	if stream {
		opts.StreamOut = findingsWriter()
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
		if errors.Is(err, ollama.ErrBadRequest) {
			fmt.Fprintf(os.Stderr, "Ollama bad request at %s. %v\n", cfg.OllamaBaseURL, err)
			return errExit(2)
		}
		return err
	}
	if stream {
		return nil
	}
	w := findingsWriter()
	if output == "json" {
		if err := writeFindingsJSON(w, stateDir); err != nil {
			return err
		}
	} else {
		if err := writeFindingsHuman(w, stateDir); err != nil {
			return err
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
		return erruser.New("Could not determine current directory.", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return err
	}
	cfg, err := config.Load(context.Background(), config.LoadOptions{RepoRoot: repoRoot})
	if err != nil {
		return err
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

func newCleanupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Remove orphan stet worktrees (not the current session's worktree)",
		RunE:  runCleanup,
	}
	return cmd
}

func runCleanup(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		e := erruser.New("Could not determine current directory.", err)
		fmt.Fprintln(os.Stderr, e)
		if u := errors.Unwrap(e); u != nil {
			fmt.Fprintf(os.Stderr, "Details: %v\n", u)
		}
		return errExit(1)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		if u := errors.Unwrap(err); u != nil {
			fmt.Fprintf(os.Stderr, "Details: %v\n", u)
		}
		return errExit(1)
	}
	cfg, err := config.Load(context.Background(), config.LoadOptions{RepoRoot: repoRoot})
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		if u := errors.Unwrap(err); u != nil {
			fmt.Fprintf(os.Stderr, "Details: %v\n", u)
		}
		return errExit(1)
	}
	stateDir := cfg.EffectiveStateDir(repoRoot)
	worktreeRoot := cfg.WorktreeRoot
	if worktreeRoot == "" {
		worktreeRoot = filepath.Join(repoRoot, ".review", "worktrees")
	}
	worktreeRoot, err = filepath.Abs(worktreeRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, erruser.New("Could not resolve worktrees path.", err))
		return errExit(1)
	}

	var currentPath string
	s, err := session.Load(stateDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		if u := errors.Unwrap(err); u != nil {
			fmt.Fprintf(os.Stderr, "Details: %v\n", u)
		}
		return errExit(1)
	}
	if s.BaselineRef != "" {
		currentPath, err = git.PathForRef(repoRoot, cfg.WorktreeRoot, s.BaselineRef)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			if u := errors.Unwrap(err); u != nil {
				fmt.Fprintf(os.Stderr, "Details: %v\n", u)
			}
			return errExit(1)
		}
		currentPath, err = filepath.Abs(currentPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, erruser.New("Could not resolve worktree path.", err))
			return errExit(1)
		}
	}

	list, err := git.List(repoRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		if u := errors.Unwrap(err); u != nil {
			fmt.Fprintf(os.Stderr, "Details: %v\n", u)
		}
		return errExit(1)
	}

	var removed int
	for _, w := range list {
		wtPath := filepath.Clean(w.Path)
		dir := filepath.Dir(wtPath)
		base := filepath.Base(wtPath)
		if dir != worktreeRoot || !strings.HasPrefix(base, "stet-") {
			continue
		}
		if currentPath != "" && wtPath == currentPath {
			continue
		}
		if err := git.Remove(repoRoot, wtPath); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			if u := errors.Unwrap(err); u != nil {
				fmt.Fprintf(os.Stderr, "Details: %v\n", u)
			}
			return errExit(1)
		}
		fmt.Fprintf(os.Stderr, "Removed worktree: %s\n", wtPath)
		removed++
	}
	if removed == 0 {
		fmt.Fprintln(os.Stderr, "No orphan worktrees.")
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
		return erruser.New("Could not determine current directory.", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return err
	}
	cfg, err := config.Load(context.Background(), config.LoadOptions{RepoRoot: repoRoot})
	if err != nil {
		return err
	}
	stateDir := cfg.EffectiveStateDir(repoRoot)
	s, err := session.Load(stateDir)
	if err != nil {
		return err
	}
	if s.BaselineRef == "" {
		fmt.Fprintln(os.Stderr, "No active session. Run 'stet start' to begin a review.")
		return errExit(1)
	}
	worktreePath, err := git.PathForRef(repoRoot, cfg.WorktreeRoot, s.BaselineRef)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "baseline: %s\n", s.BaselineRef)
	fmt.Fprintf(os.Stdout, "last_reviewed_at: %s\n", s.LastReviewedAt)
	fmt.Fprintf(os.Stdout, "worktree: %s\n", worktreePath)
	fmt.Fprintf(os.Stdout, "findings: %d\n", len(s.Findings))
	fmt.Fprintf(os.Stdout, "dismissed: %d\n", len(s.DismissedIDs))
	if s.Strictness != "" {
		fmt.Fprintf(os.Stdout, "strictness: %s\n", s.Strictness)
	}
	if s.RAGSymbolMaxDefinitions != nil {
		fmt.Fprintf(os.Stdout, "rag_symbol_max_definitions: %d\n", *s.RAGSymbolMaxDefinitions)
	}
	if s.RAGSymbolMaxTokens != nil {
		fmt.Fprintf(os.Stdout, "rag_symbol_max_tokens: %d\n", *s.RAGSymbolMaxTokens)
	}
	showIDs, _ := cmd.Flags().GetBool("ids")
	if showIDs {
		active, err := activeFindings(stateDir)
		if err != nil {
			return err
		}
		if len(active) > 0 {
			fmt.Fprintln(os.Stdout, "---")
			if err := writeFindingsWithIDs(os.Stdout, stateDir); err != nil {
				return err
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
		return erruser.New("Could not determine current directory.", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return err
	}
	cfg, err := config.Load(context.Background(), config.LoadOptions{RepoRoot: repoRoot})
	if err != nil {
		return err
	}
	stateDir := cfg.EffectiveStateDir(repoRoot)
	s, err := session.Load(stateDir)
	if err != nil {
		return err
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
		Long: `Mark a finding as dismissed so it does not resurface. The id can be the full finding id or a unique prefix (e.g. first 7 characters). Optional reason is recorded for the optimizer.

Valid reasons:
  false_positive   — Finding is not a real issue
  already_correct  — Code is already correct as-is
  wrong_suggestion — Suggestion is wrong or not applicable
  out_of_scope     — Out of scope for this review

For guidance on choosing a reason, see docs/review-quality.md.`,
		RunE: runDismiss,
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
			return errors.New("Invalid reason; use one of: false_positive, already_correct, wrong_suggestion, out_of_scope.")
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return erruser.New("Could not determine current directory.", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return err
	}
	cfg, err := config.Load(context.Background(), config.LoadOptions{RepoRoot: repoRoot})
	if err != nil {
		return err
	}
	stateDir := cfg.EffectiveStateDir(repoRoot)
	s, err := session.Load(stateDir)
	if err != nil {
		return err
	}
	if s.BaselineRef == "" {
		fmt.Fprintln(os.Stderr, run.ErrNoSession.Error())
		return errExit(1)
	}
	fullID, err := findings.ResolveFindingIDByPrefix(s.Findings, id)
	if err != nil {
		return err
	}
	alreadyDismissed := false
	for _, d := range s.DismissedIDs {
		if d == fullID {
			alreadyDismissed = true
			break
		}
	}
	if !alreadyDismissed {
		s.DismissedIDs = append(s.DismissedIDs, fullID)
		if s.FindingPromptContext == nil {
			s.FindingPromptContext = make(map[string]string)
		}
		if ctx := s.FindingPromptContext[fullID]; ctx != "" {
			s.PromptShadows = append(s.PromptShadows, session.PromptShadow{FindingID: fullID, PromptContext: ctx})
		}
		if err := session.Save(stateDir, &s); err != nil {
			return err
		}
	}
	diffRef := s.LastReviewedAt
	if diffRef == "" {
		diffRef = s.BaselineRef
	}
	ua := history.UserAction{DismissedIDs: []string{fullID}}
	if reason != "" {
		ua.Dismissals = []history.Dismissal{{FindingID: fullID, Reason: reason}}
	}
	rec := history.Record{
		DiffRef:      diffRef,
		ReviewOutput: s.Findings,
		UserAction:   ua,
	}
	if err := history.Append(stateDir, rec, history.DefaultMaxRecords); err != nil {
		return err
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
		return erruser.New("Could not determine current directory.", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return err
	}
	cfg, err := config.Load(context.Background(), config.LoadOptions{RepoRoot: repoRoot})
	if err != nil {
		return err
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
		return erruser.New("Could not determine current directory.", err)
	}
	repoRoot := ""
	if r, e := git.RepoRoot(cwd); e == nil {
		repoRoot = r
	}
	cfg, err := config.Load(cmd.Context(), config.LoadOptions{RepoRoot: repoRoot})
	if err != nil {
		return err
	}
	client := ollama.NewClient(cfg.OllamaBaseURL, nil)
	result, err := client.Check(cmd.Context(), cfg.Model)
	if err != nil {
		if errors.Is(err, ollama.ErrUnreachable) {
			fmt.Fprintf(os.Stderr, "Ollama unreachable at %s. Is the server running? For local: ollama serve.\n", cfg.OllamaBaseURL)
			fmt.Fprintf(os.Stderr, "Details: %v\n", err)
			return errExit(2)
		}
		if errors.Is(err, ollama.ErrBadRequest) {
			fmt.Fprintf(os.Stderr, "Ollama bad request at %s. %v\n", cfg.OllamaBaseURL, err)
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
	fmt.Fprintf(os.Stdout, "Model: %s\n", cfg.Model)
	return nil
}
