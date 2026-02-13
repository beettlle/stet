package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/config"
	"stet/cli/internal/findings"
	"stet/cli/internal/history"
	"stet/cli/internal/session"
)

func TestRun(t *testing.T) {
	t.Parallel()
	// Run() uses os.Args[1:]; in test that may be empty or test flags. Just ensure it returns 0, 1, or 2.
	got := Run()
	if got != 0 && got != 1 && got != 2 {
		t.Errorf("Run() = %d, want 0, 1, or 2", got)
	}
}

func TestRunCLI_doctorUnreachableExits2(t *testing.T) {
	// Do not run in parallel: test sets STET_OLLAMA_BASE_URL and captures stderr.
	// Use a closed port so Ollama is unreachable; doctor should exit 2 and print Details.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	baseURL := "http://" + addr
	orig := os.Getenv("STET_OLLAMA_BASE_URL")
	if err := os.Setenv("STET_OLLAMA_BASE_URL", baseURL); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() {
		_ = os.Setenv("STET_OLLAMA_BASE_URL", orig)
	}()

	// Capture stderr to assert underlying error is printed for troubleshooting.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })

	got := runCLI([]string{"doctor"})
	_ = w.Close()
	var stderr bytes.Buffer
	_, _ = io.Copy(&stderr, r)

	if got != 2 {
		t.Errorf("runCLI(doctor) with unreachable Ollama = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "Details:") {
		t.Errorf("stderr should contain 'Details:' for troubleshooting; got:\n%s", stderr.String())
	}
}

func TestRunCLI_doctorModelNotFoundExits1(t *testing.T) {
	// Do not run in parallel: test sets STET_OLLAMA_BASE_URL and STET_MODEL.
	// Server returns 200 with a model list that does not include the requested model.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("path = %q, want /api/tags", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":[{"name":"other:7b"}]}`))
	}))
	defer srv.Close()
	origURL := os.Getenv("STET_OLLAMA_BASE_URL")
	origModel := os.Getenv("STET_MODEL")
	if err := os.Setenv("STET_OLLAMA_BASE_URL", srv.URL); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	if err := os.Setenv("STET_MODEL", "qwen3-coder:30b"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() {
		_ = os.Setenv("STET_OLLAMA_BASE_URL", origURL)
		_ = os.Setenv("STET_MODEL", origModel)
	}()
	if got := runCLI([]string{"doctor"}); got != 1 {
		t.Errorf("runCLI(doctor) with model not in list = %d, want 1", got)
	}
}

func TestRunCLI_helpExitsZero(t *testing.T) {
	t.Parallel()
	if got := runCLI([]string{"--help"}); got != 0 {
		t.Errorf("runCLI(--help) = %d, want 0", got)
	}
}

func TestRunCLI_startFromNonGitReturnsNonZero(t *testing.T) {
	// Do not run in parallel: test changes process cwd.
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(orig)
	}()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"start"}); got == 0 {
		t.Errorf("runCLI(start) from non-git dir = %d, want non-zero", got)
	}
}

func TestRunCLI_noArgs(t *testing.T) {
	t.Parallel()
	// No subcommand: Cobra may return 0 (show usage) or 1 depending on version.
	got := runCLI([]string{})
	if got != 0 && got != 1 {
		t.Errorf("runCLI(no args) = %d, want 0 or 1", got)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "git", "init")
	runGit(t, dir, "git", "config", "user.email", "test@stet.local")
	runGit(t, dir, "git", "config", "user.name", "Test")
	writeFile(t, dir, ".gitignore", ".review\n")
	runGit(t, dir, "git", "add", ".gitignore")
	runGit(t, dir, "git", "commit", "-m", "gitignore")
	writeFile(t, dir, "f1.txt", "a\n")
	runGit(t, dir, "git", "add", "f1.txt")
	runGit(t, dir, "git", "commit", "-m", "c1")
	writeFile(t, dir, "f2.txt", "b\n")
	runGit(t, dir, "git", "add", "f2.txt")
	runGit(t, dir, "git", "commit", "-m", "c2")
	return dir
}

