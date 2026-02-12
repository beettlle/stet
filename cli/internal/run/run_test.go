package run

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/git"
	"stet/cli/internal/session"
)

const dryRunMsg = "Dry-run placeholder (CI)"

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

func runOut(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s %v: %v", name, args, err)
	}
	return strings.TrimSpace(string(out))
}

func TestStart_createsSessionAndWorktree(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot:      repo,
		StateDir:      stateDir,
		WorktreeRoot:  "",
		Ref:           "HEAD~1",
		DryRun:        true,
		Model:         "",
		OllamaBaseURL: "",
	}
	err := Start(ctx, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if s.BaselineRef == "" {
		t.Error("session.BaselineRef: want non-empty after Start")
	}
	list, err := git.List(repo)
	if err != nil {
		t.Fatalf("List worktrees: %v", err)
	}
	wantPath, err := git.PathForRef(repo, "", s.BaselineRef)
	if err != nil {
		t.Fatalf("PathForRef: %v", err)
	}
	found := false
	for _, w := range list {
		if w.Path == wantPath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("worktree path %q not in list: %+v", wantPath, list)
	}
}

func TestStart_refEqualsHEAD_skipsWorktree(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: "HEAD", DryRun: true,
		Model: "", OllamaBaseURL: "",
	}
	err := Start(ctx, opts)
	if err != nil {
		t.Fatalf("Start(HEAD): %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if s.BaselineRef == "" {
		t.Error("session.BaselineRef: want non-empty")
	}
	if s.LastReviewedAt == "" {
		t.Error("session.LastReviewedAt: want non-empty when baseline is HEAD")
	}
	list, err := git.List(repo)
	if err != nil {
		t.Fatalf("List worktrees: %v", err)
	}
	for _, w := range list {
		if strings.Contains(w.Path, "stet-") {
			t.Errorf("Start(HEAD) should not create worktree; found %q", w.Path)
		}
	}
}

func TestStart_requiresCleanWorktree(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	writeFile(t, repo, "dirty.txt", "x\n")
	opts := StartOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: "HEAD", DryRun: true}
	err := Start(ctx, opts)
	if err == nil {
		t.Fatal("Start with dirty worktree: expected error")
	}
	if !errors.Is(err, ErrDirtyWorktree) {
		t.Errorf("Start: got %v, want ErrDirtyWorktree", err)
	}
}

func TestStart_allowDirty_skipsCleanCheck(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	writeFile(t, repo, "dirty.txt", "x\n")
	opts := StartOptions{
		RepoRoot:      repo,
		StateDir:      stateDir,
		WorktreeRoot:  "",
		Ref:           "HEAD~1",
		DryRun:        true,
		AllowDirty:    true,
		Model:         "",
		OllamaBaseURL: "",
	}
	err := Start(ctx, opts)
	if err != nil {
		t.Fatalf("Start with AllowDirty and dirty worktree: %v", err)
	}
}

func TestStart_allowDirty_warnsToStderr(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	writeFile(t, repo, "dirty.txt", "x\n")

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStderr := os.Stderr
	os.Stderr = stderrW
	t.Cleanup(func() {
		os.Stderr = oldStderr
		_ = stderrW.Close()
	})

	opts := StartOptions{
		RepoRoot:      repo,
		StateDir:      stateDir,
		WorktreeRoot:  "",
		Ref:           "HEAD~1",
		DryRun:        true,
		AllowDirty:    true,
		Model:         "",
		OllamaBaseURL: "",
	}
	if err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	_ = stderrW.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, stderrR)
	stderr := buf.String()
	if !strings.Contains(stderr, "Warning") && !strings.Contains(stderr, "uncommitted") {
		t.Errorf("stderr should contain 'Warning' or 'uncommitted'; got %q", stderr)
	}
}

func TestStart_concurrentLockFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	release, err := session.AcquireLock(stateDir)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer release()

	opts := StartOptions{
		RepoRoot:      repo,
		StateDir:      stateDir,
		WorktreeRoot:  "",
		Ref:           "HEAD~1",
		DryRun:        true,
		Model:         "",
		OllamaBaseURL: "",
	}
	err = Start(ctx, opts)
	if err == nil {
		t.Fatal("Start with lock held: expected error")
	}
	if !errors.Is(err, session.ErrLocked) {
		t.Errorf("Start: got %v, want ErrLocked", err)
	}
}

