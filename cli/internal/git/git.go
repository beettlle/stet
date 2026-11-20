// Package git (git.go) provides a batched content search: one `git grep`
// process covering many patterns, so callers never spawn one grep per query.
package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// grepTimeout bounds a single batched grep. One call replaces up to
// maxSymbolCandidates (30) sequential per-symbol greps, each previously
// capped at 5s; the batch traverses the worktree once, so 10s is a generous
// ceiling while still bounding total lookup latency well below the old
// worst case.
const grepTimeout = 10 * time.Second

// grepCommand is the seam used to start git grep processes. Tests replace it
// to count invocations; production code always uses exec.CommandContext.
var grepCommand = exec.CommandContext

// GrepHit is one matched line from a batched git grep.
type GrepHit struct {
	Path    string // Repo-relative path of the matched file (slash-separated).
	Line    int    // 1-based line number of the match.
	Content string // Matched line content, without trailing newline.
}

// Grep runs a single `git grep -n -E` in repoRoot with every pattern passed
// as a separate -e argument. It returns all hits in git's output order
// (worktree traversal order, independent of how many patterns are passed),
// so per-pattern relative ordering matches a single-pattern grep exactly.
// Exit code 1 (no match at all) returns (nil, nil); other failures return
// the error for the caller to handle.
func Grep(ctx context.Context, repoRoot string, patterns []string) ([]GrepHit, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	args := make([]string, 0, 2*len(patterns)+3)
	args = append(args, "grep", "-n", "-E")
	for _, p := range patterns {
		args = append(args, "-e", p)
	}
	ctx, cancel := context.WithTimeout(ctx, grepTimeout)
	defer cancel()
	cmd := grepCommand(ctx, "git", args...)
	cmd.Dir = repoRoot
	cmd.Env = grepEnv(repoRoot)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil // git grep exits 1 when nothing matches.
		}
		return nil, err
	}
	return parseGrepOutput(string(out)), nil
}

// parseGrepOutput parses `git grep -n` output lines of the form
// "path:lineno:content". Lines that do not parse (or reference paths that
// escape the repo via "..") are skipped: git grep -n always emits this
// format, so a malformed line indicates output corruption, and the defensive
// skip matches the previous per-symbol parsers' behavior.
func parseGrepOutput(out string) []GrepHit {
	if strings.TrimSpace(out) == "" {
		return nil
	}
	var hits []GrepHit
	for _, line := range strings.Split(out, "\n") {
		idx := strings.Index(line, ":")
		if idx == -1 {
			continue
		}
		path := line[:idx]
		if path == "" || strings.Contains(path, "..") {
			continue
		}
		rest := line[idx+1:]
		idx2 := strings.Index(rest, ":")
		if idx2 == -1 {
			continue
		}
		lineno, err := strconv.Atoi(rest[:idx2])
		if err != nil || lineno < 1 {
			continue
		}
		hits = append(hits, GrepHit{Path: path, Line: lineno, Content: rest[idx2+1:]})
	}
	return hits
}

// grepEnv returns a minimal environment for git grep subprocesses: no host
// config leakage beyond PATH, prompts disabled, and the worktree pinned to
// repoRoot via GIT_DIR/GIT_WORK_TREE so grep runs against that tree even if
// the process cwd changes.
func grepEnv(repoRoot string) []string {
	gitDir := filepath.Join(repoRoot, ".git")
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_DIR=" + gitDir,
		"GIT_WORK_TREE=" + repoRoot,
	}
}
