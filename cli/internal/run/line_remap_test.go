// Tests for the dismissed-findings line re-mapping library (line_remap.go).
// Parser tests are pure (fixture text, no git); loadLineRemaps has one
// end-to-end test against a temp repo plus focused git-invocation tests.

package run

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// remapDiffFixture is a `git diff --no-color --no-renames -U0` output fixture
// with real git header shapes (single-line counts omitted, `-a,0` insertions,
// `+c,0` deletions, /dev/null sides). Per file:
//
//   - f.txt: line 3 modified in place (`@@ -3 +3 @@`), two lines inserted
//     after old 6 (`@@ -6,0 +7,2 @@`), old 9 deleted (`@@ -9 +10,0 @@`).
//     Its first hunk also removes a line whose text starts with "-- "
//     (rendered `--- sneaky.txt`), which must not parse as a file header.
//   - gone.txt: deleted at toRef (`+++ /dev/null`).
//   - d.txt: old lines 2-4 deleted (`@@ -2,3 +2,0 @@`).
//   - m.txt: three lines inserted after old 1 (`@@ -1,0 +2,3 @@`) and old
//     7-8 collapsed to one (`@@ -7,2 +10,1 @@`).
//   - new.txt: created at toRef (`--- /dev/null`).
const remapDiffFixture = `diff --git a/f.txt b/f.txt
index 4ed2032..a1a1368 100644
--- a/f.txt
+++ b/f.txt
@@ -3 +3 @@ L2
--- sneaky.txt
+L3-mod
@@ -6,0 +7,2 @@ L6
+X1
+X2
@@ -9 +10,0 @@ L8
-L9
diff --git a/gone.txt b/gone.txt
deleted file mode 100644
index 22d30e6..0000000
--- a/gone.txt
+++ /dev/null
@@ -1 +0,0 @@
-H1
diff --git a/d.txt b/d.txt
index 1111111..2222222 100644
--- a/d.txt
+++ b/d.txt
@@ -2,3 +2,0 @@ L1
-D2
-D3
-D4
diff --git a/m.txt b/m.txt
index 3333333..4444444 100644
--- a/m.txt
+++ b/m.txt
@@ -1,0 +2,3 @@ L1
+A1
+A2
+A3
@@ -7,2 +10,1 @@ L6
-L7
-L8
+L78
diff --git a/new.txt b/new.txt
new file mode 100644
index 0000000..587be6b
--- /dev/null
+++ b/new.txt
@@ -0,0 +1 @@
+N1
`

func TestParseLineRemaps_filesAndHunks(t *testing.T) {
	t.Parallel()
	got := parseLineRemaps(remapDiffFixture)

	wantHunks := map[string][]hunkRange{
		"f.txt":    {{oldStart: 3, oldCount: 1, delta: 0}, {oldStart: 6, oldCount: 0, delta: 2}, {oldStart: 9, oldCount: 1, delta: -1}},
		"gone.txt": {{oldStart: 1, oldCount: 1, delta: -1}},
		"d.txt":    {{oldStart: 2, oldCount: 3, delta: -3}},
		"m.txt":    {{oldStart: 1, oldCount: 0, delta: 3}, {oldStart: 7, oldCount: 2, delta: -1}},
		"new.txt":  {{oldStart: 0, oldCount: 0, delta: 1}},
	}
	if len(got) != len(wantHunks) {
		t.Fatalf("parseLineRemaps: got %d file sections (%v), want %d", len(got), keysOf(got), len(wantHunks))
	}
	for file, want := range wantHunks {
		rm, ok := got[file]
		if !ok {
			t.Errorf("file %q: want section, got none", file)
			continue
		}
		if !reflect.DeepEqual(rm.hunks, want) {
			t.Errorf("file %q: hunks = %+v, want %+v", file, rm.hunks, want)
		}
	}
	if got["gone.txt"].deleted != true {
		t.Error("gone.txt: deleted = false, want true (+++ /dev/null)")
	}
	for _, file := range []string{"f.txt", "d.txt", "m.txt", "new.txt"} {
		if got[file].deleted {
			t.Errorf("file %q: deleted = true, want false", file)
		}
	}
	if _, ok := got["sneaky.txt"]; ok {
		t.Error("removed line rendered `--- sneaky.txt` parsed as a file header; want it treated as hunk content")
	}
}

func TestParseLineRemaps_emptyAndMalformed(t *testing.T) {
	t.Parallel()
	if got := parseLineRemaps(""); len(got) != 0 {
		t.Errorf("empty diff: got %v, want empty map", got)
	}
	// A malformed header is skipped, not fatal; the section still exists so
	// a file with only malformed hunks reads as changed-but-unmapped.
	malformed := "diff --git a/bad.txt b/bad.txt\n--- a/bad.txt\n+++ b/bad.txt\n@@ -x +y @@\n+hello\n"
	got := parseLineRemaps(malformed)
	rm, ok := got["bad.txt"]
	if !ok {
		t.Fatal("malformed header: want bad.txt section to exist")
	}
	if len(rm.hunks) != 0 {
		t.Errorf("malformed header: hunks = %+v, want none", rm.hunks)
	}
}

