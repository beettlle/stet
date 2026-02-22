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

	"stet/cli/internal/diff"
	"stet/cli/internal/findings"
	"stet/cli/internal/git"
	"stet/cli/internal/history"
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

func ptrBool(b bool) *bool { return &b }

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

func TestStart_refEqualsHEAD_withPersistStrictness_storesInSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	strictness := "strict"
	opts := StartOptions{
		RepoRoot: repo, StateDir: stateDir, WorktreeRoot: "", Ref: "HEAD", DryRun: true,
		Model: "", OllamaBaseURL: "",
		PersistStrictness: &strictness,
	}
	if err := Start(ctx, opts); err != nil {
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
	err := Start(ctx, opts)
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
		RepoRoot:                   repo,
		StateDir:                   stateDir,
		WorktreeRoot:               "",
		Ref:                        "HEAD~1",
		DryRun:                     true,
		Model:                      "",
		OllamaBaseURL:              "",
		PersistStrictness:          &strictness,
		PersistRAGSymbolMaxDefinitions: &ragDefs,
		PersistRAGSymbolMaxTokens:      &ragTokens,
	}
	if err := Start(ctx, opts); err != nil {
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
		OllamaBaseURL:       "",
		PersistContextLimit: &contextLimit,
		PersistNumCtx:       &numCtx,
	}
	if err := Start(ctx, opts); err != nil {
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
	if err := Start(ctx, startOpts); err != nil {
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
	if err := Start(ctx, startOpts); err != nil {
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

func TestStart_stream_emitsNDJSON(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	var buf bytes.Buffer
	opts := StartOptions{
		RepoRoot:      repo,
		StateDir:      stateDir,
		WorktreeRoot:  "",
		Ref:           "HEAD~1",
		DryRun:        true,
		Model:         "",
		OllamaBaseURL: "",
		StreamOut:     &buf,
	}
	if err := Start(ctx, opts); err != nil {
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

func TestRun_forceFullReview_noSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	opts := RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true, ForceFullReview: true}
	err := Run(ctx, opts)
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
	// Run once without ForceFullReview so LastReviewedAt = HEAD; second Run would see 0 ToReview.
	runOpts := RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true}
	if err := Run(ctx, runOpts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	s1, err := session.Load(stateDir)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	afterFirstRun := len(s1.Findings)
	// Rerun with ForceFullReview: all hunks go to ToReview, so review loop runs again and appends findings (merge mode).
	runOpts.ForceFullReview = true
	if err := Run(ctx, runOpts); err != nil {
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
	runOpts := RunOptions{RepoRoot: repo, StateDir: stateDir, DryRun: true}
	if err := Run(ctx, runOpts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	runOpts.ForceFullReview = true
	runOpts.ReplaceFindings = true
	if err := Run(ctx, runOpts); err != nil {
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
			name:  "finding with Range overlapping hunk -> hunk excluded",
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
			name:     "multiple hunks one contains dismissed -> only that excluded",
			toReview: []diff.Hunk{hunkA, hunkB},
			s:        &session.Session{DismissedIDs: []string{"f1"}, Findings: []findings.Finding{{ID: "f1", File: "a.go", Line: 2}}},
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
	if err := Run(ctx, runOpts); err != nil {
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
	if err := Run(ctx, runOpts); err != nil {
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
	// Canned findings use StableFindingID(file, 1, 0, 0, dryRunMessage). Add an extra finding with
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
	if err := Run(ctx, runOpts); err != nil {
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
	if activeCount != len(s1.Findings)-1 {
		t.Errorf("active count = %d, want len(Findings)-1 = %d (one finding dismissed)", activeCount, len(s1.Findings)-1)
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
		RepoRoot:      repo,
		StateDir:      stateDir,
		WorktreeRoot:  "",
		Ref:           "HEAD~1",
		DryRun:        false,
		Model:         "m",
		OllamaBaseURL: srv.URL,
	}
	if err := Start(ctx, startOpts); err != nil {
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
		RepoRoot:      repo,
		StateDir:      stateDir,
		WorktreeRoot:  "",
		Ref:           "HEAD~1",
		DryRun:        false,
		Model:         "m",
		OllamaBaseURL: srv.URL,
	}
	if err := Start(ctx, startOpts); err != nil {
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
		RepoRoot:                   repo,
		StateDir:                   stateDir,
		WorktreeRoot:               "",
		Ref:                        "HEAD~1",
		DryRun:                     false,
		Model:                      "m",
		OllamaBaseURL:              srv.URL,
		MinConfidenceKeep:         0.6,
		MinConfidenceMaintainability: 0.7,
		ApplyFPKillList:            ptrBool(true),
	}
	if err := Start(ctx, opts); err != nil {
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
		RepoRoot:                   repo,
		StateDir:                   stateDir,
		WorktreeRoot:               "",
		Ref:                        "HEAD~1",
		DryRun:                     false,
		Model:                      "m",
		OllamaBaseURL:              srv.URL,
		MinConfidenceKeep:          0.8,
		MinConfidenceMaintainability: 0.9,
		ApplyFPKillList:            ptrBool(false),
	}
	if err := Start(ctx, opts); err != nil {
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
		RepoRoot:                    repo,
		StateDir:                    stateDir,
		WorktreeRoot:                "",
		Ref:                         "HEAD~1",
		DryRun:                      false,
		Model:                       "m",
		OllamaBaseURL:               srv.URL,
		MinConfidenceKeep:           0.8,
		MinConfidenceMaintainability: 0.9,
		Nitpicky:                    true,
	}
	if err := Start(ctx, opts); err != nil {
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
		RepoRoot:      repo,
		StateDir:      stateDir,
		WorktreeRoot:  "",
		Ref:           "HEAD~1",
		DryRun:        false,
		Model:         "m",
		OllamaBaseURL: srv.URL,
		ContextLimit:  100,
		WarnThreshold: 0.9,
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

// TestRun_pipeline_multipleHunks_ordersFindingsAndOneGeneratePerHunk asserts that
// the review pipeline sends one Generate per hunk and returns findings in hunk order.
func TestRun_pipeline_multipleHunks_ordersFindingsAndOneGeneratePerHunk(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var generateCalls int
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
}