func TestStart_baselineNotAncestor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	mainBranch := runOut(t, repo, "git", "rev-parse", "--abbrev-ref", "HEAD")
	runGit(t, repo, "git", "checkout", "--orphan", "orphan")
	writeFile(t, repo, "orphan.txt", "x\n")
	runGit(t, repo, "git", "add", "orphan.txt")
	runGit(t, repo, "git", "commit", "-m", "orphan")
	orphanSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	runGit(t, repo, "git", "checkout", mainBranch)
	opts := StartOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: orphanSHA, DryRun: true}
	err := Start(ctx, opts)
	if err == nil {
		t.Fatal("Start with non-ancestor baseline: expected error")
	}
	if !errors.Is(err, git.ErrBaselineNotAncestor) {
		t.Errorf("Start: got %v, want ErrBaselineNotAncestor", err)
	}
}

func TestFinish_removesWorktreeAndPersistsState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: "HEAD~1", DryRun: true}
	if err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	sBefore, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	finishOpts := FinishOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: ""}
	if err := Finish(ctx, finishOpts); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	list, err := git.List(repo)
	if err != nil {
		t.Fatalf("List after Finish: %v", err)
	}
	wantPath, _ := git.PathForRef(repo, "", sBefore.BaselineRef)
	for _, w := range list {
		if w.Path == wantPath {
			t.Error("worktree should be gone after Finish")
			break
		}
	}
	sAfter, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session after Finish: %v", err)
	}
	if sAfter.BaselineRef != sBefore.BaselineRef {
		t.Errorf("state persisted: BaselineRef = %q, want %q", sAfter.BaselineRef, sBefore.BaselineRef)
	}
}

func TestFinish_noSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := FinishOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: ""}
	err := Finish(ctx, opts)
	if err == nil {
		t.Fatal("Finish without start: expected error")
	}
	if !errors.Is(err, ErrNoSession) {
		t.Errorf("Finish: got %v, want ErrNoSession", err)
	}
}

func TestStart_worktreeAlreadyExists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: "HEAD~1", DryRun: true}
	if err := Start(ctx, opts); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	err := Start(ctx, opts)
	if err == nil {
		t.Fatal("second Start: expected error")
	}
	if !errors.Is(err, git.ErrWorktreeExists) {
		t.Errorf("second Start: got %v, want ErrWorktreeExists", err)
	}
}

func TestFinish_worktreeAlreadyGone(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: "HEAD~1", DryRun: true}
	if err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	path, err := git.PathForRef(repo, "", s.BaselineRef)
	if err != nil {
		t.Fatalf("PathForRef: %v", err)
	}
	if err := git.Remove(repo, path); err != nil {
		t.Fatalf("Remove worktree manually: %v", err)
	}
	err = Finish(ctx, FinishOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: ""})
	if err != nil {
		t.Fatalf("Finish when worktree already gone: %v", err)
	}
}

func TestFinish_writesGitNote(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: "HEAD~1", DryRun: true}
	if err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	wantFindingsCount := len(s.Findings)
	wantDismissalsCount := len(s.DismissedIDs)
	finishOpts := FinishOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: ""}
	if err := Finish(ctx, finishOpts); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	headSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	noteBody, err := git.GetNote(repo, git.NotesRefStet, headSHA)
	if err != nil {
		t.Fatalf("GetNote after Finish: %v", err)
	}
	var note map[string]interface{}
	if err := json.Unmarshal([]byte(noteBody), &note); err != nil {
		t.Fatalf("parse note JSON: %v", err)
	}
	for _, key := range []string{"session_id", "baseline_sha", "head_sha", "findings_count", "dismissals_count", "tool_version", "finished_at"} {
		if _, ok := note[key]; !ok {
			t.Errorf("note missing key %q", key)
		}
	}
	if sid, ok := note["session_id"].(string); !ok || sid == "" {
		t.Errorf("session_id: want non-empty string, got %T %v", note["session_id"], note["session_id"])
	}
	if note["head_sha"] != headSHA {
		t.Errorf("head_sha: got %v, want %s", note["head_sha"], headSHA)
	}
	if fc, ok := note["findings_count"].(float64); !ok || int(fc) != wantFindingsCount {
		t.Errorf("findings_count: got %v, want %d", note["findings_count"], wantFindingsCount)
	}
	if dc, ok := note["dismissals_count"].(float64); !ok || int(dc) != wantDismissalsCount {
		t.Errorf("dismissals_count: got %v, want %d", note["dismissals_count"], wantDismissalsCount)
	}
	if _, ok := note["tool_version"].(string); !ok {
		t.Errorf("tool_version: want string, got %T", note["tool_version"])
	}
	if _, ok := note["finished_at"].(string); !ok {
		t.Errorf("finished_at: want string, got %T", note["finished_at"])
	}
}