func runGit(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestRunCLI_startFinishFromGitRepo(t *testing.T) {
	// Do not run in parallel: test changes process cwd.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(orig)
	}()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run"}); got != 0 {
		t.Errorf("runCLI(start HEAD~1 --dry-run) = %d, want 0", got)
	}
	if got := runCLI([]string{"run", "--dry-run"}); got != 0 {
		t.Errorf("runCLI(run --dry-run) = %d, want 0", got)
	}
	if got := runCLI([]string{"finish"}); got != 0 {
		t.Errorf("runCLI(finish) = %d, want 0", got)
	}
}

func TestRunCLI_runWithDryRun(t *testing.T) {
	// Do not run in parallel: test changes process cwd.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(orig)
	}()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	if got := runCLI([]string{"run", "--dry-run"}); got != 0 {
		t.Errorf("runCLI(run --dry-run) = %d, want 0", got)
	}
}

func TestRunCLI_finishWithoutStartReturnsNonZero(t *testing.T) {
	// Do not run in parallel: test changes process cwd.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(orig)
	}()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"finish"}); got == 0 {
		t.Errorf("runCLI(finish) without start = %d, want non-zero", got)
	}
}

func TestRunCLI_startDryRunEmitsFindingsJSON(t *testing.T) {
	// Do not run in parallel: test changes cwd and findingsOut.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(orig)
		findingsOut = os.Stdout
	}()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	findingsOut = &buf
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run"}); got != 0 {
		t.Fatalf("runCLI(start HEAD~1 --dry-run) = %d, want 0", got)
	}
	var out struct {
		Findings []map[string]interface{} `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("parse stdout JSON: %v\noutput: %s", err, buf.Bytes())
	}
	if out.Findings == nil {
		t.Fatal("output missing findings array")
	}
	if len(out.Findings) == 0 {
		t.Fatal("expected at least one finding from dry-run (initRepo has hunks)")
	}
	required := []string{"id", "file", "line", "severity", "category", "message", "confidence"}
	for i, f := range out.Findings {
		for _, k := range required {
			if _, ok := f[k]; !ok {
				t.Errorf("finding %d missing required key %q", i, k)
			}
		}
		var conf float64
		var hasNum bool
		switch v := f["confidence"].(type) {
		case float64:
			conf, hasNum = v, true
		case int:
			conf, hasNum = float64(v), true
		case int64:
			conf, hasNum = float64(v), true
		default:
			t.Errorf("finding %d: confidence must be a number, got %T", i, f["confidence"])
		}
		if hasNum && (conf < 0 || conf > 1) {
			t.Errorf("finding %d: confidence %g must be in [0, 1]", i, conf)
		}
		cat, ok := f["category"].(string)
		if !ok {
			t.Errorf("finding %d: category must be a non-empty string, got %T", i, f["category"])
		} else if cat == "" {
			t.Errorf("finding %d: category must be non-empty", i)
		}
	}
	// At least one finding should be the canned dry-run message.
	const dryRunMsg = "Dry-run placeholder (CI)"
	var found bool
	for _, f := range out.Findings {
		if m, _ := f["message"].(string); m == dryRunMsg {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no finding with message %q", dryRunMsg)
	}
}

func TestRunCLI_runDryRunEmitsFindingsJSON(t *testing.T) {
	// Do not run in parallel: test changes cwd and findingsOut.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(orig)
		findingsOut = os.Stdout
	}()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	var buf bytes.Buffer
	findingsOut = &buf
	if got := runCLI([]string{"run", "--dry-run"}); got != 0 {
		t.Fatalf("runCLI(run --dry-run) = %d, want 0", got)
	}
	var out struct {
		Findings []map[string]interface{} `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("parse stdout JSON: %v\noutput: %s", err, buf.Bytes())
	}
	if out.Findings == nil {
		t.Fatal("output missing findings array")
	}
	required := []string{"id", "file", "line", "severity", "category", "message", "confidence"}
	for i, f := range out.Findings {
		for _, k := range required {
			if _, ok := f[k]; !ok {
				t.Errorf("finding %d missing required key %q", i, k)
			}
		}
		var conf float64
		var hasNum bool
		switch v := f["confidence"].(type) {
		case float64:
			conf, hasNum = v, true
		case int:
			conf, hasNum = float64(v), true
		case int64:
			conf, hasNum = float64(v), true
		default:
			t.Errorf("finding %d: confidence must be a number, got %T", i, f["confidence"])
		}
		if hasNum && (conf < 0 || conf > 1) {
			t.Errorf("finding %d: confidence %g must be in [0, 1]", i, conf)
		}
		cat, ok := f["category"].(string)
		if !ok {
			t.Errorf("finding %d: category must be a non-empty string, got %T", i, f["category"])
		} else if cat == "" {
			t.Errorf("finding %d: category must be non-empty", i)
		}
	}
}

