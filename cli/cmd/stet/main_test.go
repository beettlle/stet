package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"stet/cli/internal/config"
	"stet/cli/internal/criticeval"
	"stet/cli/internal/findings"
	"stet/cli/internal/git"
	"stet/cli/internal/history"
	"stet/cli/internal/run"
	"stet/cli/internal/session"
	"stet/cli/internal/systemone"
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
	origProvider, hadProvider := os.LookupEnv("STET_PROVIDER")
	if err := os.Setenv("STET_OLLAMA_BASE_URL", baseURL); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	if err := os.Setenv("STET_PROVIDER", "ollama"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() {
		_ = os.Setenv("STET_OLLAMA_BASE_URL", orig)
		if hadProvider {
			_ = os.Setenv("STET_PROVIDER", origProvider)
		} else {
			_ = os.Unsetenv("STET_PROVIDER")
		}
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
	origProvider, hadProvider := os.LookupEnv("STET_PROVIDER")
	if err := os.Setenv("STET_OLLAMA_BASE_URL", srv.URL); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	if err := os.Setenv("STET_PROVIDER", "ollama"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	if err := os.Setenv("STET_MODEL", "qwen3-coder:30b"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() {
		_ = os.Setenv("STET_OLLAMA_BASE_URL", origURL)
		_ = os.Setenv("STET_MODEL", origModel)
		if hadProvider {
			_ = os.Setenv("STET_PROVIDER", origProvider)
		} else {
			_ = os.Unsetenv("STET_PROVIDER")
		}
	}()
	if got := runCLI([]string{"doctor"}); got != 1 {
		t.Errorf("runCLI(doctor) with model not in list = %d, want 1", got)
	}
}

func TestRunCLI_benchmarkUnreachableExits2(t *testing.T) {
	// Do not run in parallel: test sets STET_OLLAMA_BASE_URL.
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
	origProvider, hadProvider := os.LookupEnv("STET_PROVIDER")
	if err := os.Setenv("STET_OLLAMA_BASE_URL", baseURL); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	if err := os.Setenv("STET_PROVIDER", "ollama"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() {
		_ = os.Setenv("STET_OLLAMA_BASE_URL", orig)
		if hadProvider {
			_ = os.Setenv("STET_PROVIDER", origProvider)
		} else {
			_ = os.Unsetenv("STET_PROVIDER")
		}
	}()

	got := runCLI([]string{"benchmark"})
	if got != 2 {
		t.Errorf("runCLI(benchmark) with unreachable Ollama = %d, want 2", got)
	}
}

func TestRunCLI_benchmarkWithMockReturnsZero(t *testing.T) {
	// Do not run in parallel: test sets STET_OLLAMA_BASE_URL and STET_MODEL.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"models":[{"name":"qwen3-coder:30b"}]}`))
			return
		}
		if r.URL.Path == "/api/generate" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"response":             `[]`,
				"done":                 true,
				"model":                "qwen3-coder:30b",
				"prompt_eval_count":    500,
				"prompt_eval_duration": 250000000,
				"eval_count":           200,
				"eval_duration":        4000000000,
				"load_duration":        0,
				"total_duration":       4250000000,
			})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	origURL := os.Getenv("STET_OLLAMA_BASE_URL")
	origModel := os.Getenv("STET_MODEL")
	origProvider, hadProvider := os.LookupEnv("STET_PROVIDER")
	if err := os.Setenv("STET_OLLAMA_BASE_URL", srv.URL); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	if err := os.Setenv("STET_PROVIDER", "ollama"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	if err := os.Setenv("STET_MODEL", "qwen3-coder:30b"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() {
		_ = os.Setenv("STET_OLLAMA_BASE_URL", origURL)
		_ = os.Setenv("STET_MODEL", origModel)
		if hadProvider {
			_ = os.Setenv("STET_PROVIDER", origProvider)
		} else {
			_ = os.Unsetenv("STET_PROVIDER")
		}
	}()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })
	got := runCLI([]string{"benchmark"})
	_ = w.Close()
	var stdout bytes.Buffer
	_, _ = io.Copy(&stdout, r)

	if got != 0 {
		t.Errorf("runCLI(benchmark) with mock server = %d, want 0", got)
	}
	output := stdout.String()
	if !strings.Contains(output, "Model: qwen3-coder:30b") {
		t.Errorf("output should contain 'Model: qwen3-coder:30b'; got:\n%s", output)
	}
	if !strings.Contains(output, "t/s") {
		t.Errorf("output should contain 't/s'; got:\n%s", output)
	}
	if !strings.Contains(output, "Reference:") {
		t.Errorf("output should contain 'Reference:'; got:\n%s", output)
	}
}

func TestRunCLI_helpExitsZero(t *testing.T) {
	t.Parallel()
	if got := runCLI([]string{"--help"}); got != 0 {
		t.Errorf("runCLI(--help) = %d, want 0", got)
	}
}

func TestRunCLI_skillPrintsToStdout(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })
	got := runCLI([]string{"skill"})
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if got != 0 {
		t.Errorf("runCLI(skill) = %d, want 0", got)
	}
	out := buf.String()
	if !strings.Contains(out, "name: stet-integration") {
		t.Errorf("skill stdout must contain name: stet-integration; got:\n%s", out)
	}
	if !strings.Contains(out, "## Commands") {
		t.Errorf("skill stdout must contain ## Commands; got:\n%s", out)
	}
	if !strings.Contains(out, "stet dismiss") {
		t.Errorf("skill stdout must contain stet dismiss; got:\n%s", out)
	}
}

func TestRunCLI_skillWriteToFile(t *testing.T) {
	// Do not run in parallel: test changes process cwd.
	dir := t.TempDir()
	outPath := filepath.Join(dir, "SKILL.md")
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	got := runCLI([]string{"skill", "--output", "SKILL.md"})
	if got != 0 {
		t.Errorf("runCLI(skill -o SKILL.md) from dir %s = %d, want 0", dir, got)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "name: stet-integration") {
		t.Errorf("skill file must contain name: stet-integration; got:\n%s", content)
	}
	if !strings.Contains(content, "## Dismiss Reasons") {
		t.Errorf("skill file must contain ## Dismiss Reasons; got:\n%s", content)
	}
}

func TestRunCLI_skillOutputPathOutsideCwdRejected(t *testing.T) {
	// Do not run in parallel: test changes process cwd.
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	// Path outside cwd (e.g. ../other/SKILL.md) must be rejected.
	got := runCLI([]string{"skill", "--output", "../other/SKILL.md"})
	if got == 0 {
		t.Error("runCLI(skill -o ../other/SKILL.md) should fail when path is outside cwd")
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

func TestRunCLI_startFromNonGitStderrHumanReadable(t *testing.T) {
	// Phase 7: user-facing error must not contain raw command names or exit codes.
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
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })
	_ = runCLI([]string{"start"})
	_ = w.Close()
	var stderr bytes.Buffer
	_, _ = io.Copy(&stderr, r)
	out := stderr.String()
	// Primary user-facing lines must not contain technical jargon; "Details:" line may.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Details:") {
			continue
		}
		if strings.Contains(line, "exit status") {
			t.Errorf("primary stderr line must not contain 'exit status'; got %q", out)
		}
		if strings.Contains(line, "rev-parse") {
			t.Errorf("primary stderr line must not contain 'rev-parse'; got %q", out)
		}
	}
	if !strings.Contains(out, "not inside a Git repository") && !strings.Contains(out, "Git repository") {
		t.Errorf("stderr should contain human message about Git repository; got %q", out)
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

func TestRunCLI_startDryRunExcludePatterns(t *testing.T) {
	// Do not run in parallel: test changes process cwd and stderr.
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
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })

	got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--exclude=*.txt"})
	_ = w.Close()
	var stderr bytes.Buffer
	_, _ = io.Copy(&stderr, r)
	if got != 0 {
		t.Fatalf("runCLI(start --exclude=*.txt) = %d, want 0; stderr:\n%s", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Excluded from review: f2.txt") {
		t.Errorf("stderr should list excluded f2.txt; got:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Nothing to review") {
		t.Errorf("stderr should say nothing to review when only hunk is excluded; got:\n%s", stderr.String())
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
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
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
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
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

func TestRunCLI_strictnessInvalidExits1(t *testing.T) {
	// Do not run in parallel: test changes process cwd and stderr.
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
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })
	got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--strictness=aggressive"})
	_ = w.Close()
	var stderr bytes.Buffer
	_, _ = io.Copy(&stderr, r)
	if got != 1 {
		t.Errorf("runCLI(start --dry-run --strictness=aggressive) = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "invalid strictness") {
		t.Errorf("stderr should contain invalid strictness error; got:\n%s", stderr.String())
	}
}

func TestRunCLI_startWithStrictnessThenRunUsesSessionStrictness(t *testing.T) {
	// Start with --strictness=lenient; run without --strictness should use session strictness.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(repo, ".review")
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--strictness=lenient"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run --strictness=lenient) = %d, want 0", got)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if s.Strictness != "lenient" {
		t.Errorf("after start with --strictness=lenient: session.Strictness = %q, want lenient", s.Strictness)
	}
	if got := runCLI([]string{"run", "--dry-run"}); got != 0 {
		t.Fatalf("runCLI(run --dry-run) without strictness flag = %d, want 0 (should use session strictness)", got)
	}
	// Session strictness unchanged after run.
	s2, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session after run: %v", err)
	}
	if s2.Strictness != "lenient" {
		t.Errorf("after run: session.Strictness = %q, want lenient", s2.Strictness)
	}
}

func TestRunCLI_startDryRunEmitsFindingsJSON(t *testing.T) {
	// Do not run in parallel: test changes cwd and overrides getFindingsOut to capture output.
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
	origOut := getFindingsOut
	getFindingsOut = func() io.Writer { return &buf }
	t.Cleanup(func() { getFindingsOut = origOut })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start HEAD~1 --dry-run --json) = %d, want 0", got)
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

func TestRunCLI_startStreamEmitsNDJSON(t *testing.T) {
	// Do not run in parallel: test changes cwd and overrides getFindingsOut to capture output.
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
	origOut := getFindingsOut
	getFindingsOut = func() io.Writer { return &buf }
	t.Cleanup(func() { getFindingsOut = origOut })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--quiet", "--json", "--stream"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run --quiet --json --stream) = %d, want 0", got)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("stream output: want at least 2 lines, got %d", len(lines))
	}
	var progressCount, findingCount int
	var gotDone bool
	for _, line := range lines {
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line %q: invalid JSON: %v", line, err)
			continue
		}
		typ, _ := obj["type"].(string)
		switch typ {
		case "progress":
			progressCount++
		case "finding":
			findingCount++
			if obj["data"] == nil {
				t.Error("finding event missing data")
			}
		case "done":
			gotDone = true
		default:
			t.Errorf("unknown event type: %q", typ)
		}
		// Stream mode must not emit single {"findings": [...]} blob
		if _, hasFindings := obj["findings"]; hasFindings {
			t.Errorf("stream output must not contain top-level findings; got line: %s", line)
		}
	}
	if progressCount < 1 {
		t.Errorf("stream: want at least 1 progress event, got %d", progressCount)
	}
	if findingCount < 1 {
		t.Errorf("stream: want at least 1 finding event, got %d", findingCount)
	}
	if !gotDone {
		t.Error("stream: want done event")
	}
}

