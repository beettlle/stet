// Restore dismissals from review history so they survive `stet finish`.
//
// Dismissals live only in the active session: `stet finish` records them in
// .review/history.jsonl, but `stet start` always created a session with
// DismissedIDs = nil, so the next review re-reported every dismissed finding.
// This file reads recent history at start and re-applies matching dismissals
// to the fresh session (PRD FR-3: dismissals survive re-runs and finish).
//
// Finding IDs are StableFindingID(file, line, range, message) and the message
// is model text, so a fresh review of the same code can produce a different ID
// for the same false positive. Matching therefore falls back to location:
// same file and category with overlapping line spans.

package run

import (
	"context"
	"os/exec"
	"sort"
	"strings"

	"stet/cli/internal/findings"
	"stet/cli/internal/git"
	"stet/cli/internal/history"
	"stet/cli/internal/session"
)

// maxRestoreHistoryRecords bounds the history scan: only the newest N records
// are considered when restoring dismissals at start (one git call per distinct
// DiffRef among them, so at most this many git calls).
const maxRestoreHistoryRecords = 50

// restorableDismissal is one dismissed finding recovered from history, with
// the location and diff ref needed to re-apply it to a new session.
type restorableDismissal struct {
	id       string // Finding ID as recorded in history.
	diffRef  string // Record's DiffRef (the reviewed HEAD at the time).
	file     string
	category findings.Category
	line     int
	// rangeStart/rangeEnd mirror Finding.Range when set; otherwise 0 and the
	// span is the single line above.
	rangeStart, rangeEnd int
}

// collectRestorableDismissals scans the newest maxRestoreHistoryRecords history
// records and returns one dismissal per distinct finding ID, keeping the most
// recent record's finding location and DiffRef. A dismissal is restorable when
// its finding ID appears in the most recent record that mentions it and the
// finding object is present in that record's ReviewOutput. IDs whose most
// recent recorded reason is already_correct are excluded: that reason marks
// auto-dismissals (the code was fixed) and must not hide future findings.
// Records are read oldest first; output is sorted by ID for determinism.
func collectRestorableDismissals(stateDir string) ([]restorableDismissal, error) {
	records, err := history.ReadRecords(stateDir)
	if err != nil {
		return nil, err
	}
	if len(records) > maxRestoreHistoryRecords {
		records = records[len(records)-maxRestoreHistoryRecords:]
	}
	latest := make(map[string]restorableDismissal)
	latestReason := make(map[string]string)   // ID -> most recent recorded reason ("" when none)
	latestHasFinding := make(map[string]bool) // ID -> finding object present in the latest mentioning record
	for _, rec := range records {
		byID := make(map[string]findings.Finding, len(rec.ReviewOutput))
		for _, f := range rec.ReviewOutput {
			if f.ID != "" {
				byID[f.ID] = f
			}
		}
		reasonByID := make(map[string]string, len(rec.UserAction.Dismissals))
		for _, d := range rec.UserAction.Dismissals {
			if d.FindingID != "" && d.Reason != "" {
				reasonByID[d.FindingID] = d.Reason
			}
		}
		for _, id := range rec.UserAction.DismissedIDs {
			f, ok := byID[id]
			latestHasFinding[id] = ok
			if !ok {
				continue // finding object not in this record's ReviewOutput
			}
			d := restorableDismissal{
				id:      id,
				diffRef: rec.DiffRef,
				file:    f.File,
				line:    f.Line,
			}
			if f.Range != nil {
				d.rangeStart, d.rangeEnd = f.Range.Start, f.Range.End
			}
			latest[id] = d
			if r, ok := reasonByID[id]; ok {
				// Only a reason actually recorded in this record updates the
				// latest reason; records without reasons (e.g. Finish) keep
				// the previous one.
				latestReason[id] = r
			}
		}
	}
	ids := make([]string, 0, len(latest))
	for id := range latest {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]restorableDismissal, 0, len(ids))
	for _, id := range ids {
		if !latestHasFinding[id] {
			continue // latest mention has no finding object: not restorable
		}
		if latestReason[id] == history.ReasonAlreadyCorrect {
			continue
		}
		out = append(out, latest[id])
	}
	return out, nil
}

