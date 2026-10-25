package run

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"stet/cli/internal/diff"
	"stet/cli/internal/findings"
	"stet/cli/internal/git"
	"stet/cli/internal/history"
	"stet/cli/internal/llm"
	"stet/cli/internal/ollama"
	"stet/cli/internal/prompt"
	"stet/cli/internal/rag"
	"stet/cli/internal/review"
	"stet/cli/internal/rules"
	"stet/cli/internal/session"
	"stet/cli/internal/systemone"
	"stet/cli/internal/testutil"
	"stet/cli/internal/trace"
)

// TestMain isolates the package's tests from host git configuration and
// background git daemons (see cli/internal/testutil).
func TestMain(m *testing.M) {
	os.Exit(testutil.RunIsolatedGit(m))
}

const dryRunMsg = "Dry-run placeholder (CI)"

func initRepo(t *testing.T) string {
	t.Helper()
	dir := testutil.TempGitDir(t)
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

func ptrBool(b bool) *bool { return &b }

func TestStart_createsSessionAndWorktree(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       true,
		Model:        "",
		Provider:     "ollama",
		LLMBaseURL:   "",
	}
	_, err := Start(ctx, opts)
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
	wantNorm, _ := filepath.EvalSymlinks(wantPath)
	for _, w := range list {
		wNorm, _ := filepath.EvalSymlinks(w.Path)
		if wantNorm == wNorm {
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
		Model: "", Provider: "ollama",
		LLMBaseURL: "",
	}
	_, err := Start(ctx, opts)
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

func TestStart_refEqualsHEAD_withPersistStrictness_storesInSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	strictness := "strict"
	opts := StartOptions{
		RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: "HEAD", DryRun: true,
		Model: "", Provider: "ollama",
		LLMBaseURL:        "",
		PersistStrictness: &strictness,
	}
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start(HEAD): %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if s.Strictness != strictness {
		t.Errorf("session.Strictness = %q, want %q", s.Strictness, strictness)
	}
}

func TestStart_negativeRAGOptions_clamped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot:                repo,
		StateDir:                stateDir,
		WorktreeRoot:            "",
		Ref:                     "HEAD~1",
		DryRun:                  true,
		RAGSymbolMaxDefinitions: -1,
		RAGSymbolMaxTokens:      -1,
	}
	_, err := Start(ctx, opts)
	if err != nil {
		t.Fatalf("Start with negative RAG options (should clamp to 0): %v", err)
	}
}

func TestStart_persistStrictnessAndRAG_storesInSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	strictness := "lenient"
	ragDefs := 5
	ragTokens := 1000
	opts := StartOptions{
		RepoRoot:                       repo,
		StateDir:                       stateDir,
		WorktreeRoot:                   "",
		Ref:                            "HEAD~1",
		DryRun:                         true,
		Model:                          "",
		Provider:                       "ollama",
		LLMBaseURL:                     "",
		PersistStrictness:              &strictness,
		PersistRAGSymbolMaxDefinitions: &ragDefs,
		PersistRAGSymbolMaxTokens:      &ragTokens,
	}
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if s.Strictness != strictness {
		t.Errorf("session.Strictness = %q, want %q", s.Strictness, strictness)
	}
	if s.RAGSymbolMaxDefinitions == nil || *s.RAGSymbolMaxDefinitions != ragDefs {
		t.Errorf("session.RAGSymbolMaxDefinitions = %v, want %d", s.RAGSymbolMaxDefinitions, ragDefs)
	}
	if s.RAGSymbolMaxTokens == nil || *s.RAGSymbolMaxTokens != ragTokens {
		t.Errorf("session.RAGSymbolMaxTokens = %v, want %d", s.RAGSymbolMaxTokens, ragTokens)
	}
}

func TestStart_refEqualsHEAD_withPersistContext_storesInSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	contextLimit := 8192
	numCtx := 8192
	opts := StartOptions{
		RepoRoot:            repo,
		StateDir:            stateDir,
		WorktreeRoot:        "",
		Ref:                 "HEAD",
		DryRun:              true,
		Model:               "",
		Provider:            "ollama",
		LLMBaseURL:          "",
		PersistContextLimit: &contextLimit,
		PersistNumCtx:       &numCtx,
	}
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start(HEAD): %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if s.ContextLimit == nil || *s.ContextLimit != contextLimit {
		t.Errorf("session.ContextLimit = %v, want %d", s.ContextLimit, contextLimit)
	}
	if s.NumCtx == nil || *s.NumCtx != numCtx {
		t.Errorf("session.NumCtx = %v, want %d", s.NumCtx, numCtx)
	}
}

func TestStart_requiresCleanWorktree(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	writeFile(t, repo, "dirty.txt", "x\n")
	opts := StartOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: "HEAD", DryRun: true}
	_, err := Start(ctx, opts)
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
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       true,
		AllowDirty:   true,
		Model:        "",
		Provider:     "ollama",
		LLMBaseURL:   "",
	}
	_, err := Start(ctx, opts)
	if err != nil {
		t.Fatalf("Start with AllowDirty and dirty worktree: %v", err)
	}
}

func TestStart_allowDirty_warnsToStderr(t *testing.T) {
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
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       true,
		AllowDirty:   true,
		Model:        "",
		Provider:     "ollama",
		LLMBaseURL:   "",
	}
	if _, err := Start(ctx, opts); err != nil {
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
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       true,
		Model:        "",
		Provider:     "ollama",
		LLMBaseURL:   "",
	}
	_, err = Start(ctx, opts)
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
	_, err := Start(ctx, opts)
	if err == nil {
		t.Fatal("Start with non-ancestor baseline: expected error")
	}
	if !errors.Is(err, git.ErrBaselineNotAncestor) {
		t.Errorf("Start: got %v, want ErrBaselineNotAncestor", err)
	}
}

func TestFinish_removesWorktreeAndClearsSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: "HEAD~1", DryRun: true}
	if _, err := Start(ctx, startOpts); err != nil {
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
	if sAfter.BaselineRef != "" {
		t.Errorf("session should be cleared after Finish: BaselineRef = %q, want empty", sAfter.BaselineRef)
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
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	_, err := Start(ctx, opts)
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
	if _, err := Start(ctx, opts); err != nil {
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
	if _, err := Start(ctx, startOpts); err != nil {
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
	for _, key := range []string{"session_id", "baseline_sha", "head_sha", "findings_count", "dismissals_count", "tool_version", "finished_at",
		"hunks_reviewed", "lines_added", "lines_removed", "chars_added", "chars_deleted", "chars_reviewed"} {
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
	// Scope fields: CountHunkScope is unit-tested in internal/diff; this test ensures Finish writes scope to the note. We assert presence and sanity (hunks_reviewed >= 1, chars_reviewed > 0 when hunks present, others non-negative). HEAD~1..HEAD has at least one hunk (last commit adds f2.txt).
	if hr, ok := note["hunks_reviewed"].(float64); !ok || hr < 1 {
		t.Errorf("hunks_reviewed: want number >= 1, got %T %v", note["hunks_reviewed"], note["hunks_reviewed"])
	}
	for _, key := range []string{"lines_added", "lines_removed", "chars_added", "chars_deleted", "chars_reviewed"} {
		if v, ok := note[key].(float64); !ok || v < 0 {
			t.Errorf("%s: want non-negative number, got %T %v", key, note[key], note[key])
		}
	}
	if hr, ok := note["hunks_reviewed"].(float64); ok && hr >= 1 {
		if cr, ok := note["chars_reviewed"].(float64); !ok || cr <= 0 {
			t.Errorf("chars_reviewed: want > 0 when hunks_reviewed >= 1, got %v", note["chars_reviewed"])
		}
	}
}

func TestFinish_appendsHistoryWhenFindings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: "HEAD~1", DryRun: true}
	if _, err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if len(s.Findings) == 0 {
		t.Fatal("need at least one finding from dry-run to test history append")
	}
	finishOpts := FinishOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: ""}
	if err := Finish(ctx, finishOpts); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	historyPath := filepath.Join(stateDir, "history.jsonl")
	data, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatalf("ReadFile history.jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 1 || lines[0] == "" {
		t.Fatalf("history.jsonl: want 1 line, got %d", len(lines))
	}
	var rec history.Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("Unmarshal history record: %v", err)
	}
	if rec.DiffRef == "" {
		t.Error("diff_ref: want non-empty")
	}
	if len(rec.ReviewOutput) != len(s.Findings) {
		t.Errorf("review_output: got %d findings, want %d", len(rec.ReviewOutput), len(s.Findings))
	}
	if rec.UserAction.FinishedAt == "" {
		t.Error("user_action.finished_at: want non-empty")
	}
}

func TestFinish_doesNotAppendHistoryWhenNoFindings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: "HEAD~1", DryRun: true}
	if _, err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	s.Findings = nil
	if err := session.Save(stateDir, &s); err != nil {
		t.Fatalf("Save session (clear findings): %v", err)
	}
	if err := Finish(ctx, FinishOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: ""}); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	historyPath := filepath.Join(stateDir, "history.jsonl")
	if _, err := os.Stat(historyPath); err == nil {
		data, _ := os.ReadFile(historyPath)
		lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		if len(lines) > 0 && lines[0] != "" {
			t.Errorf("Finish with no findings should not append history; file has %d lines", len(lines))
		}
	}
}

func TestStart_dryRun_deterministicFindings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       true,
		Model:        "",
		Provider:     "ollama",
		LLMBaseURL:   "",
	}
	if _, err := Start(ctx, opts); err != nil {
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

func TestStart_stream_emitsNDJSON(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	var buf bytes.Buffer
	opts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       true,
		Model:        "",
		Provider:     "ollama",
		LLMBaseURL:   "",
		StreamOut:    &buf,
	}
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("stream output: want at least 2 lines (progress + done), got %d", len(lines))
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
			if obj["msg"] == nil {
				t.Error("progress event missing msg")
			}
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
	}
	if progressCount < 1 {
		t.Errorf("stream: want at least 1 progress event, got %d", progressCount)
	}
	if findingCount < 1 {
		t.Errorf("stream: want at least 1 finding event (dry-run has hunks), got %d", findingCount)
	}
	if !gotDone {
		t.Error("stream: want done event")
	}
}

func TestRun_excludeLift_reviewsPreviouslySkipped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	excludeTxt := &diff.Options{ExcludePatterns: []string{"*.txt"}}
	startOpts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       true,
		DiffOpts:     excludeTxt,
	}
	if _, err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start with exclude: %v", err)
	}
	s0, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if len(s0.PendingExcludedPaths) == 0 {
		t.Fatalf("PendingExcludedPaths = %v, want non-empty after excluded start", s0.PendingExcludedPaths)
	}
	if len(s0.Findings) != 0 {
		t.Fatalf("Findings len = %d, want 0 when all hunks excluded", len(s0.Findings))
	}
	runOpts := RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true, DiffOpts: nil}
	if _, err := Run(ctx, runOpts); err != nil {
		t.Fatalf("Run after lifting exclusions: %v", err)
	}
	s1, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session after run: %v", err)
	}
	if len(s1.Findings) == 0 {
		t.Fatal("Findings: want non-empty after lifting exclusions and reviewing skipped paths")
	}
	var hasF2 bool
	for _, f := range s1.Findings {
		if f.File == "f2.txt" {
			hasF2 = true
			break
		}
	}
	if !hasF2 {
		t.Errorf("Findings = %+v, want finding for previously excluded f2.txt", s1.Findings)
	}
	if len(s1.PendingExcludedPaths) != 0 {
		t.Errorf("PendingExcludedPaths = %v, want empty after f2.txt reviewed", s1.PendingExcludedPaths)
	}
}

func TestRun_dryRun_incremental(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       true,
		Model:        "",
		Provider:     "ollama",
		LLMBaseURL:   "",
	}
	if _, err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s0, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	runOpts := RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true}
	if _, err := Run(ctx, runOpts); err != nil {
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

func TestRun_hunksReviewedStats(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startStats, err := Start(ctx, StartOptions{RepoRoot: repo, StateDir: stateDir, Ref: "HEAD~1", DryRun: true, Provider: "ollama"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if startStats.HunksReviewed <= 0 {
		t.Errorf("Start HunksReviewed = %d, want > 0", startStats.HunksReviewed)
	}
	// No new commits since start: zero-hunk early return.
	runStats, err := Run(ctx, RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if runStats.HunksReviewed != 0 {
		t.Errorf("Run with no new hunks: HunksReviewed = %d, want 0", runStats.HunksReviewed)
	}
	// New commit: dry-run reviews the new hunk.
	writeFile(t, repo, "f3.txt", "c\n")
	runOut(t, repo, "git", "add", "f3.txt")
	runOut(t, repo, "git", "commit", "-m", "c3")
	runStats, err = Run(ctx, RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true})
	if err != nil {
		t.Fatalf("Run after commit: %v", err)
	}
	if runStats.HunksReviewed <= 0 {
		t.Errorf("Run with new hunk: HunksReviewed = %d, want > 0", runStats.HunksReviewed)
	}
	// Force full review (rerun path) reviews every hunk again.
	fullStats, err := Run(ctx, RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true, ForceFullReview: true})
	if err != nil {
		t.Fatalf("Run ForceFullReview: %v", err)
	}
	if fullStats.HunksReviewed < 2 {
		t.Errorf("Run ForceFullReview: HunksReviewed = %d, want >= 2", fullStats.HunksReviewed)
	}
}

func TestRun_noSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true}
	_, err := Run(ctx, opts)
	if err == nil {
		t.Fatal("Run without start: expected error")
	}
	if !errors.Is(err, ErrNoSession) {
		t.Errorf("Run: got %v, want ErrNoSession", err)
	}
}

func TestRun_forceFullReview_noSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true, ForceFullReview: true}
	_, err := Run(ctx, opts)
	if err == nil {
		t.Fatal("Run (ForceFullReview) without start: expected error")
	}
	if !errors.Is(err, ErrNoSession) {
		t.Errorf("Run: got %v, want ErrNoSession", err)
	}
	if err != nil && !strings.Contains(err.Error(), "no active session") {
		t.Errorf("Run: error message should mention no active session; got %q", err.Error())
	}
}

func TestRun_forceFullReview_partitionsAllHunks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       true,
		Model:        "",
		Provider:     "ollama",
		LLMBaseURL:   "",
	}
	if _, err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s0, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	initialCount := len(s0.Findings)
	// Run once without ForceFullReview so LastReviewedAt = HEAD; second Run would see 0 ToReview.
	runOpts := RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true}
	if _, err := Run(ctx, runOpts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	s1, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	afterFirstRun := len(s1.Findings)
	// Rerun with ForceFullReview: all hunks go to ToReview, so review loop runs again and appends findings (merge mode).
	runOpts.ForceFullReview = true
	if _, err := Run(ctx, runOpts); err != nil {
		t.Fatalf("Run(ForceFullReview): %v", err)
	}
	s2, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if len(s2.Findings) <= afterFirstRun {
		t.Errorf("After ForceFullReview run: findings got %d, want > %d (review loop should have run)", len(s2.Findings), afterFirstRun)
	}
	if initialCount == 0 && afterFirstRun == 0 {
		t.Errorf("Start at HEAD~1 should produce findings; initial=%d afterFirstRun=%d", initialCount, afterFirstRun)
	}
}