func TestRunCLI_startDryRunDefaultOutputIsHuman(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	origOut := getFindingsOut
	defer func() {
		_ = os.Chdir(orig)
		getFindingsOut = origOut
	}()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	getFindingsOut = func() io.Writer { return &buf }
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--quiet"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run --quiet) = %d, want 0", got)
	}
	out := buf.String()
	if strings.HasPrefix(strings.TrimSpace(out), `{"findings"`) {
		t.Errorf("default output should be human-readable, not JSON; got: %s", out)
	}
	if !strings.Contains(out, "finding") {
		t.Errorf("default output should contain summary line; got: %s", out)
	}
	// Human output should include short finding IDs (same format as stet list)
	if !regexp.MustCompile(`[a-f0-9]{7}  .+:\d+`).MatchString(out) {
		t.Errorf("default output should include short finding IDs at start of each line; got: %s", out)
	}
}

func TestRunCLI_runDryRunEmitsFindingsJSON(t *testing.T) {
	// Do not run in parallel: test changes cwd and overrides getFindingsOut to capture output.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	origOut := getFindingsOut
	defer func() {
		_ = os.Chdir(orig)
		getFindingsOut = origOut
	}()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run --json) = %d, want 0", got)
	}
	var buf bytes.Buffer
	getFindingsOut = func() io.Writer { return &buf }
	if got := runCLI([]string{"run", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(run --dry-run --json) = %d, want 0", got)
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

func TestRunCLI_rerunNoSessionFails(t *testing.T) {
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
	got := runCLI([]string{"rerun", "--dry-run"})
	_ = w.Close()
	var stderr bytes.Buffer
	_, _ = io.Copy(&stderr, r)
	if got != 1 {
		t.Errorf("runCLI(rerun --dry-run) with no session = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "active session") {
		t.Errorf("stderr should contain 'active session'; got %q", stderr.String())
	}
}

func TestRunCLI_rerunDryRunRerunsAllHunks(t *testing.T) {
	// Do not run in parallel: test changes cwd and overrides getFindingsOut to capture output.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	origOut := getFindingsOut
	defer func() {
		_ = os.Chdir(orig)
		getFindingsOut = origOut
	}()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"start", "HEAD~2", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start HEAD~2 --dry-run) = %d, want 0", got)
	}
	if got := runCLI([]string{"run", "--dry-run"}); got != 0 {
		t.Fatalf("runCLI(run --dry-run) = %d, want 0", got)
	}
	// After run, LastReviewedAt = HEAD so a second run would see 0 ToReview. Rerun forces full review.
	var buf bytes.Buffer
	getFindingsOut = func() io.Writer { return &buf }
	if got := runCLI([]string{"rerun", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(rerun --dry-run --json) = %d, want 0", got)
	}
	var out struct {
		Findings []map[string]interface{} `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("parse stdout JSON: %v\noutput: %s", err, buf.Bytes())
	}
	if out.Findings == nil || len(out.Findings) == 0 {
		t.Errorf("rerun should emit findings (full re-review); got %d", len(out.Findings))
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

func TestRunCLI_startInvalidBaselineRefPrintsHint(t *testing.T) {
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
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })
	got := runCLI([]string{"start", "HEAD~999", "--dry-run"})
	_ = w.Close()
	var stderr bytes.Buffer
	_, _ = io.Copy(&stderr, r)
	if got == 0 {
		t.Errorf("runCLI(start HEAD~999) = %d, want non-zero", got)
	}
	out := stderr.String()
	if !strings.Contains(out, "Could not resolve baseline ref") {
		t.Errorf("stderr should contain 'Could not resolve baseline ref'; got %q", out)
	}
	if !strings.Contains(out, "Hint:") {
		t.Errorf("stderr should contain 'Hint:' for baseline ref resolution failure; got %q", out)
	}
	if !strings.Contains(out, "shallow") {
		t.Errorf("stderr hint should mention shallow; got %q", out)
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
	got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"})
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
	origProvider, hadProvider := os.LookupEnv("STET_PROVIDER")
	if err := os.Setenv("STET_OLLAMA_BASE_URL", srv.URL); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	if err := os.Setenv("STET_PROVIDER", "ollama"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() {
		_ = os.Setenv("STET_OLLAMA_BASE_URL", origURL)
		if hadProvider {
			_ = os.Setenv("STET_PROVIDER", origProvider)
		} else {
			_ = os.Unsetenv("STET_PROVIDER")
		}
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
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
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
	out := stderr.String()
	if !strings.Contains(out, "No active session") {
		t.Errorf("stderr should contain 'No active session'; got %q", out)
	}
	// Phase 7: errExit must not be printed as primary message (no "exit 1" on stderr).
	if strings.Contains(out, "exit 1") {
		t.Errorf("stderr must not contain 'exit 1'; got %q", out)
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
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
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

func TestRunCLI_statusWithSessionPrintsSessionOptions(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--strictness=lenient", "--rag-symbol-max-definitions=5", "--rag-symbol-max-tokens=500"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run --strictness=lenient ...) = %d, want 0", got)
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
	if !strings.Contains(out, "strictness: lenient") {
		t.Errorf("status should show strictness when set at start; got:\n%s", out)
	}
	if !strings.Contains(out, "rag_symbol_max_definitions: 5") {
		t.Errorf("status should show rag_symbol_max_definitions when set at start; got:\n%s", out)
	}
	if !strings.Contains(out, "rag_symbol_max_tokens: 500") {
		t.Errorf("status should show rag_symbol_max_tokens when set at start; got:\n%s", out)
	}
}

func TestRunCLI_statusWithIdsPrintsFindingsWithIds(t *testing.T) {
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
	origOut := getFindingsOut
	getFindingsOut = func() io.Writer { return &buf }
	t.Cleanup(func() { getFindingsOut = origOut })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	var out struct {
		Findings []findings.Finding `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil || len(out.Findings) == 0 {
		t.Fatalf("need at least one finding; err=%v", err)
	}
	wantID := out.Findings[0].ID
	wantFile := out.Findings[0].File

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })
	if got := runCLI([]string{"status", "--ids"}); got != 0 {
		t.Fatalf("runCLI(status --ids) = %d, want 0", got)
	}
	_ = w.Close()
	var stdout bytes.Buffer
	_, _ = io.Copy(&stdout, r)
	outStr := stdout.String()
	if !strings.Contains(outStr, "---") {
		t.Errorf("status --ids should contain ---; got:\n%s", outStr)
	}
	shortID := findings.ShortID(wantID)
	if !strings.Contains(outStr, shortID) {
		t.Errorf("status --ids should contain short finding ID %q; got:\n%s", shortID, outStr)
	}
	if !strings.Contains(outStr, wantFile) {
		t.Errorf("status --ids should contain file %q; got:\n%s", wantFile, outStr)
	}
}

func TestRunCLI_statusWithoutIdsUnchanged(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
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
	if strings.Contains(out, "---") {
		t.Errorf("status without --ids should not contain ---; got:\n%s", out)
	}
}

func TestRunCLI_listPrintsFindingsWithIds(t *testing.T) {
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
	origOut := getFindingsOut
	getFindingsOut = func() io.Writer { return &buf }
	t.Cleanup(func() { getFindingsOut = origOut })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	var out struct {
		Findings []findings.Finding `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil || len(out.Findings) == 0 {
		t.Fatalf("need at least one finding; err=%v", err)
	}
	wantID := out.Findings[0].ID
	wantFile := out.Findings[0].File

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })
	if got := runCLI([]string{"list"}); got != 0 {
		t.Fatalf("runCLI(list) = %d, want 0", got)
	}
	_ = w.Close()
	var stdout bytes.Buffer
	_, _ = io.Copy(&stdout, r)
	outStr := stdout.String()
	shortID := findings.ShortID(wantID)
	if !strings.Contains(outStr, shortID) {
		t.Errorf("list should contain short finding ID %q; got:\n%s", shortID, outStr)
	}
	if !strings.Contains(outStr, wantFile) {
		t.Errorf("list should contain file %q; got:\n%s", wantFile, outStr)
	}
}

func TestRunCLI_listNoSessionExitsNonZero(t *testing.T) {
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
	got := runCLI([]string{"list"})
	_ = w.Close()
	var stderr bytes.Buffer
	_, _ = io.Copy(&stderr, r)
	if got != 1 {
		t.Errorf("runCLI(list) with no session = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "No active session") {
		t.Errorf("stderr should contain 'No active session'; got %q", stderr.String())
	}
}

func TestRunCLI_listEmptyFindings(t *testing.T) {
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
	origOut := getFindingsOut
	getFindingsOut = func() io.Writer { return &buf }
	t.Cleanup(func() { getFindingsOut = origOut })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	var out struct {
		Findings []findings.Finding `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil || len(out.Findings) == 0 {
		t.Fatalf("need at least one finding; err=%v", err)
	}
	for _, f := range out.Findings {
		if got := runCLI([]string{"dismiss", f.ID}); got != 0 {
			t.Fatalf("runCLI(dismiss %q) = %d, want 0", f.ID, got)
		}
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })
	if got := runCLI([]string{"list"}); got != 0 {
		t.Fatalf("runCLI(list) = %d, want 0", got)
	}
	_ = w.Close()
	var stdout bytes.Buffer
	_, _ = io.Copy(&stdout, r)
	if stdout.Len() != 0 {
		t.Errorf("list with all findings dismissed should output nothing; got:\n%s", stdout.String())
	}
}

func TestRunCLI_dismissNoSessionExitsNonZero(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	got := runCLI([]string{"dismiss", "some-id"})
	if got != 1 {
		t.Errorf("runCLI(dismiss) with no session = %d, want 1", got)
	}
}

func TestRunCLI_dismissPersistence(t *testing.T) {
	// Do not run in parallel: test changes cwd and overrides getFindingsOut to capture output.
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
	origOut := getFindingsOut
	getFindingsOut = func() io.Writer { return &buf }
	t.Cleanup(func() { getFindingsOut = origOut })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
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
	if got := runCLI([]string{"dismiss", id}); got != 0 {
		t.Fatalf("runCLI(dismiss %q) = %d, want 0", id, got)
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
		t.Errorf("status after dismiss should show dismissed: 1; got %s", stdout.String())
	}
}

func TestRunCLI_dismissIdempotent(t *testing.T) {
	// Do not run in parallel: test changes cwd and overrides getFindingsOut to capture output.
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
	origOut := getFindingsOut
	getFindingsOut = func() io.Writer { return &buf }
	t.Cleanup(func() { getFindingsOut = origOut })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	var out struct {
		Findings []map[string]interface{} `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil || len(out.Findings) == 0 {
		t.Fatalf("need at least one finding; err=%v", err)
	}
	id, _ := out.Findings[0]["id"].(string)
	if got := runCLI([]string{"dismiss", id}); got != 0 {
		t.Fatalf("first dismiss = %d", got)
	}
	if got := runCLI([]string{"dismiss", id}); got != 0 {
		t.Errorf("second dismiss (idempotent) = %d, want 0", got)
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

func TestRunCLI_dismissWritesPromptShadows(t *testing.T) {
	// Do not run in parallel: test changes cwd and overrides getFindingsOut to capture output.
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
	origOut := getFindingsOut
	getFindingsOut = func() io.Writer { return &buf }
	t.Cleanup(func() { getFindingsOut = origOut })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
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
	if got := runCLI([]string{"dismiss", id}); got != 0 {
		t.Fatalf("runCLI(dismiss %q) = %d, want 0", id, got)
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
		t.Errorf("after dismiss: dismissed_ids should contain %q", id)
	}
	if len(s.PromptShadows) != 1 {
		t.Fatalf("after dismiss: prompt_shadows len = %d, want 1", len(s.PromptShadows))
	}
	if s.PromptShadows[0].FindingID != id {
		t.Errorf("prompt_shadows[0].finding_id = %q, want %q", s.PromptShadows[0].FindingID, id)
	}
	if s.PromptShadows[0].PromptContext == "" {
		t.Error("prompt_shadows[0].prompt_context should be non-empty (hunk content)")
	}
}

func TestRunCLI_dismissWithoutPromptContext(t *testing.T) {
	// When session has a finding but no FindingPromptContext (e.g. legacy session),
	// dismiss still adds to dismissed_ids but does not append a prompt shadow.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	cfg, _ := loadConfigForTest(repo)
	stateDir := cfg.EffectiveStateDir(repo)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}
	id := "legacy-finding-id"
	s := session.Session{
		BaselineRef:    "HEAD~1",
		LastReviewedAt: "HEAD",
		Findings:       []findings.Finding{{ID: id, File: "a.go", Line: 1, Severity: findings.SeverityInfo, Category: findings.CategoryMaintainability, Confidence: 1.0, Message: "msg"}},
		// No FindingPromptContext - e.g. session from before 6.9
	}
	if err := session.Save(stateDir, &s); err != nil {
		t.Fatalf("save session: %v", err)
	}
	if got := runCLI([]string{"dismiss", id}); got != 0 {
		t.Fatalf("runCLI(dismiss %q) = %d, want 0", id, got)
	}
	s2, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if len(s2.DismissedIDs) != 1 || s2.DismissedIDs[0] != id {
		t.Errorf("dismissed_ids = %v, want [%q]", s2.DismissedIDs, id)
	}
	if len(s2.PromptShadows) != 0 {
		t.Errorf("dismiss without prompt context should not add prompt_shadows; got len = %d", len(s2.PromptShadows))
	}
}

func TestRunCLI_dismissAppendsHistory(t *testing.T) {
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
	origOut := getFindingsOut
	getFindingsOut = func() io.Writer { return &buf }
	t.Cleanup(func() { getFindingsOut = origOut })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
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
	if got := runCLI([]string{"dismiss", id}); got != 0 {
		t.Fatalf("runCLI(dismiss %q) = %d, want 0", id, got)
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

func TestRunCLI_dismissWithReason(t *testing.T) {
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
	origOut := getFindingsOut
	getFindingsOut = func() io.Writer { return &buf }
	t.Cleanup(func() { getFindingsOut = origOut })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	var out struct {
		Findings []map[string]interface{} `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil || len(out.Findings) == 0 {
		t.Fatalf("need at least one finding; err=%v", err)
	}
	id, _ := out.Findings[0]["id"].(string)
	if got := runCLI([]string{"dismiss", id, "false_positive"}); got != 0 {
		t.Fatalf("runCLI(dismiss %q false_positive) = %d, want 0", id, got)
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

func TestRunCLI_dismissInvalidReasonExits1(t *testing.T) {
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
	origOut := getFindingsOut
	getFindingsOut = func() io.Writer { return &buf }
	t.Cleanup(func() { getFindingsOut = origOut })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	var out struct {
		Findings []map[string]interface{} `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil || len(out.Findings) == 0 {
		t.Fatalf("need at least one finding; err=%v", err)
	}
	id, _ := out.Findings[0]["id"].(string)
	if got := runCLI([]string{"dismiss", id, "invalid_reason"}); got != 1 {
		t.Errorf("runCLI(dismiss %q invalid_reason) = %d, want 1", id, got)
	}
}

func TestRunCLI_dismissByShortPrefix(t *testing.T) {
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
	origOut := getFindingsOut
	getFindingsOut = func() io.Writer { return &buf }
	t.Cleanup(func() { getFindingsOut = origOut })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	var out struct {
		Findings []findings.Finding `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil || len(out.Findings) == 0 {
		t.Fatalf("need at least one finding; err=%v", err)
	}
	fullID := out.Findings[0].ID
	shortID := findings.ShortID(fullID)
	if len(shortID) < 4 {
		t.Skip("short ID too short for prefix resolution minimum")
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { w.Close(); r.Close(); os.Stdout = oldStdout })
	if got := runCLI([]string{"dismiss", shortID}); got != 0 {
		t.Fatalf("runCLI(dismiss %q) = %d, want 0", shortID, got)
	}
	_ = w.Close()
	cfg, err := loadConfigForTest(repo)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	s, err := session.Load(cfg.EffectiveStateDir(repo))
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	var found bool
	for _, d := range s.DismissedIDs {
		if d == fullID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("dismiss by short prefix %q should have added full ID %q to DismissedIDs; got %v", shortID, fullID, s.DismissedIDs)
	}
}

func TestRunCLI_dismissNoFindingWithIdExits1(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	if got := runCLI([]string{"dismiss", "0000000"}); got != 1 {
		t.Errorf("runCLI(dismiss 0000000) = %d, want 1 (no finding with that id)", got)
	}
}

func TestRunCLI_dismissIdTooShortExits1(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	if got := runCLI([]string{"dismiss", "abc"}); got != 1 {
		t.Errorf("runCLI(dismiss abc) = %d, want 1 (id must be at least 4 characters)", got)
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

func TestRunCLI_cleanupNotGitRepoExits1(t *testing.T) {
	// Run cleanup from a directory that is not a git repo.
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	got := runCLI([]string{"cleanup"})
	if got != 1 {
		t.Errorf("runCLI(cleanup) from non-repo = %d, want 1", got)
	}
}

func TestRunCLI_cleanupNoOrphansExitsZero(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	got := runCLI([]string{"cleanup"})
	if got != 0 {
		t.Errorf("runCLI(cleanup) with no worktrees = %d, want 0", got)
	}
}

func TestRunCLI_cleanupRemovesOrphanWhenNoSession(t *testing.T) {
	// No session; create one orphan stet worktree manually, then cleanup removes it.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	worktreesDir := filepath.Join(repo, ".review", "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		t.Fatalf("mkdir worktrees: %v", err)
	}
	shortSha := runGitOut(t, repo, "git", "rev-parse", "--short=12", "HEAD~1")
	orphanPath := filepath.Join(worktreesDir, "stet-"+shortSha)
	runGit(t, repo, "git", "worktree", "add", orphanPath, "HEAD~1")
	if _, err := os.Stat(orphanPath); err != nil {
		t.Fatalf("orphan worktree should exist: %v", err)
	}

	got := runCLI([]string{"cleanup"})
	if got != 0 {
		t.Errorf("runCLI(cleanup) = %d, want 0", got)
	}
	if _, err := os.Stat(orphanPath); !os.IsNotExist(err) {
		t.Errorf("orphan worktree should be removed; stat err: %v", err)
	}
	listOut := runGitOut(t, repo, "git", "worktree", "list", "--porcelain")
	if strings.Contains(listOut, "stet-") {
		t.Errorf("git worktree list should not contain stet- worktrees after cleanup; got:\n%s", listOut)
	}
}

func TestRunCLI_cleanupRemovesOnlyOrphanWhenSessionExists(t *testing.T) {
	// Start a session (creates session worktree at HEAD~1). Add another worktree at HEAD~2 (orphan). Cleanup removes only the orphan.
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start) = %d, want 0", got)
	}
	// Session worktree is at .review/worktrees/stet-<sha of HEAD~1>. Create orphan at HEAD~2.
	worktreesDir := filepath.Join(repo, ".review", "worktrees")
	shortSha2 := runGitOut(t, repo, "git", "rev-parse", "--short=12", "HEAD~2")
	orphanPath := filepath.Join(worktreesDir, "stet-"+shortSha2)
	runGit(t, repo, "git", "worktree", "add", orphanPath, "HEAD~2")
	if _, err := os.Stat(orphanPath); err != nil {
		t.Fatalf("orphan worktree should exist: %v", err)
	}

	got := runCLI([]string{"cleanup"})
	if got != 0 {
		t.Errorf("runCLI(cleanup) = %d, want 0", got)
	}
	// Orphan (HEAD~2) should be removed.
	if _, err := os.Stat(orphanPath); !os.IsNotExist(err) {
		t.Errorf("orphan worktree should be removed; stat err: %v", err)
	}
	// Session worktree (HEAD~1) should still exist. Path = worktrees/stet-<sha of HEAD~1>.
	shortSha1 := runGitOut(t, repo, "git", "rev-parse", "--short=12", "HEAD~1")
	sessionPath := filepath.Join(worktreesDir, "stet-"+shortSha1)
	if _, err := os.Stat(sessionPath); err != nil {
		t.Errorf("session worktree should still exist: %v", err)
	}
	// Finish to clean up for next tests (optional; t.TempDir is cleaned anyway).
	_ = runCLI([]string{"finish"})
}

func TestRunCLI_statsVolumeFromNonRepoExits1(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	got := runCLI([]string{"stats", "volume", "--format=json"})
	if got != 1 {
		t.Errorf("runCLI(stats volume) from non-repo = %d, want 1", got)
	}
}

func TestRunCLI_statsVolumeJSONInRepoWithNote(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	if got := runCLI([]string{"finish"}); got != 0 {
		t.Fatalf("runCLI(finish) = %d, want 0", got)
	}
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })
	got := runCLI([]string{"stats", "volume", "--format=json", "--since=HEAD~1", "--until=HEAD"})
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if got != 0 {
		t.Fatalf("runCLI(stats volume --format=json) = %d, want 0\nstderr/out: %s", got, buf.String())
	}
	var out struct {
		SessionsCount          int     `json:"sessions_count"`
		CommitsInRange         int     `json:"commits_in_range"`
		CommitsWithNote        int     `json:"commits_with_note"`
		PercentCommitsWithNote float64 `json:"percent_commits_with_note"`
		TotalHunksReviewed     int     `json:"total_hunks_reviewed"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &out); err != nil {
		t.Fatalf("parse stats volume JSON: %v\noutput: %s", err, buf.Bytes())
	}
	if out.CommitsInRange < 1 {
		t.Errorf("commits_in_range: got %d, want >= 1", out.CommitsInRange)
	}
	if out.SessionsCount < 1 || out.CommitsWithNote < 1 {
		t.Errorf("sessions_count=%d commits_with_note=%d, want >= 1 after finish", out.SessionsCount, out.CommitsWithNote)
	}
	if out.CommitsInRange > 0 && out.PercentCommitsWithNote < 0 {
		t.Errorf("percent_commits_with_note: got %f", out.PercentCommitsWithNote)
	}
}

func TestRunCLI_statsVolumeJSONWithGitAI(t *testing.T) {
	repo := initRepo(t)
	headSHA := runGitOut(t, repo, "git", "rev-parse", "HEAD")
	note := `src/main.go
  abcd1234abcd1234 1-5
---
{"schema_version":"authorship/3.0.0","base_commit_sha":"` + headSHA + `","prompts":{"abcd1234abcd1234":{"agent_id":{"tool":"cursor","id":"x","model":"y"},"messages":[],"total_additions":5,"total_deletions":0,"accepted_lines":5,"overriden_lines":0}}}`
	if err := git.AddNote(repo, git.NotesRefAI, headSHA, note); err != nil {
		t.Fatalf("AddNote git-ai: %v", err)
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })
	got := runCLI([]string{"stats", "volume", "--format=json", "--since=HEAD~1", "--until=HEAD"})
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if got != 0 {
		t.Fatalf("runCLI(stats volume --format=json) = %d, want 0\nstderr/out: %s", got, buf.String())
	}
	var out struct {
		GitAI *struct {
			CommitsWithAINote   int `json:"commits_with_ai_note"`
			TotalAIAuthoredLines int `json:"total_ai_authored_lines"`
		} `json:"git_ai"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &out); err != nil {
		t.Fatalf("parse stats volume JSON: %v\noutput: %s", err, buf.Bytes())
	}
	if out.GitAI == nil {
		t.Fatal("git_ai: want non-nil when git-ai notes exist")
	}
	if out.GitAI.CommitsWithAINote != 1 {
		t.Errorf("git_ai.commits_with_ai_note: got %d, want 1", out.GitAI.CommitsWithAINote)
	}
	if out.GitAI.TotalAIAuthoredLines != 5 {
		t.Errorf("git_ai.total_ai_authored_lines: got %d, want 5", out.GitAI.TotalAIAuthoredLines)
	}
}

func TestRunCLI_statsVolumeHumanWithGitAI(t *testing.T) {
	repo := initRepo(t)
	headSHA := runGitOut(t, repo, "git", "rev-parse", "HEAD")
	note := `src/x.go
  aaaa1111aaaa1111 1-3
---
{"schema_version":"authorship/3.0.0","base_commit_sha":"` + headSHA + `","prompts":{"aaaa1111aaaa1111":{"agent_id":{"tool":"cursor","id":"x","model":"y"},"messages":[],"total_additions":3,"total_deletions":0,"accepted_lines":3,"overriden_lines":0}}}`
	if err := git.AddNote(repo, git.NotesRefAI, headSHA, note); err != nil {
		t.Fatalf("AddNote git-ai: %v", err)
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })
	got := runCLI([]string{"stats", "volume", "--since=HEAD~1", "--until=HEAD"})
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if got != 0 {
		t.Fatalf("runCLI(stats volume) = %d, want 0\nstderr/out: %s", got, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "Git AI (refs/notes/ai)") {
		t.Errorf("human output should contain 'Git AI (refs/notes/ai)'; got:\n%s", out)
	}
	if !strings.Contains(out, "Commits with AI note: 1") {
		t.Errorf("human output should contain 'Commits with AI note: 1'; got:\n%s", out)
	}
	if !strings.Contains(out, "Total AI-authored lines: 3") {
		t.Errorf("human output should contain 'Total AI-authored lines: 3'; got:\n%s", out)
	}
}

func TestRunCLI_statsQualityFromNonRepoExits1(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	got := runCLI([]string{"stats", "quality", "--format=json"})
	if got != 1 {
		t.Errorf("runCLI(stats quality) from non-repo = %d, want 1", got)
	}
}

func TestRunCLI_statsQualityJSONInRepo(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	// Run start --dry-run + finish so history.jsonl has at least one record.
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	if got := runCLI([]string{"finish"}); got != 0 {
		t.Fatalf("runCLI(finish) = %d, want 0", got)
	}
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })
	got := runCLI([]string{"stats", "quality", "--format=json"})
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if got != 0 {
		t.Fatalf("runCLI(stats quality --format=json) = %d, want 0\nstderr/out: %s", got, buf.String())
	}
	var out struct {
		SessionsCount      int                `json:"sessions_count"`
		TotalFindings      int                `json:"total_findings"`
		TotalDismissed     int                `json:"total_dismissed"`
		DismissalRate      float64            `json:"dismissal_rate"`
		CategoryBreakdown  map[string]int     `json:"category_breakdown"`
		DismissalsByReason map[string]int     `json:"dismissals_by_reason"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &out); err != nil {
		t.Fatalf("parse stats quality JSON: %v\noutput: %s", err, buf.Bytes())
	}
	if out.SessionsCount < 1 {
		t.Errorf("sessions_count: got %d, want >= 1 after finish", out.SessionsCount)
	}
	if out.CategoryBreakdown == nil {
		t.Error("category_breakdown: want non-nil map")
	}
	if out.DismissalsByReason == nil {
		t.Error("dismissals_by_reason: want non-nil map")
	}
}

func TestRunCLI_statsEnergyFromNonRepoExits1(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	got := runCLI([]string{"stats", "energy", "--format=json"})
	if got != 1 {
		t.Errorf("runCLI(stats energy) from non-repo = %d, want 1", got)
	}
}

func TestRunCLI_statsEnergyJSONInRepo(t *testing.T) {
	repo := initRepo(t)
	headSHA := runGitOut(t, repo, "git", "rev-parse", "HEAD")
	note := `{"session_id":"s1","baseline_sha":"` + runGitOut(t, repo, "git", "rev-parse", "HEAD~2") + `","head_sha":"` + headSHA + `","findings_count":0,"dismissals_count":0,"tool_version":"test","finished_at":"2025-01-01T00:00:00Z","eval_duration_ns":3600000000000,"prompt_tokens":1000000,"completion_tokens":100000}`
	if err := git.AddNote(repo, git.NotesRefStet, headSHA, note); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })
	got := runCLI([]string{"stats", "energy", "--format=json", "--since=HEAD~1", "--until=HEAD", "--cloud-model=gpt-4o-mini"})
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if got != 0 {
		t.Fatalf("runCLI(stats energy --format=json) = %d, want 0\nstderr/out: %s", got, buf.String())
	}
	var out struct {
		SessionsCount         int                `json:"sessions_count"`
		TotalEvalDurationNs   int64              `json:"total_eval_duration_ns"`
		TotalPromptTokens     int64              `json:"total_prompt_tokens"`
		TotalCompletionTokens int64              `json:"total_completion_tokens"`
		LocalEnergyKWh        float64            `json:"local_energy_kwh"`
		CloudCostAvoided      map[string]float64 `json:"cloud_cost_avoided"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &out); err != nil {
		t.Fatalf("parse stats energy JSON: %v\noutput: %s", err, buf.Bytes())
	}
	if out.SessionsCount != 1 {
		t.Errorf("sessions_count: got %d, want 1", out.SessionsCount)
	}
	if out.TotalEvalDurationNs != 3600000000000 {
		t.Errorf("total_eval_duration_ns: got %d, want 3600000000000", out.TotalEvalDurationNs)
	}
	if out.TotalPromptTokens != 1000000 || out.TotalCompletionTokens != 100000 {
		t.Errorf("tokens: got prompt=%d completion=%d, want 1000000, 100000", out.TotalPromptTokens, out.TotalCompletionTokens)
	}
	if out.LocalEnergyKWh < 0.029 || out.LocalEnergyKWh > 0.031 {
		t.Errorf("local_energy_kwh: got %.4f, want ~0.03 (1h at 30W)", out.LocalEnergyKWh)
	}
	if out.CloudCostAvoided == nil {
		t.Fatal("cloud_cost_avoided: want non-nil map")
	}
	if cost := out.CloudCostAvoided["gpt-4o-mini"]; cost < 0.20 || cost > 0.22 {
		t.Errorf("cloud_cost_avoided[gpt-4o-mini]: got %.4f, want ~0.21", cost)
	}
}

