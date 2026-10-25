// Tests for dismissal restoration from history (see restore_dismissals.go).

package run

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/findings"
	"stet/cli/internal/history"
	"stet/cli/internal/session"
)

// writeHistoryRecords writes records as JSONL to stateDir/history.jsonl.
func writeHistoryRecords(t *testing.T, stateDir string, records []history.Record) {
	t.Helper()
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for _, rec := range records {
		b, err := json.Marshal(rec)
		if err != nil {
			t.Fatal(err)
		}
		sb.Write(b)
		sb.WriteString("\n")
	}
	if err := os.WriteFile(filepath.Join(stateDir, "history.jsonl"), []byte(sb.String()), 0644); err != nil {
		t.Fatal(err)
	}
}

// dismissalRecord builds a history record like `stet dismiss <id> <reason>`:
// the finding object in ReviewOutput, the ID in DismissedIDs, and the reason
// in Dismissals.
func dismissalRecord(diffRef, id string, f findings.Finding, reason string) history.Record {
	rec := history.Record{
		DiffRef:      diffRef,
		ReviewOutput: []findings.Finding{f},
		UserAction: history.UserAction{
			DismissedIDs: []string{id},
		},
	}
	if reason != "" {
		rec.UserAction.Dismissals = []history.Dismissal{{FindingID: id, Reason: reason}}
	}
	return rec
}

func testFinding(id, file string, line int, cat findings.Category) findings.Finding {
	return findings.Finding{
		ID:         id,
		File:       file,
		Line:       line,
		Severity:   findings.SeverityWarning,
		Category:   cat,
		Confidence: 1.0,
		Message:    "msg " + id,
	}
}

func collectIDs(ds []restorableDismissal) map[string]restorableDismissal {
	out := make(map[string]restorableDismissal, len(ds))
	for _, d := range ds {
		out[d.id] = d
	}
	return out
}

func TestCollectRestorableDismissals_newest50Window(t *testing.T) {
	t.Parallel()
	stateDir := t.TempDir()
	var records []history.Record
	// 55 records (indices 0..54); the newest-50 window is indices 5..54.
	for i := 0; i < 55; i++ {
		id := "id" + strings.Repeat("x", i) // distinct IDs, sortable
		records = append(records, dismissalRecord("headsha", id, testFinding(id, "f"+id+".txt", i, findings.CategoryBug), history.ReasonFalsePositive))
	}
	// "old" sits at index 4, just outside the window; "edge" at index 5, just inside.
	records[4] = dismissalRecord("headsha", "old", testFinding("old", "old.txt", 1, findings.CategoryBug), history.ReasonFalsePositive)
	records[5] = dismissalRecord("headsha", "edge", testFinding("edge", "edge.txt", 2, findings.CategoryBug), history.ReasonFalsePositive)
	writeHistoryRecords(t, stateDir, records)

	got, err := collectRestorableDismissals(stateDir)
	if err != nil {
		t.Fatalf("collectRestorableDismissals: %v", err)
	}
	ids := collectIDs(got)
	if _, ok := ids["old"]; ok {
		t.Error("dismissal from record 45 (outside newest-50 window): want excluded")
	}
	if _, ok := ids["edge"]; !ok {
		t.Error("dismissal from record 50 (last record inside window): want included")
	}
	if _, ok := ids["id"+strings.Repeat("x", 54)]; !ok {
		t.Error("dismissal from newest record: want included")
	}
}