func TestRun_replaceFindings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       true,
		Model:        "",
		Provider:     "ollama",
		LLMBaseURL:   "",
	}
	if _, err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	runOpts := RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true}
	if _, err := Run(ctx, runOpts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	runOpts.ForceFullReview = true
	runOpts.ReplaceFindings = true
	if _, err := Run(ctx, runOpts); err != nil {
		t.Fatalf("Run(ReplaceFindings): %v", err)
	}
	s1, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	// Replace mode: session findings are only the new run's findings (one per hunk in ToReview).
	if len(s1.Findings) == 0 {
		t.Errorf("ReplaceFindings run should produce findings; got 0")
	}
	// DismissedIDs should be empty after replace (clean slate).
	if len(s1.DismissedIDs) != 0 {
		t.Errorf("DismissedIDs after replace: got %d, want 0", len(s1.DismissedIDs))
	}
	// All findings should be dry-run placeholder (from this run only).
	for i, f := range s1.Findings {
		if f.Message != dryRunMsg {
			t.Errorf("finding[%d].Message = %q, want %q", i, f.Message, dryRunMsg)
		}
	}
}

func TestStart_dryRun_noHunks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	// Baseline = HEAD so baseline..HEAD is empty (no hunks).
	opts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD",
		DryRun:       true,
		Model:        "",
		Provider:     "ollama",
		LLMBaseURL:   "",
	}
	if _, err := Start(ctx, opts); err != nil {
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

// TestStart_dryRun_multipleHunksSameFile_distinctIDs asserts that multiple hunks
// in the same file produce distinct finding IDs so stet list shows one line per finding.
func TestStart_dryRun_multipleHunksSameFile_distinctIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	// One file with enough lines so two distant edits produce two hunks (context ~3).
	lines := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\nline12\nline13\nline14\nline15\n"
	writeFile(t, repo, "multi.txt", lines)
	runGit(t, repo, "git", "add", "multi.txt")
	runGit(t, repo, "git", "commit", "-m", "add multi")
	writeFile(t, repo, "multi.txt", "X\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\nline12\nline13\nline14\nY\n")
	runGit(t, repo, "git", "add", "multi.txt")
	runGit(t, repo, "git", "commit", "-m", "two edits")
	opts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       true,
		Model:        "",
		Provider:     "ollama",
		LLMBaseURL:   "",
	}
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if len(s.Findings) < 2 {
		t.Fatalf("want at least 2 findings (two hunks in same file), got %d", len(s.Findings))
	}
	seen := make(map[string]struct{})
	for _, f := range s.Findings {
		if f.ID == "" {
			t.Error("finding has empty ID")
			continue
		}
		if _, ok := seen[f.ID]; ok {
			t.Errorf("duplicate finding ID %q; each hunk must have a distinct ID", f.ID)
		}
		seen[f.ID] = struct{}{}
	}
}

func TestRun_dryRun_withNewCommitAppendsFindings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	// Start at HEAD~1: baseline = c1, last_reviewed_at = HEAD (c2). One hunk to review.
	startOpts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       true,
		Model:        "",
		Provider:     "ollama",
		LLMBaseURL:   "",
	}
	if _, err := Start(ctx, startOpts); err != nil {
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
	if _, err := Run(ctx, runOpts); err != nil {
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

func TestAddressedFindingIDs(t *testing.T) {
	t.Parallel()
	// Hunk with @@ -1,3 +1,4 @@ gives new file lines 1-4.
	hunkWithRange := diff.Hunk{
		FilePath:   "a.go",
		RawContent: "@@ -1,3 +1,4 @@\n context\n+added",
		Context:    "",
	}
	// Hunk with unparseable header does not cover any line.
	hunkBadHeader := diff.Hunk{FilePath: "b.go", RawContent: "no @@ here", Context: ""}
	// Second file, lines 10-11.
	hunkOther := diff.Hunk{
		FilePath:   "c.go",
		RawContent: "@@ -9,2 +10,2 @@\n old\n+new",
		Context:    "",
	}

	tests := []struct {
		name          string
		hunks         []diff.Hunk
		existing      []findings.Finding
		newIDSet      map[string]struct{}
		wantAddressed []string
	}{
		{
			name:          "finding in hunk not in new set -> addressed",
			hunks:         []diff.Hunk{hunkWithRange},
			existing:      []findings.Finding{{ID: "f1", File: "a.go", Line: 2}},
			newIDSet:      map[string]struct{}{},
			wantAddressed: []string{"f1"},
		},
		{
			name:          "finding in hunk but in new set -> not addressed",
			hunks:         []diff.Hunk{hunkWithRange},
			existing:      []findings.Finding{{ID: "f1", File: "a.go", Line: 2}},
			newIDSet:      map[string]struct{}{"f1": {}},
			wantAddressed: nil,
		},
		{
			name:          "finding not in any hunk -> not addressed",
			hunks:         []diff.Hunk{hunkWithRange},
			existing:      []findings.Finding{{ID: "f1", File: "other.go", Line: 2}},
			newIDSet:      map[string]struct{}{},
			wantAddressed: nil,
		},
		{
			name:  "multiple at same location one in new set",
			hunks: []diff.Hunk{hunkWithRange},
			existing: []findings.Finding{
				{ID: "f1", File: "a.go", Line: 2},
				{ID: "f2", File: "a.go", Line: 2},
			},
			newIDSet:      map[string]struct{}{"f1": {}},
			wantAddressed: []string{"f2"},
		},
		{
			name:          "unparseable hunk header -> no coverage",
			hunks:         []diff.Hunk{hunkBadHeader},
			existing:      []findings.Finding{{ID: "f1", File: "b.go", Line: 1}},
			newIDSet:      map[string]struct{}{},
			wantAddressed: nil,
		},
		{
			name:  "range start used when present",
			hunks: []diff.Hunk{hunkWithRange},
			existing: []findings.Finding{{
				ID: "f1", File: "a.go", Line: 0,
				Range: &findings.LineRange{Start: 2, End: 3},
			}},
			newIDSet:      map[string]struct{}{},
			wantAddressed: []string{"f1"},
		},
		{
			name:          "finding in second hunk",
			hunks:         []diff.Hunk{hunkWithRange, hunkOther},
			existing:      []findings.Finding{{ID: "f2", File: "c.go", Line: 10}},
			newIDSet:      map[string]struct{}{},
			wantAddressed: []string{"f2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addressedFindingIDs(tt.hunks, tt.existing, tt.newIDSet)
			if len(got) != len(tt.wantAddressed) {
				t.Errorf("addressedFindingIDs() len = %d, want %d; got %v", len(got), len(tt.wantAddressed), got)
				return
			}
			gotSet := make(map[string]struct{})
			for _, id := range got {
				gotSet[id] = struct{}{}
			}
			for _, id := range tt.wantAddressed {
				if _, ok := gotSet[id]; !ok {
					t.Errorf("addressedFindingIDs() missing %q; got %v", id, got)
				}
			}
		})
	}
}

func TestFilterHunksWithDismissedFindings(t *testing.T) {
	t.Parallel()
	hunkA := diff.Hunk{FilePath: "a.go", RawContent: "@@ -1,3 +1,4 @@\n context\n+added", Context: ""}
	hunkB := diff.Hunk{FilePath: "b.go", RawContent: "@@ -9,2 +10,2 @@\n old\n+new", Context: ""}

	tests := []struct {
		name            string
		toReview        []diff.Hunk
		s               *session.Session
		forceFullReview bool
		wantLen         int
		wantSkipped     int
	}{
		{
			name:            "forceFullReview true returns all",
			toReview:        []diff.Hunk{hunkA},
			s:               &session.Session{DismissedIDs: []string{"f1"}, Findings: []findings.Finding{{ID: "f1", File: "a.go", Line: 2}}},
			forceFullReview: true,
			wantLen:         1,
			wantSkipped:     0,
		},
		{
			name:            "nil session returns all",
			toReview:        []diff.Hunk{hunkA},
			s:               nil,
			forceFullReview: false,
			wantLen:         1,
			wantSkipped:     0,
		},
		{
			name:            "empty DismissedIDs returns all",
			toReview:        []diff.Hunk{hunkA},
			s:               &session.Session{DismissedIDs: nil, Findings: []findings.Finding{}},
			forceFullReview: false,
			wantLen:         1,
			wantSkipped:     0,
		},
		{
			name:            "finding in hunk dismissed -> hunk excluded",
			toReview:        []diff.Hunk{hunkA},
			s:               &session.Session{DismissedIDs: []string{"f1"}, Findings: []findings.Finding{{ID: "f1", File: "a.go", Line: 2}}},
			forceFullReview: false,
			wantLen:         0,
			wantSkipped:     1,
		},
		{
			name:            "finding not in hunk -> hunk kept",
			toReview:        []diff.Hunk{hunkA},
			s:               &session.Session{DismissedIDs: []string{"f1"}, Findings: []findings.Finding{{ID: "f1", File: "other.go", Line: 2}}},
			forceFullReview: false,
			wantLen:         1,
			wantSkipped:     0,
		},
		{
			name:     "finding with Range overlapping hunk -> hunk excluded",
			toReview: []diff.Hunk{hunkA},
			s: &session.Session{
				DismissedIDs: []string{"f1"},
				Findings:     []findings.Finding{{ID: "f1", File: "a.go", Line: 0, Range: &findings.LineRange{Start: 2, End: 3}}},
			},
			forceFullReview: false,
			wantLen:         0,
			wantSkipped:     1,
		},
		{
			name:            "unresolved DismissedID -> no exclusion",
			toReview:        []diff.Hunk{hunkA},
			s:               &session.Session{DismissedIDs: []string{"unknown"}, Findings: []findings.Finding{{ID: "f1", File: "a.go", Line: 2}}},
			forceFullReview: false,
			wantLen:         1,
			wantSkipped:     0,
		},
		{
			name:            "multiple hunks one contains dismissed -> only that excluded",
			toReview:        []diff.Hunk{hunkA, hunkB},
			s:               &session.Session{DismissedIDs: []string{"f1"}, Findings: []findings.Finding{{ID: "f1", File: "a.go", Line: 2}}},
			forceFullReview: false,
			wantLen:         1,
			wantSkipped:     1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, skipped := filterHunksWithDismissedFindings(tt.toReview, tt.s, tt.forceFullReview)
			if len(got) != tt.wantLen {
				t.Errorf("filterHunksWithDismissedFindings() len = %d, want %d", len(got), tt.wantLen)
			}
			if skipped != tt.wantSkipped {
				t.Errorf("filterHunksWithDismissedFindings() skipped = %d, want %d", skipped, tt.wantSkipped)
			}
		})
	}
}

// TestRun_dismissedHunksNotSentToModel: when a finding is dismissed, hunks containing that
// finding are not sent to the LLM on subsequent runs. Start produces findings, we dismiss
// one, add a new commit so the same file has ToReview hunks, then Run. The dismissed hunk
// is filtered out so we get fewer (or zero) new findings.
func TestRun_dismissedHunksNotSentToModel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       true,
		Model:        "",
		Provider:     "ollama",
		LLMBaseURL:   "",
	}
	if _, err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s0, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if len(s0.Findings) < 1 {
		t.Fatalf("need at least one finding after Start; got %d", len(s0.Findings))
	}
	// Dismiss the first finding (in f2.txt from initRepo)
	firstID := s0.Findings[0].ID
	firstFile := s0.Findings[0].File
	s0.DismissedIDs = append(s0.DismissedIDs, firstID)
	if err := session.Save(stateDir, &s0); err != nil {
		t.Fatalf("Save session: %v", err)
	}
	// New commit modifying the same file so its hunk is in ToReview
	writeFile(t, repo, firstFile, "b\nx\n")
	runGit(t, repo, "git", "add", firstFile)
	runGit(t, repo, "git", "commit", "-m", "change for re-review")
	// Run: the only ToReview hunk contains the dismissed finding -> filtered out -> 0 hunks sent
	runOpts := RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true}
	if _, err := Run(ctx, runOpts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	s1, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session after run: %v", err)
	}
	// No new findings should be added (dismissed hunk was not sent to model)
	if len(s1.Findings) != len(s0.Findings) {
		t.Errorf("Findings count: got %d, want %d (dismissed hunk should not produce new findings)", len(s1.Findings), len(s0.Findings))
	}
}

// TestRun_forceFullReview_skipsDismissedFilter: when ForceFullReview is true, dismissed
// hunks are still sent to the LLM (filter is bypassed).
func TestRun_forceFullReview_skipsDismissedFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       true,
		Model:        "",
		Provider:     "ollama",
		LLMBaseURL:   "",
	}
	if _, err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s0, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if len(s0.Findings) < 1 {
		t.Fatalf("need at least one finding after Start; got %d", len(s0.Findings))
	}
	firstID := s0.Findings[0].ID
	firstFile := s0.Findings[0].File
	s0.DismissedIDs = append(s0.DismissedIDs, firstID)
	if err := session.Save(stateDir, &s0); err != nil {
		t.Fatalf("Save session: %v", err)
	}
	writeFile(t, repo, firstFile, "b\nx\n")
	runGit(t, repo, "git", "add", firstFile)
	runGit(t, repo, "git", "commit", "-m", "change for re-review")
	// Run with ForceFullReview: filter is skipped, hunk is sent, we get new findings
	runOpts := RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true, ForceFullReview: true}
	if _, err := Run(ctx, runOpts); err != nil {
		t.Fatalf("Run(ForceFullReview): %v", err)
	}
	s1, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session after run: %v", err)
	}
	// With ForceFullReview, the hunk is sent so we get additional findings (merge mode appends)
	if len(s1.Findings) <= len(s0.Findings) {
		t.Errorf("ForceFullReview: findings got %d, want > %d (dismissed filter should be bypassed)", len(s1.Findings), len(s0.Findings))
	}
}

