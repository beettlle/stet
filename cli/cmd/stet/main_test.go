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
	defer func() {
		_ = os.Chdir(orig)
		findingsOut = os.Stdout
	}()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	findingsOut = &buf
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
	if got := runCLI([]string{"start", "HEAD~1", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start --dry-run --json) = %d, want 0", got)
	}
	var buf bytes.Buffer
	findingsOut = &buf
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
	if got := runCLI([]string{"start", "HEAD~2", "--dry-run", "--json"}); got != 0 {
		t.Fatalf("runCLI(start HEAD~2 --dry-run) = %d, want 0", got)
	}
	if got := runCLI([]string{"run", "--dry-run"}); got != 0 {
		t.Fatalf("runCLI(run --dry-run) = %d, want 0", got)
	}
	// After run, LastReviewedAt = HEAD so a second run would see 0 ToReview. Rerun forces full review.
	var buf bytes.Buffer
	findingsOut = &buf
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
	findingsOut = &buf
	t.Cleanup(func() { findingsOut = os.Stdout })
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
	findingsOut = &buf
	t.Cleanup(func() { findingsOut = os.Stdout })
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
	findingsOut = &buf
	t.Cleanup(func() { findingsOut = os.Stdout })
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
	findingsOut = &buf
	t.Cleanup(func() { findingsOut = os.Stdout })
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
	findingsOut = &buf
	t.Cleanup(func() { findingsOut = os.Stdout })
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
	findingsOut = &buf
	t.Cleanup(func() { findingsOut = os.Stdout })
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
	findingsOut = &buf
	t.Cleanup(func() { findingsOut = os.Stdout })
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
	_, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })
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