func TestCollectRestorableDismissals_alreadyCorrectExcluded(t *testing.T) {
	t.Parallel()
	stateDir := t.TempDir()
	fp := testFinding("fp", "fp.txt", 1, findings.CategoryBug)
	ac := testFinding("ac", "ac.txt", 2, findings.CategoryBug)
	mixed := testFinding("mixed", "mixed.txt", 3, findings.CategoryBug)
	records := []history.Record{
		// "mixed" first dismissed as false_positive...
		dismissalRecord("sha1", "mixed", mixed, history.ReasonFalsePositive),
		// ...then auto-dismissed (already_correct): most recent reason wins.
		dismissalRecord("sha1", "mixed", mixed, history.ReasonAlreadyCorrect),
		// "ac" auto-dismissed only.
		dismissalRecord("sha1", "ac", ac, history.ReasonAlreadyCorrect),
		// "fp" dismissed as false_positive only.
		dismissalRecord("sha1", "fp", fp, history.ReasonFalsePositive),
	}
	writeHistoryRecords(t, stateDir, records)

	got, err := collectRestorableDismissals(stateDir)
	if err != nil {
		t.Fatalf("collectRestorableDismissals: %v", err)
	}
	ids := collectIDs(got)
	if _, ok := ids["ac"]; ok {
		t.Error("already_correct dismissal: want excluded")
	}
	if _, ok := ids["mixed"]; ok {
		t.Error("false_positive later auto-dismissed (latest reason already_correct): want excluded")
	}
	if _, ok := ids["fp"]; !ok {
		t.Error("false_positive dismissal: want included")
	}
}