// TestRun_autoDismiss_addressedFindingNotRereported: after Start (dry-run) we have one finding per hunk.
// We add an extra finding to the session at the same file/line as a reviewed hunk but with a different ID.
// Run again (dry-run) produces canned findings with deterministic IDs; the extra finding's ID is not in
// the new set, so it is auto-dismissed. We assert it is in DismissedIDs and excluded from active output.
func TestRun_autoDismiss_addressedFindingNotRereported(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       true,
		Model:        "",
		Provider:     "ollama",
		LLMBaseURL:   "",
	}
	if _, err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s0, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	// Canned findings use a hunk-specific ID (StableFindingID with message + StrictHunkID). Add an extra finding with
	// same file as first hunk, line 1 (inside any hunk), but a different ID so it won't be in newFindings.
	if len(s0.Findings) < 1 {
		t.Fatalf("need at least one finding after Start; got %d", len(s0.Findings))
	}
	firstHunkFile := s0.Findings[0].File
	extraID := "extra-addressed-finding"
	extra := findings.Finding{
		ID:       extraID,
		File:     firstHunkFile,
		Line:     1,
		Severity: findings.SeverityInfo,
		Category: findings.CategoryMaintainability,
		Message:  "pre-existing finding",
	}
	s0.Findings = append(s0.Findings, extra)
	if err := session.Save(stateDir, &s0); err != nil {
		t.Fatalf("Save session: %v", err)
	}
	// Add a new commit so Run has hunks to review (baseline..HEAD vs last_reviewed_at).
	writeFile(t, repo, firstHunkFile, "new line\nb\n")
	runGit(t, repo, "git", "add", firstHunkFile)
	runGit(t, repo, "git", "commit", "-m", "change for re-review")
	// Run: dry-run will add one finding per hunk (deterministic IDs). Our extra finding
	// is in a reviewed hunk and its ID is not in the new set -> auto-dismissed.
	runOpts := RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true}
	if _, err := Run(ctx, runOpts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	s1, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	hasExtra := false
	for _, id := range s1.DismissedIDs {
		if id == extraID {
			hasExtra = true
			break
		}
	}
	if !hasExtra {
		t.Errorf("DismissedIDs: want %q (auto-dismissed); got %v", extraID, s1.DismissedIDs)
	}
	// Findings are not removed from session; activeFindings() filters by DismissedIDs.
	// Verify the extra finding is excluded from the active set (as list/status/JSON would show).
	dismissedSet := make(map[string]struct{}, len(s1.DismissedIDs))
	for _, id := range s1.DismissedIDs {
		dismissedSet[id] = struct{}{}
	}
	activeCount := 0
	extraInActive := false
	for _, f := range s1.Findings {
		if _, dismissed := dismissedSet[f.ID]; !dismissed {
			activeCount++
			if f.ID == extraID {
				extraInActive = true
			}
		}
	}
	if extraInActive {
		t.Error("auto-dismissed finding should not appear in active set (filtered by DismissedIDs)")
	}
	// With hunk-specific IDs, the prior canned finding for this file (different hunk) is also addressed, so we may have more than one dismissed.
	wantActive := len(s1.Findings) - len(s1.DismissedIDs)
	if activeCount != wantActive {
		t.Errorf("active count = %d, want len(Findings)-len(DismissedIDs) = %d", activeCount, wantActive)
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
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       false,
		Model:        "m",
		Provider:     "ollama",
		LLMBaseURL:   srv.URL,
	}
	if _, err := Start(ctx, opts); err != nil {
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

// TestStart_capturesOllamaUsage_whenNotDryRun asserts that when Start runs with
// DryRun false and the mock Ollama returns usage fields, the session stores
// LastRunPromptTokens, LastRunCompletionTokens, and LastRunEvalDurationNs (Phase 9.1).
func TestStart_capturesOllamaUsage_whenNotDryRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	validResp := `[{"file":"a.go","line":1,"severity":"warning","category":"style","message":"mock finding"}]`
	wantPromptTokens := 100
	wantCompletionTokens := 42
	wantEvalDurationNs := int64(500_000_000)
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"response":             validResp,
			"done":                 true,
			"prompt_eval_count":    wantPromptTokens,
			"eval_count":           wantCompletionTokens,
			"prompt_eval_duration": int64(50_000_000),
			"eval_duration":        wantEvalDurationNs,
		})
	}))
	defer srv.Close()

	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       false,
		Model:        "m",
		Provider:     "ollama",
		LLMBaseURL:   srv.URL,
	}
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if s.LastRunPromptTokens != int64(wantPromptTokens) {
		t.Errorf("LastRunPromptTokens = %d, want %d", s.LastRunPromptTokens, wantPromptTokens)
	}
	if s.LastRunCompletionTokens != int64(wantCompletionTokens) {
		t.Errorf("LastRunCompletionTokens = %d, want %d", s.LastRunCompletionTokens, wantCompletionTokens)
	}
	if s.LastRunEvalDurationNs != wantEvalDurationNs {
		t.Errorf("LastRunEvalDurationNs = %d, want %d", s.LastRunEvalDurationNs, wantEvalDurationNs)
	}
}

// TestFinish_writesUsageFields_whenCaptureUsageTrue asserts that when the session has
// usage data and STET_CAPTURE_USAGE is true (default), Finish writes model, prompt_tokens,
// completion_tokens, and eval_duration_ns to the git note (Phase 9.3).
func TestFinish_writesUsageFields_whenCaptureUsageTrue(t *testing.T) {
	ctx := context.Background()
	oldVal := os.Getenv("STET_CAPTURE_USAGE")
	os.Setenv("STET_CAPTURE_USAGE", "true")
	defer func() {
		if oldVal == "" {
			os.Unsetenv("STET_CAPTURE_USAGE")
		} else {
			os.Setenv("STET_CAPTURE_USAGE", oldVal)
		}
	}()

	validResp := `[{"file":"a.go","line":1,"severity":"warning","category":"style","message":"mock finding"}]`
	wantPromptTokens := 100
	wantCompletionTokens := 42
	wantEvalDurationNs := int64(500_000_000)
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"response":             validResp,
			"done":                 true,
			"prompt_eval_count":    wantPromptTokens,
			"eval_count":           wantCompletionTokens,
			"prompt_eval_duration": int64(50_000_000),
			"eval_duration":        wantEvalDurationNs,
		})
	}))
	defer srv.Close()

	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       false,
		Model:        "m",
		Provider:     "ollama",
		LLMBaseURL:   srv.URL,
	}
	if _, err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
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
	if _, ok := note["model"]; !ok {
		t.Error("note missing model (expected when STET_CAPTURE_USAGE true)")
	}
	for _, key := range []string{"prompt_tokens", "completion_tokens", "eval_duration_ns"} {
		if _, ok := note[key]; !ok {
			t.Errorf("note missing %q (expected when STET_CAPTURE_USAGE true)", key)
		}
	}
	if pt, ok := note["prompt_tokens"].(float64); ok && int(pt) != wantPromptTokens {
		t.Errorf("prompt_tokens = %v, want %d", pt, wantPromptTokens)
	}
	if ct, ok := note["completion_tokens"].(float64); ok && int(ct) != wantCompletionTokens {
		t.Errorf("completion_tokens = %v, want %d", ct, wantCompletionTokens)
	}
	if ed, ok := note["eval_duration_ns"].(float64); ok && int64(ed) != wantEvalDurationNs {
		t.Errorf("eval_duration_ns = %v, want %d", ed, wantEvalDurationNs)
	}
}

// TestFinish_omitsUsageFields_whenCaptureUsageFalse asserts that when STET_CAPTURE_USAGE
// is false, Finish does not write prompt_tokens, completion_tokens, or eval_duration_ns
// to the git note (Phase 9.3).
func TestFinish_omitsUsageFields_whenCaptureUsageFalse(t *testing.T) {
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"response":          validResp,
			"done":              true,
			"prompt_eval_count": 50,
			"eval_count":        20,
			"eval_duration":     int64(100_000_000),
		})
	}))
	defer srv.Close()

	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       false,
		Model:        "m",
		Provider:     "ollama",
		LLMBaseURL:   srv.URL,
	}
	if _, err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	oldVal := os.Getenv("STET_CAPTURE_USAGE")
	os.Setenv("STET_CAPTURE_USAGE", "false")
	defer func() {
		if oldVal == "" {
			os.Unsetenv("STET_CAPTURE_USAGE")
		} else {
			os.Setenv("STET_CAPTURE_USAGE", oldVal)
		}
	}()

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
	for _, key := range []string{"prompt_tokens", "completion_tokens", "eval_duration_ns"} {
		if _, ok := note[key]; ok {
			t.Errorf("note must not contain %q when STET_CAPTURE_USAGE=false", key)
		}
	}
}

// TestStart_abstentionFilter_dropsLowConfidence asserts that findings with confidence < 0.8
// are dropped by the Phase 6.3 abstention filter and do not appear in the session.
func TestStart_abstentionFilter_dropsLowConfidence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lowConfResp := `[{"file":"a.go","line":1,"severity":"warning","category":"style","message":"mock","confidence":0.7}]`
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": lowConfResp, "done": true})
	}))
	defer srv.Close()

	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       false,
		Model:        "m",
		Provider:     "ollama",
		LLMBaseURL:   srv.URL,
	}
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if len(s.Findings) != 0 {
		t.Errorf("Findings: abstention filter should drop confidence 0.7; got %d findings", len(s.Findings))
	}
}

// TestStart_abstentionFilter_dropsMaintainabilityUnderThreshold asserts that maintainability
// findings with confidence < 0.9 are dropped and do not appear in the session.
func TestStart_abstentionFilter_dropsMaintainabilityUnderThreshold(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	maintResp := `[{"file":"a.go","line":1,"severity":"info","category":"maintainability","message":"add comments","confidence":0.85}]`
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": maintResp, "done": true})
	}))
	defer srv.Close()

	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       false,
		Model:        "m",
		Provider:     "ollama",
		LLMBaseURL:   srv.URL,
	}
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if len(s.Findings) != 0 {
		t.Errorf("Findings: abstention filter should drop maintainability with confidence 0.85; got %d findings", len(s.Findings))
	}
}

// TestStart_abstentionFilter_keepsHighConfidence asserts that findings that pass the
// abstention filter (e.g. security with confidence 0.9) still appear in the session.
func TestStart_abstentionFilter_keepsHighConfidence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	keepResp := `[{"file":"a.go","line":1,"severity":"error","category":"security","message":"injection risk","confidence":0.9}]`
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": keepResp, "done": true})
	}))
	defer srv.Close()

	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       false,
		Model:        "m",
		Provider:     "ollama",
		LLMBaseURL:   srv.URL,
	}
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if len(s.Findings) != 1 {
		t.Fatalf("Findings: want 1 (high-confidence security kept), got %d", len(s.Findings))
	}
	f := s.Findings[0]
	if f.Category != "security" || f.Confidence != 0.9 || f.Message != "injection risk" {
		t.Errorf("finding: category=%q confidence=%g message=%q; want security, 0.9, injection risk", f.Category, f.Confidence, f.Message)
	}
}

// TestStart_fpKillList_filtersBannedPhrase asserts that the Phase 6.5 FP kill list
// suppresses findings whose message matches a banned phrase (e.g. "Consider adding comments"),
// regardless of confidence. Mock returns such a finding with confidence 0.9 (passes abstention);
// it must not appear in the final session.
func TestStart_fpKillList_filtersBannedPhrase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bannedResp := `[{"file":"a.go","line":1,"severity":"info","category":"maintainability","message":"Consider adding comments","confidence":0.9}]`
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": bannedResp, "done": true})
	}))
	defer srv.Close()

	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       false,
		Model:        "m",
		Provider:     "ollama",
		LLMBaseURL:   srv.URL,
	}
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if len(s.Findings) != 0 {
		t.Errorf("Findings: FP kill list should drop 'Consider adding comments'; got %d findings", len(s.Findings))
	}
}

// TestStart_strictPreset_keepsLowerConfidence asserts that with strict thresholds (0.6, 0.7),
// a finding with confidence 0.65 is kept in the session.
func TestStart_strictPreset_keepsLowerConfidence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lowConfResp := `[{"file":"a.go","line":1,"severity":"warning","category":"correctness","message":"borderline","confidence":0.65}]`
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": lowConfResp, "done": true})
	}))
	defer srv.Close()

	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot:                     repo,
		StateDir:                     stateDir,
		WorktreeRoot:                 "",
		Ref:                          "HEAD~1",
		DryRun:                       false,
		Model:                        "m",
		Provider:                     "ollama",
		LLMBaseURL:                   srv.URL,
		MinConfidenceKeep:            0.6,
		MinConfidenceMaintainability: 0.7,
		ApplyFPKillList:              ptrBool(true),
	}
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if len(s.Findings) != 1 {
		t.Fatalf("Findings: strict preset should keep confidence 0.65; got %d findings", len(s.Findings))
	}
	if s.Findings[0].Confidence != 0.65 || s.Findings[0].Message != "borderline" {
		t.Errorf("finding: confidence=%g message=%q; want 0.65, borderline", s.Findings[0].Confidence, s.Findings[0].Message)
	}
}

// TestStart_strictPlus_keepsBannedPhrase asserts that when ApplyFPKillList is false (strict+),
// a finding whose message matches the FP kill list (e.g. "Consider adding comments") is kept.
func TestStart_strictPlus_keepsBannedPhrase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bannedResp := `[{"file":"a.go","line":1,"severity":"info","category":"maintainability","message":"Consider adding comments","confidence":0.9}]`
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": bannedResp, "done": true})
	}))
	defer srv.Close()

	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot:                     repo,
		StateDir:                     stateDir,
		WorktreeRoot:                 "",
		Ref:                          "HEAD~1",
		DryRun:                       false,
		Model:                        "m",
		Provider:                     "ollama",
		LLMBaseURL:                   srv.URL,
		MinConfidenceKeep:            0.8,
		MinConfidenceMaintainability: 0.9,
		ApplyFPKillList:              ptrBool(false),
	}
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if len(s.Findings) != 1 {
		t.Fatalf("Findings: strict+ (no FP filter) should keep 'Consider adding comments'; got %d findings", len(s.Findings))
	}
	if s.Findings[0].Message != "Consider adding comments" {
		t.Errorf("finding message = %q; want Consider adding comments", s.Findings[0].Message)
	}
}

// TestStart_nitpicky_keepsBannedPhrase asserts that when Nitpicky is true, the FP kill list
// is not applied, so a finding whose message matches the FP kill list (e.g. "Consider adding comments") is kept.
func TestStart_nitpicky_keepsBannedPhrase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bannedResp := `[{"file":"a.go","line":1,"severity":"info","category":"maintainability","message":"Consider adding comments","confidence":0.9}]`
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": bannedResp, "done": true})
	}))
	defer srv.Close()

	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot:                     repo,
		StateDir:                     stateDir,
		WorktreeRoot:                 "",
		Ref:                          "HEAD~1",
		DryRun:                       false,
		Model:                        "m",
		Provider:                     "ollama",
		LLMBaseURL:                   srv.URL,
		MinConfidenceKeep:            0.8,
		MinConfidenceMaintainability: 0.9,
		Nitpicky:                     true,
	}
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if len(s.Findings) != 1 {
		t.Fatalf("Findings: nitpicky should keep 'Consider adding comments' (no FP filter); got %d findings", len(s.Findings))
	}
	if s.Findings[0].Message != "Consider adding comments" {
		t.Errorf("finding message = %q; want Consider adding comments", s.Findings[0].Message)
	}
}

// praiseOnlyResp is a canned LLM response with a single praise-only info finding (issue #25).
const praiseOnlyResp = `[{"file":"a.go","line":1,"severity":"info","category":"correctness","message":"This correctly handles the empty input.","confidence":0.95}]`

// startWithPraiseOnlyResponse runs Start against a fake Ollama server returning
// praiseOnlyResp and returns the stored session findings.
func startWithPraiseOnlyResponse(t *testing.T, nitpicky bool) []findings.Finding {
	t.Helper()
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": praiseOnlyResp, "done": true})
	}))
	defer srv.Close()

	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := StartOptions{
		RepoRoot:                     repo,
		StateDir:                     stateDir,
		Ref:                          "HEAD~1",
		Model:                        "m",
		Provider:                     "ollama",
		LLMBaseURL:                   srv.URL,
		MinConfidenceKeep:            0.8,
		MinConfidenceMaintainability: 0.9,
		Nitpicky:                     nitpicky,
	}
	if _, err := Start(context.Background(), opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	return s.Findings
}

// TestStart_praiseFilter_dropsPraiseOnlyInfo asserts that with FP filtering on
// (default), a praise-only info finding without a suggestion is not stored.
func TestStart_praiseFilter_dropsPraiseOnlyInfo(t *testing.T) {
	t.Parallel()
	if got := startWithPraiseOnlyResponse(t, false); len(got) != 0 {
		t.Errorf("Findings: praise filter should drop praise-only info finding; got %+v", got)
	}
}