func TestRunCLI_startDirtyWorktreePrintsHint(t *testing.T) {
	// Do not run in parallel: test changes cwd and errHintOut.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(orig)
		errHintOut = os.Stderr
	}()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	// Uncommitted change so start fails with ErrDirtyWorktree.
	writeFile(t, repo, "dirty.txt", "uncommitted\n")
	var buf bytes.Buffer
	errHintOut = &buf
	got := runCLI([]string{"start", "HEAD~1"})
	if got == 0 {
		t.Errorf("runCLI(start) with dirty worktree = %d, want non-zero", got)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Commit or stash")) {
		t.Errorf("stderr hint missing 'Commit or stash': %q", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte("stet start")) {
		t.Errorf("stderr hint missing 'stet start': %q", buf.String())
	}
}

func TestRunCLI_startBaselineNotAncestorPrintsClearMessage(t *testing.T) {
	// Do not run in parallel: test changes cwd and stderr.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	mainBranch := runGitOut(t, repo, "git", "rev-parse", "--abbrev-ref", "HEAD")
	runGit(t, repo, "git", "checkout", "--orphan", "orphan")
	writeFile(t, repo, "orphan.txt", "x\n")
	runGit(t, repo, "git", "add", "orphan.txt")
	runGit(t, repo, "git", "commit", "-m", "orphan")
	orphanSHA := runGitOut(t, repo, "git", "rev-parse", "HEAD")
	runGit(t, repo, "git", "checkout", mainBranch)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })
	got := runCLI([]string{"start", orphanSHA, "--dry-run"})
	_ = w.Close()
	var stderr bytes.Buffer
	_, _ = io.Copy(&stderr, r)
	if got == 0 {
		t.Errorf("runCLI(start <orphan-sha>) = %d, want non-zero", got)
	}
	if !strings.Contains(stderr.String(), "not an ancestor") {
		t.Errorf("stderr should contain 'not an ancestor'; got %q", stderr.String())
	}
}

func runGitOut(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s %v: %v", name, args, err)
	}
	return strings.TrimSpace(string(out))
}

func TestRunCLI_startConcurrentLockPrintsClearMessage(t *testing.T) {
	// Do not run in parallel: test holds lock and changes cwd, errHintOut.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig); errHintOut = os.Stderr })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	cfg, _ := loadConfigForTest(repo)
	stateDir := cfg.EffectiveStateDir(repo)
	release, err := session.AcquireLock(stateDir)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer release()

	var buf bytes.Buffer
	errHintOut = &buf
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })
	got := runCLI([]string{"start", "HEAD~1", "--dry-run"})
	_ = w.Close()
	var stderrBuf bytes.Buffer
	_, _ = io.Copy(&stderrBuf, r)
	combined := buf.String() + stderrBuf.String()
	if got == 0 {
		t.Errorf("runCLI(start) with lock held = %d, want non-zero", got)
	}
	if !strings.Contains(combined, "finish or cleanup") && !strings.Contains(combined, "session already active") {
		t.Errorf("stderr should contain 'finish or cleanup' or 'session already active'; got %q", combined)
	}
}

func TestRunCLI_startAllowDirtyProceedsWithWarning(t *testing.T) {
	// Do not run in parallel: test changes cwd and stderr.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	writeFile(t, repo, "dirty.txt", "uncommitted\n")

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStderr := os.Stderr
	os.Stderr = stderrW
	t.Cleanup(func() { os.Stderr = oldStderr })
	got := runCLI([]string{"start", "HEAD~1", "--allow-dirty", "--dry-run"})
	_ = stderrW.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, stderrR)
	stderr := buf.String()
	if got != 0 {
		t.Errorf("runCLI(start --allow-dirty --dry-run) with dirty worktree = %d, want 0", got)
	}
	if !strings.Contains(stderr, "Warning") && !strings.Contains(stderr, "uncommitted") {
		t.Errorf("stderr should contain 'Warning' or 'uncommitted'; got %q", stderr)
	}
}

