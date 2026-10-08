// Package testutil provides helpers for isolating git-heavy tests from the
// host machine's git configuration and background git daemons.
//
// # Why this package exists
//
// A global git config that routes trace2 events to a background daemon (for
// example trace2.eventtarget=af_unix:…/.git-ai/internal/daemon/trace2.sock)
// makes every git subprocess report to that daemon. The daemon reacts to
// commits by asynchronously writing bookkeeping files into the repository:
// .git/ai/working_logs/…, refs/notes/ai, and logs/refs/notes/ai. Tests create
// throwaway repositories under t.TempDir(); when the daemon writes during
// t.TempDir's os.RemoveAll cleanup, the cleanup fails with
// "unlinkat …/.git: directory not empty" and the test is marked failed even
// though every assertion passed.
//
// Two defenses are required and provided here:
//
//   - RunIsolatedGit (call from TestMain) points GIT_CONFIG_GLOBAL at a
//     minimal temporary gitconfig and sets GIT_CONFIG_NOSYSTEM=1, so host
//     trace2 targets and other host/system config never leak into test git
//     subprocesses.
//   - TempGitDir replaces t.TempDir() for directories that hold git
//     repositories; its cleanup retries RemoveAll with a bounded linear
//     backoff so an in-flight daemon write cannot fail the test.
package testutil

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

const (
	// gitConfigContent is the entire content of the temporary global
	// gitconfig installed by RunIsolatedGit. It contains nothing but the
	// default branch setting, so tests may rely on `git init` defaulting to
	// branch main.
	gitConfigContent = "[init]\n\tdefaultBranch = main\n"

	// removeMaxAttempts bounds how many times TempGitDir's cleanup tries
	// os.RemoveAll before reporting a failure.
	removeMaxAttempts = 10

	// removeBackoffStep is the linear backoff unit: after attempt N (1-based),
	// cleanup waits removeBackoffStep * N before the next attempt, giving an
	// external daemon a bounded window (~2.25s total) to finish writing.
	removeBackoffStep = 50 * time.Millisecond
)

// RunIsolatedGit runs the package tests with an isolated git configuration
// and returns the exit code for os.Exit in TestMain:
//
//	func TestMain(m *testing.M) { os.Exit(testutil.RunIsolatedGit(m)) }
//
// It writes a temporary global gitconfig, points GIT_CONFIG_GLOBAL at it,
// sets GIT_CONFIG_NOSYSTEM=1, runs m.Run, and removes the temporary
// gitconfig. The variables are installed with os.Setenv before any test
// runs, which — unlike t.Setenv — is safe in packages that use t.Parallel.
// If the isolation environment cannot be installed, it reports the error to
// stderr and returns 1 so the failure is visible.
func RunIsolatedGit(m *testing.M) int {
	code, err := runIsolated(m.Run, os.Setenv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testutil.RunIsolatedGit: %v\n", err)
		return 1
	}
	return code
}

// runIsolated installs the isolated git environment, runs run, removes the
// temporary gitconfig, and returns run's exit code. It is the testable core
// of RunIsolatedGit: tests pass a run func that observes the environment and
// a setenv func that can be made to fail.
func runIsolated(run func() int, setenv func(key, value string) error) (int, error) {
	cleanup, err := isolateGitEnv("", setenv)
	if err != nil {
		return 1, err
	}
	defer cleanup()
	return run(), nil
}

// isolateGitEnv writes the temporary gitconfig into tempDir ("" uses the OS
// temp directory) and sets the isolation environment variables via setenv.
// The returned cleanup removes the gitconfig file but leaves the environment
// variables in place: in TestMain usage the process is about to exit, and a
// deleted path is inert.
func isolateGitEnv(tempDir string, setenv func(key, value string) error) (func(), error) {
	f, err := os.CreateTemp(tempDir, "stet-test-gitconfig-*")
	if err != nil {
		return nil, fmt.Errorf("create temp gitconfig: %w", err)
	}
	path := f.Name()
	_, wErr := f.WriteString(gitConfigContent)
	if err := errors.Join(wErr, f.Close()); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("write %s: %w", path, err)
	}
	for _, kv := range [...][2]string{
		{"GIT_CONFIG_GLOBAL", path},
		{"GIT_CONFIG_NOSYSTEM", "1"},
	} {
		if err := setenv(kv[0], kv[1]); err != nil {
			return nil, fmt.Errorf("set %s: %w", kv[0], err)
		}
	}
	return func() { _ = os.Remove(path) }, nil
}

// TempGitDir creates a temporary directory intended to hold a git repository
// (or a directory containing one) and registers cleanup that tolerates
// concurrent external writers. Use it in place of t.TempDir() whenever the
// directory becomes or contains a git repository:
//
//	dir := testutil.TempGitDir(t)
//
// The cleanup retries os.RemoveAll up to removeMaxAttempts times with a
// linear backoff of removeBackoffStep per attempt number, because a
// git-ai-style daemon may still be writing into .git when the test finishes.
// Only a failure of the final attempt is reported via t.Errorf; assertions
// elsewhere in the test stay authoritative.
func TempGitDir(t testing.TB) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "stet-git-test-*")
	if err != nil {
		t.Fatalf("TempGitDir: %v", err)
	}
	t.Cleanup(func() { removeWithRetry(t, dir, os.RemoveAll) })
	return dir
}

// removeWithRetry removes dir, retrying transient failures (typically
// "unlinkat …: directory not empty" while an external daemon is still
// writing into the tree) with linear backoff. remove is injectable so tests
// can simulate transient and permanent failures.
func removeWithRetry(t testing.TB, dir string, remove func(string) error) {
	t.Helper()
	for attempt := 1; attempt <= removeMaxAttempts; attempt++ {
		err := remove(dir)
		if err == nil {
			return
		}
		if attempt == removeMaxAttempts {
			t.Errorf("TempGitDir cleanup: RemoveAll %s: still failing after %d attempts: %v", dir, removeMaxAttempts, err)
			return
		}
		time.Sleep(time.Duration(attempt) * removeBackoffStep)
	}
}