// TestStart_nitpicky_keepsPraiseOnlyInfo asserts that nitpicky mode (FP filtering
// off) skips the praise filter, so the praise-only info finding is stored.
func TestStart_nitpicky_keepsPraiseOnlyInfo(t *testing.T) {
	t.Parallel()
	got := startWithPraiseOnlyResponse(t, true)
	if len(got) != 1 {
		t.Fatalf("Findings: nitpicky should keep praise-only finding; got %d findings", len(got))
	}
	if got[0].Message != "This correctly handles the empty input." {
		t.Errorf("finding message = %q; want praise message", got[0].Message)
	}
}

// TestRunReviewPipeline_praiseFilter asserts the pipeline drops praise-only findings
// and traces "Praise filter: N -> M" only when ApplyFP is set.
func TestRunReviewPipeline_praiseFilter(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": praiseOnlyResp, "done": true})
	}))
	defer srv.Close()

	client, err := llm.NewClient("ollama", srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	systemBase, err := prompt.SystemPrompt(t.TempDir(), "")
	if err != nil {
		t.Fatalf("SystemPrompt: %v", err)
	}
	hunk := diff.Hunk{FilePath: "a.go", RawContent: "@@ -1 +1 @@\npackage main\n", Context: "package main\n"}
	for _, tc := range []struct {
		name      string
		applyFP   bool
		wantCount int
		wantTrace bool
	}{
		{name: "fp_on_drops", applyFP: true, wantCount: 0, wantTrace: true},
		{name: "fp_off_keeps", applyFP: false, wantCount: 1, wantTrace: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var traceBuf bytes.Buffer
			collected, _, _, _, _, err := runReviewPipeline(context.Background(), reviewPipelineOpts{
				Client:     client,
				Model:      "m",
				Hunks:      []diff.Hunk{hunk},
				GenOpts:    &ollama.GenerateOptions{},
				SystemBase: systemBase,
				ApplyFP:    tc.applyFP,
				TraceOut:   trace.New(&traceBuf),
			})
			if err != nil {
				t.Fatalf("runReviewPipeline: %v", err)
			}
			if len(collected) != tc.wantCount {
				t.Errorf("collected = %+v, want %d findings", collected, tc.wantCount)
			}
			gotTrace := strings.Contains(traceBuf.String(), "Praise filter: 1 -> 0")
			if gotTrace != tc.wantTrace {
				t.Errorf("trace contains Praise filter: 1 -> 0 = %v, want %v; trace = %q", gotTrace, tc.wantTrace, traceBuf.String())
			}
		})
	}
}

