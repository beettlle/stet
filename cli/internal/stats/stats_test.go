package stats

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"stet/cli/internal/git"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@stet.local")
	run(t, dir, "git", "config", "user.name", "Test")
	writeFile(t, dir, "f1.txt", "a\n")
	run(t, dir, "git", "add", "f1.txt")
	run(t, dir, "git", "commit", "-m", "c1")
	writeFile(t, dir, "f2.txt", "b\n")
	run(t, dir, "git", "add", "f2.txt")
	run(t, dir, "git", "commit", "-m", "c2")
	writeFile(t, dir, "f3.txt", "c\n")
	run(t, dir, "git", "add", "f3.txt")
	run(t, dir, "git", "commit", "-m", "c3")
	return dir
}

func run(t *testing.T, dir, name string, args ...string) {
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

func TestVolume_twoNotesInRange(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	headSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	head1SHA := runOut(t, repo, "git", "rev-parse", "HEAD~1")
	note1 := `{"session_id":"s1","baseline_sha":"` + runOut(t, repo, "git", "rev-parse", "HEAD~2") + `","head_sha":"` + head1SHA + `","findings_count":0,"dismissals_count":0,"tool_version":"test","finished_at":"2025-01-01T00:00:00Z","hunks_reviewed":2,"lines_added":5,"lines_removed":1,"chars_added":20,"chars_deleted":3,"chars_reviewed":100}`
	note2 := `{"session_id":"s2","baseline_sha":"` + head1SHA + `","head_sha":"` + headSHA + `","findings_count":0,"dismissals_count":0,"tool_version":"test","finished_at":"2025-01-01T00:00:00Z","hunks_reviewed":1,"lines_added":2,"lines_removed":0,"chars_added":10,"chars_deleted":0,"chars_reviewed":50}`
	if err := git.AddNote(repo, git.NotesRefStet, head1SHA, note1); err != nil {
		t.Fatalf("AddNote HEAD~1: %v", err)
	}
	if err := git.AddNote(repo, git.NotesRefStet, headSHA, note2); err != nil {
		t.Fatalf("AddNote HEAD: %v", err)
	}
	res, err := Volume(repo, "HEAD~2", "HEAD")
	if err != nil {
		t.Fatalf("Volume: %v", err)
	}
	if res.CommitsInRange != 2 {
		t.Errorf("CommitsInRange: got %d, want 2", res.CommitsInRange)
	}
	if res.SessionsCount != 2 || res.CommitsWithNote != 2 {
		t.Errorf("SessionsCount=%d CommitsWithNote=%d, want 2, 2", res.SessionsCount, res.CommitsWithNote)
	}
	if res.TotalHunksReviewed != 3 {
		t.Errorf("TotalHunksReviewed: got %d, want 3", res.TotalHunksReviewed)
	}
	if res.TotalLinesAdded != 7 || res.TotalLinesRemoved != 1 {
		t.Errorf("TotalLinesAdded=%d TotalLinesRemoved=%d, want 7, 1", res.TotalLinesAdded, res.TotalLinesRemoved)
	}
	if res.TotalCharsReviewed != 150 {
		t.Errorf("TotalCharsReviewed: got %d, want 150", res.TotalCharsReviewed)
	}
	if res.PercentCommitsWithNote != 100.0 {
		t.Errorf("PercentCommitsWithNote: got %.1f, want 100", res.PercentCommitsWithNote)
	}
}

func TestVolume_emptyRange(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	res, err := Volume(repo, "HEAD", "HEAD")
	if err != nil {
		t.Fatalf("Volume: %v", err)
	}
	if res.CommitsInRange != 0 || res.SessionsCount != 0 {
		t.Errorf("empty range: CommitsInRange=%d SessionsCount=%d, want 0, 0", res.CommitsInRange, res.SessionsCount)
	}
	if res.PercentCommitsWithNote != 0 {
		t.Errorf("PercentCommitsWithNote: got %f, want 0", res.PercentCommitsWithNote)
	}
}

func TestVolume_noNotesInRange(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	res, err := Volume(repo, "HEAD~2", "HEAD")
	if err != nil {
		t.Fatalf("Volume: %v", err)
	}
	if res.CommitsInRange != 2 {
		t.Errorf("CommitsInRange: got %d, want 2", res.CommitsInRange)
	}
	if res.SessionsCount != 0 || res.CommitsWithNote != 0 {
		t.Errorf("SessionsCount=%d CommitsWithNote=%d, want 0, 0", res.SessionsCount, res.CommitsWithNote)
	}
	if res.PercentCommitsWithNote != 0 {
		t.Errorf("PercentCommitsWithNote: got %f, want 0", res.PercentCommitsWithNote)
	}
}

func TestVolume_malformedNoteSkipped(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	headSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	if err := git.AddNote(repo, git.NotesRefStet, headSHA, `not json at all`); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	res, err := Volume(repo, "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("Volume: %v", err)
	}
	// Malformed note is skipped; commit is still in range.
	if res.CommitsInRange != 1 {
		t.Errorf("CommitsInRange: got %d, want 1", res.CommitsInRange)
	}
	if res.SessionsCount != 0 || res.CommitsWithNote != 0 {
		t.Errorf("malformed note should be skipped: SessionsCount=%d CommitsWithNote=%d", res.SessionsCount, res.CommitsWithNote)
	}
}

func TestVolume_resultJSONRoundtrip(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	res, err := Volume(repo, "HEAD", "HEAD")
	if err != nil {
		t.Fatalf("Volume: %v", err)
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded VolumeResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(decoded, *res) {
		t.Errorf("roundtrip: decoded %+v != original %+v", decoded, *res)
	}
}

func TestVolume_withoutGitAI_gitAIIsNil(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	res, err := Volume(repo, "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("Volume: %v", err)
	}
	if res.GitAI != nil {
		t.Errorf("Volume without git-ai: GitAI should be nil, got %+v", res.GitAI)
	}
}

func TestVolume_withGitAI_populatesGitAI(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	headSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	head1SHA := runOut(t, repo, "git", "rev-parse", "HEAD~1")
	note := `src/main.go
  abcd1234abcd1234 1-5
---
{"schema_version":"authorship/3.0.0","base_commit_sha":"` + headSHA + `","prompts":{"abcd1234abcd1234":{"agent_id":{"tool":"cursor","id":"x","model":"y"},"messages":[],"total_additions":5,"total_deletions":0,"accepted_lines":5,"overriden_lines":0}}}`
	if err := git.AddNote(repo, git.NotesRefAI, headSHA, note); err != nil {
		t.Fatalf("AddNote git-ai: %v", err)
	}
	res, err := Volume(repo, head1SHA, headSHA)
	if err != nil {
		t.Fatalf("Volume: %v", err)
	}
	if res.GitAI == nil {
		t.Fatal("Volume with git-ai: GitAI should be populated")
	}
	if res.GitAI.CommitsWithAINote != 1 {
		t.Errorf("GitAI.CommitsWithAINote: got %d, want 1", res.GitAI.CommitsWithAINote)
	}
	if res.GitAI.TotalAIAuthoredLines != 5 {
		t.Errorf("GitAI.TotalAIAuthoredLines: got %d, want 5", res.GitAI.TotalAIAuthoredLines)
	}
}