func TestRunCLI_startWorktreeExistsPrintsHint(t *testing.T) {
	// Do not run in parallel: test changes cwd, errHintOut, and env.
	// Mock Ollama so the second start passes the upfront Check and fails at git.Create (worktree exists).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":[{"name":"qwen3-coder:30b"}]}`))
	}))
	defer srv.Close()
	origURL := os.Getenv("STET_OLLAMA_BASE_URL")
	if err := os.Setenv("STET_OLLAMA_BASE_URL", srv.URL); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() {
		_ = os.Setenv("STET_OLLAMA_BASE_URL", origURL)
	}()

	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(orig)
		errHintOut = os.Stderr
	}()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	// Create a worktree so second start fails with ErrWorktreeExists.
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	var buf bytes.Buffer
	errHintOut = &buf
	got := runCLI([]string{"start", "HEAD~1"})
	if got == 0 {
		t.Errorf("runCLI(start) with existing worktree = %d, want non-zero", got)
	}
	if !bytes.Contains(buf.Bytes(), []byte("stet finish")) {
		t.Errorf("stderr hint missing 'stet finish': %q", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte("stet start")) {
		t.Errorf("stderr hint missing 'stet start': %q", buf.String())
	}
}

func TestRunCLI_statusNoSessionExitsNonZero(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })
	got := runCLI([]string{"status"})
	_ = w.Close()
	var stderr bytes.Buffer
	_, _ = io.Copy(&stderr, r)
	if got != 1 {
		t.Errorf("runCLI(status) with no session = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "No active session") {
		t.Errorf("stderr should contain 'No active session'; got %q", stderr.String())
	}
}

func TestRunCLI_statusWithSessionPrintsFields(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run"}); got != 0 {
		t.Fatalf("runCLI(start HEAD~1 --dry-run) = %d, want 0", got)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })
	if got := runCLI([]string{"status"}); got != 0 {
		t.Fatalf("runCLI(status) = %d, want 0", got)
	}
	_ = w.Close()
	var stdout bytes.Buffer
	_, _ = io.Copy(&stdout, r)
	out := stdout.String()
	for _, sub := range []string{"baseline:", "last_reviewed_at:", "worktree:", "findings:", "dismissed:"} {
		if !strings.Contains(out, sub) {
			t.Errorf("status output missing %q; got:\n%s", sub, out)
		}
	}
	if !strings.Contains(out, "findings: 1") && !strings.Contains(out, "findings: 2") {
		t.Errorf("status should show finding count; got:\n%s", out)
	}
}

func TestRunCLI_approveNoSessionExitsNonZero(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	got := runCLI([]string{"approve", "some-id"})
	if got != 1 {
		t.Errorf("runCLI(approve) with no session = %d, want 1", got)
	}
}

