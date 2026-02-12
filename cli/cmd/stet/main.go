package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"stet/cli/internal/config"
	"stet/cli/internal/findings"
	"stet/cli/internal/git"
	"stet/cli/internal/ollama"
	"stet/cli/internal/run"
	"stet/cli/internal/session"
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

// writeFindingsJSON loads the session from stateDir and writes {"findings": [...]} to w.
func writeFindingsJSON(w io.Writer, stateDir string) error {
	s, err := session.Load(stateDir)
	if err != nil {
		return fmt.Errorf("write findings: load session: %w", err)
	}
	payload := struct {
		Findings []findings.Finding `json:"findings"`
	}{Findings: s.Findings}
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
	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	ref := "HEAD"
	if len(args) > 0 {
		ref = args[0]
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
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
		RepoRoot:       repoRoot,
		StateDir:       stateDir,
		WorktreeRoot:   cfg.WorktreeRoot,
		Ref:            ref,
		DryRun:         dryRun,
		Model:          cfg.Model,
		OllamaBaseURL:  cfg.OllamaBaseURL,
		ContextLimit:   cfg.ContextLimit,
		WarnThreshold:  cfg.WarnThreshold,
		Timeout:        cfg.Timeout,
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
			fmt.Fprintf(errHintOut, "Hint: Run: stet finish && stet start %s\n", ref)
			return errors.New("worktree already exists")
		}
		return err
	}
	if err := writeFindingsJSON(findingsOut, stateDir); err != nil {
		return fmt.Errorf("start: %w", err)
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
	opts := run.RunOptions{
		RepoRoot:      repoRoot,
		StateDir:      stateDir,
		DryRun:        dryRun,
		Model:         cfg.Model,
		OllamaBaseURL: cfg.OllamaBaseURL,
		ContextLimit:  cfg.ContextLimit,
		WarnThreshold: cfg.WarnThreshold,
		Timeout:       cfg.Timeout,
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
	if err := writeFindingsJSON(findingsOut, stateDir); err != nil {
		return fmt.Errorf("run: %w", err)
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
		return err
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