// TestRemapSpan_table covers the span rules from the task contract against
// the fixture: spans above/inside/below changes, insertions and deletions
// above and directly after the span, multi-hunk accumulation, unchanged and
// deleted files.
func TestRemapSpan_table(t *testing.T) {
	t.Parallel()
	remaps := parseLineRemaps(remapDiffFixture)
	tests := []struct {
		name       string
		file       string
		start, end int
		wantStart  int
		wantEnd    int
		wantOK     bool
	}{
		// f.txt: mod at 3 (delta 0), insertion after 6 (delta +2),
		// deletion at 9 (delta -1).
		{name: "lines above modification", file: "f.txt", start: 1, end: 2, wantStart: 1, wantEnd: 2, wantOK: true},
		{name: "lines below modification before insertion", file: "f.txt", start: 4, end: 5, wantStart: 4, wantEnd: 5, wantOK: true},
		{name: "span inside modification", file: "f.txt", start: 3, end: 3, wantOK: false},
		{name: "span overlapping modification start", file: "f.txt", start: 2, end: 3, wantOK: false},
		{name: "span overlapping modification end", file: "f.txt", start: 3, end: 4, wantOK: false},
		{name: "insertion directly after span keeps it", file: "f.txt", start: 5, end: 6, wantStart: 5, wantEnd: 6, wantOK: true},
		{name: "insertion directly before span shifts it", file: "f.txt", start: 7, end: 8, wantStart: 9, wantEnd: 10, wantOK: true},
		{name: "span straddling insertion keeps bounds", file: "f.txt", start: 5, end: 7, wantStart: 5, wantEnd: 7, wantOK: true},
		{name: "span on deleted line", file: "f.txt", start: 9, end: 9, wantOK: false},
		{name: "span overlapping deleted line from above", file: "f.txt", start: 8, end: 9, wantOK: false},
		{name: "all hunks accumulate below them", file: "f.txt", start: 10, end: 10, wantStart: 11, wantEnd: 11, wantOK: true},
		// d.txt: lines 2-4 deleted (delta -3).
		{name: "deleted lines above shift by -N", file: "d.txt", start: 6, end: 8, wantStart: 3, wantEnd: 5, wantOK: true},
		{name: "deletion below leaves span", file: "d.txt", start: 1, end: 1, wantStart: 1, wantEnd: 1, wantOK: true},
		{name: "span inside deletion", file: "d.txt", start: 3, end: 3, wantOK: false},
		{name: "span overlapping deletion end", file: "d.txt", start: 4, end: 5, wantOK: false},
		// m.txt: insertion after 1 (delta +3), modification at 7-8 (delta -1);
		// both end before old line 10, so the span shifts by +3 -1 = +2.
		{name: "multiple hunks accumulate", file: "m.txt", start: 10, end: 12, wantStart: 12, wantEnd: 14, wantOK: true},
		{name: "span between hunks shifts by earlier hunk only", file: "m.txt", start: 3, end: 4, wantStart: 6, wantEnd: 7, wantOK: true},
		{name: "span inside later modification", file: "m.txt", start: 8, end: 8, wantOK: false},
		// gone.txt: deleted at toRef — nothing survives.
		{name: "deleted file rejects span", file: "gone.txt", start: 1, end: 1, wantOK: false},
		{name: "deleted file rejects out-of-range span", file: "gone.txt", start: 100, end: 200, wantOK: false},
		// new.txt: insertion before line 1 (delta +1).
		{name: "insertion at top of file shifts span", file: "new.txt", start: 1, end: 1, wantStart: 2, wantEnd: 2, wantOK: true},
		// Unchanged file: identity.
		{name: "file absent from diff is identity", file: "unchanged.txt", start: 4, end: 9, wantStart: 4, wantEnd: 9, wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd, ok := remaps.remapSpan(tt.file, tt.start, tt.end)
			if ok != tt.wantOK {
				t.Fatalf("remapSpan(%s, %d, %d) ok = %v, want %v", tt.file, tt.start, tt.end, ok, tt.wantOK)
			}
			if ok && (gotStart != tt.wantStart || gotEnd != tt.wantEnd) {
				t.Errorf("remapSpan(%s, %d, %d) = (%d, %d), want (%d, %d)",
					tt.file, tt.start, tt.end, gotStart, gotEnd, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestRemapSpan_nilMapIsIdentity(t *testing.T) {
	t.Parallel()
	var remaps lineRemaps
	gotStart, gotEnd, ok := remaps.remapSpan("any.txt", 7, 9)
	if !ok || gotStart != 7 || gotEnd != 9 {
		t.Errorf("nil map remapSpan = (%d, %d, %v), want (7, 9, true)", gotStart, gotEnd, ok)
	}
}

func TestLoadLineRemaps_endToEnd(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	writeFile(t, dir, "f.txt", "L1\nL2\nL3\nL4\nL5\nL6\nL7\nL8\nL9\nL10\n")
	writeFile(t, dir, "g.txt", "G1\nG2\n")
	writeFile(t, dir, "gone.txt", "W1\n")
	runGit(t, dir, "git", "add", ".")
	runGit(t, dir, "git", "commit", "-m", "base")
	fromRef := runOut(t, dir, "git", "rev-parse", "HEAD")

	// Modify L3 in place, insert X1/X2 after L6, delete L9; prepend G0 to
	// g.txt; delete gone.txt.
	writeFile(t, dir, "f.txt", "L1\nL2\nL3-mod\nL4\nL5\nL6\nX1\nX2\nL7\nL8\nL10\n")
	writeFile(t, dir, "g.txt", "G0\nG1\nG2\n")
	if err := os.Remove(filepath.Join(dir, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "git", "add", "-A")
	runGit(t, dir, "git", "commit", "-m", "edit")
	toRef := runOut(t, dir, "git", "rev-parse", "HEAD")

	// One call covers every requested file, changed or not.
	remaps, err := loadLineRemaps(context.Background(), dir, fromRef, toRef,
		[]string{"f.txt", "g.txt", "gone.txt", "f1.txt"})
	if err != nil {
		t.Fatalf("loadLineRemaps: %v", err)
	}

	// f.txt: old 7-8 now sit at 9-10 (two lines inserted above), old 9 is
	// gone (deleted line rejects), old 10 shifted by +2 -1 = +1.
	if s, e, ok := remaps.remapSpan("f.txt", 7, 8); !ok || s != 9 || e != 10 {
		t.Errorf("f.txt span (7,8) = (%d, %d, %v), want (9, 10, true)", s, e, ok)
	}
	if _, _, ok := remaps.remapSpan("f.txt", 3, 3); ok {
		t.Error("f.txt span (3,3) on modified line: want ok=false")
	}
	if _, _, ok := remaps.remapSpan("f.txt", 9, 9); ok {
		t.Error("f.txt span (9,9) on deleted line: want ok=false")
	}
	if s, e, ok := remaps.remapSpan("f.txt", 10, 10); !ok || s != 11 || e != 11 {
		t.Errorf("f.txt span (10,10) = (%d, %d, %v), want (11, 11, true)", s, e, ok)
	}

	// g.txt: one line inserted above shifts old 2 to new 3.
	if s, e, ok := remaps.remapSpan("g.txt", 2, 2); !ok || s != 3 || e != 3 {
		t.Errorf("g.txt span (2,2) = (%d, %d, %v), want (3, 3, true)", s, e, ok)
	}

	// gone.txt was deleted: nothing survives.
	if _, _, ok := remaps.remapSpan("gone.txt", 1, 1); ok {
		t.Error("gone.txt deleted at toRef: want ok=false")
	}

	// f1.txt is unchanged between the refs: absent from the map, identity.
	if _, ok := remaps["f1.txt"]; ok {
		t.Error("f1.txt unchanged between refs: want no map entry")
	}
	if s, e, ok := remaps.remapSpan("f1.txt", 1, 1); !ok || s != 1 || e != 1 {
		t.Errorf("f1.txt identity span = (%d, %d, %v), want (1, 1, true)", s, e, ok)
	}
}

func TestLoadLineRemaps_emptyFilesMakesNoGitCall(t *testing.T) {
	t.Parallel()
	// The directory is not a git repo, so any git call would fail; an empty
	// file list must return before git runs.
	remaps, err := loadLineRemaps(context.Background(), t.TempDir(), "no-such-ref", "head", nil)
	if err != nil {
		t.Fatalf("empty files list: unexpected error: %v", err)
	}
	if len(remaps) != 0 {
		t.Errorf("empty files list: remaps = %v, want empty", remaps)
	}
}

func TestLoadLineRemaps_invalidRefReturnsError(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	_, err := loadLineRemaps(context.Background(), dir, "definitely-not-a-ref", "HEAD", []string{"f1.txt"})
	if err == nil {
		t.Fatal("loadLineRemaps with unresolvable ref: want error, got nil")
	}
}

func TestLoadLineRemaps_limitsToRequestedFiles(t *testing.T) {
	t.Parallel()
	dir := initRepo(t) // f1.txt and f2.txt exist from initRepo
	writeFile(t, dir, "f1.txt", "a-changed\n")
	writeFile(t, dir, "f2.txt", "b-changed\n")
	runGit(t, dir, "git", "add", "-A")
	runGit(t, dir, "git", "commit", "-m", "both")

	remaps, err := loadLineRemaps(context.Background(), dir, "HEAD~1", "HEAD", []string{"f1.txt"})
	if err != nil {
		t.Fatalf("loadLineRemaps: %v", err)
	}
	if _, ok := remaps["f1.txt"]; !ok {
		t.Error("f1.txt requested and changed: want map entry")
	}
	if _, ok := remaps["f2.txt"]; ok {
		t.Error("f2.txt not requested: want excluded from map")
	}
}

// keysOf lists a lineRemaps' keys for failure messages.
func keysOf(m lineRemaps) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