func TestParseContextPreset(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      string
		want    int
		wantErr bool
	}{
		{"4k", "4k", 4096, false},
		{"8k", "8k", 8192, false},
		{"16k", "16k", 16384, false},
		{"32k", "32k", 32768, false},
		{"64k", "64k", 65536, false},
		{"128k", "128k", 131072, false},
		{"256k", "256k", 262144, false},
		{"32k uppercase", "32K", 32768, false},
		{"4k mixed", "4K", 4096, false},
		{"invalid 1k", "1k", 0, true},
		{"invalid 33k", "33k", 0, true},
		{"invalid no k", "32", 0, true},
		{"empty", "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseContextPreset(tt.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseContextPreset(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseContextPreset(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestErrExit_Error(t *testing.T) {
	t.Parallel()
	if got := errExit(2).Error(); got != "exit 2" {
		t.Errorf("got %q", got)
	}
}

func TestFormatLLMRejected(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	formatLLMRejected(&buf, "openai", "http://localhost:1234/v1", errors.New("bad request"))
	out := buf.String()
	if !strings.Contains(out, "openai") || !strings.Contains(out, "bad request") {
		t.Errorf("output = %q", out)
	}
}

func TestPrintLLMUnreachable_timeoutHint(t *testing.T) {
	var stderr bytes.Buffer
	orig := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	printLLMUnreachable("ollama", "http://localhost:11434", context.DeadlineExceeded)
	_ = w.Close()
	_, _ = stderr.ReadFrom(r)
}

func TestWriteFindingsHuman_singleFinding(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := session.Session{
		BaselineRef: "abc",
		Findings: []findings.Finding{
			{ID: "finding-id-1234", File: "a.go", Line: 10, Severity: findings.SeverityWarning, Category: findings.CategoryBug, Confidence: 1.0, Message: "fix me"},
		},
	}
	if err := session.Save(dir, &s); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := writeFindingsHuman(&buf, dir, nil); err != nil {
		t.Fatalf("writeFindingsHuman: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "a.go:10") || !strings.Contains(out, "fix me") || !strings.Contains(out, "1 finding.") {
		t.Errorf("output = %q", out)
	}
}

func TestWriteFindingsHuman_withStats(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := session.Session{
		BaselineRef: "abc",
		Findings: []findings.Finding{
			{ID: "f1", File: "a.go", Line: 1, Severity: findings.SeverityInfo, Category: findings.CategoryStyle, Confidence: 1.0, Message: "m1"},
			{ID: "f2", File: "b.go", Line: 2, Severity: findings.SeverityInfo, Category: findings.CategoryStyle, Confidence: 1.0, Message: "m2"},
		},
	}
	if err := session.Save(dir, &s); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	stats := &run.RunStats{EvalDurationNs: 1_000_000_000, PromptTokens: 100, CompletionTokens: 50}
	if err := writeFindingsHuman(&buf, dir, stats); err != nil {
		t.Fatalf("writeFindingsHuman: %v", err)
	}
	if !strings.Contains(buf.String(), "tokens/sec") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestWriteFindingsHuman_singleFindingWithStats(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := session.Session{
		BaselineRef: "abc",
		Findings: []findings.Finding{
			{ID: "finding-id-1234", File: "a.go", Line: 10, Severity: findings.SeverityWarning, Category: findings.CategoryBug, Confidence: 1.0, Message: "fix me"},
		},
	}
	if err := session.Save(dir, &s); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	stats := &run.RunStats{EvalDurationNs: 2_000_000_000, PromptTokens: 200, CompletionTokens: 100}
	if err := writeFindingsHuman(&buf, dir, stats); err != nil {
		t.Fatalf("writeFindingsHuman: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "1 finding at") || !strings.Contains(out, "tokens/sec") {
		t.Errorf("output = %q", out)
	}
}

func TestWriteFindingsWithIDs_usesRangeStart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	start := 5
	s := session.Session{
		BaselineRef: "abc",
		Findings: []findings.Finding{
			{ID: "f1", File: "a.go", Line: 1, Range: &findings.LineRange{Start: start, End: 7}, Severity: findings.SeverityInfo, Category: findings.CategoryStyle, Confidence: 1.0, Message: "m"},
		},
	}
	if err := session.Save(dir, &s); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := writeFindingsWithIDs(&buf, dir); err != nil {
		t.Fatalf("writeFindingsWithIDs: %v", err)
	}
	if !strings.Contains(buf.String(), "a.go:5") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestRunCLI_commitMsgPrintsSuggestion(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "change.txt", "new\n")
	runGit(t, repo, "git", "add", "change.txt")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/tags") || strings.HasSuffix(r.URL.Path, "/models") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"models": []map[string]interface{}{{"name": "m"}}})
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"response":"feat: add change","done":true}` + "\n"))
	}))
	defer srv.Close()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	origURL := os.Getenv("STET_OLLAMA_BASE_URL")
	_ = os.Setenv("STET_OLLAMA_BASE_URL", srv.URL)
	t.Cleanup(func() { _ = os.Setenv("STET_OLLAMA_BASE_URL", origURL) })
	var stdout bytes.Buffer
	origOut := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origOut })
	if got := runCLI([]string{"commitmsg"}); got != 0 {
		t.Fatalf("runCLI(commitmsg) = %d, want 0", got)
	}
	_ = w.Close()
	_, _ = stdout.ReadFrom(r)
}

func TestOverridesFromFlags_noFlagsNil(t *testing.T) {
	cmd := newStartCmd()
	o, err := overridesFromFlags(cmd)
	if err != nil {
		t.Fatalf("overridesFromFlags: %v", err)
	}
	if o != nil {
		t.Errorf("got %+v, want nil", o)
	}
}

func TestOverridesFromFlags_providerOpenAIAndTimeout(t *testing.T) {
	cmd := newStartCmd()
	if err := cmd.Flags().Set("provider", "openai"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("openai-base-url", "http://lm:1234/v1"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("timeout", "90s"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("context", "32k"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("rag-call-graph", "true"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("verify", "true"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("exclude", "*.md"); err != nil {
		t.Fatal(err)
	}
	o, err := overridesFromFlags(cmd)
	if err != nil {
		t.Fatalf("overridesFromFlags: %v", err)
	}
	if o == nil || o.Provider == nil || *o.Provider != "openai" {
		t.Fatalf("provider override: %+v", o)
	}
	if o.OpenAIBaseURL == nil || *o.OpenAIBaseURL != "http://lm:1234/v1" {
		t.Errorf("openai base url: %+v", o)
	}
	if o.Timeout == nil || *o.Timeout != 90*time.Second {
		t.Errorf("timeout: %+v", o)
	}
	if o.ContextLimit == nil || *o.ContextLimit != 32768 {
		t.Errorf("context: %+v", o)
	}
	if o.RAGCallGraphEnabled == nil || !*o.RAGCallGraphEnabled {
		t.Errorf("rag call graph: %+v", o)
	}
	if o.CriticEnabled == nil || !*o.CriticEnabled {
		t.Errorf("critic: %+v", o)
	}
	if o.ExcludePatterns == nil || len(*o.ExcludePatterns) != 1 {
		t.Errorf("exclude: %+v", o)
	}
}

func TestOverridesFromFlags_invalidProvider(t *testing.T) {
	cmd := newStartCmd()
	if err := cmd.Flags().Set("provider", "bad"); err != nil {
		t.Fatal(err)
	}
	_, err := overridesFromFlags(cmd)
	if err == nil {
		t.Fatal("expected error for invalid provider")
	}
}

func TestOverridesFromFlags_negativeNumCtx(t *testing.T) {
	cmd := newStartCmd()
	if err := cmd.Flags().Set("num-ctx", "-1"); err != nil {
		t.Fatal(err)
	}
	_, err := overridesFromFlags(cmd)
	if err == nil {
		t.Fatal("expected error for negative num-ctx")
	}
	if !strings.Contains(err.Error(), "--num-ctx must be 0 or positive") {
		t.Errorf("error = %v, want num-ctx validation message", err)
	}
}

func TestRunCLI_statsQualityHumanInRepo(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"stats", "quality"}); got != 0 {
		t.Fatalf("runCLI(stats quality) = %d, want 0", got)
	}
}

func TestRunCLI_statsEnergyHumanInRepo(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"stats", "energy"}); got != 0 {
		t.Fatalf("runCLI(stats energy) = %d, want 0", got)
	}
}

func TestRunCLI_runAfterStartDryRun(t *testing.T) {
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
		t.Fatalf("start = %d", got)
	}
	if got := runCLI([]string{"run", "--dry-run"}); got != 0 {
		t.Fatalf("run = %d", got)
	}
}

func TestRunCLI_rerunAfterStartDryRun(t *testing.T) {
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
		t.Fatalf("start = %d", got)
	}
	if got := runCLI([]string{"rerun", "--dry-run"}); got != 0 {
		t.Fatalf("rerun = %d", got)
	}
}

func TestWriteFindingsJSON_writeError(t *testing.T) {
	t.Parallel()
	err := writeFindingsJSON(&failWriteCloser{}, t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFormatLLMRejected_emptyBaseURL(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	formatLLMRejected(&buf, "ollama", "", errors.New("rejected"))
	if !strings.Contains(buf.String(), "(no URL)") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestWriteFindingsHuman_multipleFindingsNoStats(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := session.Session{
		BaselineRef: "abc",
		Findings: []findings.Finding{
			{ID: "f1", File: "a.go", Line: 1, Severity: findings.SeverityInfo, Category: findings.CategoryStyle, Confidence: 1.0, Message: "m1"},
			{ID: "f2", File: "b.go", Line: 2, Severity: findings.SeverityWarning, Category: findings.CategoryBug, Confidence: 1.0, Message: "m2"},
		},
	}
	if err := session.Save(dir, &s); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := writeFindingsHuman(&buf, dir, nil); err != nil {
		t.Fatalf("writeFindingsHuman: %v", err)
	}
	if !strings.Contains(buf.String(), "2 finding(s).") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestRunCLI_commitMsgNoChangesExits1(t *testing.T) {
	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"commitmsg"}); got != 1 {
		t.Errorf("runCLI(commitmsg) with no changes = %d, want 1", got)
	}
}

func TestRunCLI_commitMsgUnreachableLLMExits2(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "x.txt", "change\n")
	runGit(t, repo, "git", "add", "x.txt")
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	origURL := os.Getenv("STET_OLLAMA_BASE_URL")
	_ = os.Setenv("STET_OLLAMA_BASE_URL", "http://127.0.0.1:1")
	t.Cleanup(func() { _ = os.Setenv("STET_OLLAMA_BASE_URL", origURL) })
	if got := runCLI([]string{"commitmsg"}); got != 2 {
		t.Errorf("runCLI(commitmsg) unreachable = %d, want 2", got)
	}
}

func TestRunCLI_doctorReachableExits0(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":[{"name":"qwen3-coder:30b"}]}`))
	}))
	defer srv.Close()
	origURL := os.Getenv("STET_OLLAMA_BASE_URL")
	origModel := os.Getenv("STET_MODEL")
	_ = os.Setenv("STET_OLLAMA_BASE_URL", srv.URL)
	_ = os.Setenv("STET_MODEL", "qwen3-coder:30b")
	t.Cleanup(func() {
		_ = os.Setenv("STET_OLLAMA_BASE_URL", origURL)
		_ = os.Setenv("STET_MODEL", origModel)
	})
	if got := runCLI([]string{"doctor"}); got != 0 {
		t.Errorf("runCLI(doctor) reachable = %d, want 0", got)
	}
}