// TestRunReviewPipeline_evidenceAddedLines asserts the pipeline keeps a finding
// within 3 lines of an added line, drops one on a context line farther away,
// and traces "Evidence (added lines ±3): N -> M".
func TestRunReviewPipeline_evidenceAddedLines(t *testing.T) {
	t.Parallel()
	const resp = `[{"file":"a.go","line":5,"severity":"warning","category":"bug","message":"Nil pointer dereference when cfg is nil.","suggestion":"Check cfg for nil.","confidence":0.95},` +
		`{"file":"a.go","line":1,"severity":"warning","category":"bug","message":"Unchecked error from os.Open.","suggestion":"Handle the error.","confidence":0.95}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": resp, "done": true})
	}))
	defer srv.Close()

	client, err := llm.NewClient("ollama", srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	systemBase, err := prompt.SystemPrompt(t.TempDir(), "")
	if err != nil {
		t.Fatalf("SystemPrompt: %v", err)
	}
	// New-file lines 1-4 and 6-8 are context; line 5 is the only added line.
	raw := "@@ -1,7 +1,8 @@\n a\n b\n c\n d\n+e\n f\n g\n h\n"
	hunk := diff.Hunk{FilePath: "a.go", RawContent: raw, Context: raw}
	var traceBuf bytes.Buffer
	collected, _, _, _, _, err := runReviewPipeline(context.Background(), reviewPipelineOpts{
		Client:     client,
		Model:      "m",
		Hunks:      []diff.Hunk{hunk},
		GenOpts:    &ollama.GenerateOptions{},
		SystemBase: systemBase,
		TraceOut:   trace.New(&traceBuf),
	})
	if err != nil {
		t.Fatalf("runReviewPipeline: %v", err)
	}
	if len(collected) != 1 || collected[0].Line != 5 {
		t.Fatalf("collected = %+v, want only the finding on added line 5", collected)
	}
	if !strings.Contains(traceBuf.String(), "Evidence (added lines ±3): 2 -> 1") {
		t.Errorf("trace missing evidence label; trace = %q", traceBuf.String())
	}
}

// existenceClaimResp is a canned reviewer response whose single finding claims
// `compareTaskIds` is not defined — the SP-831 pattern from issue #33.
const existenceClaimResp = `[ {"file":"a.go","line":2,"severity":"error","category":"correctness","message":"` + "`compareTaskIds` is not defined." + `","confidence":0.95} ]`

// TestRunReviewPipeline_existenceFilterBeforeCritic asserts the existence-claim
// filter runs before the critic in every strictness mode (ApplyFP on and off): a
// finding whose claim the post-change file disproves is dropped without a critic
// request (the critic model receives nothing), and the trace logs
// "Existence claims: 1 -> 0".
func TestRunReviewPipeline_existenceFilterBeforeCritic(t *testing.T) {
	t.Parallel()
	const existenceFileContent = "const x = 1;\nlet compareTaskIds = (a, b) => a === b;\nconst y = 2;\n"
	for _, tc := range []struct {
		name    string
		applyFP bool
	}{
		{name: "fp_off_strictness_plus", applyFP: false},
		{name: "fp_on_strictness_default", applyFP: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var criticRequests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Model string `json:"model"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body.Model == "critic-model" {
					criticRequests.Add(1)
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": existenceClaimResp, "done": true})
			}))
			defer srv.Close()

			client, err := llm.NewClient("ollama", srv.URL, srv.Client())
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			systemBase, err := prompt.SystemPrompt(t.TempDir(), "")
			if err != nil {
				t.Fatalf("SystemPrompt: %v", err)
			}
			repo := t.TempDir()
			if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte(existenceFileContent), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			raw := "@@ -1,2 +1,3 @@\n const x = 1;\n+let compareTaskIds = (a, b) => a === b;\n const y = 2;\n"
			hunk := diff.Hunk{FilePath: "a.go", RawContent: raw, Context: raw}
			var traceBuf bytes.Buffer
			collected, _, _, _, _, err := runReviewPipeline(context.Background(), reviewPipelineOpts{
				Client:        client,
				Model:         "m",
				Hunks:         []diff.Hunk{hunk},
				GenOpts:       &ollama.GenerateOptions{},
				SystemBase:    systemBase,
				RepoRoot:      repo,
				ApplyFP:       tc.applyFP,
				CriticEnabled: true,
				CriticModel:   "critic-model",
				TraceOut:      trace.New(&traceBuf),
			})
			if err != nil {
				t.Fatalf("runReviewPipeline: %v", err)
			}
			if len(collected) != 0 {
				t.Errorf("collected = %+v; want the disproven existence claim dropped", collected)
			}
			if got := criticRequests.Load(); got != 0 {
				t.Errorf("critic received %d requests; want 0 (existence filter must run before the critic)", got)
			}
			if !strings.Contains(traceBuf.String(), "Existence claims: 1 -> 0") {
				t.Errorf("trace missing \"Existence claims: 1 -> 0\"; trace = %q", traceBuf.String())
			}
		})
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
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       false,
		Model:        "m",
		Provider:     "ollama",
		LLMBaseURL:   srv.URL,
	}
	_, err := Start(ctx, opts)
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
		RepoRoot:      repo,
		StateDir:      stateDir,
		WorktreeRoot:  "",
		Ref:           "HEAD~1",
		DryRun:        false,
		Model:         "m",
		Provider:      "ollama",
		LLMBaseURL:    srv.URL,
		ContextLimit:  100,
		WarnThreshold: 0.9,
	}
	_, err = Start(ctx, opts)
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
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       true,
		Model:        "",
		Provider:     "ollama",
		LLMBaseURL:   "",
	}
	if _, err := Start(ctx, startOpts); err != nil {
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
		Provider:      "ollama",
		LLMBaseURL:    srv.URL,
		ContextLimit:  100,
		WarnThreshold: 0.9,
	}
	if _, err := Run(ctx, runOpts); err != nil {
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

// TestRun_pipeline_multipleHunks_ordersFindingsAndOneGeneratePerHunk asserts that
// the review pipeline sends one Generate per hunk, returns findings in hunk order,
// and sends a short keep_alive on the last request so the model is unloaded.
func TestRun_pipeline_multipleHunks_ordersFindingsAndOneGeneratePerHunk(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var generateCalls int
	var keepAlivePerRequest []interface{}
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
		var body struct {
			KeepAlive interface{} `json:"keep_alive"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		keepAlivePerRequest = append(keepAlivePerRequest, body.KeepAlive)
		generateCalls++
		msg := "pipeline-hunk-" + string(rune('0'+generateCalls))
		if generateCalls > 9 {
			msg = "pipeline-hunk-" + string(rune('0'+generateCalls/10)) + string(rune('0'+generateCalls%10))
		}
		resp := `[{"file":"a.go","line":1,"severity":"info","category":"style","message":"` + msg + `"}]`
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": resp, "done": true})
	}))
	defer srv.Close()

	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	writeFile(t, repo, "f3.txt", "c\n")
	writeFile(t, repo, "f4.txt", "d\n")
	runGit(t, repo, "git", "add", "f3.txt", "f4.txt")
	runGit(t, repo, "git", "commit", "-m", "c3")
	// Start with mock: pipeline runs during Start and reviews 2 hunks (f3, f4).
	opts := StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       false,
		Model:        "m",
		Provider:     "ollama",
		LLMBaseURL:   srv.URL,
	}
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	wantHunks := 2
	if generateCalls != wantHunks {
		t.Errorf("generate calls = %d, want %d (one per hunk)", generateCalls, wantHunks)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if len(s.Findings) < wantHunks {
		t.Errorf("Findings: want at least %d, got %d", wantHunks, len(s.Findings))
	}
	for i := 0; i < wantHunks && i < len(s.Findings); i++ {
		wantMsg := "pipeline-hunk-" + string(rune('0'+i+1))
		if !strings.Contains(s.Findings[i].Message, wantMsg) {
			t.Errorf("Findings[%d].Message = %q, want to contain %q (order)", i, s.Findings[i].Message, wantMsg)
		}
	}
	if len(keepAlivePerRequest) != wantHunks {
		t.Errorf("keep_alive recorded for %d requests, want %d", len(keepAlivePerRequest), wantHunks)
	} else if wantHunks > 0 {
		last := keepAlivePerRequest[wantHunks-1]
		if last == nil {
			t.Error("last /api/generate request should send keep_alive (short value to unload model)")
		} else if n, ok := last.(float64); !ok || n != 0 {
			t.Errorf("last request keep_alive = %v (type %T), want 0 for good stewardship", last, last)
		}
	}
}

// TestSuppressionWiring_systemPromptContainsSection asserts that when suppression
// examples are loaded from history and passed to PrepareHunkPrompt (per-hunk
// wiring), the returned system prompt contains the "Previously dismissed false
// positives" section with the example.
func TestSuppressionWiring_systemPromptContainsSection(t *testing.T) {
	t.Parallel()
	stateDir := t.TempDir()
	rec := history.Record{
		DiffRef: "HEAD",
		ReviewOutput: []findings.Finding{
			{ID: "f1", File: "pkg/foo.go", Line: 42, Severity: findings.SeverityInfo, Category: findings.CategoryMaintainability, Confidence: 0.9, Message: "Consider adding comments"},
		},
		UserAction: history.UserAction{
			Dismissals: []history.Dismissal{{FindingID: "f1", Reason: history.ReasonFalsePositive}},
		},
	}
	if err := history.Append(stateDir, rec, 0); err != nil {
		t.Fatalf("history.Append: %v", err)
	}
	examples, err := history.SuppressionExamples(stateDir, 50, 30)
	if err != nil {
		t.Fatalf("SuppressionExamples: %v", err)
	}
	if len(examples) == 0 {
		t.Fatal("SuppressionExamples: want at least one example")
	}
	systemBase, err := prompt.SystemPrompt(stateDir, "")
	if err != nil {
		t.Fatalf("SystemPrompt: %v", err)
	}
	hunk := diff.Hunk{FilePath: "pkg/foo.go", RawContent: "@@ -1,1 +1,1 @@\n code\n", Context: "code"}
	ctx := context.Background()
	system, _, err := review.PrepareHunkPrompt(ctx, systemBase, hunk, nil, "", 32768, 0, 0, false, 0, 0, 0, false, examples, nil, 0, nil)
	if err != nil {
		t.Fatalf("PrepareHunkPrompt: %v", err)
	}
	if !strings.Contains(system, "## Previously dismissed false positives (do not report or discuss)") {
		t.Error("system prompt should contain suppression section header")
	}
	wantEx := "pkg/foo.go:42: Consider adding comments"
	if !strings.Contains(system, wantEx) {
		t.Errorf("system prompt should contain example %q", wantEx)
	}
}

func TestMaybeAutoFinish_zeroActiveFindings_finishes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{
		RepoRoot: repo,
		StateDir: stateDir,
		Ref:      "HEAD",
		Provider: "ollama",
	}
	if _, err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	finished, err := MaybeAutoFinish(ctx, MaybeAutoFinishOptions{
		RepoRoot: repo, StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("MaybeAutoFinish: %v", err)
	}
	if !finished {
		t.Fatal("MaybeAutoFinish: want finished=true")
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if s.BaselineRef != "" {
		t.Errorf("session should be cleared after auto-finish; BaselineRef = %q", s.BaselineRef)
	}
}

func TestMaybeAutoFinish_activeFindings_skips(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{
		RepoRoot: repo, StateDir: stateDir, Ref: "HEAD~1", DryRun: true,
		Provider: "ollama",
	}
	if _, err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	finished, err := MaybeAutoFinish(ctx, MaybeAutoFinishOptions{
		RepoRoot: repo, StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("MaybeAutoFinish: %v", err)
	}
	if finished {
		t.Fatal("MaybeAutoFinish: want finished=false when findings remain")
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if s.BaselineRef == "" {
		t.Fatal("session should remain when active findings exist")
	}
}

func TestActiveFindingCount_excludesDismissed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{
		RepoRoot: repo, StateDir: stateDir, Ref: "HEAD~1", DryRun: true,
		Provider: "ollama",
	}
	if _, err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if len(s.Findings) == 0 {
		t.Fatal("want findings from dry-run start")
	}
	s.DismissedIDs = []string{s.Findings[0].ID}
	if err := session.Save(stateDir, &s); err != nil {
		t.Fatalf("Save session: %v", err)
	}
	count, err := ActiveFindingCount(stateDir)
	if err != nil {
		t.Fatalf("ActiveFindingCount: %v", err)
	}
	want := len(s.Findings) - 1
	if count != want {
		t.Errorf("ActiveFindingCount = %d, want %d", count, want)
	}
}

func TestTruncateForPromptContext(t *testing.T) {
	t.Parallel()
	short := "hello"
	if got := truncateForPromptContext(short, 10); got != short {
		t.Errorf("short string: got %q", got)
	}
	long := strings.Repeat("x", 20)
	got := truncateForPromptContext(long, 10)
	if len(got) <= 10 || !strings.Contains(got, "[truncated]") {
		t.Errorf("truncated: got len %d, want truncation marker", len(got))
	}
	if truncateForPromptContext("x", 0) != "x" {
		t.Error("maxLen 0 should return unchanged")
	}
}

func TestLogExcludedPaths(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logExcludedPaths(&buf, []string{"a.go", "b.go"})
	out := buf.String()
	if !strings.Contains(out, "a.go") || !strings.Contains(out, "b.go") {
		t.Errorf("output = %q", out)
	}
}

func TestMergePendingExcluded(t *testing.T) {
	t.Parallel()
	got := mergePendingExcluded([]string{"a.go"}, []string{"b.go", "a.go"})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if mergePendingExcluded(nil, nil) != nil {
		t.Error("nil inputs should return nil when skipped empty")
	}
	if got := mergePendingExcluded([]string{"x"}, nil); len(got) != 1 || got[0] != "x" {
		t.Errorf("no skipped: got %v", got)
	}
}

func TestWriteStreamLine_andTryWriteStreamLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := writeStreamLine(&buf, map[string]string{"type": "progress"}); err != nil {
		t.Fatalf("writeStreamLine: %v", err)
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Error("want trailing newline")
	}
	tryWriteStreamLine(&buf, map[string]int{"n": 1})
}

func TestRunPromptShadows(t *testing.T) {
	t.Parallel()
	if runPromptShadows(nil) != nil {
		t.Error("nil session should return nil")
	}
	s := &session.Session{PromptShadows: []session.PromptShadow{
		{FindingID: "f1", PromptContext: "ctx"},
	}}
	out := runPromptShadows(s)
	if len(out) != 1 || out[0].FindingID != "f1" {
		t.Errorf("got %+v", out)
	}
}

func TestCaptureUsage(t *testing.T) {
	old := os.Getenv("STET_CAPTURE_USAGE")
	defer func() {
		if old == "" {
			os.Unsetenv("STET_CAPTURE_USAGE")
		} else {
			os.Setenv("STET_CAPTURE_USAGE", old)
		}
	}()
	for _, v := range []string{"0", "false", "no", "off"} {
		os.Setenv("STET_CAPTURE_USAGE", v)
		if captureUsage() {
			t.Errorf("captureUsage(%q) = true, want false", v)
		}
	}
	os.Setenv("STET_CAPTURE_USAGE", "true")
	if !captureUsage() {
		t.Error("captureUsage(true) = false, want true")
	}
	os.Unsetenv("STET_CAPTURE_USAGE")
	if !captureUsage() {
		t.Error("default should be true")
	}
}

func TestRemoveReviewedFromPending(t *testing.T) {
	t.Parallel()
	hunk := diff.Hunk{FilePath: "done.go"}
	got := removeReviewedFromPending([]string{"done.go", "pending.go"}, []diff.Hunk{hunk})
	if len(got) != 1 || got[0] != "pending.go" {
		t.Errorf("got %v", got)
	}
}

func TestTryWriteStreamLine_logsOnError(t *testing.T) {
	t.Parallel()
	tryWriteStreamLine(&failWriter{}, map[string]string{"x": "y"})
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRunReviewPipeline_emptyHunks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	collected, fpc, sp, sc, sd, err := runReviewPipeline(ctx, reviewPipelineOpts{})
	if err != nil {
		t.Fatalf("runReviewPipeline: %v", err)
	}
	if collected != nil || len(fpc) != 0 || sp != 0 || sc != 0 || sd != 0 {
		t.Errorf("empty hunks: collected=%v fpc=%v sp=%d sc=%d sd=%d", collected, fpc, sp, sc, sd)
	}
}

func TestRunReviewPipeline_singleHunkWithStream(t *testing.T) {
	validResp := `[{"file":"a.go","line":1,"severity":"info","category":"style","message":"pipeline-unit"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(w)
		_ = enc.Encode(map[string]interface{}{"response": validResp, "done": true})
	}))
	defer srv.Close()

	client, err := llm.NewClient("ollama", srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	stateDir := t.TempDir()
	systemBase, err := prompt.SystemPrompt(stateDir, "")
	if err != nil {
		t.Fatalf("SystemPrompt: %v", err)
	}
	var stream bytes.Buffer
	hunk := diff.Hunk{FilePath: "a.go", RawContent: "@@ -1 +1 @@\npackage main\n", Context: "package main\n"}
	ctx := context.Background()
	collected, fpc, _, _, _, err := runReviewPipeline(ctx, reviewPipelineOpts{
		Client:                client,
		Model:                 "m",
		Hunks:                 []diff.Hunk{hunk},
		GenOpts:               &ollama.GenerateOptions{},
		SystemBase:            systemBase,
		RepoRoot:              "",
		EffectiveContextLimit: 32768,
		StreamOut:             &stream,
	})
	if err != nil {
		t.Fatalf("runReviewPipeline: %v", err)
	}
	if len(collected) != 1 {
		t.Fatalf("collected len = %d, want 1", len(collected))
	}
	if collected[0].Message != "pipeline-unit" {
		t.Errorf("message = %q", collected[0].Message)
	}
	if len(fpc) != 1 {
		t.Errorf("findingPromptContext len = %d, want 1", len(fpc))
	}
	if !strings.Contains(stream.String(), `"type":"progress"`) {
		t.Errorf("stream output missing progress: %q", stream.String())
	}
}

// fakeCriticLLM implements llm.Client for applyCritic tests. Only Generate is
// exercised by the critic path; the other methods exist to satisfy the interface.
type fakeCriticLLM struct {
	errors    []error                   // error per Generate call (last repeats); nil entries succeed
	responses []string                  // response body per Generate call (last repeats)
	calls     int                       // total Generate calls
	models    []string                  // model per call
	systems   []string                  // system prompt per call (rules-selector tests)
	prompts   []string                  // user prompt per call (critic-input tests)
	genOpts   []*ollama.GenerateOptions // options per call
}

func (f *fakeCriticLLM) generate(model, systemPrompt, userPrompt string, opts *ollama.GenerateOptions) (*ollama.GenerateResult, error) {
	f.calls++
	f.models = append(f.models, model)
	f.systems = append(f.systems, systemPrompt)
	f.prompts = append(f.prompts, userPrompt)
	f.genOpts = append(f.genOpts, opts)
	if i := f.calls - 1; i < len(f.errors) && f.errors[i] != nil {
		return nil, f.errors[i]
	}
	i := len(f.responses) - 1
	if f.calls-1 < len(f.responses) {
		i = f.calls - 1
	}
	return &ollama.GenerateResult{Response: f.responses[i]}, nil
}

func (f *fakeCriticLLM) Check(ctx context.Context, model string) (*ollama.CheckResult, error) {
	return &ollama.CheckResult{}, nil
}

func (f *fakeCriticLLM) Generate(ctx context.Context, model, systemPrompt, userPrompt string, opts *ollama.GenerateOptions) (*ollama.GenerateResult, error) {
	return f.generate(model, systemPrompt, userPrompt, opts)
}

func (f *fakeCriticLLM) GeneratePlain(ctx context.Context, model, systemPrompt, userPrompt string, opts *ollama.GenerateOptions) (*ollama.GenerateResult, error) {
	return f.generate(model, systemPrompt, userPrompt, opts)
}

func (f *fakeCriticLLM) GenerateWithMessages(ctx context.Context, model string, messages []ollama.Message, opts *ollama.GenerateOptions) (*ollama.GenerateResult, error) {
	return f.generate(model, "", "", opts)
}

// criticContextCapForTest mirrors review's defaultCriticContextCap (16384):
// the cap is unexported, so the run-package truncation tests pin the literal.
const criticContextCapForTest = 16384

// preparedUserPromptFor builds a user prompt in the shape PrepareHunkPrompt
// produces for an expanded hunk: the hunk block (with enclosing-function
// context) plus the RAG symbol-definitions block via
// prompt.UserPromptWithRAGPlacement. The expanded-context and definition
// markers let tests assert what the critic actually receives.
func preparedUserPromptFor(hunk diff.Hunk) string {
	defs := []rag.Definition{{Symbol: "helperFunc", File: "helper.go", Line: 3, Signature: "func helperFunc() {}"}}
	return prompt.UserPromptWithRAGPlacement(hunk.Context, prompt.FormatSymbolDefinitions(defs, 0))
}

func TestApplyCritic(t *testing.T) {
	t.Parallel()
	yes := `{"verdict":"yes","reason":"ok"}`
	no := `{"verdict":"no","reason":"bad"}`
	hunk := diff.Hunk{FilePath: "a.go", RawContent: "package main\n"}
	p := &preparedPrompt{Hunk: hunk}
	mkBatch := func() []findings.Finding {
		return []findings.Finding{
			{ID: "f1", File: "a.go", Line: 1, Severity: "warning", Category: "bug", Message: "m1"},
			{ID: "f2", File: "a.go", Line: 2, Severity: "warning", Category: "style", Message: "m2"},
		}
	}

	t.Run("disabled returns batch unchanged with no calls", func(t *testing.T) {
		fake := &fakeCriticLLM{}
		in := mkBatch()
		out, err := applyCritic(context.Background(), reviewPipelineOpts{Client: fake}, p, in)
		if err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		if len(out) != 2 || out[0].ID != "f1" || out[1].ID != "f2" {
			t.Errorf("out = %+v, want input unchanged", out)
		}
		if fake.calls != 0 {
			t.Errorf("calls = %d, want 0", fake.calls)
		}
	})

	t.Run("empty batch returns unchanged with no calls", func(t *testing.T) {
		fake := &fakeCriticLLM{}
		out, err := applyCritic(context.Background(), reviewPipelineOpts{Client: fake, CriticEnabled: true, CriticModel: "critic"}, p, nil)
		if err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		if out != nil {
			t.Errorf("out = %+v, want nil", out)
		}
		if fake.calls != 0 {
			t.Errorf("calls = %d, want 0", fake.calls)
		}
	})

	t.Run("keeps all when critic says yes", func(t *testing.T) {
		var traceBuf bytes.Buffer
		fake := &fakeCriticLLM{responses: []string{yes, yes}}
		opts := reviewPipelineOpts{Client: fake, Model: "main", CriticEnabled: true, CriticModel: "critic", TraceOut: trace.New(&traceBuf)}
		out, err := applyCritic(context.Background(), opts, p, mkBatch())
		if err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		if len(out) != 2 {
			t.Fatalf("kept %d findings, want 2", len(out))
		}
		if fake.calls != 2 {
			t.Errorf("calls = %d, want 2 (one per finding)", fake.calls)
		}
		if got := traceBuf.String(); !strings.Contains(got, "Critic: 2 -> 2") {
			t.Errorf("trace = %q, want Critic: 2 -> 2", got)
		}
	})

	t.Run("drops findings the critic rejects", func(t *testing.T) {
		var traceBuf bytes.Buffer
		fake := &fakeCriticLLM{responses: []string{yes, no}}
		opts := reviewPipelineOpts{Client: fake, Model: "main", CriticEnabled: true, CriticModel: "critic", TraceOut: trace.New(&traceBuf)}
		out, err := applyCritic(context.Background(), opts, p, mkBatch())
		if err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		if len(out) != 1 || out[0].ID != "f1" {
			t.Fatalf("kept = %+v, want only f1", out)
		}
		if got := traceBuf.String(); !strings.Contains(got, "Critic: 2 -> 1") {
			t.Errorf("trace = %q, want Critic: 2 -> 1", got)
		}
	})

	t.Run("generate error returns user error and traces", func(t *testing.T) {
		var traceBuf bytes.Buffer
		genErr := errors.New("connection refused")
		fake := &fakeCriticLLM{errors: []error{genErr}}
		opts := reviewPipelineOpts{Client: fake, Model: "main", CriticEnabled: true, CriticModel: "critic", TraceOut: trace.New(&traceBuf)}
		out, err := applyCritic(context.Background(), opts, p, mkBatch())
		if out != nil {
			t.Errorf("out = %+v, want nil", out)
		}
		if err == nil {
			t.Fatal("applyCritic err = nil, want Critic failed error")
		}
		if !strings.Contains(err.Error(), "Critic failed for a.go.") {
			t.Errorf("err = %v, want Critic failed for a.go.", err)
		}
		if !errors.Is(err, genErr) {
			t.Errorf("err %v does not wrap %v", err, genErr)
		}
		if got := traceBuf.String(); !strings.Contains(got, "Critic request failed: connection refused") {
			t.Errorf("trace = %q, want Critic request failed", got)
		}
		if fake.calls != 1 {
			t.Errorf("calls = %d, want 1 (stop on first error)", fake.calls)
		}
	})

	t.Run("same model keeps alive and propagates max completion tokens", func(t *testing.T) {
		fake := &fakeCriticLLM{responses: []string{yes}}
		opts := reviewPipelineOpts{
			Client: fake, Model: "m", CriticEnabled: true, CriticModel: "m",
			GenOpts: &ollama.GenerateOptions{MaxCompletionTokens: 512},
		}
		if _, err := applyCritic(context.Background(), opts, p, mkBatch()[:1]); err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		if fake.models[0] != "m" {
			t.Errorf("critic model = %q, want m", fake.models[0])
		}
		if fake.genOpts[0].KeepAlive != keepAliveDuringRun {
			t.Errorf("KeepAlive = %v, want %v", fake.genOpts[0].KeepAlive, keepAliveDuringRun)
		}
		if fake.genOpts[0].MaxCompletionTokens != 512 {
			t.Errorf("MaxCompletionTokens = %d, want 512", fake.genOpts[0].MaxCompletionTokens)
		}
	})

	t.Run("different model does not keep alive", func(t *testing.T) {
		fake := &fakeCriticLLM{responses: []string{yes}}
		opts := reviewPipelineOpts{Client: fake, Model: "main", CriticEnabled: true, CriticModel: "critic"}
		if _, err := applyCritic(context.Background(), opts, p, mkBatch()[:1]); err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		if fake.genOpts[0].KeepAlive != 0 {
			t.Errorf("KeepAlive = %v, want 0 (critic model differs from main; VerifyFinding default)", fake.genOpts[0].KeepAlive)
		}
	})

	t.Run("retries once on parse failure then keeps", func(t *testing.T) {
		fake := &fakeCriticLLM{responses: []string{"not json at all", yes}}
		opts := reviewPipelineOpts{Client: fake, Model: "main", CriticEnabled: true, CriticModel: "critic"}
		out, err := applyCritic(context.Background(), opts, p, mkBatch()[:1])
		if err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		if len(out) != 1 {
			t.Fatalf("kept %d findings, want 1 after retry", len(out))
		}
		if fake.calls != 2 {
			t.Errorf("calls = %d, want 2 (initial + parse retry)", fake.calls)
		}
	})

	t.Run("critic receives prepared user prompt with expanded context and RAG definitions", func(t *testing.T) {
		prep := diff.Hunk{
			FilePath:   "a.go",
			RawContent: "package main\n",
			Context:    "## Enclosing function context\n\n```go\nfunc expandedEnclosing() { UNIQUE_EXPANDED_MARKER }\n```\n\n## Diff hunk\n\npackage main\n",
		}
		user := preparedUserPromptFor(prep)
		if !strings.Contains(user, "UNIQUE_EXPANDED_MARKER") || !strings.Contains(user, "func helperFunc() {}") {
			t.Fatalf("prepared prompt should carry the expanded context and the RAG definition, got %q", user)
		}
		pp := &preparedPrompt{User: user, Hunk: prep}
		fake := &fakeCriticLLM{responses: []string{yes, yes}}
		opts := reviewPipelineOpts{Client: fake, Model: "main", CriticEnabled: true, CriticModel: "critic"}
		out, err := applyCritic(context.Background(), opts, pp, mkBatch())
		if err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		if len(out) != 2 {
			t.Fatalf("kept %d findings, want 2", len(out))
		}
		for i, prompt := range fake.prompts {
			for _, want := range []string{"## Enclosing function context", "UNIQUE_EXPANDED_MARKER", "## Symbol definitions (for context)", "func helperFunc() {}"} {
				if !strings.Contains(prompt, want) {
					t.Errorf("critic prompt %d should contain %q from the prepared user prompt, got %q", i, want, prompt)
				}
			}
		}
	})

	t.Run("falls back to raw hunk when prepared prompt is empty", func(t *testing.T) {
		fh := diff.Hunk{FilePath: "a.go", RawContent: "RAW_FALLBACK_HUNK_BODY"}
		pp := &preparedPrompt{Hunk: fh}
		fake := &fakeCriticLLM{responses: []string{yes}}
		opts := reviewPipelineOpts{Client: fake, Model: "main", CriticEnabled: true, CriticModel: "critic"}
		if _, err := applyCritic(context.Background(), opts, pp, mkBatch()[:1]); err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		if !strings.Contains(fake.prompts[0], fh.RawContent) {
			t.Errorf("critic prompt = %q, want it to contain the raw hunk fallback %q", fake.prompts[0], fh.RawContent)
		}
	})

	t.Run("truncates prepared prompt over the critic context cap", func(t *testing.T) {
		big := strings.Repeat("z", criticContextCapForTest+500)
		pp := &preparedPrompt{User: big, Hunk: diff.Hunk{FilePath: "a.go", RawContent: "package main\n"}}
		fake := &fakeCriticLLM{responses: []string{yes}}
		opts := reviewPipelineOpts{Client: fake, Model: "main", CriticEnabled: true, CriticModel: "critic"}
		if _, err := applyCritic(context.Background(), opts, pp, mkBatch()[:1]); err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		got := fake.prompts[0]
		if !strings.Contains(got, "\n[truncated]") {
			t.Errorf("critic prompt should contain the [truncated] marker, got %q", got)
		}
		if !strings.Contains(got, strings.Repeat("z", criticContextCapForTest)) {
			t.Errorf("critic prompt should keep the first %d chars before truncating", criticContextCapForTest)
		}
		if strings.Contains(got, strings.Repeat("z", criticContextCapForTest+1)) {
			t.Errorf("critic prompt should not carry more than %d chars of the prepared prompt", criticContextCapForTest)
		}
	})

	t.Run("trace prints critic input length next to the Critic line", func(t *testing.T) {
		var traceBuf bytes.Buffer
		user := preparedUserPromptFor(diff.Hunk{FilePath: "a.go", RawContent: "package main\n", Context: "expanded context"})
		pp := &preparedPrompt{User: user, Hunk: diff.Hunk{FilePath: "a.go", RawContent: "package main\n"}}
		fake := &fakeCriticLLM{responses: []string{yes}}
		opts := reviewPipelineOpts{Client: fake, Model: "main", CriticEnabled: true, CriticModel: "critic", TraceOut: trace.New(&traceBuf)}
		if _, err := applyCritic(context.Background(), opts, pp, mkBatch()[:1]); err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		got := traceBuf.String()
		wantInput := fmt.Sprintf("Critic input: %d chars", len(user))
		if !strings.Contains(got, wantInput) {
			t.Errorf("trace = %q, want %q", got, wantInput)
		}
		ci, cl := strings.Index(got, "Critic input:"), strings.Index(got, "Critic: 1 -> 1")
		if ci < 0 || cl < 0 || ci > cl {
			t.Errorf("trace = %q, want Critic input line printed once next to the Critic line", got)
		}
		if strings.Count(got, "Critic input:") != 1 {
			t.Errorf("trace = %q, want exactly one Critic input line per hunk", got)
		}
	})
}

// fakeSystemOneDecider implements review.SystemOneDecider for applyCritic
// System One tests. It records every request; responses/errors repeat the
// last entry once the slice is exhausted.
type fakeSystemOneDecider struct {
	responses []*systemone.Response // response per Decide call (last repeats)
	errs      []error               // error per Decide call (last repeats); nil entries succeed
	calls     int
	requests  []systemone.Request
}

func (f *fakeSystemOneDecider) Decide(ctx context.Context, req systemone.Request) (*systemone.Response, error) {
	f.calls++
	f.requests = append(f.requests, req)
	if len(f.errs) > 0 {
		i := f.calls - 1
		if i >= len(f.errs) {
			i = len(f.errs) - 1
		}
		if f.errs[i] != nil {
			return nil, f.errs[i]
		}
	}
	i := len(f.responses) - 1
	if f.calls-1 < len(f.responses) {
		i = f.calls - 1
	}
	return f.responses[i], nil
}

// noulResponse builds a System One response mapping question key -> P(yes).
func noulResponse(probs map[string]float64) *systemone.Response {
	answers := make(map[string]systemone.NoulAnswer, len(probs))
	for k, v := range probs {
		answers[k] = systemone.NoulAnswer{Noul: v}
	}
	return &systemone.Response{Model: "openjev-0.1", Answers: answers}
}

// fakeChoiceDecider implements rules.ChoiceDecider for rules-selector tests.
// It records every request; responses/errors repeat the last entry once the
// slice is exhausted.
// fakeChoiceDecider is called concurrently by runReviewPipeline's prepare
// workers, so mu guards every field the call mutates or reads.
type fakeChoiceDecider struct {
	mu        sync.Mutex
	responses []*systemone.ChoiceResponse // response per DecideChoice call (last repeats)
	errs      []error                     // error per call (last repeats); nil entries succeed
	calls     int
	requests  []systemone.ChoiceRequest
}

func (f *fakeChoiceDecider) DecideChoice(ctx context.Context, req systemone.ChoiceRequest) (*systemone.ChoiceResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.requests = append(f.requests, req)
	if len(f.errs) > 0 {
		i := f.calls - 1
		if i >= len(f.errs) {
			i = len(f.errs) - 1
		}
		if f.errs[i] != nil {
			return nil, f.errs[i]
		}
	}
	i := len(f.responses) - 1
	if f.calls-1 < len(f.responses) {
		i = f.calls - 1
	}
	return f.responses[i], nil
}

// choiceResponse builds a System One choice response with per-option
// probabilities for the selector's single question (key "q0").
func choiceResponse(probs map[string]float64) *systemone.ChoiceResponse {
	return &systemone.ChoiceResponse{
		Model: "sel-model",
		Answers: map[string]systemone.ChoiceAnswer{
			"q0": {Probabilities: probs},
		},
	}
}

// selectorRule returns an always-apply rule whose content splits into three
// named sections (Error handling, Validation, Naming).
func selectorRule() rules.CursorRule {
	content := "## Error handling\n\nReturn wrapped errors at boundaries.\n\n## Validation\n\nValidate untrusted input at the system boundary.\n\n## Naming\n\nUse consistent, descriptive identifiers."
	return rules.CursorRule{Source: "std.mdc", AlwaysApply: true, Content: content}
}

func mkSelectorOpts(decider *fakeChoiceDecider, warnOut io.Writer, hunks []diff.Hunk) reviewPipelineOpts {
	return reviewPipelineOpts{
		Client:               &fakeCriticLLM{responses: []string{"[{\"file\":\"a.go\",\"line\":1,\"severity\":\"warning\",\"category\":\"style\",\"message\":\"m\"}]"}},
		Model:                "main",
		Hunks:                hunks,
		GenOpts:              &ollama.GenerateOptions{},
		SystemBase:           "BASE",
		RulesByFile:          map[string][]rules.CursorRule{"a.go": {selectorRule()}},
		RulesSelectorClient:  decider,
		RulesSelectorBaseURL: "http://selector.example:8080",
		RulesSelectorModel:   "sel-model",
		RulesSelectorWarnOut: warnOut,
	}
}

// TestSelectRuleSectionsForHunk covers the per-hunk selection helper: disabled,
// no matching rules, success, and fail-open error handling.
func TestSelectRuleSectionsForHunk(t *testing.T) {
	t.Parallel()
	hunk := diff.Hunk{FilePath: "a.go", RawContent: "package main\n"}

	t.Run("disabled returns nil with no requests", func(t *testing.T) {
		t.Parallel()
		decider := &fakeChoiceDecider{}
		got := selectRuleSectionsForHunk(context.Background(), reviewPipelineOpts{RulesSelectorClient: nil}, hunk, []rules.CursorRule{selectorRule()}, nil)
		if got != nil {
			t.Errorf("sections = %v, want nil when selector disabled", got)
		}
		if decider.calls != 0 {
			t.Errorf("decider calls = %d, want 0", decider.calls)
		}
	})

	t.Run("no matching rules returns nil with no requests", func(t *testing.T) {
		t.Parallel()
		decider := &fakeChoiceDecider{}
		nonMatching := rules.CursorRule{Source: "ts.mdc", Globs: []string{"*.ts"}, Content: "## TS\n\nPrefer const."}
		got := selectRuleSectionsForHunk(context.Background(), mkSelectorOpts(decider, io.Discard, nil), hunk, []rules.CursorRule{nonMatching}, nil)
		if got != nil {
			t.Errorf("sections = %v, want nil when no rules match the file", got)
		}
		if decider.calls != 0 {
			t.Errorf("decider calls = %d, want 0 (no server request without candidates)", decider.calls)
		}
	})

	t.Run("success returns ranked sections and sends one choice request", func(t *testing.T) {
		t.Parallel()
		decider := &fakeChoiceDecider{responses: []*systemone.ChoiceResponse{choiceResponse(map[string]float64{"s2": 0.6, "s0": 0.3, "s1": 0.1})}}
		got := selectRuleSectionsForHunk(context.Background(), mkSelectorOpts(decider, io.Discard, nil), hunk, []rules.CursorRule{selectorRule()}, nil)
		if len(got) != 3 {
			t.Fatalf("sections = %d, want 3 (all fit in MaxRuleTokens)", len(got))
		}
		if got[0].Heading != "Naming" || got[1].Heading != "Error handling" || got[2].Heading != "Validation" {
			t.Errorf("ranked order = %v, want Naming, Error handling, Validation", []string{got[0].Heading, got[1].Heading, got[2].Heading})
		}
		if decider.calls != 1 {
			t.Errorf("decider calls = %d, want 1", decider.calls)
		}
		req := decider.requests[0]
		if req.Model != "sel-model" {
			t.Errorf("request model = %q, want sel-model (RulesSelectorModel passed through)", req.Model)
		}
		if !strings.Contains(req.State, "package main") {
			t.Errorf("request state must carry the hunk content; got %q", req.State)
		}
		if len(req.Questions) != 1 {
			t.Errorf("request questions = %d, want 1 choice question", len(req.Questions))
		}
	})

	t.Run("selector error returns nil with one warning", func(t *testing.T) {
		t.Parallel()
		decider := &fakeChoiceDecider{errs: []error{errors.New("connection refused")}}
		var warnBuf bytes.Buffer
		warn := &rulesSelectorState{warnOut: &warnBuf, baseURL: "http://selector.example:8080"}
		got := selectRuleSectionsForHunk(context.Background(), mkSelectorOpts(decider, io.Discard, nil), hunk, []rules.CursorRule{selectorRule()}, warn)
		if got != nil {
			t.Errorf("sections = %v, want nil on selector error", got)
		}
		if !strings.Contains(warnBuf.String(), "http://selector.example:8080") {
			t.Errorf("warning must name the server URL; got %q", warnBuf.String())
		}
		if strings.Contains(warnBuf.String(), "supersecret") {
			t.Errorf("warning must never contain the API key; got %q", warnBuf.String())
		}
	})
}

// TestRunReviewPipeline_rulesSelector asserts the pipeline's enabled-path
// behaviour: one selector request per hunk, ranked sections in each system
// prompt, and a single fail-open warning when selection fails.
func TestRunReviewPipeline_rulesSelector(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hunks := []diff.Hunk{
		{FilePath: "a.go", RawContent: "hunk one\n"},
		{FilePath: "a.go", RawContent: "hunk two\n"},
		{FilePath: "a.go", RawContent: "hunk three\n"},
	}

	t.Run("one request per hunk and ranked sections in prompt", func(t *testing.T) {
		t.Parallel()
		decider := &fakeChoiceDecider{responses: []*systemone.ChoiceResponse{choiceResponse(map[string]float64{"s2": 0.6, "s0": 0.3, "s1": 0.1})}}
		var warnBuf bytes.Buffer
		opts := mkSelectorOpts(decider, &warnBuf, hunks)
		fake := &fakeCriticLLM{responses: []string{"[{\"file\":\"a.go\",\"line\":1,\"severity\":\"warning\",\"category\":\"style\",\"message\":\"m\"}]"}}
		opts.Client = fake
		collected, _, _, _, _, err := runReviewPipeline(ctx, opts)
		if err != nil {
			t.Fatalf("runReviewPipeline: %v", err)
		}
		if decider.calls != len(hunks) {
			t.Errorf("decider calls = %d, want %d (one request per hunk)", decider.calls, len(hunks))
		}
		if len(fake.systems) != len(hunks) {
			t.Fatalf("generate calls = %d, want %d", len(fake.systems), len(hunks))
		}
		naming := "Use consistent, descriptive identifiers."
		errorHandling := "Return wrapped errors at boundaries."
		validation := "Validate untrusted input at the system boundary."
		for i, system := range fake.systems {
			iNaming, iError := strings.Index(system, naming), strings.Index(system, errorHandling)
			iValidation := strings.Index(system, validation)
			if iNaming < 0 || iError < 0 || iValidation < 0 {
				t.Errorf("hunk %d system prompt missing ranked sections:\n%s", i, system)
				continue
			}
			if !(iNaming < iError && iError < iValidation) {
				t.Errorf("hunk %d sections not in ranked order (Naming, Error handling, Validation):\n%s", i, system)
			}
		}
		if len(collected) != len(hunks) {
			t.Errorf("collected = %d, want %d (one mock finding per hunk)", len(collected), len(hunks))
		}
	})

	t.Run("selector error falls back with exactly one warning for the run", func(t *testing.T) {
		t.Parallel()
		decider := &fakeChoiceDecider{errs: []error{errors.New("connection refused")}}
		var warnBuf bytes.Buffer
		opts := mkSelectorOpts(decider, &warnBuf, hunks)
		fake := &fakeCriticLLM{responses: []string{"[{\"file\":\"a.go\",\"line\":1,\"severity\":\"warning\",\"category\":\"style\",\"message\":\"m\"}]"}}
		opts.Client = fake
		collected, _, _, _, _, err := runReviewPipeline(ctx, opts)
		if err != nil {
			t.Fatalf("runReviewPipeline: %v (selection must not fail the review)", err)
		}
		if len(collected) != len(hunks) {
			t.Errorf("collected = %d, want %d", len(collected), len(hunks))
		}
		fullRule := selectorRule().Content
		for i, system := range fake.systems {
			if !strings.Contains(system, fullRule) {
				t.Errorf("hunk %d system prompt must use default rule assembly (full rule content):\n%s", i, system)
			}
		}
		if got := strings.Count(warnBuf.String(), "rules selector unavailable"); got != 1 {
			t.Errorf("warnings = %d, want exactly 1 per run; output:\n%s", got, warnBuf.String())
		}
		if !strings.Contains(warnBuf.String(), "http://selector.example:8080") {
			t.Errorf("warning must name the server URL; got %q", warnBuf.String())
		}
	})

	t.Run("disabled leaves prompts identical to default assembly", func(t *testing.T) {
		t.Parallel()
		decider := &fakeChoiceDecider{}
		opts := mkSelectorOpts(decider, io.Discard, hunks)
		fake := &fakeCriticLLM{responses: []string{"[{\"file\":\"a.go\",\"line\":1,\"severity\":\"warning\",\"category\":\"style\",\"message\":\"m\"}]"}}
		opts.Client = fake
		opts.RulesSelectorClient = nil
		if _, _, _, _, _, err := runReviewPipeline(ctx, opts); err != nil {
			t.Fatalf("runReviewPipeline: %v", err)
		}
		if decider.calls != 0 {
			t.Errorf("decider calls = %d, want 0 when selector disabled", decider.calls)
		}
		wantSystem := "BASE\n\n## Project review criteria\n" + selectorRule().Content
		for i, system := range fake.systems {
			if system != wantSystem {
				t.Errorf("hunk %d disabled-path system prompt:\n got: %q\nwant: %q", i, system, wantSystem)
			}
		}
	})
}

// TestStart_dryRun_rulesSelector_neverContactsServer asserts that a dry-run
// start with the rules selector configured makes zero selector requests.
func TestStart_dryRun_rulesSelector_neverContactsServer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	decider := &fakeChoiceDecider{}
	opts := StartOptions{
		RepoRoot:             repo,
		StateDir:             stateDir,
		WorktreeRoot:         "",
		Ref:                  "HEAD~1",
		DryRun:               true,
		RulesSelectorClient:  decider,
		RulesSelectorBaseURL: "http://selector.example:8080",
		RulesSelectorModel:   "sel-model",
	}
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if decider.calls != 0 {
		t.Errorf("decider calls = %d, want 0 (dry-run never contacts the selector server)", decider.calls)
	}
}

// annotateResponse builds a System One choice response for the annotator's
// per-finding question (key "q0" in single-finding batches) choosing optionID
// with the given probability.
func annotateResponse(optionID string, prob float64) *systemone.ChoiceResponse {
	return &systemone.ChoiceResponse{
		Model: "ann-model",
		Answers: map[string]systemone.ChoiceAnswer{
			"q0": {Choice: optionID, Probabilities: map[string]float64{optionID: prob}},
		},
	}
}

// mockFindingJSON is the one-finding LLM response shared by the annotation
// pipeline tests (same shape as the selector pipeline tests use).
const mockFindingJSON = `[{"file":"a.go","line":1,"severity":"warning","category":"style","message":"m"}]`

// mkAnnotateOpts builds reviewPipelineOpts with the rules annotator enabled and
// the selector disabled, mirroring mkSelectorOpts.
func mkAnnotateOpts(decider *fakeChoiceDecider, warnOut io.Writer, hunks []diff.Hunk) reviewPipelineOpts {
	return reviewPipelineOpts{
		Client:               &fakeCriticLLM{responses: []string{mockFindingJSON}},
		Model:                "main",
		Hunks:                hunks,
		GenOpts:              &ollama.GenerateOptions{},
		SystemBase:           "BASE",
		RulesByFile:          map[string][]rules.CursorRule{"a.go": {selectorRule()}},
		RulesAnnotateClient:  decider,
		RulesAnnotateBaseURL: "http://annotate.example:8080",
		RulesAnnotateModel:   "ann-model",
		RulesAnnotateWarnOut: warnOut,
	}
}

// TestAnnotateRuleConflictsForBatch covers the per-hunk annotation helper:
// disabled, empty batch, no matching rules, success, fail-open error handling,
// and the over-budget skip.
func TestAnnotateRuleConflictsForBatch(t *testing.T) {
	t.Parallel()
	hunk := diff.Hunk{FilePath: "a.go", RawContent: "package main\n"}
	mkBatch := func() []findings.Finding {
		return []findings.Finding{
			{ID: "f1", File: "a.go", Line: 1, Severity: "warning", Category: "style", Message: "m1"},
			{ID: "f2", File: "a.go", Line: 2, Severity: "info", Category: "bug", Message: "m2"},
		}
	}

	t.Run("disabled returns batch unchanged with no requests", func(t *testing.T) {
		t.Parallel()
		decider := &fakeChoiceDecider{}
		batch := mkBatch()
		got := annotateRuleConflictsForBatch(context.Background(), reviewPipelineOpts{RulesAnnotateClient: nil}, hunk, []rules.CursorRule{selectorRule()}, batch, nil)
		if len(got) != len(batch) {
			t.Errorf("len = %d, want %d", len(got), len(batch))
		}
		for i, f := range got {
			if f.RuleConflict != nil {
				t.Errorf("finding %d annotated with annotator disabled: %+v", i, *f.RuleConflict)
			}
		}
		if decider.calls != 0 {
			t.Errorf("decider calls = %d, want 0", decider.calls)
		}
	})

	t.Run("empty batch makes no requests", func(t *testing.T) {
		t.Parallel()
		decider := &fakeChoiceDecider{}
		got := annotateRuleConflictsForBatch(context.Background(), mkAnnotateOpts(decider, io.Discard, nil), hunk, []rules.CursorRule{selectorRule()}, nil, nil)
		if got != nil {
			t.Errorf("got = %v, want nil for empty batch", got)
		}
		if decider.calls != 0 {
			t.Errorf("decider calls = %d, want 0", decider.calls)
		}
	})

	t.Run("no matching rules makes no requests", func(t *testing.T) {
		t.Parallel()
		decider := &fakeChoiceDecider{}
		nonMatching := rules.CursorRule{Source: "ts.mdc", Globs: []string{"*.ts"}, Content: "## TS\n\nPrefer const."}
		got := annotateRuleConflictsForBatch(context.Background(), mkAnnotateOpts(decider, io.Discard, nil), hunk, []rules.CursorRule{nonMatching}, mkBatch(), nil)
		if len(got) != 2 {
			t.Errorf("len = %d, want 2", len(got))
		}
		if decider.calls != 0 {
			t.Errorf("decider calls = %d, want 0 (no server request without candidate sections)", decider.calls)
		}
	})

	t.Run("success annotates without changing count or order", func(t *testing.T) {
		t.Parallel()
		// One request carries both findings' questions; answers are keyed by batch index.
		resp := &systemone.ChoiceResponse{
			Model: "ann-model",
			Answers: map[string]systemone.ChoiceAnswer{
				"q0": {Choice: "s2", Probabilities: map[string]float64{"s2": 0.9}},     // f1 -> Naming section
				"q1": {Choice: "none", Probabilities: map[string]float64{"none": 0.9}}, // f2 -> no conflict
			},
		}
		decider := &fakeChoiceDecider{responses: []*systemone.ChoiceResponse{resp}}
		batch := mkBatch()
		got := annotateRuleConflictsForBatch(context.Background(), mkAnnotateOpts(decider, io.Discard, nil), hunk, []rules.CursorRule{selectorRule()}, batch, nil)
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2 (annotation must not change finding count)", len(got))
		}
		if got[0].ID != "f1" || got[1].ID != "f2" {
			t.Errorf("order changed: %q %q, want f1 f2", got[0].ID, got[1].ID)
		}
		if got[0].RuleConflict == nil {
			t.Fatalf("f1 not annotated; want RuleConflict set")
		}
		if got[0].RuleConflict.Source != "std.mdc" || got[0].RuleConflict.Section != "Naming" || got[0].RuleConflict.Probability != 0.9 {
			t.Errorf("f1 RuleConflict = %+v, want std.mdc/Naming/0.9", *got[0].RuleConflict)
		}
		if got[1].RuleConflict != nil {
			t.Errorf("f2 annotated with none-choice: %+v", *got[1].RuleConflict)
		}
		// One request for the two findings (under the 32-per-request cap).
		if decider.calls != 1 {
			t.Errorf("decider calls = %d, want 1", decider.calls)
		}
		req := decider.requests[0]
		if req.Model != "ann-model" {
			t.Errorf("request model = %q, want ann-model (RulesAnnotateModel passed through)", req.Model)
		}
		if !strings.Contains(req.State, "package main") {
			t.Errorf("request state must carry the hunk content; got %q", req.State)
		}
		if len(req.Questions) != 2 {
			t.Errorf("request questions = %d, want 2 (one per finding)", len(req.Questions))
		}
	})

	t.Run("error leaves findings unannotated with one warning", func(t *testing.T) {
		t.Parallel()
		decider := &fakeChoiceDecider{errs: []error{errors.New("connection refused")}}
		var warnBuf bytes.Buffer
		batch := mkBatch()
		got := annotateRuleConflictsForBatch(context.Background(), mkAnnotateOpts(decider, &warnBuf, nil), hunk, []rules.CursorRule{selectorRule()}, batch, &rulesAnnotateState{warnOut: &warnBuf, baseURL: "http://annotate.example:8080"})
		if len(got) != 2 {
			t.Errorf("len = %d, want 2 (annotation failure must not drop findings)", len(got))
		}
		for i, f := range got {
			if f.RuleConflict != nil {
				t.Errorf("finding %d annotated after error: %+v", i, *f.RuleConflict)
			}
		}
		if !strings.Contains(warnBuf.String(), "http://annotate.example:8080") {
			t.Errorf("warning must name the server URL; got %q", warnBuf.String())
		}
	})

	t.Run("over-budget batch is skipped with no request", func(t *testing.T) {
		t.Parallel()
		decider := &fakeChoiceDecider{}
		big := make([]findings.Finding, 65) // annotateMaxQuestions (64) + 1
		for i := range big {
			big[i] = findings.Finding{ID: fmt.Sprintf("f%d", i), File: "a.go", Line: i + 1, Severity: "info", Category: "style", Message: "m"}
		}
		got := annotateRuleConflictsForBatch(context.Background(), mkAnnotateOpts(decider, io.Discard, nil), hunk, []rules.CursorRule{selectorRule()}, big, nil)
		if len(got) != 65 {
			t.Errorf("len = %d, want 65 (oversized batch must be skipped, not degraded)", len(got))
		}
		if decider.calls != 0 {
			t.Errorf("decider calls = %d, want 0 (batches over the question budget are skipped)", decider.calls)
		}
	})
}

