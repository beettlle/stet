package git

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAddNote_and_GetNote(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	headSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	body := `{"session_id":"s1","baseline_sha":"b","head_sha":"h","findings_count":2,"dismissals_count":1,"tool_version":"dev","finished_at":"2025-02-12T12:00:00Z"}`
	if err := AddNote(repo, NotesRefStet, headSHA, body); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	got, err := GetNote(repo, NotesRefStet, headSHA)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if got != body {
		t.Errorf("GetNote: got %q, want %q", got, body)
	}
}

func TestAddNote_overwrites(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	headSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	if err := AddNote(repo, NotesRefStet, headSHA, "first"); err != nil {
		t.Fatalf("AddNote first: %v", err)
	}
	if err := AddNote(repo, NotesRefStet, headSHA, "second"); err != nil {
		t.Fatalf("AddNote second: %v", err)
	}
	got, err := GetNote(repo, NotesRefStet, headSHA)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if got != "second" {
		t.Errorf("GetNote: got %q, want second", got)
	}
}

func TestGetNote_noNote(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	headSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	_, err := GetNote(repo, NotesRefStet, headSHA)
	if err == nil {
		t.Fatal("GetNote: expected error when note does not exist")
	}
	if !strings.Contains(err.Error(), "no note") {
		t.Errorf("GetNote error: want 'no note' in message, got %v", err)
	}
}

func TestAddNote_invalidArgs(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	headSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	if err := AddNote("", NotesRefStet, headSHA, "body"); err == nil {
		t.Error("AddNote(empty repo): expected error")
	}
	if err := AddNote(repo, "", headSHA, "body"); err == nil {
		t.Error("AddNote(empty notesRef): expected error")
	}
	if err := AddNote(repo, NotesRefStet, "", "body"); err == nil {
		t.Error("AddNote(empty commitRef): expected error")
	}
}

func TestGetNote_invalidArgs(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	if _, err := GetNote("", NotesRefStet, "HEAD"); err == nil {
		t.Error("GetNote(empty repo): expected error")
	}
	if _, err := GetNote(repo, NotesRefStet, ""); err == nil {
		t.Error("GetNote(empty commitRef): expected error")
	}
}

func TestNotesRefStet_value(t *testing.T) {
	if NotesRefStet != "refs/notes/stet" {
		t.Errorf("NotesRefStet = %q, want refs/notes/stet", NotesRefStet)
	}
}

func TestNotesRefAI_value(t *testing.T) {
	if NotesRefAI != "refs/notes/ai" {
		t.Errorf("NotesRefAI = %q, want refs/notes/ai", NotesRefAI)
	}
}

// TestAddNote_JSON_roundtrip verifies that a JSON payload matching the
// finish-note schema can be written and read (no escaping issues).
func TestAddNote_JSON_roundtrip(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	headSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	payload := map[string]interface{}{
		"session_id":       "abc123",
		"baseline_sha":     strings.Repeat("a", 40),
		"head_sha":         headSHA,
		"findings_count":   3,
		"dismissals_count": 1,
		"tool_version":     "1.0.0",
		"finished_at":      "2025-02-12T14:30:00Z",
	}
	enc, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	body := string(enc)
	if err := AddNote(repo, NotesRefStet, headSHA, body); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	got, err := GetNote(repo, NotesRefStet, headSHA)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("Unmarshal note: %v", err)
	}
	if decoded["session_id"] != "abc123" {
		t.Errorf("session_id: got %v", decoded["session_id"])
	}
	if decoded["findings_count"] != float64(3) {
		t.Errorf("findings_count: got %v", decoded["findings_count"])
	}
}
