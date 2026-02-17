package stats

import (
	"testing"

	"stet/cli/internal/git"
)

func TestGitAIInUse_refDoesNotExist(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	ok, err := GitAIInUse(repo)
	if err != nil {
		t.Fatalf("GitAIInUse: %v", err)
	}
	if ok {
		t.Error("GitAIInUse with no git-ai: want false, got true")
	}
}

func TestGitAIInUse_refExists(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	headSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	if err := git.AddNote(repo, git.NotesRefAI, headSHA, "minimal note"); err != nil {
		t.Fatalf("AddNote to create refs/notes/ai: %v", err)
	}
	ok, err := GitAIInUse(repo)
	if err != nil {
		t.Fatalf("GitAIInUse: %v", err)
	}
	if !ok {
		t.Error("GitAIInUse after adding note: want true, got false")
	}
}

func TestGitAIMetrics_validNote(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	headSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	head1SHA := runOut(t, repo, "git", "rev-parse", "HEAD~1")
	note1 := `src/main.go
  abcd1234abcd1234 1-10
---
{"schema_version":"authorship/3.0.0","base_commit_sha":"` + head1SHA + `","prompts":{"abcd1234abcd1234":{"agent_id":{"tool":"cursor","id":"x","model":"y"},"messages":[],"total_additions":10,"total_deletions":0,"accepted_lines":10,"overriden_lines":0}}}`
	note2 := `src/lib.go
  efgh5678efgh5678 1-5,7
---
{"schema_version":"authorship/3.0.0","base_commit_sha":"` + headSHA + `","prompts":{"efgh5678efgh5678":{"agent_id":{"tool":"cursor","id":"y","model":"z"},"messages":[],"total_additions":6,"total_deletions":0,"accepted_lines":6,"overriden_lines":0}}}`
	if err := git.AddNote(repo, git.NotesRefAI, head1SHA, note1); err != nil {
		t.Fatalf("AddNote HEAD~1: %v", err)
	}
	if err := git.AddNote(repo, git.NotesRefAI, headSHA, note2); err != nil {
		t.Fatalf("AddNote HEAD: %v", err)
	}
	res, err := GitAIMetrics(repo, "HEAD~2", "HEAD")
	if err != nil {
		t.Fatalf("GitAIMetrics: %v", err)
	}
	if res.CommitsWithAINote != 2 {
		t.Errorf("CommitsWithAINote: got %d, want 2", res.CommitsWithAINote)
	}
	if res.TotalAIAuthoredLines != 16 {
		t.Errorf("TotalAIAuthoredLines: got %d, want 16 (10+6)", res.TotalAIAuthoredLines)
	}
}

func TestGitAIMetrics_emptyRange(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	if err := git.AddNote(repo, git.NotesRefAI, runOut(t, repo, "git", "rev-parse", "HEAD"), "x"); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	res, err := GitAIMetrics(repo, "HEAD", "HEAD")
	if err != nil {
		t.Fatalf("GitAIMetrics: %v", err)
	}
	if res.CommitsWithAINote != 0 || res.TotalAIAuthoredLines != 0 {
		t.Errorf("empty range: got %d commits, %d lines, want 0, 0", res.CommitsWithAINote, res.TotalAIAuthoredLines)
	}
}

func TestGitAIMetrics_malformedNoteSkipped(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	headSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	if err := git.AddNote(repo, git.NotesRefAI, headSHA, "not valid git-ai format"); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	res, err := GitAIMetrics(repo, "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("GitAIMetrics: %v", err)
	}
	if res.CommitsWithAINote != 0 || res.TotalAIAuthoredLines != 0 {
		t.Errorf("malformed note should be skipped: got %d commits, %d lines", res.CommitsWithAINote, res.TotalAIAuthoredLines)
	}
}

func TestParseGitAINote_multiplePrompts(t *testing.T) {
	body := `src/a.go
  aaaa1111aaaa1111 1-5
src/b.go
  bbbb2222bbbb2222 1-3
---
{"schema_version":"authorship/3.0.0","base_commit_sha":"abc","prompts":{"aaaa1111aaaa1111":{"accepted_lines":5,"total_additions":5,"total_deletions":0,"overriden_lines":0},"bbbb2222bbbb2222":{"accepted_lines":3,"total_additions":3,"total_deletions":0,"overriden_lines":0}}}`
	lines, err := parseGitAINote(body)
	if err != nil {
		t.Fatalf("parseGitAINote: %v", err)
	}
	if lines != 8 {
		t.Errorf("parseGitAINote: got %d lines, want 8 (5+3)", lines)
	}
}

func TestParseGitAINote_noDivider(t *testing.T) {
	lines, err := parseGitAINote("just some text\nno metadata")
	if err == nil {
		t.Error("parseGitAINote (no divider): want error")
	}
	if lines != 0 {
		t.Errorf("parseGitAINote (no divider): got %d lines, want 0", lines)
	}
}