// TestRunReviewPipeline_rulesAnnotate asserts the pipeline's enabled-path
// annotation behaviour: findings annotated after the critic with count/order
// unchanged, exactly one fail-open warning when annotation fails, and zero
// requests when disabled.
func TestRunReviewPipeline_rulesAnnotate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hunks := []diff.Hunk{
		{FilePath: "a.go", RawContent: "hunk one\n"},
		{FilePath: "a.go", RawContent: "hunk two\n"},
	}

	t.Run("enabled annotates surviving findings without changing count or order", func(t *testing.T) {
		t.Parallel()
		decider := &fakeChoiceDecider{responses: []*systemone.ChoiceResponse{annotateResponse("s0", 0.8)}}
		opts := mkAnnotateOpts(decider, io.Discard, hunks)
		opts.Client = &fakeCriticLLM{responses: []string{mockFindingJSON}}
		collected, _, _, _, _, err := runReviewPipeline(ctx, opts)
		if err != nil {
			t.Fatalf("runReviewPipeline: %v", err)
		}
		if len(collected) != len(hunks) {
			t.Fatalf("collected = %d, want %d (one mock finding per hunk)", len(collected), len(hunks))
		}
		for i, f := range collected {
			if f.RuleConflict == nil {
				t.Fatalf("finding %d not annotated; want RuleConflict set", i)
			}
			if f.RuleConflict.Source != "std.mdc" || f.RuleConflict.Section != "Error handling" || f.RuleConflict.Probability != 0.8 {
				t.Errorf("finding %d RuleConflict = %+v, want std.mdc/Error handling/0.8", i, *f.RuleConflict)
			}
		}
		if decider.calls != len(hunks) {
			t.Errorf("decider calls = %d, want %d (one annotation request per hunk)", decider.calls, len(hunks))
		}
		// Annotation runs after the critic: the request state carries the hunk text.
		if !strings.Contains(decider.requests[0].State, "hunk one") {
			t.Errorf("annotation request state must carry the hunk content; got %q", decider.requests[0].State)
		}
	})

	t.Run("error keeps findings unannotated with exactly one warning for the run", func(t *testing.T) {
		t.Parallel()
		decider := &fakeChoiceDecider{errs: []error{errors.New("connection refused")}}
		var warnBuf bytes.Buffer
		opts := mkAnnotateOpts(decider, &warnBuf, hunks)
		opts.Client = &fakeCriticLLM{responses: []string{mockFindingJSON}}
		collected, _, _, _, _, err := runReviewPipeline(ctx, opts)
		if err != nil {
			t.Fatalf("runReviewPipeline: %v (annotation failure must not fail the review)", err)
		}
		if len(collected) != len(hunks) {
			t.Errorf("collected = %d, want %d", len(collected), len(hunks))
		}
		for i, f := range collected {
			if f.RuleConflict != nil {
				t.Errorf("finding %d annotated after error: %+v", i, *f.RuleConflict)
			}
		}
		if got := strings.Count(warnBuf.String(), "rules annotation unavailable"); got != 1 {
			t.Errorf("warnings = %d, want exactly 1 per run; output:\n%s", got, warnBuf.String())
		}
		if !strings.Contains(warnBuf.String(), "http://annotate.example:8080") {
			t.Errorf("warning must name the server URL; got %q", warnBuf.String())
		}
		if strings.Contains(warnBuf.String(), "supersecret") {
			t.Errorf("warning must never contain the API key; got %q", warnBuf.String())
		}
	})

	t.Run("disabled leaves findings unchanged with no requests", func(t *testing.T) {
		t.Parallel()
		decider := &fakeChoiceDecider{}
		opts := mkAnnotateOpts(decider, io.Discard, hunks)
		opts.Client = &fakeCriticLLM{responses: []string{mockFindingJSON}}
		opts.RulesAnnotateClient = nil
		collected, _, _, _, _, err := runReviewPipeline(ctx, opts)
		if err != nil {
			t.Fatalf("runReviewPipeline: %v", err)
		}
		if decider.calls != 0 {
			t.Errorf("decider calls = %d, want 0 when annotator disabled", decider.calls)
		}
		for i, f := range collected {
			if f.RuleConflict != nil {
				t.Errorf("finding %d annotated with annotator disabled: %+v", i, *f.RuleConflict)
			}
		}
	})
}