// filterUnchangedFiles keeps only dismissals whose file is unchanged between
// the dismissal's DiffRef and headRef, so its line numbers still apply. One
// `git diff --name-only` runs per distinct DiffRef; dismissals whose DiffRef
// no longer resolves are dropped.
func filterUnchangedFiles(ctx context.Context, repoRoot string, dismissals []restorableDismissal, headRef string) []restorableDismissal {
	if len(dismissals) == 0 {
		return nil
	}
	type refState struct {
		changed map[string]struct{}
		ok      bool
	}
	byRef := make(map[string]refState)
	out := make([]restorableDismissal, 0, len(dismissals))
	for _, d := range dismissals {
		st, seen := byRef[d.diffRef]
		if !seen {
			paths, err := changedFilesSince(ctx, repoRoot, d.diffRef, headRef)
			st = refState{changed: paths, ok: err == nil}
			byRef[d.diffRef] = st
		}
		if !st.ok {
			continue // DiffRef no longer resolves; skip its dismissals
		}
		if _, changed := st.changed[d.file]; changed {
			continue // file changed since the record; line numbers may not apply
		}
		out = append(out, d)
	}
	return out
}

// changedFilesSince returns the set of repo-relative paths that differ between
// ref and headRef.
func changedFilesSince(ctx context.Context, repoRoot, ref, headRef string) (map[string]struct{}, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--no-color", "--name-only", ref, headRef)
	cmd.Dir = repoRoot
	cmd.Env = git.MinimalEnv()
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	files := make(map[string]struct{})
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files[line] = struct{}{}
		}
	}
	return files, nil
}

// dismissalSpan returns the 1-based inclusive line span of a restorable
// dismissal: Range.Start..Range.End when recorded, else the single Line.
func dismissalSpan(d restorableDismissal) (start, end int) {
	if d.rangeStart > 0 || d.rangeEnd > 0 {
		return d.rangeStart, d.rangeEnd
	}
	return d.line, d.line
}

// findingSpan returns the 1-based inclusive line span of a finding:
// Range.Start..Range.End when a meaningful Range is set, else the single Line.
// The condition mirrors dismissalSpan so a degenerate Range{0,0} falls back to
// Line on both sides of the match.
func findingSpan(f findings.Finding) (start, end int) {
	if f.Range != nil && (f.Range.Start > 0 || f.Range.End > 0) {
		return f.Range.Start, f.Range.End
	}
	return f.Line, f.Line
}

// matchesRestorableDismissal reports whether finding f matches a restorable
// dismissal d: same finding ID, or same file and category with overlapping
// line spans (spans use Range when set, else Line; see findingSpan).
func matchesRestorableDismissal(d restorableDismissal, f findings.Finding) bool {
	if d.id != "" && d.id == f.ID {
		return true
	}
	if d.file != f.File || d.category != f.Category {
		return false
	}
	ds, de := dismissalSpan(d)
	fs, fe := findingSpan(f)
	return ds <= fe && fs <= de
}

// applyRestoredDismissals appends the IDs of findings in collected that match
// a restorable dismissal to s.DismissedIDs (no duplicates) and returns how many
// findings were dismissed by restoration. Findings stay in s.Findings, so
// status shows them as dismissed and list hides them.
func applyRestoredDismissals(s *session.Session, collected []findings.Finding, restorable []restorableDismissal) int {
	if s == nil || len(collected) == 0 || len(restorable) == 0 {
		return 0
	}
	existing := make(map[string]struct{}, len(s.DismissedIDs))
	for _, id := range s.DismissedIDs {
		existing[id] = struct{}{}
	}
	restored := 0
	for _, f := range collected {
		if f.ID == "" {
			continue
		}
		if _, already := existing[f.ID]; already {
			continue
		}
		for _, d := range restorable {
			if matchesRestorableDismissal(d, f) {
				existing[f.ID] = struct{}{}
				s.DismissedIDs = append(s.DismissedIDs, f.ID)
				restored++
				break
			}
		}
	}
	return restored
}
