package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"stet/cli/internal/findings"
)

func TestSuppressionExamples_fixtureReturnsExpectedFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, historyFilename)
	rec := Record{
		DiffRef: "HEAD",
		ReviewOutput: []findings.Finding{
			{ID: "f1", File: "pkg/foo.go", Line: 42, Severity: findings.SeverityInfo, Category: findings.CategoryMaintainability, Confidence: 0.9, Message: "Consider adding comments"},
		},
		UserAction: UserAction{
			Dismissals: []Dismissal{{FindingID: "f1", Reason: ReasonFalsePositive}},
		},
	}
	writeRecord(t, path, rec)
	examples, err := SuppressionExamples(dir, 50, 30)
	if err != nil {
		t.Fatalf("SuppressionExamples: %v", err)
	}
	if len(examples) != 1 {
		t.Fatalf("got %d examples, want 1", len(examples))
	}
	want := "pkg/foo.go:42: Consider adding comments"
	if examples[0] != want {
		t.Errorf("example = %q, want %q", examples[0], want)
	}
}

func TestSuppressionExamples_deduplication(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, historyFilename)
	// Two records with same resolved message (same file:line: message after normalize).
	rec1 := Record{
		DiffRef: "r1",
		ReviewOutput: []findings.Finding{
			{ID: "a", File: "a.go", Line: 1, Severity: findings.SeverityWarning, Category: findings.CategoryBug, Confidence: 1, Message: "same msg"},
		},
		UserAction: UserAction{Dismissals: []Dismissal{{FindingID: "a"}}},
	}
	rec2 := Record{
		DiffRef: "r2",
		ReviewOutput: []findings.Finding{
			{ID: "b", File: "a.go", Line: 1, Severity: findings.SeverityWarning, Category: findings.CategoryBug, Confidence: 1, Message: "same msg"},
		},
		UserAction: UserAction{Dismissals: []Dismissal{{FindingID: "b"}}},
	}
	writeRecord(t, path, rec1)
	appendRecord(t, path, rec2)
	examples, err := SuppressionExamples(dir, 50, 30)
	if err != nil {
		t.Fatalf("SuppressionExamples: %v", err)
	}
	if len(examples) != 1 {
		t.Errorf("got %d examples after dedup, want 1", len(examples))
	}
}

func TestSuppressionExamples_maxRecords(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, historyFilename)
	// Three records; maxRecords=2 should use only last two.
	rec1 := Record{
		DiffRef: "r1",
		ReviewOutput: []findings.Finding{
			{ID: "old", File: "old.go", Line: 1, Severity: findings.SeverityWarning, Category: findings.CategoryBug, Confidence: 1, Message: "old"},
		},
		UserAction: UserAction{Dismissals: []Dismissal{{FindingID: "old"}}},
	}
	rec2 := Record{
		DiffRef: "r2",
		ReviewOutput: []findings.Finding{
			{ID: "mid", File: "mid.go", Line: 1, Severity: findings.SeverityWarning, Category: findings.CategoryBug, Confidence: 1, Message: "mid"},
		},
		UserAction: UserAction{Dismissals: []Dismissal{{FindingID: "mid"}}},
	}
	rec3 := Record{
		DiffRef: "r3",
		ReviewOutput: []findings.Finding{
			{ID: "new", File: "new.go", Line: 1, Severity: findings.SeverityWarning, Category: findings.CategoryBug, Confidence: 1, Message: "new"},
		},
		UserAction: UserAction{Dismissals: []Dismissal{{FindingID: "new"}}},
	}
	writeRecord(t, path, rec1)
	appendRecord(t, path, rec2)
	appendRecord(t, path, rec3)
	examples, err := SuppressionExamples(dir, 2, 30)
	if err != nil {
		t.Fatalf("SuppressionExamples: %v", err)
	}
	if len(examples) != 2 {
		t.Fatalf("got %d examples, want 2 (last two records)", len(examples))
	}
	// Order: oldest of the slice first = rec2 then rec3.
	if examples[0] != "mid.go:1: mid" || examples[1] != "new.go:1: new" {
		t.Errorf("got %q, want [mid.go:1: mid, new.go:1: new]", examples)
	}
}