// TestStart_dryRun_rulesAnnotate_neverContactsServer asserts that a dry-run
// start with the rules annotator configured makes zero annotation requests.
func TestStart_dryRun_rulesAnnotate_neverContactsServer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	decider := &fakeChoiceDecider{}
	opts := StartOptions{
		RepoRoot:             repo,
		StateDir:             stateDir,
		WorktreeRoot:         "",
		Ref:                  "HEAD~1",
		DryRun:               true,
		RulesAnnotateClient:  decider,
		RulesAnnotateBaseURL: "http://annotate.example:8080",
		RulesAnnotateModel:   "ann-model",
	}
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if decider.calls != 0 {
		t.Errorf("decider calls = %d, want 0 (dry-run never contacts the annotation server)", decider.calls)
	}
}

func TestApplyCriticSystemOne(t *testing.T) {
	t.Parallel()
	hunk := diff.Hunk{FilePath: "a.go", RawContent: "package main\n"}
	p := &preparedPrompt{Hunk: hunk}
	mkBatch := func() []findings.Finding {
		return []findings.Finding{
			{ID: "f1", File: "a.go", Line: 1, Severity: "warning", Category: "bug", Message: "m1"},
			{ID: "f2", File: "a.go", Line: 2, Severity: "warning", Category: "style", Message: "m2"},
		}
	}
	mkOpts := func(decider *fakeSystemOneDecider) reviewPipelineOpts {
		return reviewPipelineOpts{
			CriticEnabled:    true,
			CriticBackend:    "systemone",
			SystemOneClient:  decider,
			SystemOneBaseURL: "http://localhost:11434",
			SystemOneModel:   "openjev-0.1",
			SystemOneMinKeep: 0.5,
			SystemOneSamples: 3,
		}
	}

	t.Run("disabled returns batch unchanged with no calls", func(t *testing.T) {
		decider := &fakeSystemOneDecider{responses: []*systemone.Response{noulResponse(map[string]float64{"f0": 0.9, "f1": 0.9})}}
		opts := mkOpts(decider)
		opts.CriticEnabled = false
		in := mkBatch()
		out, err := applyCritic(context.Background(), opts, p, in)
		if err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		if len(out) != 2 || out[0].ID != "f1" || out[1].ID != "f2" {
			t.Errorf("out = %+v, want input unchanged", out)
		}
		if decider.calls != 0 {
			t.Errorf("decider calls = %d, want 0", decider.calls)
		}
	})

	t.Run("empty batch returns unchanged with no calls", func(t *testing.T) {
		decider := &fakeSystemOneDecider{responses: []*systemone.Response{noulResponse(nil)}}
		out, err := applyCritic(context.Background(), mkOpts(decider), p, nil)
		if err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		if out != nil {
			t.Errorf("out = %+v, want nil", out)
		}
		if decider.calls != 0 {
			t.Errorf("decider calls = %d, want 0", decider.calls)
		}
	})

	t.Run("keeps findings at or above minKeep and drops below", func(t *testing.T) {
		decider := &fakeSystemOneDecider{responses: []*systemone.Response{noulResponse(map[string]float64{"f0": 0.9997, "f1": 0.2})}}
		out, err := applyCritic(context.Background(), mkOpts(decider), p, mkBatch())
		if err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		if len(out) != 1 || out[0].ID != "f1" {
			t.Fatalf("kept = %+v, want only f1 (0.9997 >= 0.5; 0.2 dropped)", out)
		}
		if decider.calls != 1 {
			t.Errorf("decider calls = %d, want 1 (one request per hunk)", decider.calls)
		}
		req := decider.requests[0]
		if req.Model != "openjev-0.1" {
			t.Errorf("request model = %q, want openjev-0.1 (SystemOneModel passed through)", req.Model)
		}
		if req.Samples != 3 {
			t.Errorf("request samples = %d, want 3 (SystemOneSamples passed through)", req.Samples)
		}
		if !strings.Contains(req.State, "package main") {
			t.Errorf("request state = %q, want hunk content (p.Hunk.RawContent) in state", req.State)
		}
		if len(req.Questions) != 2 {
			t.Errorf("request questions = %d, want 2 (one per finding)", len(req.Questions))
		}
	})

	t.Run("trace summary and per-finding score lines", func(t *testing.T) {
		var traceBuf bytes.Buffer
		decider := &fakeSystemOneDecider{responses: []*systemone.Response{noulResponse(map[string]float64{"f0": 0.9997, "f1": 0.2})}}
		opts := mkOpts(decider)
		opts.TraceOut = trace.New(&traceBuf)
		out, err := applyCritic(context.Background(), opts, p, mkBatch())
		if err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		if len(out) != 1 {
			t.Fatalf("kept %d findings, want 1", len(out))
		}
		got := traceBuf.String()
		for _, want := range []string{
			"Critic (systemone): 2 -> 1",
			"score=0.9997 id=f1",
			"score=0.2000 id=f2",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("trace = %q, want %q", got, want)
			}
		}
		if strings.Contains(got, "questions") || strings.Contains(got, `"state"`) {
			t.Errorf("trace = %q, want no request bodies logged", got)
		}
	})

	t.Run("state carries the prepared user prompt with expanded context and RAG definitions", func(t *testing.T) {
		prep := diff.Hunk{
			FilePath:   "a.go",
			RawContent: "package main\n",
			Context:    "## Enclosing function context\n\n```go\nfunc expandedEnclosing() { UNIQUE_EXPANDED_MARKER }\n```\n\n## Diff hunk\n\npackage main\n",
		}
		user := preparedUserPromptFor(prep)
		if !strings.Contains(user, "UNIQUE_EXPANDED_MARKER") || !strings.Contains(user, "func helperFunc() {}") {
			t.Fatalf("prepared prompt should carry the expanded context and the RAG definition, got %q", user)
		}
		sp := &preparedPrompt{User: user, Hunk: prep}
		decider := &fakeSystemOneDecider{responses: []*systemone.Response{noulResponse(map[string]float64{"f0": 0.9, "f1": 0.9})}}
		out, err := applyCritic(context.Background(), mkOpts(decider), sp, mkBatch())
		if err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		if len(out) != 2 {
			t.Fatalf("kept %d findings, want 2", len(out))
		}
		req := decider.requests[0]
		for _, want := range []string{"Code under review:", "## Enclosing function context", "UNIQUE_EXPANDED_MARKER", "## Symbol definitions (for context)", "func helperFunc() {}"} {
			if !strings.Contains(req.State, want) {
				t.Errorf("request state should contain %q from the prepared user prompt, got %q", want, req.State)
			}
		}
	})

	t.Run("truncates prepared prompt over the critic context cap and traces input length", func(t *testing.T) {
		var traceBuf bytes.Buffer
		big := strings.Repeat("z", criticContextCapForTest+500)
		sp := &preparedPrompt{User: big, Hunk: diff.Hunk{FilePath: "a.go", RawContent: "package main\n"}}
		decider := &fakeSystemOneDecider{responses: []*systemone.Response{noulResponse(map[string]float64{"f0": 0.9, "f1": 0.9})}}
		opts := mkOpts(decider)
		opts.TraceOut = trace.New(&traceBuf)
		if _, err := applyCritic(context.Background(), opts, sp, mkBatch()); err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		state := decider.requests[0].State
		if !strings.HasSuffix(state, "\n[truncated]") {
			t.Errorf("request state should end with the [truncated] marker, got suffix %q", state[len(state)-30:])
		}
		if !strings.Contains(state, strings.Repeat("z", criticContextCapForTest)) {
			t.Errorf("request state should keep the first %d chars before truncating", criticContextCapForTest)
		}
		if strings.Contains(state, strings.Repeat("z", criticContextCapForTest+1)) {
			t.Errorf("request state should not carry more than %d chars of the prepared prompt", criticContextCapForTest)
		}
		if got := traceBuf.String(); !strings.Contains(got, fmt.Sprintf("Critic input: %d chars", len(big))) {
			t.Errorf("trace = %q, want critic input length line", got)
		}
	})

	t.Run("decider error aborts with user message naming base URL", func(t *testing.T) {
		var traceBuf bytes.Buffer
		decideErr := errors.New("connection refused")
		decider := &fakeSystemOneDecider{errs: []error{decideErr}}
		opts := mkOpts(decider)
		opts.TraceOut = trace.New(&traceBuf)
		out, err := applyCritic(context.Background(), opts, p, mkBatch())
		if out != nil {
			t.Errorf("out = %+v, want nil", out)
		}
		if err == nil {
			t.Fatal("applyCritic err = nil, want Critic failed error")
		}
		want := "Critic failed for a.go (System One at http://localhost:11434)."
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want it to contain %q", err, want)
		}
		if !errors.Is(err, decideErr) {
			t.Errorf("err %v does not wrap %v", err, decideErr)
		}
		if strings.Contains(strings.ToLower(err.Error()), "sk-") {
			t.Errorf("err = %v, want no API key material", err)
		}
		if got := traceBuf.String(); !strings.Contains(got, "Critic request failed:") || !strings.Contains(got, "connection refused") {
			t.Errorf("trace = %q, want Critic request failed with connection refused", got)
		}
		if decider.calls != 1 {
			t.Errorf("decider calls = %d, want 1 (stop on first error)", decider.calls)
		}
	})

	t.Run("nil client aborts instead of silently disabling verification", func(t *testing.T) {
		opts := mkOpts(nil)
		opts.SystemOneClient = nil
		out, err := applyCritic(context.Background(), opts, p, mkBatch())
		if out != nil {
			t.Errorf("out = %+v, want nil", out)
		}
		if err == nil {
			t.Fatal("applyCritic err = nil, want Critic failed error")
		}
		want := "Critic failed for a.go (System One at http://localhost:11434)."
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want it to contain %q", err, want)
		}
	})

	t.Run("empty CriticModel does not block systemone backend", func(t *testing.T) {
		decider := &fakeSystemOneDecider{responses: []*systemone.Response{noulResponse(map[string]float64{"f0": 0.9})}}
		opts := mkOpts(decider)
		opts.CriticModel = "" // LLM guard must not apply to the System One branch
		out, err := applyCritic(context.Background(), opts, p, mkBatch()[:1])
		if err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		if len(out) != 1 {
			t.Fatalf("kept %d findings, want 1", len(out))
		}
		if decider.calls != 1 {
			t.Errorf("decider calls = %d, want 1", decider.calls)
		}
	})

	t.Run("llm backend still uses VerifyFinding", func(t *testing.T) {
		decider := &fakeSystemOneDecider{responses: []*systemone.Response{noulResponse(map[string]float64{"f0": 0.9})}}
		llmFake := &fakeCriticLLM{responses: []string{"{\"verdict\":\"yes\",\"reason\":\"ok\"}", "{\"verdict\":\"no\",\"reason\":\"bad\"}"}}
		opts := reviewPipelineOpts{
			Client: llmFake, Model: "main", CriticEnabled: true,
			CriticBackend: "llm", CriticModel: "critic",
			SystemOneClient:  decider,
			SystemOneBaseURL: "http://localhost:11434",
			SystemOneModel:   "openjev-0.1",
			SystemOneMinKeep: 0.5,
		}
		out, err := applyCritic(context.Background(), opts, p, mkBatch())
		if err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		if len(out) != 1 || out[0].ID != "f1" {
			t.Fatalf("kept = %+v, want only f1 (LLM verdicts yes/no)", out)
		}
		if llmFake.calls != 2 {
			t.Errorf("llm calls = %d, want 2 (one per finding)", llmFake.calls)
		}
		if decider.calls != 0 {
			t.Errorf("decider calls = %d, want 0 (llm backend must not touch System One)", decider.calls)
		}
	})

	t.Run("default backend empty uses llm critic", func(t *testing.T) {
		decider := &fakeSystemOneDecider{responses: []*systemone.Response{noulResponse(map[string]float64{"f0": 0.9})}}
		llmFake := &fakeCriticLLM{responses: []string{"{\"verdict\":\"yes\",\"reason\":\"ok\"}"}}
		opts := reviewPipelineOpts{
			Client: llmFake, Model: "main", CriticEnabled: true, CriticModel: "critic",
			SystemOneClient: decider,
		}
		out, err := applyCritic(context.Background(), opts, p, mkBatch()[:1])
		if err != nil {
			t.Fatalf("applyCritic: %v", err)
		}
		if len(out) != 1 {
			t.Fatalf("kept %d findings, want 1", len(out))
		}
		if llmFake.calls != 1 || decider.calls != 0 {
			t.Errorf("llm calls = %d, decider calls = %d; want 1 and 0 (default backend unchanged)", llmFake.calls, decider.calls)
		}
	})
}