func TestRunCLI_approvePersistence(t *testing.T) {
	// Do not run in parallel: test changes cwd and findingsOut.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	findingsOut = &buf
	t.Cleanup(func() { findingsOut = os.Stdout })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	var out struct {
		Findings []map[string]interface{} `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil || len(out.Findings) == 0 {
		t.Fatalf("need at least one finding; err=%v", err)
	}
	id, _ := out.Findings[0]["id"].(string)
	if id == "" {
		t.Fatal("finding missing id")
	}
	if got := runCLI([]string{"approve", id}); got != 0 {
		t.Fatalf("runCLI(approve %q) = %d, want 0", id, got)
	}
	ro, wo, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = wo
	t.Cleanup(func() { os.Stdout = oldStdout })
	if got := runCLI([]string{"status"}); got != 0 {
		t.Fatalf("runCLI(status) = %d", got)
	}
	_ = wo.Close()
	var stdout bytes.Buffer
	_, _ = io.Copy(&stdout, ro)
	if !strings.Contains(stdout.String(), "dismissed: 1") {
		t.Errorf("status after approve should show dismissed: 1; got %s", stdout.String())
	}
}

func TestRunCLI_approveIdempotent(t *testing.T) {
	// Do not run in parallel: test changes cwd and findingsOut.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	findingsOut = &buf
	t.Cleanup(func() { findingsOut = os.Stdout })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	var out struct {
		Findings []map[string]interface{} `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil || len(out.Findings) == 0 {
		t.Fatalf("need at least one finding; err=%v", err)
	}
	id, _ := out.Findings[0]["id"].(string)
	if got := runCLI([]string{"approve", id}); got != 0 {
		t.Fatalf("first approve = %d", got)
	}
	if got := runCLI([]string{"approve", id}); got != 0 {
		t.Errorf("second approve (idempotent) = %d, want 0", got)
	}
	cfg, _ := loadConfigForTest(repo)
	stateDir := cfg.EffectiveStateDir(repo)
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	count := 0
	for _, d := range s.DismissedIDs {
		if d == id {
			count++
		}
	}
	if count != 1 {
		t.Errorf("dismissed_ids should contain id once, got %d", count)
	}
}

func TestRunCLI_approveDoesNotWritePromptShadows(t *testing.T) {
	// Do not run in parallel: test changes cwd and findingsOut.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	findingsOut = &buf
	t.Cleanup(func() { findingsOut = os.Stdout })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	var out struct {
		Findings []map[string]interface{} `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil || len(out.Findings) == 0 {
		t.Fatalf("need at least one finding; err=%v", err)
	}
	id, _ := out.Findings[0]["id"].(string)
	if got := runCLI([]string{"approve", id}); got != 0 {
		t.Fatalf("runCLI(approve %q) = %d, want 0", id, got)
	}
	cfg, _ := loadConfigForTest(repo)
	stateDir := cfg.EffectiveStateDir(repo)
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	found := false
	for _, d := range s.DismissedIDs {
		if d == id {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("after approve: dismissed_ids should contain %q", id)
	}
	if len(s.PromptShadows) != 0 {
		t.Errorf("Phase 4.4: approve must not write prompt_shadows; got len = %d", len(s.PromptShadows))
	}
}

func TestRunCLI_approveAppendsHistory(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	findingsOut = &buf
	t.Cleanup(func() { findingsOut = os.Stdout })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	var out struct {
		Findings []map[string]interface{} `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil || len(out.Findings) == 0 {
		t.Fatalf("need at least one finding; err=%v", err)
	}
	id, _ := out.Findings[0]["id"].(string)
	if id == "" {
		t.Fatal("finding missing id")
	}
	if got := runCLI([]string{"approve", id}); got != 0 {
		t.Fatalf("runCLI(approve %q) = %d, want 0", id, got)
	}
	cfg, _ := loadConfigForTest(repo)
	stateDir := cfg.EffectiveStateDir(repo)
	historyPath := filepath.Join(stateDir, "history.jsonl")
	data, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatalf("ReadFile history.jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) < 1 || lines[0] == "" {
		t.Fatalf("history.jsonl: want at least 1 line, got %d", len(lines))
	}
	var rec history.Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("Unmarshal history record: %v", err)
	}
	if len(rec.UserAction.DismissedIDs) != 1 || rec.UserAction.DismissedIDs[0] != id {
		t.Errorf("user_action.dismissed_ids: got %v, want [%q]", rec.UserAction.DismissedIDs, id)
	}
}

func TestRunCLI_approveWithReason(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	findingsOut = &buf
	t.Cleanup(func() { findingsOut = os.Stdout })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	var out struct {
		Findings []map[string]interface{} `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil || len(out.Findings) == 0 {
		t.Fatalf("need at least one finding; err=%v", err)
	}
	id, _ := out.Findings[0]["id"].(string)
	if got := runCLI([]string{"approve", id, "false_positive"}); got != 0 {
		t.Fatalf("runCLI(approve %q false_positive) = %d, want 0", id, got)
	}
	cfg, _ := loadConfigForTest(repo)
	stateDir := cfg.EffectiveStateDir(repo)
	historyPath := filepath.Join(stateDir, "history.jsonl")
	data, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatalf("ReadFile history.jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) < 1 {
		t.Fatalf("history.jsonl: want at least 1 line, got %d", len(lines))
	}
	var rec history.Record
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &rec); err != nil {
		t.Fatalf("Unmarshal last history record: %v", err)
	}
	if len(rec.UserAction.Dismissals) != 1 || rec.UserAction.Dismissals[0].FindingID != id || rec.UserAction.Dismissals[0].Reason != history.ReasonFalsePositive {
		t.Errorf("user_action.dismissals: got %+v, want [{FindingID:%q Reason:%q}]", rec.UserAction.Dismissals, id, history.ReasonFalsePositive)
	}
}

func TestRunCLI_approveInvalidReasonExits1(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	findingsOut = &buf
	t.Cleanup(func() { findingsOut = os.Stdout })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	var out struct {
		Findings []map[string]interface{} `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil || len(out.Findings) == 0 {
		t.Fatalf("need at least one finding; err=%v", err)
	}
	id, _ := out.Findings[0]["id"].(string)
	if got := runCLI([]string{"approve", id, "invalid_reason"}); got != 1 {
		t.Errorf("runCLI(approve %q invalid_reason) = %d, want 1", id, got)
	}
}

func loadConfigForTest(repoRoot string) (*config.Config, error) {
	return config.Load(context.Background(), config.LoadOptions{RepoRoot: repoRoot})
}

func TestWriteFindingsJSON_filtersDismissed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := session.Session{
		BaselineRef:    "abc",
		LastReviewedAt: "def",
		Findings: []findings.Finding{
			{ID: "f1", File: "a.go", Line: 1, Severity: findings.SeverityInfo, Category: findings.CategoryStyle, Confidence: 1.0, Message: "m1"},
			{ID: "f2", File: "b.go", Line: 2, Severity: findings.SeverityWarning, Category: findings.CategoryBug, Confidence: 1.0, Message: "m2"},
		},
		DismissedIDs: []string{"f1"},
	}
	if err := session.Save(dir, &s); err != nil {
		t.Fatalf("save session: %v", err)
	}
	var buf bytes.Buffer
	if err := writeFindingsJSON(&buf, dir); err != nil {
		t.Fatalf("writeFindingsJSON: %v", err)
	}
	var out struct {
		Findings []findings.Finding `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(out.Findings) != 1 {
		t.Fatalf("findings len = %d, want 1 (dismissed f1 excluded)", len(out.Findings))
	}
	if out.Findings[0].ID != "f2" {
		t.Errorf("remaining finding id = %q, want f2", out.Findings[0].ID)
	}
}

func TestWriteFindingsJSON_emptyFindingsEmitsArray(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := session.Session{
		BaselineRef:    "abc",
		LastReviewedAt: "def",
		Findings:       nil,
		DismissedIDs:   nil,
	}
	if err := session.Save(dir, &s); err != nil {
		t.Fatalf("save session: %v", err)
	}
	var buf bytes.Buffer
	if err := writeFindingsJSON(&buf, dir); err != nil {
		t.Fatalf("writeFindingsJSON: %v", err)
	}
	raw := buf.String()
	if strings.Contains(raw, `"findings":null`) {
		t.Errorf("output must not contain findings:null; got %q", raw)
	}
	if !strings.Contains(raw, `"findings":[]`) {
		t.Errorf("output must contain findings:[]; got %q", raw)
	}
	var out struct {
		Findings []findings.Finding `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(out.Findings) != 0 {
		t.Errorf("findings len = %d, want 0", len(out.Findings))
	}
}

func TestRunCLI_optimizeNotConfiguredExitsNonZero(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	origEnv := os.Getenv("STET_OPTIMIZER_SCRIPT")
	os.Unsetenv("STET_OPTIMIZER_SCRIPT")
	t.Cleanup(func() { _ = os.Setenv("STET_OPTIMIZER_SCRIPT", origEnv) })
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })
	got := runCLI([]string{"optimize"})
	_ = w.Close()
	var stderr bytes.Buffer
	_, _ = io.Copy(&stderr, r)
	if got != 1 {
		t.Errorf("runCLI(optimize) with no config = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "Optimizer not configured") {
		t.Errorf("stderr should mention Optimizer not configured; got %q", stderr.String())
	}
}

func TestRunCLI_optimizeScriptFailsExitsNonZero(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	origEnv := os.Getenv("STET_OPTIMIZER_SCRIPT")
	if err := os.Setenv("STET_OPTIMIZER_SCRIPT", "false"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() { _ = os.Setenv("STET_OPTIMIZER_SCRIPT", origEnv) })
	got := runCLI([]string{"optimize"})
	if got != 1 {
		t.Errorf("runCLI(optimize) with failing script = %d, want 1", got)
	}
}

func TestRunCLI_optimizeScriptSucceedsExitsZero(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	origEnv := os.Getenv("STET_OPTIMIZER_SCRIPT")
	if err := os.Setenv("STET_OPTIMIZER_SCRIPT", "true"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() { _ = os.Setenv("STET_OPTIMIZER_SCRIPT", origEnv) })
	got := runCLI([]string{"optimize"})
	if got != 0 {
		t.Errorf("runCLI(optimize) with true = %d, want 0", got)
	}
}