func TestCollectRestorableDismissals_findingMissingFromReviewOutput(t *testing.T) {
	t.Parallel()
	stateDir := t.TempDir()
	missing := testFinding("missing", "missing.txt", 1, findings.CategoryBug)
	records := []history.Record{
		dismissalRecord("sha1", "missing", missing, history.ReasonFalsePositive),
		// A later record lists the ID but its ReviewOutput lacks the finding
		// object: not restorable from this record.
		{
			DiffRef:      "sha2",
			ReviewOutput: nil,
			UserAction:   history.UserAction{DismissedIDs: []string{"missing"}},
		},
	}
	writeHistoryRecords(t, stateDir, records)

	got, err := collectRestorableDismissals(stateDir)
	if err != nil {
		t.Fatalf("collectRestorableDismissals: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("dismissal whose finding is missing from the newest record's ReviewOutput: want excluded, got %v", got)
	}
}

func TestCollectRestorableDismissals_noReason_kept(t *testing.T) {
	t.Parallel()
	stateDir := t.TempDir()
	f := testFinding("noreason", "n.txt", 1, findings.CategoryBug)
	// Finish-style record: DismissedIDs without per-finding reasons.
	records := []history.Record{
		dismissalRecord("sha1", "noreason", f, ""),
	}
	writeHistoryRecords(t, stateDir, records)

	got, err := collectRestorableDismissals(stateDir)
	if err != nil {
		t.Fatalf("collectRestorableDismissals: %v", err)
	}
	if len(got) != 1 || got[0].id != "noreason" {
		t.Errorf("dismissal without recorded reason: want included, got %v", got)
	}
}

func TestCollectRestorableDismissals_latestLocationAndDiffRefWin(t *testing.T) {
	t.Parallel()
	stateDir := t.TempDir()
	f1 := testFinding("dup", "old.txt", 1, findings.CategoryBug)
	f2 := testFinding("dup", "new.txt", 9, findings.CategoryBug)
	records := []history.Record{
		dismissalRecord("sha1", "dup", f1, history.ReasonFalsePositive),
		dismissalRecord("sha2", "dup", f2, history.ReasonFalsePositive),
	}
	writeHistoryRecords(t, stateDir, records)

	got, err := collectRestorableDismissals(stateDir)
	if err != nil {
		t.Fatalf("collectRestorableDismissals: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want one dismissal per distinct ID, got %v", got)
	}
	if got[0].file != "new.txt" || got[0].line != 9 || got[0].diffRef != "sha2" {
		t.Errorf("want most recent record's location and DiffRef, got %+v", got[0])
	}
}

func TestCollectRestorableDismissals_locationMatchesRewordedFinding(t *testing.T) {
	t.Parallel()
	stateDir := t.TempDir()
	dismissed := testFinding("old-id", "a.go", 12, findings.CategorySecurity)
	writeHistoryRecords(t, stateDir, []history.Record{
		dismissalRecord("sha1", "old-id", dismissed, history.ReasonFalsePositive),
	})

	got, err := collectRestorableDismissals(stateDir)
	if err != nil {
		t.Fatalf("collectRestorableDismissals: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 restorable dismissal, got %v", got)
	}
	if got[0].category != findings.CategorySecurity {
		t.Errorf("category = %q, want %q", got[0].category, findings.CategorySecurity)
	}
	reworded := testFinding("new-id", "a.go", 12, findings.CategorySecurity)
	if !matchesRestorableDismissal(got[0], reworded) {
		t.Error("finding re-reported with a new ID at the same file, line, and category: want match")
	}
}

func TestRemapRestorableDismissals_shiftsSpansAndKeepsUnchanged(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	// State at the dismissal's DiffRef: "l2" sits on line 2 of f2.txt.
	writeFile(t, repo, "f2.txt", "b\nl2\nl3\n")
	runGit(t, repo, "git", "add", "f2.txt")
	runGit(t, repo, "git", "commit", "-m", "c3")
	diffRef := runOut(t, repo, "git", "rev-parse", "HEAD")
	// Edit ABOVE line 2: insert two lines after line 1, shifting "l2" to line 4.
	writeFile(t, repo, "f2.txt", "b\nx1\nx2\nl2\nl3\n")
	runGit(t, repo, "git", "add", "f2.txt")
	runGit(t, repo, "git", "commit", "-m", "c4")
	head := runOut(t, repo, "git", "rev-parse", "HEAD")

	dismissals := []restorableDismissal{
		{id: "shifted", diffRef: diffRef, file: "f2.txt", line: 2, category: findings.CategoryBug},
		{id: "ranged", diffRef: diffRef, file: "f2.txt", line: 0, rangeStart: 2, rangeEnd: 3, category: findings.CategoryBug},
		{id: "otherfile", diffRef: diffRef, file: "f1.txt", line: 1, category: findings.CategoryBug}, // unchanged file
	}
	got := remapRestorableDismissals(ctx, repo, dismissals, head)
	ids := collectIDs(got)
	if len(got) != 3 {
		t.Fatalf("remapRestorableDismissals kept = %d, want 3; got %v", len(got), got)
	}
	shifted := ids["shifted"]
	if shifted.line != 4 {
		t.Errorf("shifted dismissal line = %d, want 4 (shifted by 2)", shifted.line)
	}
	// A reworded finding at the shifted line matches; one at the old line does not.
	if !matchesRestorableDismissal(shifted, testFinding("new-id", "f2.txt", 4, findings.CategoryBug)) {
		t.Error("reworded finding at the shifted line 4: want match")
	}
	if matchesRestorableDismissal(shifted, testFinding("new-id", "f2.txt", 2, findings.CategoryBug)) {
		t.Error("finding at the pre-shift line 2: want no match")
	}
	ranged := ids["ranged"]
	if ranged.rangeStart != 4 || ranged.rangeEnd != 5 {
		t.Errorf("ranged dismissal span = [%d,%d], want [4,5] (shifted by 2)", ranged.rangeStart, ranged.rangeEnd)
	}
	if other := ids["otherfile"]; other.line != 1 {
		t.Errorf("unchanged-file dismissal line = %d, want 1 (identity)", other.line)
	}
}

func TestRemapRestorableDismissals_dropsEditedSpansDeletedFilesBadRefs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	writeFile(t, repo, "f2.txt", "b\nl2\nl3\n")
	runGit(t, repo, "git", "add", "f2.txt")
	runGit(t, repo, "git", "commit", "-m", "c3")
	diffRef := runOut(t, repo, "git", "rev-parse", "HEAD")
	// Edit the dismissed line itself (line 2) and delete f1.txt entirely.
	writeFile(t, repo, "f2.txt", "b\nL2\nl3\n")
	runGit(t, repo, "git", "add", "f2.txt")
	runGit(t, repo, "git", "rm", "f1.txt")
	runGit(t, repo, "git", "commit", "-m", "c4")
	head := runOut(t, repo, "git", "rev-parse", "HEAD")

	dismissals := []restorableDismissal{
		{id: "edited", diffRef: diffRef, file: "f2.txt", line: 2, category: findings.CategoryBug},
		{id: "survivor", diffRef: diffRef, file: "f2.txt", line: 3, category: findings.CategoryBug},
		{id: "deleted", diffRef: diffRef, file: "f1.txt", line: 1, category: findings.CategoryBug},
		{id: "unresolvable", diffRef: "does-not-exist", file: "f2.txt", line: 1, category: findings.CategoryBug},
	}
	got := remapRestorableDismissals(ctx, repo, dismissals, head)
	ids := collectIDs(got)
	if len(got) != 1 {
		t.Fatalf("remapRestorableDismissals kept = %d, want 1; got %v", len(got), got)
	}
	if _, ok := ids["edited"]; ok {
		t.Error("dismissal whose own line was edited: want dropped")
	}
	if _, ok := ids["deleted"]; ok {
		t.Error("dismissal for a deleted file: want dropped")
	}
	if _, ok := ids["unresolvable"]; ok {
		t.Error("dismissal with unresolvable DiffRef: want dropped")
	}
	if _, ok := ids["survivor"]; !ok {
		t.Error("dismissal below the edit: want kept")
	}
}