func TestStart_dryRun_deterministicFindings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot:      repo,
		StateDir:      stateDir,
		WorktreeRoot:  "",
		Ref:           "HEAD~1",
		DryRun:        true,
		Model:         "",
		OllamaBaseURL: "",
	}
	if err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if s.LastReviewedAt == "" {
		t.Error("LastReviewedAt: want non-empty after dry-run start")
	}
	if len(s.Findings) < 1 {
		t.Errorf("Findings: want at least 1 after dry-run, got %d", len(s.Findings))
	}
	for _, f := range s.Findings {
		if f.Message != dryRunMsg {
			t.Errorf("finding message = %q, want %q", f.Message, dryRunMsg)
		}
	}
}

func TestRun_dryRun_incremental(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{
		RepoRoot:      repo,
		StateDir:      stateDir,
		WorktreeRoot:  "",
		Ref:           "HEAD~1",
		DryRun:        true,
		Model:         "",
		OllamaBaseURL: "",
	}
	if err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s0, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	runOpts := RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true}
	if err := Run(ctx, runOpts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	s1, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session after run: %v", err)
	}
	if s1.LastReviewedAt == "" {
		t.Error("LastReviewedAt: want non-empty after run")
	}
	if s1.LastReviewedAt != s0.LastReviewedAt {
		t.Logf("LastReviewedAt updated: %q -> %q (expected when no new commits)", s0.LastReviewedAt, s1.LastReviewedAt)
	}
}

func TestRun_noSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true}
	err := Run(ctx, opts)
	if err == nil {
		t.Fatal("Run without start: expected error")
	}
	if !errors.Is(err, ErrNoSession) {
		t.Errorf("Run: got %v, want ErrNoSession", err)
	}
}

func TestStart_dryRun_noHunks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	// Baseline = HEAD so baseline..HEAD is empty (no hunks).
	opts := StartOptions{
		RepoRoot:      repo,
		StateDir:      stateDir,
		WorktreeRoot:  "",
		Ref:           "HEAD",
		DryRun:        true,
		Model:         "",
		OllamaBaseURL: "",
	}
	if err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	headSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	if s.LastReviewedAt != headSHA {
		t.Errorf("LastReviewedAt = %q, want %q", s.LastReviewedAt, headSHA)
	}
	if len(s.Findings) != 0 {
		t.Errorf("Findings: want 0 when no hunks, got %d", len(s.Findings))
	}
}

func TestRun_dryRun_withNewCommitAppendsFindings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	// Start at HEAD~1: baseline = c1, last_reviewed_at = HEAD (c2). One hunk to review.
	startOpts := StartOptions{
		RepoRoot:      repo,
		StateDir:      stateDir,
		WorktreeRoot:  "",
		Ref:           "HEAD~1",
		DryRun:        true,
		Model:         "",
		OllamaBaseURL: "",
	}
	if err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s0, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	initialCount := len(s0.Findings)
	// Add a new commit so Run has to-review hunks (baseline..HEAD now includes new change).
	writeFile(t, repo, "f3.txt", "c\n")
	runGit(t, repo, "git", "add", "f3.txt")
	runGit(t, repo, "git", "commit", "-m", "c3")
	runOpts := RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true}
	if err := Run(ctx, runOpts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	s1, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if len(s1.Findings) <= initialCount {
		t.Errorf("Findings after run with new commit: got %d, want > %d", len(s1.Findings), initialCount)
	}
	for i := initialCount; i < len(s1.Findings); i++ {
		if s1.Findings[i].Message != dryRunMsg {
			t.Errorf("new finding[%d].Message = %q, want %q", i, s1.Findings[i].Message, dryRunMsg)
		}
	}
}

