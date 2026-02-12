package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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
	// Do not run in parallel: test sets STET_OLLAMA_BASE_URL.
	// Use a closed port so Ollama is unreachable; doctor should exit 2.
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
	if got := runCLI([]string{"doctor"}); got != 2 {
		t.Errorf("runCLI(doctor) with unreachable Ollama = %d, want 2", got)
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
	required := []string{"id", "file", "line", "severity", "category", "message"}
	for i, f := range out.Findings {
		for _, k := range required {
			if _, ok := f[k]; !ok {
				t.Errorf("finding %d missing required key %q", i, k)
			}
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
	required := []string{"id", "file", "line", "severity", "category", "message"}
	for i, f := range out.Findings {
		for _, k := range required {
			if _, ok := f[k]; !ok {
				t.Errorf("finding %d missing required key %q", i, k)
			}
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
	if !bytes.Contains(buf.Bytes(), []byte("stet finish && stet start HEAD~1")) {
		t.Errorf("stderr hint missing one-liner 'stet finish && stet start HEAD~1': %q", buf.String())
	}
}