func TestFindingsWriter_nilUsesStdout(t *testing.T) {
	old := getFindingsOut
	getFindingsOut = func() io.Writer { return nil }
	t.Cleanup(func() { getFindingsOut = old })
	w := findingsWriter()
	if w == nil {
		t.Fatal("findingsWriter should never return nil")
	}
}

func TestPrintLLMUnreachable_emptyBaseURL(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })
	printLLMUnreachable("openai", "", errors.New("dial"))
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if !strings.Contains(buf.String(), "(no URL)") {
		t.Errorf("stderr = %q", buf.String())
	}
}

func TestRunCLI_startFinishCleanupDryRun(t *testing.T) {
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
		t.Fatalf("start = %d", got)
	}
	if got := runCLI([]string{"finish"}); got != 0 {
		t.Fatalf("finish = %d", got)
	}
	if got := runCLI([]string{"cleanup"}); got != 0 {
		t.Fatalf("cleanup = %d", got)
	}
}

func TestTryAutoFinishZero_zeroFindingsFinishes(t *testing.T) {
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
	origOut := getFindingsOut
	getFindingsOut = func() io.Writer { return &buf }
	t.Cleanup(func() { getFindingsOut = origOut })
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run) = %d, want 0", got)
	}
	var out struct {
		Findings []map[string]interface{} `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil || len(out.Findings) == 0 {
		t.Fatalf("need findings from dry-run; err=%v", err)
	}
	for _, f := range out.Findings {
		id, _ := f["id"].(string)
		if id == "" {
			t.Fatal("finding missing id")
		}
		if got := runCLI([]string{"dismiss", id}); got != 0 {
			t.Fatalf("runCLI(dismiss %q) = %d, want 0", id, got)
		}
	}
	stateDir := filepath.Join(repo, ".review")
	cfg := &config.Config{AutoFinishZeroFindings: true}
	if err := tryAutoFinishZero(context.Background(), cfg, repo, stateDir, false); err != nil {
		t.Fatalf("tryAutoFinishZero: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if s.BaselineRef != "" {
		t.Errorf("session should be cleared after auto-finish; BaselineRef = %q", s.BaselineRef)
	}
}

func TestRunCLI_commitMsgStagedOnlyNoStagedExits1(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "unstaged.txt", "x\n")
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	if got := runCLI([]string{"commitmsg", "--staged-only"}); got != 1 {
		t.Errorf("runCLI(commitmsg --staged-only) = %d, want 1", got)
	}
}

type failWriteCloser struct{}

func (failWriteCloser) Write([]byte) (int, error) { return 0, errors.New("fail") }

func TestOverridesFromFlags_criticBackendSystemOne(t *testing.T) {
	cmd := newStartCmd()
	if err := cmd.Flags().Set("verify", "true"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("critic-backend", "systemone"); err != nil {
		t.Fatal(err)
	}
	o, err := overridesFromFlags(cmd)
	if err != nil {
		t.Fatalf("overridesFromFlags: %v", err)
	}
	if o == nil || o.CriticBackend == nil || *o.CriticBackend != "systemone" {
		t.Fatalf("critic-backend override: %+v", o)
	}
	if o.CriticEnabled == nil || !*o.CriticEnabled {
		t.Errorf("critic enabled: %+v", o)
	}
	// run and rerun accept the same flag (addRunLikeFlags).
	runCmd := newRunCmd()
	if err := runCmd.Flags().Set("critic-backend", "LLM"); err != nil {
		t.Fatal(err)
	}
	o2, err := overridesFromFlags(runCmd)
	if err != nil {
		t.Fatalf("overridesFromFlags(run): %v", err)
	}
	if o2 == nil || o2.CriticBackend == nil || *o2.CriticBackend != "llm" {
		t.Fatalf("run critic-backend override: %+v", o2)
	}
	rerunCmd := newRerunCmd()
	if err := rerunCmd.Flags().Set("critic-backend", "SystemOne"); err != nil {
		t.Fatal(err)
	}
	o3, err := overridesFromFlags(rerunCmd)
	if err != nil {
		t.Fatalf("overridesFromFlags(rerun): %v", err)
	}
	if o3 == nil || o3.CriticBackend == nil || *o3.CriticBackend != "systemone" {
		t.Fatalf("rerun critic-backend override: %+v", o3)
	}
}

func TestOverridesFromFlags_invalidCriticBackend(t *testing.T) {
	cmd := newStartCmd()
	if err := cmd.Flags().Set("critic-backend", "bogus"); err != nil {
		t.Fatal(err)
	}
	if _, err := overridesFromFlags(cmd); err == nil {
		t.Fatal("expected error for invalid --critic-backend")
	}
}

func TestSystemOneRunOptions(t *testing.T) {
	t.Run("nil client when critic disabled", func(t *testing.T) {
		cfg := &config.Config{CriticEnabled: false, CriticBackend: "systemone", SystemOneBaseURL: "http://127.0.0.1:8080", SystemOneModel: "openjev-latest", SystemOneMinKeep: 0.5, SystemOneSamples: 2}
		client, baseURL, model, minKeep, samples := systemOneRunOptions(cfg)
		if client != nil {
			t.Errorf("client = %v, want nil when critic disabled", client)
		}
		if baseURL != "http://127.0.0.1:8080" || model != "openjev-latest" || minKeep != 0.5 || samples != 2 {
			t.Errorf("fields = %q %q %v %d, want config values", baseURL, model, minKeep, samples)
		}
	})
	t.Run("nil client when backend llm", func(t *testing.T) {
		cfg := &config.Config{CriticEnabled: true, CriticBackend: "llm", SystemOneBaseURL: "http://127.0.0.1:8080"}
		client, _, _, _, _ := systemOneRunOptions(cfg)
		if client != nil {
			t.Errorf("client = %v, want nil when backend llm", client)
		}
	})
	t.Run("client built when enabled and systemone", func(t *testing.T) {
		cfg := &config.Config{CriticEnabled: true, CriticBackend: "systemone", SystemOneBaseURL: "http://127.0.0.1:9090/", SystemOneAPIKey: "k", SystemOneModel: "openjev-latest", SystemOneMinKeep: 0.5, SystemOneSamples: 3}
		client, baseURL, model, minKeep, samples := systemOneRunOptions(cfg)
		if client == nil {
			t.Fatal("client = nil, want non-nil when critic enabled and backend systemone")
		}
		if baseURL != "http://127.0.0.1:9090/" || model != "openjev-latest" || minKeep != 0.5 || samples != 3 {
			t.Errorf("fields = %q %q %v %d, want config values", baseURL, model, minKeep, samples)
		}
	})
}

// TestRunCLI_startSystemOneCriticDropsLowScoredFinding runs an end-to-end
// `stet start --verify --critic-backend systemone` against a mock Ollama that
// reports one finding and a mock System One server that scores it P(yes)=0.1
// (below the default systemone_min_keep of 0.5). The critic must query the
// System One server (with the env-only API key as bearer token) and the
// low-scored finding must be dropped from the session and the output.
func TestRunCLI_startSystemOneCriticDropsLowScoredFinding(t *testing.T) {
	// Do not run in parallel: test changes cwd, stdout/stderr, and env.
	var sysOneRequests []systemone.Request
	var sawAuthKey bool
	sysSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/systemone" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") == "Bearer test-key-123" {
			sawAuthKey = true
		}
		var req systemone.Request
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		sysOneRequests = append(sysOneRequests, req)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(systemone.Response{Model: req.Model, Answers: map[string]systemone.NoulAnswer{"f0": {Noul: 0.1}}})
	}))
	defer sysSrv.Close()

	llmResp := `[{"file":"a.go","line":1,"severity":"warning","category":"style","message":"mock finding"}]`
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"models": []map[string]interface{}{{"name": "m"}}})
			return
		}
		if r.URL.Path != "/api/generate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": llmResp, "done": true})
	}))
	defer llmSrv.Close()

	setTestEnv(t, "STET_PROVIDER", "ollama")
	setTestEnv(t, "STET_OLLAMA_BASE_URL", llmSrv.URL)
	setTestEnv(t, "STET_SYSTEMONE_BASE_URL", sysSrv.URL)
	setTestEnv(t, "STET_SYSTEMONE_MODEL", "openjev-test")
	setTestEnv(t, "STET_SYSTEMONE_API_KEY", "test-key-123")

	repo := initRepo(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}

	// Capture stdout (findings JSON) and stderr (progress/errors) to assert the API key never leaks.
	stdoutR, stdoutW, _ := os.Pipe()
	stderrR, stderrW, _ := os.Pipe()
	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutW, stderrW
	got := runCLI([]string{"start", "HEAD~1", "--verify", "--critic-backend", "systemone", "--json"})
	os.Stdout, os.Stderr = oldStdout, oldStderr
	_ = stdoutW.Close()
	_ = stderrW.Close()
	var stdoutBuf, stderrBuf bytes.Buffer
	_, _ = stdoutBuf.ReadFrom(stdoutR)
	_, _ = stderrBuf.ReadFrom(stderrR)
	combined := stdoutBuf.String() + stderrBuf.String()

	if got != 0 {
		t.Fatalf("runCLI(start --verify --critic-backend systemone) = %d, want 0; stderr: %s", got, stderrBuf.String())
	}
	if len(sysOneRequests) == 0 {
		t.Fatal("System One server received no requests; critic did not run")
	}
	if sysOneRequests[0].Model != "openjev-test" {
		t.Errorf("System One request model = %q, want openjev-test (from STET_SYSTEMONE_MODEL)", sysOneRequests[0].Model)
	}
	if !sawAuthKey {
		t.Error("System One request missing Authorization: Bearer test-key-123 (STET_SYSTEMONE_API_KEY not wired)")
	}
	s, err := session.Load(filepath.Join(repo, ".review"))
	if err != nil {
		t.Fatalf("session.Load: %v", err)
	}
	if len(s.Findings) != 0 {
		t.Errorf("session findings = %d, want 0 (P(yes)=0.1 below default min_keep 0.5 drops the finding)", len(s.Findings))
	}
	if strings.Contains(combined, "test-key-123") {
		t.Error("API key must never appear in CLI output")
	}
}

// setTestEnv sets env and restores the previous value at test cleanup.
func setTestEnv(t *testing.T, key, val string) {
	t.Helper()
	orig, had := os.LookupEnv(key)
	if err := os.Setenv(key, val); err != nil {
		t.Fatalf("setenv %s: %v", key, err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, orig)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

// doctorLLMServer returns a mock Ollama server whose /api/tags lists the given
// model names, so the doctor LLM check passes.
func doctorLLMServer(models ...string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		type tagModel struct {
			Name string `json:"name"`
		}
		resp := struct {
			Models []tagModel `json:"models"`
		}{}
		for _, name := range models {
			resp.Models = append(resp.Models, tagModel{Name: name})
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// doctorSysOneServer returns a mock System One server for doctor tests with
// canned responses for /health and /v1/models. Each request it receives is
// appended to *reqs as "METHOD path" so tests can assert exactly which
// endpoints doctor called (never POST /v1/systemone).
func doctorSysOneServer(t *testing.T, reqs *[]string, healthStatus int, healthBody string, modelsStatus int, modelsBody string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reqs = append(*reqs, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(healthStatus)
			_, _ = w.Write([]byte(healthBody))
			return
		case "/v1/models":
			w.WriteHeader(modelsStatus)
			_, _ = w.Write([]byte(modelsBody))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// setDoctorSysOneEnv configures the environment for doctor System One tests:
// a healthy mock Ollama, the critic enabled with the systemone backend, and
// the given System One server URL and model. STET_SYSTEMONE_API_KEY is set so
// tests can also assert the key never leaks into output.
func setDoctorSysOneEnv(t *testing.T, llmURL, sysOneURL string) {
	t.Helper()
	setTestEnv(t, "STET_PROVIDER", "ollama")
	setTestEnv(t, "STET_OLLAMA_BASE_URL", llmURL)
	setTestEnv(t, "STET_MODEL", "m")
	setTestEnv(t, "STET_CRITIC_ENABLED", "1")
	setTestEnv(t, "STET_CRITIC_BACKEND", "systemone")
	setTestEnv(t, "STET_SYSTEMONE_BASE_URL", sysOneURL)
	setTestEnv(t, "STET_SYSTEMONE_MODEL", "openjev-test")
	setTestEnv(t, "STET_SYSTEMONE_API_KEY", "doctor-test-key")
}

// runCLIWithOutput runs runCLI(args) capturing os.Stdout and os.Stderr.
// Tests using it must not run in parallel.
func runCLIWithOutput(args []string) (code int, stdout, stderr string) {
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutW, stderrW
	code = runCLI(args)
	os.Stdout, os.Stderr = oldStdout, oldStderr
	_ = stdoutW.Close()
	_ = stderrW.Close()
	var stdoutBuf, stderrBuf bytes.Buffer
	_, _ = stdoutBuf.ReadFrom(stdoutR)
	_, _ = stderrBuf.ReadFrom(stderrR)
	return code, stdoutBuf.String(), stderrBuf.String()
}

// assertNoSystemOneDecisionCall fails the test if doctor issued any request
// other than /health or /v1/models (e.g. POST /v1/systemone).
func assertNoSystemOneDecisionCall(t *testing.T, reqs []string) {
	t.Helper()
	for _, req := range reqs {
		if strings.Contains(req, "/v1/systemone") {
			t.Errorf("doctor issued a decision request %q; it must only check /health and /v1/models", req)
		}
	}
}

// TestRunCLI_doctorSystemOneOK runs doctor with the critic enabled on the
// systemone backend against healthy mock Ollama and System One servers. It
// must exit 0, print the OK lines, and only touch /health and /v1/models.
func TestRunCLI_doctorSystemOneOK(t *testing.T) {
	// Do not run in parallel: test sets env and captures stdout/stderr.
	var sysReqs []string
	sysSrv := doctorSysOneServer(t, &sysReqs,
		http.StatusOK, `{"status":"ok"}`,
		http.StatusOK, `{"models":[{"name":"openjev-test"},{"name":"openjev-0.1"}]}`)
	defer sysSrv.Close()
	llmSrv := doctorLLMServer("m")
	defer llmSrv.Close()
	setDoctorSysOneEnv(t, llmSrv.URL, sysSrv.URL)

	code, stdout, stderr := runCLIWithOutput([]string{"doctor"})
	if code != 0 {
		t.Errorf("runCLI(doctor) with healthy System One = %d, want 0; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "System One OK") {
		t.Errorf("stdout should contain 'System One OK'; got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "System One model: openjev-test") {
		t.Errorf("stdout should contain 'System One model: openjev-test'; got:\n%s", stdout)
	}
	assertNoSystemOneDecisionCall(t, sysReqs)
	if len(sysReqs) != 2 {
		t.Errorf("System One server requests = %v, want exactly [GET /health GET /v1/models]", sysReqs)
	}
	if strings.Contains(stdout+stderr, "doctor-test-key") {
		t.Error("API key must never appear in doctor output")
	}
}

// TestRunCLI_doctorSystemOneUnreachableExits2 points the System One base URL
// at a closed port; doctor must exit 2 and report the server as unreachable.
// The client retries unreachable connections, so this test takes a few seconds.
func TestRunCLI_doctorSystemOneUnreachableExits2(t *testing.T) {
	// Do not run in parallel: test sets env and captures stdout/stderr.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	baseURL := "http://" + addr
	llmSrv := doctorLLMServer("m")
	defer llmSrv.Close()
	setDoctorSysOneEnv(t, llmSrv.URL, baseURL)

	code, _, stderr := runCLIWithOutput([]string{"doctor"})
	if code != 2 {
		t.Errorf("runCLI(doctor) with unreachable System One = %d, want 2; stderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "System One server unreachable at "+baseURL) {
		t.Errorf("stderr should contain 'System One server unreachable at %s'; got:\n%s", baseURL, stderr)
	}
}

// TestRunCLI_doctorSystemOneHealthUnauthorizedExits2 returns 401 from /health;
// doctor must exit 2, hint at STET_SYSTEMONE_API_KEY, and never print the key.
func TestRunCLI_doctorSystemOneHealthUnauthorizedExits2(t *testing.T) {
	// Do not run in parallel: test sets env and captures stdout/stderr.
	var sysReqs []string
	sysSrv := doctorSysOneServer(t, &sysReqs,
		http.StatusUnauthorized, `{"error":"invalid api key"}`,
		http.StatusOK, `{"models":[{"name":"openjev-test"}]}`)
	defer sysSrv.Close()
	llmSrv := doctorLLMServer("m")
	defer llmSrv.Close()
	setDoctorSysOneEnv(t, llmSrv.URL, sysSrv.URL)

	code, stdout, stderr := runCLIWithOutput([]string{"doctor"})
	if code != 2 {
		t.Errorf("runCLI(doctor) with 401 System One health = %d, want 2; stderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "STET_SYSTEMONE_API_KEY") {
		t.Errorf("stderr should hint at STET_SYSTEMONE_API_KEY; got:\n%s", stderr)
	}
	// Health failed, so Models must not have been requested.
	if len(sysReqs) != 1 || sysReqs[0] != "GET /health" {
		t.Errorf("System One server requests = %v, want only [GET /health]", sysReqs)
	}
	if strings.Contains(stdout+stderr, "doctor-test-key") {
		t.Error("API key must never appear in doctor output")
	}
}

// TestRunCLI_doctorSystemOneModelsUnauthorizedExits2 returns 401 from
// /v1/models only; doctor must exit 2 with the API key hint.
func TestRunCLI_doctorSystemOneModelsUnauthorizedExits2(t *testing.T) {
	// Do not run in parallel: test sets env and captures stdout/stderr.
	var sysReqs []string
	sysSrv := doctorSysOneServer(t, &sysReqs,
		http.StatusOK, `{"status":"ok"}`,
		http.StatusUnauthorized, `{"error":"invalid api key"}`)
	defer sysSrv.Close()
	llmSrv := doctorLLMServer("m")
	defer llmSrv.Close()
	setDoctorSysOneEnv(t, llmSrv.URL, sysSrv.URL)

	code, _, stderr := runCLIWithOutput([]string{"doctor"})
	if code != 2 {
		t.Errorf("runCLI(doctor) with 401 System One models = %d, want 2; stderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "STET_SYSTEMONE_API_KEY") {
		t.Errorf("stderr should hint at STET_SYSTEMONE_API_KEY; got:\n%s", stderr)
	}
}

// TestRunCLI_doctorSystemOneModelNotFoundExits1 returns a healthy server whose
// model list does not include the configured model; doctor must exit 1 and
// print the configured model, the base URL, and the available names.
func TestRunCLI_doctorSystemOneModelNotFoundExits1(t *testing.T) {
	// Do not run in parallel: test sets env and captures stdout/stderr.
	var sysReqs []string
	sysSrv := doctorSysOneServer(t, &sysReqs,
		http.StatusOK, `{"status":"ok"}`,
		http.StatusOK, `{"models":[{"name":"openjev-latest"},{"name":"openjev-0.1"}]}`)
	defer sysSrv.Close()
	llmSrv := doctorLLMServer("m")
	defer llmSrv.Close()
	setDoctorSysOneEnv(t, llmSrv.URL, sysSrv.URL)

	code, stdout, stderr := runCLIWithOutput([]string{"doctor"})
	if code != 1 {
		t.Errorf("runCLI(doctor) with missing System One model = %d, want 1; stderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, `System One model "openjev-test" not found at `+sysSrv.URL) {
		t.Errorf("stderr should name the missing model and base URL; got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "openjev-latest, openjev-0.1") {
		t.Errorf("stderr should list available model names; got:\n%s", stderr)
	}
	if strings.Contains(stdout, "System One OK") {
		t.Errorf("stdout must not print OK when the model is missing; got:\n%s", stdout)
	}
}

// TestRunCLI_doctorSystemOneDisabledMakesNoRequest verifies doctor output is
// unchanged when the critic is disabled, and when it is enabled but uses the
// llm backend: no System One request is made and no System One line is printed.
func TestRunCLI_doctorSystemOneDisabledMakesNoRequest(t *testing.T) {
	// Do not run in parallel: test sets env and captures stdout/stderr.
	var sysReqs []string
	sysSrv := doctorSysOneServer(t, &sysReqs,
		http.StatusOK, `{"status":"ok"}`,
		http.StatusOK, `{"models":[{"name":"openjev-test"}]}`)
	defer sysSrv.Close()
	llmSrv := doctorLLMServer("m")
	defer llmSrv.Close()
	setTestEnv(t, "STET_PROVIDER", "ollama")
	setTestEnv(t, "STET_OLLAMA_BASE_URL", llmSrv.URL)
	setTestEnv(t, "STET_MODEL", "m")
	setTestEnv(t, "STET_SYSTEMONE_BASE_URL", sysSrv.URL)
	setTestEnv(t, "STET_SYSTEMONE_MODEL", "openjev-test")

	// Critic disabled (default): doctor must not contact the System One server.
	code, stdout, _ := runCLIWithOutput([]string{"doctor"})
	if code != 0 {
		t.Errorf("runCLI(doctor) with critic disabled = %d, want 0", code)
	}
	if len(sysReqs) != 0 {
		t.Errorf("System One server requests = %v with critic disabled, want none", sysReqs)
	}
	if strings.Contains(stdout, "System One") {
		t.Errorf("stdout must not mention System One when the critic is disabled; got:\n%s", stdout)
	}

	// Critic enabled but backend llm: still no System One requests.
	setTestEnv(t, "STET_CRITIC_ENABLED", "1")
	setTestEnv(t, "STET_CRITIC_BACKEND", "llm")
	code, stdout, _ = runCLIWithOutput([]string{"doctor"})
	if code != 0 {
		t.Errorf("runCLI(doctor) with critic backend llm = %d, want 0", code)
	}
	if len(sysReqs) != 0 {
		t.Errorf("System One server requests = %v with critic backend llm, want none", sysReqs)
	}
	if strings.Contains(stdout, "System One") {
		t.Errorf("stdout must not mention System One when the critic backend is llm; got:\n%s", stdout)
	}
}

// ===== critic-eval command tests =====

// criticEvalSysOneServer returns a mock System One decision server for
// critic-eval tests. Every POST /v1/systemone body is answered with noul 0.9
// when the body contains the "critic-yes" marker (hunk of a real finding) and
// noul 0.1 otherwise (false-positive hunk); every requested question key gets
// an answer. Each request is appended to *reqs as "METHOD path".
func criticEvalSysOneServer(t *testing.T, reqs *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reqs = append(*reqs, r.Method+" "+r.URL.Path)
		if r.URL.Path != "/v1/systemone" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var req systemone.Request
		if err := json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		noul := 0.1
		if strings.Contains(string(body), "critic-yes") {
			noul = 0.9
		}
		answers := make(map[string]systemone.NoulAnswer, len(req.Questions))
		for key := range req.Questions {
			answers[key] = systemone.NoulAnswer{Noul: noul}
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(systemone.Response{Model: req.Model, Answers: answers})
	}))
}

// writeCriticEvalDatasetN writes a curated dataset with keeps keep findings
// whose hunks contain the "critic-yes" marker (the mock server answers 0.9)
// and drops drop findings whose hunks contain "critic-no" (answers 0.1).
// Hunks are unique so every example is its own hunk group (one decision
// request each), keeping request counts deterministic.
func writeCriticEvalDatasetN(t *testing.T, keeps, drops int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dataset.jsonl")
	var b strings.Builder
	for i := 1; i <= keeps; i++ {
		fmt.Fprintf(&b, `{"finding":{"id":"k%d","file":"keep.go","line":%d,"severity":"warning","category":"bug","confidence":0.9,"message":"real issue %d"},"hunk":"// critic-yes\nfunc keep%d() { _ = %d }\n","label":"keep"}`+"\n", i, i, i, i, i)
	}
	for i := 1; i <= drops; i++ {
		fmt.Fprintf(&b, `{"finding":{"id":"d%d","file":"drop.go","line":%d,"severity":"style","category":"style","confidence":0.5,"message":"nit %d"},"hunk":"// critic-no\nfunc drop%d() { _ = %d }\n","label":"drop"}`+"\n", i, i, i, i, i)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write dataset: %v", err)
	}
	return path
}

// setCriticEvalEnv configures System One for critic-eval tests: base URL,
// model, sample count, and a test API key that tests assert never leaks into
// the command output.
func setCriticEvalEnv(t *testing.T, sysOneURL string) {
	t.Helper()
	setTestEnv(t, "STET_SYSTEMONE_BASE_URL", sysOneURL)
	setTestEnv(t, "STET_SYSTEMONE_MODEL", "openjev-test")
	setTestEnv(t, "STET_SYSTEMONE_SAMPLES", "1")
	setTestEnv(t, "STET_SYSTEMONE_API_KEY", "critic-eval-test-key")
}

// TestRunCLI_criticEvalHuman runs critic-eval with a curated dataset against
// the mock System One server. It must exit 0 and print the human report:
// label and source counts, the threshold table, latency, the recommendation,
// the small-sample warning, and never the API key. It must work with the
// critic disabled (critic-eval is an evaluation tool, not a run).
func TestRunCLI_criticEvalHuman(t *testing.T) {
	// Do not run in parallel: test sets env and captures stdout/stderr.
	var sysReqs []string
	sysSrv := criticEvalSysOneServer(t, &sysReqs)
	defer sysSrv.Close()
	datasetPath := writeCriticEvalDatasetN(t, 3, 3)
	setCriticEvalEnv(t, sysSrv.URL)

	code, stdout, stderr := runCLIWithOutput([]string{"critic-eval", "--dataset", datasetPath})
	if code != 0 {
		t.Fatalf("runCLI(critic-eval) = %d, want 0; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Dataset: 3 keep, 3 drop") {
		t.Errorf("stdout should contain per-label counts; got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Sources: curated 6") {
		t.Errorf("stdout should contain per-source counts; got:\n%s", stdout)
	}
	for _, col := range []string{"threshold", "false positives dropped", "real findings lost", "kept"} {
		if !strings.Contains(stdout, col) {
			t.Errorf("stdout table should contain column %q; got:\n%s", col, stdout)
		}
	}
	if !strings.Contains(stdout, "Latency: p50 ") {
		t.Errorf("stdout should contain latency percentiles; got:\n%s", stdout)
	}
	// Keeps all score 0.9, so even threshold 0.9 loses no real findings:
	// the recommendation must be the highest sweep threshold.
	if !strings.Contains(stdout, "Recommended systemone_min_keep: 0.9") {
		t.Errorf("stdout should recommend 0.9; got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Warning: recommendation unreliable") {
		t.Errorf("stdout should warn the recommendation is unreliable (3+3 examples < 10 per label); got:\n%s", stdout)
	}
	if len(sysReqs) != 6 {
		t.Errorf("System One requests = %v, want 6 POST /v1/systemone (one per unique hunk)", sysReqs)
	}
	for _, req := range sysReqs {
		if req != "POST /v1/systemone" {
			t.Errorf("unexpected request %q; critic-eval must only POST /v1/systemone", req)
		}
	}
	if strings.Contains(stdout+stderr, "critic-eval-test-key") {
		t.Error("API key must never appear in critic-eval output")
	}
}

// TestRunCLI_criticEvalJSON runs critic-eval with --json: the stdout must be
// a JSON document that parses to the criticeval.Report shape with the expected
// counts, nine sweep thresholds, the recommendation, and the warning.
func TestRunCLI_criticEvalJSON(t *testing.T) {
	// Do not run in parallel: test sets env and captures stdout/stderr.
	var sysReqs []string
	sysSrv := criticEvalSysOneServer(t, &sysReqs)
	defer sysSrv.Close()
	datasetPath := writeCriticEvalDatasetN(t, 3, 3)
	setCriticEvalEnv(t, sysSrv.URL)

	code, stdout, stderr := runCLIWithOutput([]string{"critic-eval", "--dataset", datasetPath, "--json"})
	if code != 0 {
		t.Fatalf("runCLI(critic-eval --json) = %d, want 0; stderr: %s", code, stderr)
	}
	var rep criticeval.Report
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("stdout is not a valid critic eval report: %v\nstdout:\n%s", err, stdout)
	}
	if rep.Counts[criticeval.LabelKeep] != 3 || rep.Counts[criticeval.LabelDrop] != 3 {
		t.Errorf("Counts = %v, want keep 3 drop 3", rep.Counts)
	}
	if len(rep.Thresholds) != 9 {
		t.Errorf("len(Thresholds) = %d, want 9 (0.1..0.9)", len(rep.Thresholds))
	}
	if rep.Recommended == nil || *rep.Recommended != 0.9 {
		t.Errorf("Recommended = %v, want 0.9", rep.Recommended)
	}
	if len(rep.Warnings) != 1 || rep.Warnings[0] != "recommendation unreliable" {
		t.Errorf("Warnings = %v, want [recommendation unreliable]", rep.Warnings)
	}
	if rep.LatencyP50 < 0 || rep.LatencyP95 < rep.LatencyP50 {
		t.Errorf("latency percentiles inconsistent: p50=%v p95=%v", rep.LatencyP50, rep.LatencyP95)
	}
	if strings.Contains(stdout+stderr, "critic-eval-test-key") {
		t.Error("API key must never appear in critic-eval JSON output")
	}
}

// TestRunCLI_criticEvalEmptyStateExits1 runs critic-eval with no --dataset
// against an empty state dir: it must exit 1 with a clear message explaining
// how to create labeled examples, and must not contact System One.
func TestRunCLI_criticEvalEmptyStateExits1(t *testing.T) {
	// Do not run in parallel: test sets env and captures stdout/stderr.
	var sysReqs []string
	sysSrv := criticEvalSysOneServer(t, &sysReqs)
	defer sysSrv.Close()
	setCriticEvalEnv(t, sysSrv.URL)
	setTestEnv(t, "STET_STATE_DIR", t.TempDir()) // exists but has no history/session

	code, _, stderr := runCLIWithOutput([]string{"critic-eval"})
	if code != 1 {
		t.Fatalf("runCLI(critic-eval) with empty state = %d, want 1", code)
	}
	if !strings.Contains(stderr, "No labeled examples for critic eval") {
		t.Errorf("stderr should explain there are no labeled examples; got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "false_positive") || !strings.Contains(stderr, "--dataset") {
		t.Errorf("stderr should explain how to create examples (dismiss with false_positive/wrong_suggestion or pass --dataset); got:\n%s", stderr)
	}
	if len(sysReqs) != 0 {
		t.Errorf("System One requests = %v, want none when there is no dataset", sysReqs)
	}
}

// TestRunCLI_criticEvalUnreachableExits2 points the System One base URL at a
// closed port: critic-eval must exit 2 and name the base URL, matching stet
// doctor. Uses a one-example dataset so only one retried request (~7s).
func TestRunCLI_criticEvalUnreachableExits2(t *testing.T) {
	// Do not run in parallel: test sets env and captures stdout/stderr.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	baseURL := "http://" + addr
	datasetPath := writeCriticEvalDatasetN(t, 1, 0)
	setCriticEvalEnv(t, baseURL)

	code, _, stderr := runCLIWithOutput([]string{"critic-eval", "--dataset", datasetPath})
	if code != 2 {
		t.Fatalf("runCLI(critic-eval) with unreachable System One = %d, want 2; stderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "System One server unreachable at "+baseURL) {
		t.Errorf("stderr should contain the unreachable message with the base URL; got:\n%s", stderr)
	}
	if strings.Contains(stderr, "critic-eval-test-key") {
		t.Error("API key must never appear in critic-eval output")
	}
}

// TestRunCLI_criticEvalMissingDatasetExits1 passes a --dataset path that does
// not exist: critic-eval must exit 1 and report the load failure.
func TestRunCLI_criticEvalMissingDatasetExits1(t *testing.T) {
	// Do not run in parallel: test sets env and captures stdout/stderr.
	var sysReqs []string
	sysSrv := criticEvalSysOneServer(t, &sysReqs)
	defer sysSrv.Close()
	setCriticEvalEnv(t, sysSrv.URL)

	missing := filepath.Join(t.TempDir(), "missing.jsonl")
	code, _, stderr := runCLIWithOutput([]string{"critic-eval", "--dataset", missing})
	if code != 1 {
		t.Fatalf("runCLI(critic-eval --dataset <missing>) = %d, want 1", code)
	}
	if !strings.Contains(stderr, "Could not load curated dataset.") {
		t.Errorf("stderr should report the dataset load failure; got:\n%s", stderr)
	}
	if len(sysReqs) != 0 {
		t.Errorf("System One requests = %v, want none when the dataset fails to load", sysReqs)
	}
}

// TestPrintCriticEvalReportNoRecommendation unit-tests the human report when
// no threshold qualifies: the no-threshold message must be printed, the
// recommendation line must not, and a nil dataset must omit the Sources line.
func TestPrintCriticEvalReportNoRecommendation(t *testing.T) {
	t.Parallel()
	rep := criticeval.Report{Counts: map[criticeval.Label]int{criticeval.LabelKeep: 4, criticeval.LabelDrop: 1}}
	var buf bytes.Buffer
	printCriticEvalReport(&buf, nil, rep)
	out := buf.String()
	if !strings.Contains(out, "Dataset: 4 keep, 1 drop") {
		t.Errorf("report should contain per-label counts; got:\n%s", out)
	}
	if !strings.Contains(out, "No threshold keeps real-finding loss ≤ 10%.") {
		t.Errorf("report should contain the no-threshold message when Recommended is nil; got:\n%s", out)
	}
	if strings.Contains(out, "Recommended systemone_min_keep") {
		t.Errorf("report must not recommend a threshold when Recommended is nil; got:\n%s", out)
	}
	if strings.Contains(out, "Sources:") {
		t.Errorf("nil dataset should omit the Sources line; got:\n%s", out)
	}
}