func TestSuppressionExamples_maxExamples(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, historyFilename)
	// Build 35 resolvable examples; cap at 30, oldest dropped.
	var lines [][]byte
	for i := 0; i < 35; i++ {
		id := fmt.Sprintf("id-%d", i)
		rec := Record{
			DiffRef: "r",
			ReviewOutput: []findings.Finding{
				{ID: id, File: "f.go", Line: i + 1, Severity: findings.SeverityWarning, Category: findings.CategoryBug, Confidence: 1, Message: "msg"},
			},
			UserAction: UserAction{Dismissals: []Dismissal{{FindingID: id}}},
		}
		b, _ := json.Marshal(rec)
		lines = append(lines, b)
	}
	data := lines[0]
	for i := 1; i < len(lines); i++ {
		data = append(data, '\n')
		data = append(data, lines[i]...)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	examples, err := SuppressionExamples(dir, 50, 30)
	if err != nil {
		t.Fatalf("SuppressionExamples: %v", err)
	}
	if len(examples) != 30 {
		t.Errorf("got %d examples, want 30 (capped)", len(examples))
	}
	// Newest 30: lines 6..35 (0-indexed 5..34).
	if len(examples) > 0 && examples[0] != "f.go:6: msg" {
		t.Errorf("first (oldest of kept) = %q, want f.go:6: msg", examples[0])
	}
}

func TestSuppressionExamples_emptyOrMissingReturnsNil(t *testing.T) {
	dir := t.TempDir()
	// No history file.
	examples, err := SuppressionExamples(dir, 50, 30)
	if err != nil {
		t.Fatalf("SuppressionExamples (no file): %v", err)
	}
	if examples != nil {
		t.Errorf("got %v, want nil", examples)
	}
	// Empty Dismissals.
	path := filepath.Join(dir, historyFilename)
	rec := Record{
		DiffRef:      "HEAD",
		ReviewOutput: []findings.Finding{{ID: "f", File: "a.go", Line: 1, Severity: findings.SeverityInfo, Category: findings.CategoryBug, Confidence: 1, Message: "m"}},
		UserAction:   UserAction{DismissedIDs: []string{"f"}}, // no Dismissals with FindingID
	}
	writeRecord(t, path, rec)
	examples, err = SuppressionExamples(dir, 50, 30)
	if err != nil {
		t.Fatalf("SuppressionExamples (empty dismissals): %v", err)
	}
	if len(examples) != 0 && examples != nil {
		t.Errorf("got %d examples, want 0/nil", len(examples))
	}
	// Second record has Dismissals but no ReviewOutput (empty when decoded), so no match.
	rec2 := Record{DiffRef: "r2", UserAction: UserAction{Dismissals: []Dismissal{{FindingID: "x"}}}}
	appendRecord(t, path, rec2)
	examples, err = SuppressionExamples(dir, 50, 30)
	if err != nil {
		t.Fatalf("SuppressionExamples (no review output): %v", err)
	}
	if len(examples) != 0 && examples != nil {
		t.Errorf("got %d examples (second record has no ReviewOutput), want 0", len(examples))
	}
}

func TestSuppressionExamples_zeroMaxReturnsNil(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, historyFilename)
	rec := Record{
		DiffRef: "HEAD",
		ReviewOutput: []findings.Finding{
			{ID: "f", File: "a.go", Line: 1, Severity: findings.SeverityInfo, Category: findings.CategoryBug, Confidence: 1, Message: "m"},
		},
		UserAction: UserAction{Dismissals: []Dismissal{{FindingID: "f"}}},
	}
	writeRecord(t, path, rec)
	examples, err := SuppressionExamples(dir, 0, 30)
	if err != nil {
		t.Fatalf("SuppressionExamples(maxRecords=0): %v", err)
	}
	if examples != nil {
		t.Errorf("got %v, want nil", examples)
	}
	examples, err = SuppressionExamples(dir, 50, 0)
	if err != nil {
		t.Fatalf("SuppressionExamples(maxExamples=0): %v", err)
	}
	if examples != nil {
		t.Errorf("got %v, want nil", examples)
	}
}

func writeRecord(t *testing.T, path string, rec Record) {
	t.Helper()
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}

func appendRecord(t *testing.T, path string, rec Record) {
	t.Helper()
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		t.Fatal(err)
	}
}
