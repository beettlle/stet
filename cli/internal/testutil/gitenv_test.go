package testutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeTB records Errorf reports and Cleanup registrations without a real
// *testing.T. Only Helper, Errorf, and Cleanup are safe to call on it; any
// other testing.TB method would hit the embedded nil interface.
type fakeTB struct {
	testing.TB
	errors   []string
	cleanups []func()
}

func (f *fakeTB) Helper() {}

func (f *fakeTB) Errorf(format string, args ...any) {
	f.errors = append(f.errors, fmt.Sprintf(format, args...))
}

func (f *fakeTB) Cleanup(fn func()) { f.cleanups = append(f.cleanups, fn) }

// TestRunIsolated_isolationDuringRun verifies via the run-func core that the
// environment variables are installed while run executes, that the temporary
// gitconfig contains only the default-branch setting, and that the file is
// removed afterwards.
func TestRunIsolated_isolationDuringRun(t *testing.T) {
	var globalPath, noSystem, content string
	code, err := runIsolated(func() int {
		globalPath = os.Getenv("GIT_CONFIG_GLOBAL")
		noSystem = os.Getenv("GIT_CONFIG_NOSYSTEM")
		b, readErr := os.ReadFile(globalPath)
		if readErr != nil {
			t.Errorf("read %s during run: %v", globalPath, readErr)
		}
		content = string(b)
		return 7
	}, os.Setenv)
	if err != nil {
		t.Fatalf("runIsolated: %v", err)
	}
	if code != 7 {
		t.Errorf("code = %d, want 7 (run's return value)", code)
	}
	if globalPath == "" {
		t.Fatal("GIT_CONFIG_GLOBAL not set during run")
	}
	if noSystem != "1" {
		t.Errorf("GIT_CONFIG_NOSYSTEM = %q during run, want 1", noSystem)
	}
	if !strings.Contains(content, "[init]") || !strings.Contains(content, "defaultBranch = main") {
		t.Errorf("gitconfig content = %q, want [init] with defaultBranch = main", content)
	}
	if _, statErr := os.Stat(globalPath); !os.IsNotExist(statErr) {
		t.Errorf("temp gitconfig %s not removed after run: %v", globalPath, statErr)
	}
}

// TestRunIsolated_gitConfigContentIsMinimal guards against the temporary
// gitconfig growing host-specific settings: it must be exactly the default
// branch configuration.
func TestRunIsolated_gitConfigContentIsMinimal(t *testing.T) {
	if gitConfigContent != "[init]\n\tdefaultBranch = main\n" {
		t.Errorf("gitConfigContent = %q, want exactly [init] defaultBranch = main", gitConfigContent)
	}
}

// TestRunIsolated_setenvFailureAborts verifies that a failing environment
// installation aborts before run executes and surfaces the error.
func TestRunIsolated_setenvFailureAborts(t *testing.T) {
	boom := errors.New("setenv: not allowed")
	ran := false
	code, err := runIsolated(func() int {
		ran = true
		return 0
	}, func(string, string) error { return boom })
	if err == nil || !strings.Contains(err.Error(), boom.Error()) {
		t.Errorf("err = %v, want it to wrap %v", err, boom)
	}
	if code != 1 {
		t.Errorf("code = %d, want 1 when isolation fails", code)
	}
	if ran {
		t.Error("run must not execute when the isolation environment fails to install")
	}
}

// TestIsolateGitEnv_tempDirFailure verifies the CreateTemp error path via an
// injected non-writable temp directory.
func TestIsolateGitEnv_tempDirFailure(t *testing.T) {
	cleanup, err := isolateGitEnv(filepath.Join(t.TempDir(), "missing"), os.Setenv)
	if err == nil {
		t.Error("expected error when the temp directory does not exist")
	}
	if cleanup != nil {
		cleanup()
		t.Error("cleanup must be nil when isolation fails")
	}
}

// TestRemoveWithRetry_succeedsAfterTransientFailures injects a remove func
// that fails twice before succeeding, mimicking a daemon finishing its write
// into .git.
func TestRemoveWithRetry_succeedsAfterTransientFailures(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	transient := errors.New("unlinkat .git: directory not empty")
	calls := 0
	fake := &fakeTB{}
	removeWithRetry(fake, dir, func(path string) error {
		calls++
		if calls <= 2 {
			return transient
		}
		return os.RemoveAll(path)
	})
	if calls != 3 {
		t.Errorf("remove attempts = %d, want 3", calls)
	}
	if len(fake.errors) != 0 {
		t.Errorf("unexpected error reports: %v", fake.errors)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("dir %s still exists after successful cleanup: %v", dir, err)
	}
}

// TestRemoveWithRetry_reportsFinalFailure verifies that a permanently failing
// remove is retried exactly removeMaxAttempts times and reported once via
// testing.TB.Errorf (not Fatal, so the suite keeps running).
func TestRemoveWithRetry_reportsFinalFailure(t *testing.T) {
	boom := errors.New("unlinkat: directory not empty")
	fake := &fakeTB{}
	attempts := 0
	removeWithRetry(fake, filepath.Join(t.TempDir(), "gone"), func(string) error {
		attempts++
		return boom
	})
	if attempts != removeMaxAttempts {
		t.Errorf("remove attempts = %d, want %d", attempts, removeMaxAttempts)
	}
	if len(fake.errors) != 1 {
		t.Fatalf("error reports = %d (%v), want exactly 1", len(fake.errors), fake.errors)
	}
	for _, want := range []string{"TempGitDir cleanup", fmt.Sprintf("%d attempts", removeMaxAttempts), boom.Error()} {
		if !strings.Contains(fake.errors[0], want) {
			t.Errorf("report %q missing %q", fake.errors[0], want)
		}
	}
}

// TestTempGitDir_createsDirAndRegistersCleanup verifies the happy path: the
// directory exists, exactly one cleanup is registered, and running it removes
// the directory without reporting errors.
func TestTempGitDir_createsDirAndRegistersCleanup(t *testing.T) {
	fake := &fakeTB{}
	dir := TempGitDir(fake)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("Stat %s: err=%v isDir=%v", dir, err, info != nil && info.IsDir())
	}
	if len(fake.cleanups) != 1 {
		t.Fatalf("registered cleanups = %d, want 1", len(fake.cleanups))
	}
	fake.cleanups[0]()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("dir %s still exists after cleanup: %v", dir, err)
	}
	if len(fake.errors) != 0 {
		t.Errorf("unexpected error reports: %v", fake.errors)
	}
}