func TestRunReviewPipeline_systemOneCritic(t *testing.T) {
	t.Parallel()
	validResp := `[{"file":"a.go","line":1,"severity":"warning","category":"bug","message":"pipeline-sysone"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(w)
		_ = enc.Encode(map[string]interface{}{"response": validResp, "done": true})
	}))
	defer srv.Close()

	client, err := llm.NewClient("ollama", srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	stateDir := t.TempDir()
	systemBase, err := prompt.SystemPrompt(stateDir, "")
	if err != nil {
		t.Fatalf("SystemPrompt: %v", err)
	}
	decider := &fakeSystemOneDecider{responses: []*systemone.Response{noulResponse(map[string]float64{"f0": 0.1})}}
	var traceBuf bytes.Buffer
	hunk := diff.Hunk{FilePath: "a.go", RawContent: "@@ -1 +1 @@\npackage main\n", Context: "package main\n"}
	collected, _, _, _, _, err := runReviewPipeline(context.Background(), reviewPipelineOpts{
		Client:           client,
		Model:            "m",
		Hunks:            []diff.Hunk{hunk},
		GenOpts:          &ollama.GenerateOptions{},
		SystemBase:       systemBase,
		CriticEnabled:    true,
		CriticBackend:    "systemone",
		SystemOneClient:  decider,
		SystemOneBaseURL: srv.URL,
		SystemOneModel:   "openjev-0.1",
		SystemOneMinKeep: 0.5,
		TraceOut:         trace.New(&traceBuf),
	})
	if err != nil {
		t.Fatalf("runReviewPipeline: %v", err)
	}
	if len(collected) != 0 {
		t.Errorf("collected = %+v, want empty (score 0.1 < 0.5)", collected)
	}
	if decider.calls != 1 {
		t.Errorf("decider calls = %d, want 1", decider.calls)
	}
	if len(decider.requests) == 1 && !strings.Contains(decider.requests[0].State, "package main") {
		t.Errorf("request state = %q, want hunk content", decider.requests[0].State)
	}
	if got := traceBuf.String(); !strings.Contains(got, "Critic (systemone): 1 -> 0") {
		t.Errorf("trace = %q, want Critic (systemone): 1 -> 0", got)
	}
}

func TestStart_dryRun_neverCallsSystemOne(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	decider := &fakeSystemOneDecider{responses: []*systemone.Response{noulResponse(map[string]float64{"f0": 0.9})}}
	opts := StartOptions{
		RepoRoot:         repo,
		StateDir:         stateDir,
		WorktreeRoot:     "",
		Ref:              "HEAD~1",
		DryRun:           true,
		Model:            "",
		Provider:         "ollama",
		LLMBaseURL:       "",
		CriticEnabled:    true,
		CriticBackend:    "systemone",
		SystemOneClient:  decider,
		SystemOneBaseURL: "http://localhost:11434",
		SystemOneModel:   "openjev-0.1",
		SystemOneMinKeep: 0.5,
	}
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if decider.calls != 0 {
		t.Errorf("decider calls = %d, want 0 (dry-run must never reach the System One critic)", decider.calls)
	}
}

func TestRun_dryRun_neverCallsSystemOne(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	startOpts := StartOptions{
		RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: "HEAD~1", DryRun: true,
		Model: "", Provider: "ollama", LLMBaseURL: "",
	}
	if _, err := Start(ctx, startOpts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	decider := &fakeSystemOneDecider{responses: []*systemone.Response{noulResponse(map[string]float64{"f0": 0.9})}}
	runOpts := RunOptions{
		RepoRoot: repo, StateDir: stateDir, DryRun: true,
		CriticEnabled:    true,
		CriticBackend:    "systemone",
		SystemOneClient:  decider,
		SystemOneBaseURL: "http://localhost:11434",
		SystemOneModel:   "openjev-0.1",
		SystemOneMinKeep: 0.5,
	}
	if _, err := Run(ctx, runOpts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if decider.calls != 0 {
		t.Errorf("decider calls = %d, want 0 (dry-run must never reach the System One critic)", decider.calls)
	}
}