func TestStart_withMockOllama(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	validResp := `[{"file":"a.go","line":1,"severity":"warning","category":"style","message":"mock finding"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": validResp, "done": true})
	}))
	defer srv.Close()

	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot:      repo,
		StateDir:      stateDir,
		WorktreeRoot:  "",
		Ref:           "HEAD~1",
		DryRun:        false,
		Model:         "m",
		OllamaBaseURL: srv.URL,
	}
	if err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if s.LastReviewedAt == "" {
		t.Error("LastReviewedAt: want non-empty after start with Ollama")
	}
	if len(s.Findings) < 1 {
		t.Errorf("Findings: want at least 1, got %d", len(s.Findings))
	}
}

// TestStart_removesWorktreeOnFailure asserts that when Start fails after creating the
// worktree (e.g. Ollama generate fails), the worktree is removed and does not pollute the repo.
func TestStart_removesWorktreeOnFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Tags succeed so upfront check passes and worktree is created; generate fails so review loop errors.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"models": []map[string]interface{}{{"name": "m"}}})
			return
		}
		if r.URL.Path == "/api/generate" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot:      repo,
		StateDir:      stateDir,
		WorktreeRoot:  "",
		Ref:           "HEAD~1",
		DryRun:        false,
		Model:         "m",
		OllamaBaseURL: srv.URL,
	}
	err := Start(ctx, opts)
	if err == nil {
		t.Fatal("Start: expected error when generate fails, got nil")
	}

	// Worktree must have been removed so we do not leave it behind.
	wantPath, pathErr := git.PathForRef(repo, "", "HEAD~1")
	if pathErr != nil {
		t.Fatalf("PathForRef: %v", pathErr)
	}
	if _, statErr := os.Stat(wantPath); statErr == nil {
		t.Errorf("worktree path %q should not exist after Start failure (cleanup expected)", wantPath)
	} else if !os.IsNotExist(statErr) {
		t.Errorf("Stat worktree path: %v", statErr)
	}
	list, listErr := git.List(repo)
	if listErr != nil {
		t.Fatalf("List worktrees: %v", listErr)
	}
	for _, w := range list {
		if w.Path == wantPath {
			t.Errorf("worktree list should not contain %q after Start failure", wantPath)
			break
		}
	}
}

// TestStart_tokenWarningWhenOverThreshold asserts that when ContextLimit and WarnThreshold
// are set so the prompt exceeds the threshold, a warning is written to stderr (Phase 3.2).
// Not parallel: captures os.Stderr so must run alone to avoid races.
func TestStart_tokenWarningWhenOverThreshold(t *testing.T) {
	ctx := context.Background()
	validResp := `[{"file":"a.go","line":1,"severity":"info","category":"style","message":"ok"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": validResp, "done": true})
	}))
	defer srv.Close()

	// Capture stderr.
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStderr := os.Stderr
	os.Stderr = stderrW
	t.Cleanup(func() {
		os.Stderr = oldStderr
		_ = stderrW.Close()
	})

	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	// Low context limit and high threshold so default prompt + hunk easily exceeds.
	opts := StartOptions{
		RepoRoot:       repo,
		StateDir:       stateDir,
		WorktreeRoot:   "",
		Ref:            "HEAD~1",
		DryRun:         false,
		Model:          "m",
		OllamaBaseURL:  srv.URL,
		ContextLimit:   100,
		WarnThreshold:  0.9,
	}
	err = Start(ctx, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	_ = stderrW.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, stderrR)
	stderr := buf.String()
	if !strings.Contains(stderr, "exceeds") {
		t.Errorf("stderr should contain token warning (exceeds); got: %q", stderr)
	}
	if !strings.Contains(stderr, "context limit") {
		t.Errorf("stderr should contain 'context limit'; got: %q", stderr)
	}
}

// TestRun_tokenWarningWhenOverThreshold asserts that Run writes a token warning to stderr
// when ContextLimit and WarnThreshold are set so the prompt exceeds the threshold (Phase 3.2).
// Not parallel: captures os.Stderr so must run alone to avoid races.
func TestRun_tokenWarningWhenOverThreshold(t *testing.T) {
	ctx := context.Background()
	validResp := `[{"file":"b.go","line":1,"severity":"info","category":"style","message":"ok"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": validResp, "done": true})
	}))
	defer srv.Close()

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStderr := os.Stderr
	os.Stderr = stderrW
	t.Cleanup(func() {
		os.Stderr = oldStderr
		_ = stderrW.Close()
	})

	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	// Start a session with dry-run so Run has something to run against.
	startOpts := StartOptions{
		RepoRoot:      repo,
		StateDir:      stateDir,
		WorktreeRoot:  "",
		Ref:           "HEAD~1",
		DryRun:        true,
		Model:         "",
		OllamaBaseURL: "",
	}
	if err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// New commit so Run has to-review hunks.
	writeFile(t, repo, "f3.txt", "c\n")
	runGit(t, repo, "git", "add", "f3.txt")
	runGit(t, repo, "git", "commit", "-m", "c3")

	runOpts := RunOptions{
		RepoRoot:      repo,
		StateDir:      stateDir,
		DryRun:        false,
		Model:         "m",
		OllamaBaseURL: srv.URL,
		ContextLimit:  100,
		WarnThreshold: 0.9,
	}
	if err := Run(ctx, runOpts); err != nil {
		t.Fatalf("Run: %v", err)
	}

	_ = stderrW.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, stderrR)
	stderr := buf.String()
	if !strings.Contains(stderr, "exceeds") {
		t.Errorf("stderr should contain token warning (exceeds); got: %q", stderr)
	}
}