func TestRemapRestorableDismissals_empty(t *testing.T) {
	t.Parallel()
	if got := remapRestorableDismissals(context.Background(), t.TempDir(), nil, "HEAD"); got != nil {
		t.Errorf("empty input: want nil, got %v", got)
	}
}

func TestMatchesRestorableDismissal(t *testing.T) {
	t.Parallel()
	bug := findings.CategoryBug
	tests := []struct {
		name  string
		d     restorableDismissal
		f     findings.Finding
		match bool
	}{
		{
			name:  "same ID matches even with different location fields",
			d:     restorableDismissal{id: "abc", file: "other.txt", category: findings.CategoryStyle, line: 99},
			f:     testFinding("abc", "a.txt", 1, bug),
			match: true,
		},
		{
			name:  "different ID, same file and category, overlapping ranges",
			d:     restorableDismissal{id: "old", file: "a.txt", category: bug, rangeStart: 5, rangeEnd: 10},
			f:     findings.Finding{ID: "new", File: "a.txt", Category: bug, Range: &findings.LineRange{Start: 8, End: 12}},
			match: true,
		},
		{
			name:  "ranges touching at a boundary overlap",
			d:     restorableDismissal{id: "old", file: "a.txt", category: bug, rangeStart: 5, rangeEnd: 10},
			f:     findings.Finding{ID: "new", File: "a.txt", Category: bug, Range: &findings.LineRange{Start: 10, End: 12}},
			match: true,
		},
		{
			name:  "adjacent ranges do not overlap",
			d:     restorableDismissal{id: "old", file: "a.txt", category: bug, rangeStart: 5, rangeEnd: 10},
			f:     findings.Finding{ID: "new", File: "a.txt", Category: bug, Range: &findings.LineRange{Start: 11, End: 15}},
			match: false,
		},
		{
			name:  "same file and line, different category: no match",
			d:     restorableDismissal{id: "old", file: "a.txt", category: bug, line: 7},
			f:     findings.Finding{ID: "new", File: "a.txt", Line: 7, Category: findings.CategoryStyle},
			match: false,
		},
		{
			name:  "same category and overlap, different file: no match",
			d:     restorableDismissal{id: "old", file: "a.txt", category: bug, line: 7},
			f:     findings.Finding{ID: "new", File: "b.txt", Line: 7, Category: bug},
			match: false,
		},
		{
			name:  "single line spans overlap",
			d:     restorableDismissal{id: "old", file: "a.txt", category: bug, line: 7},
			f:     findings.Finding{ID: "new", File: "a.txt", Line: 7, Category: bug},
			match: true,
		},
		{
			name:  "inverted dismissal range falls back to Line",
			d:     restorableDismissal{id: "old", file: "a.txt", category: bug, line: 7, rangeStart: 10, rangeEnd: 5},
			f:     findings.Finding{ID: "new", File: "a.txt", Line: 7, Category: bug},
			match: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesRestorableDismissal(tt.d, tt.f); got != tt.match {
				t.Errorf("matchesRestorableDismissal(%+v, %+v) = %v, want %v", tt.d, tt.f, got, tt.match)
			}
		})
	}
}

