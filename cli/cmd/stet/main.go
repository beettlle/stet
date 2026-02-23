package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"stet/cli/internal/benchmark"
	"stet/cli/internal/commitmsg"
	"stet/cli/internal/config"
	"stet/cli/internal/erruser"
	"stet/cli/internal/findings"
	"stet/cli/internal/git"
	"stet/cli/internal/history"
	"stet/cli/internal/ollama"
	"stet/cli/internal/run"
	"stet/cli/internal/session"
	"stet/cli/internal/stats"
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

const hintBaselineRefResolution = "Hint: If you used HEAD~N, the repo may be shallow; try the commit SHA (e.g. stet start <sha>)."

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

// writeFindingsHuman writes a human-readable summary to w: one line per finding (id  file:line  severity  message), then a summary line.
// When stats is non-nil and EvalDurationNs > 0, the summary line includes " at Y tokens/sec.".
func writeFindingsHuman(w io.Writer, stateDir string, stats *run.RunStats) error {
	active, err := activeFindings(stateDir)
	if err != nil {
		return err
	}
	for _, f := range active {
		line := f.Line
		if f.Range != nil {
			line = f.Range.Start
		}
		if _, err := fmt.Fprintf(w, "%s  %s:%d  %s  %s\n", findings.ShortID(f.ID), f.File, line, strings.ToUpper(string(f.Severity)), f.Message); err != nil {
			return erruser.New("Could not write findings.", err)
		}
	}
	n := len(active)
	if stats != nil && stats.EvalDurationNs > 0 {
		totalTokens := stats.PromptTokens + stats.CompletionTokens
		secs := float64(stats.EvalDurationNs) / 1e9
		tokensPerSec := float64(totalTokens) / secs
		if n != 1 {
			if _, err := fmt.Fprintf(w, "%d finding(s) at %.1f tokens/sec.\n", n, tokensPerSec); err != nil {
				return erruser.New("Could not write findings.", err)
			}
		} else {
			if _, err := fmt.Fprintf(w, "1 finding at %.1f tokens/sec.\n", tokensPerSec); err != nil {
				return erruser.New("Could not write findings.", err)
			}
		}
	} else {
		if n != 1 {
			if _, err := fmt.Fprintf(w, "%d finding(s).\n", n); err != nil {
				return erruser.New("Could not write findings.", err)
			}
		} else {
			if _, err := fmt.Fprintln(w, "1 finding."); err != nil {
				return erruser.New("Could not write findings.", err)
			}
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
		if _, err := fmt.Fprintf(w, "%s  %s:%d  %s  %s\n", findings.ShortID(f.ID), f.File, line, strings.ToUpper(string(f.Severity)), f.Message); err != nil {
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
	rootCmd.AddCommand(newCommitMsgCmd())
	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.AddCommand(newBenchmarkCmd())
	rootCmd.AddCommand(newStatsCmd())
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
		if strings.Contains(err.Error(), "Could not resolve baseline ref") {
			fmt.Fprintln(os.Stderr, hintBaselineRefResolution)
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
	cmd.Flags().Bool("rag-call-graph", false, "Enable RAG call-graph (callers/callees) for Go hunks; overrides config and env")
	cmd.Flags().String("strictness", "", "Review strictness preset: strict, default, lenient, strict+, default+, lenient+ (overrides config and env)")
	cmd.Flags().Bool("nitpicky", false, "Enable nitpicky mode: report typos, grammar, style, and convention violations; do not filter those findings")
	cmd.Flags().String("context", "", "Context window preset: 4k, 8k, 16k, 32k, 64k, 128k, 256k (sets both context_limit and num_ctx)")
	cmd.Flags().Int("num-ctx", 0, "Context window size in tokens (0 = use config); overrides config and --context; sets both context_limit and num_ctx")
	cmd.Flags().Bool("trace", false, "Print internal steps to stderr (partition, rules, RAG, prompts, LLM I/O)")
	cmd.Flags().Bool("search-replace", false, "Use search-replace style diff in the prompt (experimental; compare token usage and finding quality)")
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
	overrides, err := overridesFromFlags(cmd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return errExit(1)
	}
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
	var persistContextLimit *int
	var persistNumCtx *int
	if overrides != nil && (overrides.ContextLimit != nil || overrides.NumCtx != nil) {
		persistContextLimit = &cfg.ContextLimit
		persistNumCtx = &cfg.NumCtx
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
		RAGCallGraphEnabled:            cfg.RAGCallGraphEnabled,
		RAGCallersMax:                  cfg.RAGCallersMax,
		RAGCalleesMax:                  cfg.RAGCalleesMax,
		RAGCallGraphMaxTokens:          cfg.RAGCallGraphMaxTokens,
		MinConfidenceKeep:              minKeep,
		MinConfidenceMaintainability:   minMaint,
		ApplyFPKillList:                &applyFP,
		Nitpicky:                       cfg.Nitpicky,
		PersistStrictness:              persistStrictness,
		PersistRAGSymbolMaxDefinitions: persistRAGDefs,
		PersistRAGSymbolMaxTokens:      persistRAGTokens,
		PersistNitpicky:                persistNitpicky,
		PersistContextLimit:            persistContextLimit,
		PersistNumCtx:                  persistNumCtx,
		TraceOut:                       traceOut,
		UseSearchReplaceFormat:         getSearchReplaceFlag(cmd),
		SuppressionEnabled:             cfg.SuppressionEnabled,
		SuppressionHistoryCount:        cfg.SuppressionHistoryCount,
	}
	if stream {
		opts.StreamOut = findingsWriter()
	}
	stats, err := run.Start(cmd.Context(), opts)
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
		if err := writeFindingsHuman(w, stateDir, &stats); err != nil {
			return err
		}
	}
	return nil
}

// addRunLikeFlags registers the flags shared by run and rerun (dry-run, quiet, output, json, stream, rag-symbol-*, strictness, nitpicky, context, num-ctx, trace).
func addRunLikeFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("dry-run", false, "Skip LLM; inject canned findings for CI")
	cmd.Flags().BoolP("quiet", "q", false, "Suppress progress (use for scripts and IDE integration)")
	cmd.Flags().String("output", "human", "Output format: human (default) or json")
	cmd.Flags().Bool("json", false, "Emit findings as JSON to stdout (same as --output=json)")
	cmd.Flags().Bool("stream", false, "Emit progress and findings as NDJSON (one event per line); requires --output=json")
	cmd.Flags().Int("rag-symbol-max-definitions", 0, "Max symbol definitions to inject (0 = use config); overrides config and env")
	cmd.Flags().Int("rag-symbol-max-tokens", 0, "Max tokens for symbol-definitions block (0 = use config); overrides config and env")
	cmd.Flags().Bool("rag-call-graph", false, "Enable RAG call-graph (callers/callees) for Go hunks; overrides config and env")
	cmd.Flags().String("strictness", "", "Review strictness preset: strict, default, lenient, strict+, default+, lenient+ (overrides config and env)")
	cmd.Flags().Bool("nitpicky", false, "Enable nitpicky mode: report typos, grammar, style, and convention violations; do not filter those findings")
	cmd.Flags().String("context", "", "Context window preset: 4k, 8k, 16k, 32k, 64k, 128k, 256k (sets both context_limit and num_ctx)")
	cmd.Flags().Int("num-ctx", 0, "Context window size in tokens (0 = use config); overrides config and --context; sets both context_limit and num_ctx")
	cmd.Flags().Bool("trace", false, "Print internal steps to stderr (partition, rules, RAG, prompts, LLM I/O)")
	cmd.Flags().Bool("search-replace", false, "Use search-replace style diff in the prompt (experimental; compare token usage and finding quality)")
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

// parseContextPreset maps preset strings (4k, 8k, ..., 256k) to token counts. Case-insensitive. Returns error for invalid values.
func parseContextPreset(s string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "4k":
		return 4096, nil
	case "8k":
		return 8192, nil
	case "16k":
		return 16384, nil
	case "32k":
		return 32768, nil
	case "64k":
		return 65536, nil
	case "128k":
		return 131072, nil
	case "256k":
		return 262144, nil
	default:
		return 0, erruser.New("invalid --context: must be one of 4k, 8k, 16k, 32k, 64k, 128k, 256k", nil)
	}
}

// overridesFromFlags returns Overrides for RAG, strictness, nitpicky, and context when the corresponding flags were set (start/run both define these flags).
func overridesFromFlags(cmd *cobra.Command) (*config.Overrides, error) {
	defChanged := cmd.Flags().Lookup("rag-symbol-max-definitions") != nil && cmd.Flags().Lookup("rag-symbol-max-definitions").Changed
	tokChanged := cmd.Flags().Lookup("rag-symbol-max-tokens") != nil && cmd.Flags().Lookup("rag-symbol-max-tokens").Changed
	ragCallGraphChanged := cmd.Flags().Lookup("rag-call-graph") != nil && cmd.Flags().Lookup("rag-call-graph").Changed
	strictnessChanged := cmd.Flags().Lookup("strictness") != nil && cmd.Flags().Lookup("strictness").Changed
	nitpickyChanged := cmd.Flags().Lookup("nitpicky") != nil && cmd.Flags().Lookup("nitpicky").Changed
	contextChanged := cmd.Flags().Lookup("context") != nil && cmd.Flags().Lookup("context").Changed
	numCtxChanged := cmd.Flags().Lookup("num-ctx") != nil && cmd.Flags().Lookup("num-ctx").Changed
	if !defChanged && !tokChanged && !ragCallGraphChanged && !strictnessChanged && !nitpickyChanged && !contextChanged && !numCtxChanged {
		return nil, nil
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
	if ragCallGraphChanged {
		v, _ := cmd.Flags().GetBool("rag-call-graph")
		o.RAGCallGraphEnabled = &v
	}
	if strictnessChanged {
		v, _ := cmd.Flags().GetString("strictness")
		o.Strictness = &v
	}
	if nitpickyChanged {
		v, _ := cmd.Flags().GetBool("nitpicky")
		o.Nitpicky = &v
	}
	if numCtxChanged {
		v, err := cmd.Flags().GetInt("num-ctx")
		if err != nil {
			return nil, erruser.New("--num-ctx: invalid value", err)
		}
		if v < 0 {
			return nil, erruser.New("--num-ctx must be 0 or positive", nil)
		}
		o.NumCtx = &v
		o.ContextLimit = &v
	} else if contextChanged {
		v, _ := cmd.Flags().GetString("context")
		n, err := parseContextPreset(v)
		if err != nil {
			return nil, err
		}
		o.ContextLimit = &n
		o.NumCtx = &n
	}
	return o, nil
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
	overrides, err := overridesFromFlags(cmd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return errExit(1)
	}
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
	effectiveContextLimit := cfg.ContextLimit
	if overrides != nil && overrides.ContextLimit != nil {
		effectiveContextLimit = *overrides.ContextLimit
	} else if s.ContextLimit != nil {
		effectiveContextLimit = *s.ContextLimit
	}
	effectiveNumCtx := cfg.NumCtx
	if overrides != nil && overrides.NumCtx != nil {
		effectiveNumCtx = *overrides.NumCtx
	} else if s.NumCtx != nil {
		effectiveNumCtx = *s.NumCtx
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
		ContextLimit:                 effectiveContextLimit,
		WarnThreshold:                cfg.WarnThreshold,
		Timeout:                      cfg.Timeout,
		Temperature:                  cfg.Temperature,
		NumCtx:                       effectiveNumCtx,
		Verbose:                      verbose,
		StreamOut:                    nil,
		RAGSymbolMaxDefinitions:      effectiveRAGDefs,
		RAGSymbolMaxTokens:           effectiveRAGTokens,
		RAGCallGraphEnabled:          cfg.RAGCallGraphEnabled,
		RAGCallersMax:                cfg.RAGCallersMax,
		RAGCalleesMax:                cfg.RAGCalleesMax,
		RAGCallGraphMaxTokens:        cfg.RAGCallGraphMaxTokens,
		MinConfidenceKeep:            minKeep,
		MinConfidenceMaintainability: minMaint,
		ApplyFPKillList:              &applyFP,
		Nitpicky:                     effectiveNitpicky,
		TraceOut:                     traceOut,
		UseSearchReplaceFormat:       getSearchReplaceFlag(cmd),
		SuppressionEnabled:           cfg.SuppressionEnabled,
		SuppressionHistoryCount:      cfg.SuppressionHistoryCount,
	}
	if stream {
		opts.StreamOut = findingsWriter()
	}
	stats, err := run.Run(cmd.Context(), opts)
	if err != nil {
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
		if err := writeFindingsHuman(w, stateDir, &stats); err != nil {
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
	overrides, err := overridesFromFlags(cmd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return errExit(1)
	}
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
	effectiveContextLimit := cfg.ContextLimit
	if overrides != nil && overrides.ContextLimit != nil {
		effectiveContextLimit = *overrides.ContextLimit
	} else if s.ContextLimit != nil {
		effectiveContextLimit = *s.ContextLimit
	}
	effectiveNumCtx := cfg.NumCtx
	if overrides != nil && overrides.NumCtx != nil {
		effectiveNumCtx = *overrides.NumCtx
	} else if s.NumCtx != nil {
		effectiveNumCtx = *s.NumCtx
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
		ContextLimit:                 effectiveContextLimit,
		WarnThreshold:                cfg.WarnThreshold,
		Timeout:                      cfg.Timeout,
		Temperature:                  cfg.Temperature,
		NumCtx:                       effectiveNumCtx,
		Verbose:                      verbose,
		StreamOut:                    nil,
		RAGSymbolMaxDefinitions:      effectiveRAGDefs,
		RAGSymbolMaxTokens:           effectiveRAGTokens,
		RAGCallGraphEnabled:          cfg.RAGCallGraphEnabled,
		RAGCallersMax:                cfg.RAGCallersMax,
		RAGCalleesMax:                cfg.RAGCalleesMax,
		RAGCallGraphMaxTokens:        cfg.RAGCallGraphMaxTokens,
		MinConfidenceKeep:            minKeep,
		MinConfidenceMaintainability: minMaint,
		ApplyFPKillList:              &applyFP,
		Nitpicky:                     effectiveNitpicky,
		TraceOut:                     traceOut,
		UseSearchReplaceFormat:       getSearchReplaceFlag(cmd),
		ForceFullReview:             true,
		ReplaceFindings:             replace,
		SuppressionEnabled:          cfg.SuppressionEnabled,
		SuppressionHistoryCount:     cfg.SuppressionHistoryCount,
	}
	if stream {
		opts.StreamOut = findingsWriter()
	}
	stats, err := run.Run(cmd.Context(), opts)
	if err != nil {
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
		if err := writeFindingsHuman(w, stateDir, &stats); err != nil {
			return err
		}
	}
	return nil
}

func getSearchReplaceFlag(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("search-replace")
	return v
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
		d := history.Dismissal{FindingID: fullID, Reason: reason}
		if s.FindingPromptContext != nil && s.FindingPromptContext[fullID] != "" {
			d.PromptContext = s.FindingPromptContext[fullID]
		}
		ua.Dismissals = []history.Dismissal{d}
	}
	rec := history.Record{
		DiffRef:      diffRef,
		ReviewOutput: s.Findings,
		UserAction:   ua,
		RunConfig:    history.NewRunConfigSnapshot(cfg.Model, cfg.Strictness, cfg.RAGSymbolMaxDefinitions, cfg.RAGSymbolMaxTokens, cfg.Nitpicky),
	}
	if err := history.Append(stateDir, rec, history.DefaultMaxRecords); err != nil {
		return err
	}
	return nil
}

func newCommitMsgCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commitmsg",
		Short: "Generate a conventional git commit message from uncommitted changes using the local LLM",
		Long:  "Prints a suggested commit message to stdout. By default the message covers all uncommitted changes. Use --commit to commit with that message (stages all changes first unless --staged-only), or --commit-and-review to commit then run review (stet start if no session, else stet run). Use --staged-only to use only staged changes for the message and to commit only what is already staged.",
		RunE:  runCommitMsg,
	}
	cmd.Flags().Bool("staged-only", false, "Use only staged changes for the message (default: staged + unstaged)")
	cmd.Flags().Bool("commit", false, "Commit staged changes with the generated message")
	cmd.Flags().Bool("commit-and-review", false, "Commit with the message, then run review (start session if none, else run)")
	cmd.Flags().String("context", "", "Context window preset: 4k, 8k, 16k, 32k, 64k, 128k, 256k (sets both context_limit and num_ctx; used for suggest and for review when --commit-and-review)")
	cmd.Flags().Int("num-ctx", 0, "Context window size in tokens (0 = use config); overrides config and --context; sets both context_limit and num_ctx")
	return cmd
}

func runCommitMsg(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return erruser.New("Could not determine current directory.", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return err
	}
	overrides, err := overridesFromFlags(cmd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return errExit(1)
	}
	cfg, err := config.Load(cmd.Context(), config.LoadOptions{RepoRoot: repoRoot, Overrides: overrides})
	if err != nil {
		return err
	}
	stagedOnly, _ := cmd.Flags().GetBool("staged-only")
	doCommit, _ := cmd.Flags().GetBool("commit")
	commitAndReview, _ := cmd.Flags().GetBool("commit-and-review")
	diff, err := git.UncommittedDiff(cmd.Context(), repoRoot, stagedOnly)
	if err != nil {
		return err
	}
	if diff == "" {
		if stagedOnly {
			return erruser.New("No staged changes to summarize. Stage changes with git add.", nil)
		}
		return erruser.New("No uncommitted changes to summarize.", nil)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	client := ollama.NewClient(cfg.OllamaBaseURL, &http.Client{Timeout: timeout})
	if _, err := client.Check(cmd.Context(), cfg.Model); err != nil {
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
	opts := &ollama.GenerateOptions{
		Temperature: 0.2,
		NumCtx:      cfg.NumCtx,
	}
	if opts.NumCtx <= 0 {
		opts.NumCtx = 32768
	}
	msg, err := commitmsg.Suggest(cmd.Context(), client, cfg.Model, diff, opts)
	if err != nil {
		if errors.Is(err, ollama.ErrUnreachable) {
			fmt.Fprintf(os.Stderr, "Ollama unreachable. Details: %v\n", err)
			return errExit(2)
		}
		if errors.Is(err, ollama.ErrBadRequest) {
			fmt.Fprintf(os.Stderr, "Ollama bad request. %v\n", err)
			return errExit(2)
		}
		return err
	}
	if msg == "" {
		return erruser.New("Model produced an empty commit message.", nil)
	}
	if !doCommit && !commitAndReview {
		fmt.Println(msg)
		return nil
	}
	if !stagedOnly {
		addCmd := exec.CommandContext(cmd.Context(), "git", "add", "-A")
		addCmd.Dir = repoRoot
		addCmd.Env = git.MinimalEnv()
		if err := addCmd.Run(); err != nil {
			return erruser.New("Could not stage changes (git add -A).", err)
		}
	}
	commitCmd := exec.CommandContext(cmd.Context(), "git", "commit", "-F", "-")
	commitCmd.Dir = repoRoot
	commitCmd.Stdin = strings.NewReader(msg)
	commitCmd.Env = git.MinimalEnv()
	if err := commitCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			return erruser.New("Git commit failed.", err)
		}
		return err
	}
	if !commitAndReview {
		return nil
	}
	stateDir := cfg.EffectiveStateDir(repoRoot)
	s, err := session.Load(stateDir)
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
	effectiveContextLimit := cfg.ContextLimit
	if overrides != nil && overrides.ContextLimit != nil {
		effectiveContextLimit = *overrides.ContextLimit
	} else if s.ContextLimit != nil {
		effectiveContextLimit = *s.ContextLimit
	}
	effectiveNumCtx := cfg.NumCtx
	if overrides != nil && overrides.NumCtx != nil {
		effectiveNumCtx = *overrides.NumCtx
	} else if s.NumCtx != nil {
		effectiveNumCtx = *s.NumCtx
	}
	runOpts := run.RunOptions{
		RepoRoot:                     repoRoot,
		StateDir:                     stateDir,
		DryRun:                       false,
		Model:                        cfg.Model,
		OllamaBaseURL:                cfg.OllamaBaseURL,
		ContextLimit:                 effectiveContextLimit,
		WarnThreshold:                cfg.WarnThreshold,
		Timeout:                      cfg.Timeout,
		Temperature:                  cfg.Temperature,
		NumCtx:                       effectiveNumCtx,
		Verbose:                      true,
		StreamOut:                    nil,
		RAGSymbolMaxDefinitions:      cfg.RAGSymbolMaxDefinitions,
		RAGSymbolMaxTokens:            cfg.RAGSymbolMaxTokens,
		RAGCallGraphEnabled:          cfg.RAGCallGraphEnabled,
		RAGCallersMax:                cfg.RAGCallersMax,
		RAGCalleesMax:                cfg.RAGCalleesMax,
		RAGCallGraphMaxTokens:        cfg.RAGCallGraphMaxTokens,
		MinConfidenceKeep:            minKeep,
		MinConfidenceMaintainability: minMaint,
		ApplyFPKillList:              &applyFP,
		Nitpicky:                     cfg.Nitpicky,
		SuppressionEnabled:           cfg.SuppressionEnabled,
		SuppressionHistoryCount:     cfg.SuppressionHistoryCount,
	}
	var persistContextLimit, persistNumCtx *int
	if overrides != nil && (overrides.ContextLimit != nil || overrides.NumCtx != nil) {
		persistContextLimit = &cfg.ContextLimit
		persistNumCtx = &cfg.NumCtx
	}
	if s.BaselineRef == "" {
		startOpts := run.StartOptions{
			RepoRoot:                       repoRoot,
			StateDir:                       stateDir,
			WorktreeRoot:                   cfg.WorktreeRoot,
			Ref:                            "HEAD~1",
			DryRun:                         false,
			AllowDirty:                     true,
			Model:                          cfg.Model,
			OllamaBaseURL:                  cfg.OllamaBaseURL,
			ContextLimit:                   effectiveContextLimit,
			WarnThreshold:                  cfg.WarnThreshold,
			Timeout:                        cfg.Timeout,
			Temperature:                   cfg.Temperature,
			NumCtx:                         effectiveNumCtx,
			Verbose:                        true,
			StreamOut:                      nil,
			RAGSymbolMaxDefinitions:        cfg.RAGSymbolMaxDefinitions,
			RAGSymbolMaxTokens:             cfg.RAGSymbolMaxTokens,
			RAGCallGraphEnabled:            cfg.RAGCallGraphEnabled,
			RAGCallersMax:                  cfg.RAGCallersMax,
			RAGCalleesMax:                  cfg.RAGCalleesMax,
			RAGCallGraphMaxTokens:          cfg.RAGCallGraphMaxTokens,
			MinConfidenceKeep:              minKeep,
			MinConfidenceMaintainability:   minMaint,
			ApplyFPKillList:                &applyFP,
			Nitpicky:                       cfg.Nitpicky,
			PersistContextLimit:            persistContextLimit,
			PersistNumCtx:                  persistNumCtx,
			SuppressionEnabled:            cfg.SuppressionEnabled,
			SuppressionHistoryCount:       cfg.SuppressionHistoryCount,
		}
		if _, err := run.Start(cmd.Context(), startOpts); err != nil {
			if errors.Is(err, ollama.ErrUnreachable) {
				fmt.Fprintf(os.Stderr, "Ollama unreachable. Details: %v\n", err)
				return errExit(2)
			}
			if errors.Is(err, run.ErrDirtyWorktree) {
				return err
			}
			if errors.Is(err, git.ErrWorktreeExists) {
				return errors.New("worktree already exists; run stet finish first")
			}
			if errors.Is(err, session.ErrLocked) {
				return errors.New("finish or cleanup current review first")
			}
			return err
		}
	}
	runStats, err := run.Run(cmd.Context(), runOpts)
	if err != nil {
		if errors.Is(err, run.ErrNoSession) {
			return err
		}
		if errors.Is(err, ollama.ErrUnreachable) {
			fmt.Fprintf(os.Stderr, "Ollama unreachable. Details: %v\n", err)
			return errExit(2)
		}
		return err
	}
	w := findingsWriter()
	return writeFindingsHuman(w, stateDir, &runStats)
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

func newBenchmarkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Measure model throughput (tokens/s) for the configured model",
		RunE:  runBenchmark,
	}
	cmd.Flags().String("model", "", "Override model (default: from config)")
	cmd.Flags().Bool("warmup", false, "Run warmup call before measuring (load model, discard metrics)")
	return cmd
}

func runBenchmark(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cmd.Context(), config.LoadOptions{RepoRoot: ""})
	if err != nil {
		return err
	}
	model, _ := cmd.Flags().GetString("model")
	if model == "" {
		model = cfg.Model
	}
	client := ollama.NewClient(cfg.OllamaBaseURL, nil)
	result, err := client.Check(cmd.Context(), model)
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
		fmt.Fprintf(os.Stderr, "Model %q not found. Pull it with: ollama pull %s\n", model, model)
		return errExit(1)
	}
	opts := &ollama.GenerateOptions{
		Temperature: cfg.Temperature,
		NumCtx:      cfg.NumCtx,
	}
	warmup, _ := cmd.Flags().GetBool("warmup")
	if warmup {
		_, _ = benchmark.Run(cmd.Context(), client, model, opts)
	}
	br, err := benchmark.Run(cmd.Context(), client, model, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Benchmark failed: %v\n", err)
		return errExit(1)
	}
	fmt.Fprintf(os.Stdout, "Model: %s\n", br.Model)
	fmt.Fprintf(os.Stdout, "Eval rate:     %6.1f t/s (generation)\n", br.EvalRateTPS)
	if br.PromptEvalRateTPS > 0 {
		fmt.Fprintf(os.Stdout, "Prompt eval:   %6.0f t/s (prefill)\n", br.PromptEvalRateTPS)
	}
	if br.LoadDurationNs > 0 {
		fmt.Fprintf(os.Stdout, "Load:          %6.1fs (cold start)\n", float64(br.LoadDurationNs)/1e9)
	}
	fmt.Fprintln(os.Stdout, "---")
	fmt.Fprintf(os.Stdout, "Tokens: %d in, %d out | Total: %.1fs\n", br.PromptEvalCount, br.EvalCount, float64(br.TotalDurationNs)/1e9)
	if br.EstimatedSecPerTypicalHunk > 0 {
		fmt.Fprintf(os.Stdout, "Typical stet hunk (~%dk prompt, ~%d out): ~%.1fs estimated\n",
			benchmark.TypicalPromptTokens/1000, benchmark.TypicalOutputTokens, br.EstimatedSecPerTypicalHunk)
	}
	fmt.Fprintln(os.Stdout, "---")
	fmt.Fprintln(os.Stdout, "Reference: 8B ~35-50 t/s, 30B ~15-25 t/s, 70B ~5-8 t/s (M1 Ultra)")
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

func newStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Impact reporting (volume, quality, energy)",
	}
	cmd.AddCommand(newStatsVolumeCmd())
	cmd.AddCommand(newStatsQualityCmd())
	cmd.AddCommand(newStatsEnergyCmd())
	return cmd
}

func newStatsVolumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volume",
		Short: "Report review volume over a ref range",
		RunE:  runStatsVolume,
	}
	cmd.Flags().String("since", "main", "Start of ref range (e.g. main or \"30 days ago\")")
	cmd.Flags().String("until", "HEAD", "End of ref range")
	cmd.Flags().String("format", "human", "Output format: human or json")
	return cmd
}

func runStatsVolume(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return erruser.New("Could not determine current directory.", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return err
	}
	since, _ := cmd.Flags().GetString("since")
	until, _ := cmd.Flags().GetString("until")
	format, _ := cmd.Flags().GetString("format")
	if format != "human" && format != "json" {
		return errors.New("Invalid output format; use human or json.")
	}
	res, err := stats.Volume(repoRoot, since, until)
	if err != nil {
		return erruser.New("Could not compute volume stats.", err)
	}
	if format == "json" {
		data, err := json.Marshal(res)
		if err != nil {
			return erruser.New("Could not write volume stats.", err)
		}
		if _, err := os.Stdout.Write(data); err != nil {
			return erruser.New("Could not write volume stats.", err)
		}
		fmt.Fprintln(os.Stdout)
		return nil
	}
	fmt.Fprintf(os.Stdout, "Sessions: %d\n", res.SessionsCount)
	fmt.Fprintf(os.Stdout, "Commits in range: %d\n", res.CommitsInRange)
	fmt.Fprintf(os.Stdout, "Commits with Stet note: %d (%.1f%%)\n", res.CommitsWithNote, res.PercentCommitsWithNote)
	fmt.Fprintf(os.Stdout, "Hunks reviewed: %d\n", res.TotalHunksReviewed)
	fmt.Fprintf(os.Stdout, "Lines added: %d\n", res.TotalLinesAdded)
	fmt.Fprintf(os.Stdout, "Lines removed: %d\n", res.TotalLinesRemoved)
	fmt.Fprintf(os.Stdout, "Chars added: %d\n", res.TotalCharsAdded)
	fmt.Fprintf(os.Stdout, "Chars deleted: %d\n", res.TotalCharsDeleted)
	fmt.Fprintf(os.Stdout, "Chars reviewed: %d\n", res.TotalCharsReviewed)
	if res.GitAI != nil {
		fmt.Fprintf(os.Stdout, "Git AI (refs/notes/ai):\n")
		fmt.Fprintf(os.Stdout, "Commits with AI note: %d\n", res.GitAI.CommitsWithAINote)
		fmt.Fprintf(os.Stdout, "Total AI-authored lines: %d\n", res.GitAI.TotalAIAuthoredLines)
	}
	return nil
}

func newStatsQualityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quality",
		Short: "Report review quality from history (dismissal rate, actionability, category breakdown)",
		RunE:  runStatsQuality,
	}
	cmd.Flags().String("format", "human", "Output format: human or json")
	return cmd
}

func runStatsQuality(cmd *cobra.Command, args []string) error {
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
	format, _ := cmd.Flags().GetString("format")
	if format != "human" && format != "json" {
		return errors.New("Invalid output format; use human or json.")
	}
	res, err := stats.Quality(stateDir)
	if err != nil {
		return erruser.New("Could not compute quality stats.", err)
	}
	if format == "json" {
		data, err := json.Marshal(res)
		if err != nil {
			return erruser.New("Could not write quality stats.", err)
		}
		if _, err := os.Stdout.Write(data); err != nil {
			return erruser.New("Could not write quality stats.", err)
		}
		fmt.Fprintln(os.Stdout)
		return nil
	}
	fmt.Fprintf(os.Stdout, "Sessions: %d\n", res.SessionsCount)
	fmt.Fprintf(os.Stdout, "Total findings: %d\n", res.TotalFindings)
	fmt.Fprintf(os.Stdout, "Total dismissed: %d\n", res.TotalDismissed)
	fmt.Fprintf(os.Stdout, "Dismissal rate: %.2f\n", res.DismissalRate)
	fmt.Fprintf(os.Stdout, "Acceptance rate: %.2f\n", res.AcceptanceRate)
	fmt.Fprintf(os.Stdout, "False positive rate: %.2f\n", res.FalsePositiveRate)
	fmt.Fprintf(os.Stdout, "Actionability: %.2f\n", res.Actionability)
	fmt.Fprintf(os.Stdout, "Clean commit rate: %.2f\n", res.CleanCommitRate)
	if res.FindingDensity != 0 {
		fmt.Fprintf(os.Stdout, "Finding density (per 1k tokens): %.2f\n", res.FindingDensity)
	}
	fmt.Fprintf(os.Stdout, "Dismissals by reason: %v\n", res.DismissalsByReason)
	fmt.Fprintf(os.Stdout, "Category breakdown: %v\n", res.CategoryBreakdown)
	return nil
}

func newStatsEnergyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "energy",
		Short: "Report local energy and cloud cost avoided from notes",
		Long: `Aggregates eval_duration_ns, prompt_tokens, and completion_tokens from refs/notes/stet.
Computes local energy (kWh) and cloud cost avoided ($). Estimates only; model equivalence
is heuristic; local cost excludes electricity.`,
		RunE: runStatsEnergy,
	}
	cmd.Flags().Int("watts", 30, "Assumed power draw (W) for local energy calculation")
	cmd.Flags().StringSlice("cloud-model", nil, "Cloud model for cost estimate: NAME (preset) or NAME:in_per_million:out_per_million (repeatable)")
	cmd.Flags().String("since", "main", "Start of ref range (e.g. main or \"30 days ago\")")
	cmd.Flags().String("until", "HEAD", "End of ref range")
	cmd.Flags().String("format", "human", "Output format: human or json")
	return cmd
}

func runStatsEnergy(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return erruser.New("Could not determine current directory.", err)
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return err
	}
	watts, _ := cmd.Flags().GetInt("watts")
	if watts < 0 {
		watts = 0
	}
	cloudModelStrs, _ := cmd.Flags().GetStringSlice("cloud-model")
	var cloudModels []stats.CloudModel
	for _, s := range cloudModelStrs {
		m, err := stats.ParseCloudModel(s)
		if err != nil {
			return err
		}
		cloudModels = append(cloudModels, m)
	}
	since, _ := cmd.Flags().GetString("since")
	until, _ := cmd.Flags().GetString("until")
	format, _ := cmd.Flags().GetString("format")
	if format != "human" && format != "json" {
		return errors.New("Invalid output format; use human or json.")
	}
	res, err := stats.Energy(repoRoot, since, until, watts, cloudModels)
	if err != nil {
		return erruser.New("Could not compute energy stats.", err)
	}
	if format == "json" {
		data, err := json.Marshal(res)
		if err != nil {
			return erruser.New("Could not write energy stats.", err)
		}
		if _, err := os.Stdout.Write(data); err != nil {
			return erruser.New("Could not write energy stats.", err)
		}
		fmt.Fprintln(os.Stdout)
		return nil
	}
	fmt.Fprintf(os.Stdout, "Sessions: %d\n", res.SessionsCount)
	fmt.Fprintf(os.Stdout, "Total eval duration: %.1f s\n", float64(res.TotalEvalDurationNs)/1e9)
	fmt.Fprintf(os.Stdout, "Total prompt tokens: %d\n", res.TotalPromptTokens)
	fmt.Fprintf(os.Stdout, "Total completion tokens: %d\n", res.TotalCompletionTokens)
	fmt.Fprintf(os.Stdout, "Local energy: %.4f kWh\n", res.LocalEnergyKWh)
	for name, cost := range res.CloudCostAvoided {
		fmt.Fprintf(os.Stdout, "Cloud cost avoided (%s): $%.2f\n", name, cost)
	}
	fmt.Fprintln(os.Stdout, "Estimates only; model equivalence heuristic; local cost excludes electricity.")
	return nil
}