func TestApplyRestoredDismissals(t *testing.T) {
	t.Parallel()
	restorable := []restorableDismissal{
		{id: "old1", file: "a.txt", category: findings.CategoryBug, line: 1},
		{id: "old2", file: "b.txt", category: findings.CategoryBug, line: 2},
	}
	collected := []findings.Finding{
		testFinding("new1", "a.txt", 1, findings.CategoryBug), // matches old1 by location
		testFinding("old2", "b.txt", 2, findings.CategoryBug), // matches old2 by ID
		testFinding("other", "c.txt", 3, findings.CategoryBug),
	}
	s := &session.Session{DismissedIDs: []string{"already"}}
	got := applyRestoredDismissals(s, collected, restorable)
	if got != 2 {
		t.Errorf("restored = %d, want 2", got)
	}
	want := map[string]bool{"already": true, "new1": true, "old2": true}
	if len(s.DismissedIDs) != len(want) {
		t.Fatalf("DismissedIDs = %v, want %v", s.DismissedIDs, want)
	}
	for _, id := range s.DismissedIDs {
		if !want[id] {
			t.Errorf("unexpected dismissed ID %q", id)
		}
	}
	if len(collected) != 3 {
		t.Errorf("findings must stay in collected, got %d", len(collected))
	}

	// No duplicates: already-dismissed IDs are not appended again.
	if got := applyRestoredDismissals(s, collected, restorable); got != 0 {
		t.Errorf("second apply restored = %d, want 0 (no duplicates)", got)
	}

	// Early returns.
	if got := applyRestoredDismissals(nil, collected, restorable); got != 0 {
		t.Errorf("nil session restored = %d, want 0", got)
	}
	if got := applyRestoredDismissals(s, nil, restorable); got != 0 {
		t.Errorf("no findings restored = %d, want 0", got)
	}
	if got := applyRestoredDismissals(s, collected, nil); got != 0 {
		t.Errorf("no restorable dismissals restored = %d, want 0", got)
	}
}

// dismissLikeCLI dismisses a finding the way `stet dismiss <id> <reason>` does:
// adds the ID to the session's dismissed list, saves the session, and appends
// a history record with the finding object, the ID, and the reason.
func dismissLikeCLI(t *testing.T, stateDir, id, reason string) {
	t.Helper()
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	s.DismissedIDs = append(s.DismissedIDs, id)
	if err := session.Save(stateDir, &s); err != nil {
		t.Fatal(err)
	}
	findingByID := make(map[string]findings.Finding, len(s.Findings))
	for _, f := range s.Findings {
		findingByID[f.ID] = f
	}
	f, ok := findingByID[id]
	if !ok {
		t.Fatalf("finding %q not in session", id)
	}
	_ = f
	diffRef := s.LastReviewedAt
	if diffRef == "" {
		diffRef = s.BaselineRef
	}
	rec := history.Record{
		DiffRef:      diffRef,
		ReviewOutput: s.Findings,
		UserAction: history.UserAction{
			DismissedIDs: []string{id},
			Dismissals:   []history.Dismissal{{FindingID: id, Reason: reason}},
		},
	}
	if err := history.Append(stateDir, rec, history.DefaultMaxRecords); err != nil {
		t.Fatal(err)
	}
}

func startOpts(repo, stateDir string) StartOptions {
	return StartOptions{
		RepoRoot:     repo,
		StateDir:     stateDir,
		WorktreeRoot: "",
		Ref:          "HEAD~1",
		DryRun:       true,
		Model:        "",
		Provider:     "ollama",
		LLMBaseURL:   "",
	}
}

func TestStart_restoresDismissals_sameBaseline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")

	// First start: one dry-run finding.
	if _, err := Start(ctx, startOpts(repo, stateDir)); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	s1, err := session.Load(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s1.Findings) != 1 {
		t.Fatalf("first session findings = %d, want 1", len(s1.Findings))
	}
	id := s1.Findings[0].ID
	if id == "" {
		t.Fatal("finding ID is empty")
	}
	// Dismiss the way stet dismiss does, then finish the session.
	dismissLikeCLI(t, stateDir, id, history.ReasonFalsePositive)
	if err := Finish(ctx, FinishOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: ""}); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	// Second start on the same baseline: the re-reported finding must be
	// restored as dismissed.
	if _, err := Start(ctx, startOpts(repo, stateDir)); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	s2, err := session.Load(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.Findings) != 1 {
		t.Fatalf("second session findings = %d, want 1 (findings stay, only IDs dismissed)", len(s2.Findings))
	}
	if s2.Findings[0].ID != id {
		t.Fatalf("re-reported finding ID = %q, want %q", s2.Findings[0].ID, id)
	}
	dismissed := false
	for _, d := range s2.DismissedIDs {
		if d == id {
			dismissed = true
		}
	}
	if !dismissed {
		t.Errorf("re-reported finding ID %q not in DismissedIDs %v", id, s2.DismissedIDs)
	}
	active, err := ActiveFindingCount(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Errorf("active finding count = %d, want 0", active)
	}
}

// Renamed premise under SP-072: a file change below the dismissal no longer
// drops it (the span survives the shift); only an edit to the dismissed lines
// themselves does.
func TestStart_noRestore_dismissedLinesEdited(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")

	if _, err := Start(ctx, startOpts(repo, stateDir)); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	s1, err := session.Load(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	id := s1.Findings[0].ID
	dismissLikeCLI(t, stateDir, id, history.ReasonFalsePositive)
	if err := Finish(ctx, FinishOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: ""}); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	// Edit the finding's own line (line 1 of f2.txt) in a new commit: the
	// dismissal must not be restored, its code changed.
	writeFile(t, repo, "f2.txt", "b2\nchanged\n")
	runGit(t, repo, "git", "add", "f2.txt")
	runGit(t, repo, "git", "commit", "-m", "c3")

	if _, err := Start(ctx, startOpts(repo, stateDir)); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	s2, err := session.Load(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.DismissedIDs) != 0 {
		t.Errorf("DismissedIDs = %v, want empty (dismissed lines were edited)", s2.DismissedIDs)
	}
}

// Not parallel: captures os.Stderr so must run alone to avoid races.
func TestStart_historyReadError_warnsAndContinues(t *testing.T) {
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")
	// Corrupt history: ReadRecords must fail, Start must only warn.
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "history.jsonl"), []byte("{not json\n"), 0644); err != nil {
		t.Fatal(err)
	}

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStderr := os.Stderr
	os.Stderr = stderrW
	t.Cleanup(func() {
		os.Stderr = oldStderr
		_ = stderrR.Close()
	})

	opts := startOpts(repo, stateDir)
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("Start with corrupt history: %v (want warning, not failure)", err)
	}
	_ = stderrW.Close()
	out, err := io.ReadAll(stderrR)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "warning: could not restore dismissals:") {
		t.Errorf("stderr = %q, want dismissal restore warning", string(out))
	}
	// The session is still usable and has no restored dismissals.
	s, err := session.Load(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.DismissedIDs) != 0 {
		t.Errorf("DismissedIDs = %v, want empty", s.DismissedIDs)
	}
}

// Not parallel: captures os.Stderr so must run alone to avoid races.
func TestStart_traceAndVerboseReportRestoredCount(t *testing.T) {
	ctx := context.Background()
	repo := initRepo(t)
	stateDir := filepath.Join(repo, ".review")

	if _, err := Start(ctx, startOpts(repo, stateDir)); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	s1, err := session.Load(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	id := s1.Findings[0].ID
	dismissLikeCLI(t, stateDir, id, history.ReasonFalsePositive)
	if err := Finish(ctx, FinishOptions{RepoRoot: repo, StateDir: stateDir, WorktreeRoot: ""}); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	var traceBuf bytes.Buffer
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStderr := os.Stderr
	os.Stderr = stderrW
	t.Cleanup(func() {
		os.Stderr = oldStderr
		_ = stderrR.Close()
	})

	opts := startOpts(repo, stateDir)
	opts.TraceOut = &traceBuf
	opts.Verbose = true
	if _, err := Start(ctx, opts); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	_ = stderrW.Close()
	stderrOut, err := io.ReadAll(stderrR)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(traceBuf.String(), "Restorable dismissals from history: 1") {
		t.Errorf("trace = %q, want restorable dismissals count", traceBuf.String())
	}
	if !strings.Contains(traceBuf.String(), "restored 1 dismissals from history") {
		t.Errorf("trace = %q, want restored count", traceBuf.String())
	}
	if !strings.Contains(string(stderrOut), "restored 1 dismissals from history") {
		t.Errorf("stderr = %q, want restored count (verbose)", string(stderrOut))
	}
}
