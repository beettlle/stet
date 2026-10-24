// Line re-mapping for dismissed findings across commits.
//
// A dismissal records a 1-based inclusive line span observed at one commit
// (its DiffRef). When edits land above or on those lines before the next
// review, the stored span no longer points at the dismissed code — and
// filterUnchangedFiles drops dismissals whose file changed at all. This file
// translates a span recorded at one ref into the same code's span at another
// ref: loadLineRemaps parses ONE `git diff -U0` between the refs into changed
// ranges per file, and remapSpan applies the translation. SP-072 wires both
// into dismissal restoration and matching.

package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"stet/cli/internal/git"
)

// devNull is git's sentinel for a missing side of a diff (file created or
// deleted between the refs).
const devNull = "/dev/null"

// hunkRange is the old-side footprint of one -U0 hunk: which old-file lines
// changed, and the net line-count change the hunk applies after them.
type hunkRange struct {
	// oldStart is the first old-file line of the hunk. When oldCount is 0
	// (pure insertion) oldStart names the line BEFORE the insertion point,
	// per git's `-a,0` convention (0 = before the first line).
	oldStart int
	// oldCount is how many old-file lines the hunk changed (git omits the
	// count when it is 1). Pure insertions have oldCount 0: no old line was
	// modified or deleted.
	oldCount int
	// delta is newCount - oldCount: the shift applied to every old line that
	// comes after the hunk.
	delta int
}

// fileRemap holds the changed ranges parsed for one file between two refs.
type fileRemap struct {
	hunks []hunkRange
	// deleted reports the file no longer exists at toRef (git emits
	// `+++ /dev/null`): no line span survives.
	deleted bool
}

// lineRemaps maps repo-relative paths to per-file remap info for one ref
// pair. Files absent from the map are unchanged between the refs, so
// remapSpan on them is identity; a nil map behaves the same way.
type lineRemaps map[string]*fileRemap

// loadLineRemaps runs one `git diff --no-color --no-renames -U0` from fromRef
// to toRef covering all requested files (invocation conventions mirror
// changedFilesSince: git.MinimalEnv, cmd.Dir = repoRoot) and parses the
// `@@ -a,b +c,d @@` headers per file. An empty files list makes no git call.
// The result maps each requested file that differs between the refs to its
// remap info; every other file is absent.
func loadLineRemaps(ctx context.Context, repoRoot, fromRef, toRef string, files []string) (lineRemaps, error) {
	if len(files) == 0 {
		return lineRemaps{}, nil
	}
	args := append([]string{"diff", "--no-color", "--no-renames", "-U0", fromRef, toRef, "--"}, files...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoRoot
	cmd.Env = git.MinimalEnv()
	out, err := cmd.Output()
	if err != nil {
		if detail := gitErrDetail(err); detail != "" {
			return nil, fmt.Errorf("git diff %s %s: %w: %s", fromRef, toRef, err, detail)
		}
		return nil, fmt.Errorf("git diff %s %s: %w", fromRef, toRef, err)
	}
	parsed := parseLineRemaps(string(out))
	// The pathspec already limits the diff to the requested files; the filter
	// below bounds the result to them regardless, so a stray section can
	// never enter the map.
	requested := make(map[string]struct{}, len(files))
	for _, f := range files {
		requested[f] = struct{}{}
	}
	remaps := make(lineRemaps, len(files))
	for path, rm := range parsed {
		if _, want := requested[path]; want {
			remaps[path] = rm
		}
	}
	return remaps, nil
}

// remapSpan translates the 1-based inclusive span start..end (recorded at
// fromRef) into toRef lines.
//
//   - File absent from the diff (unchanged between the refs): identity, ok=true.
//   - Every span line outside the changed old ranges: the span shifts by the
//     cumulative delta of all hunks that end before start, ok=true. A pure
//     insertion (-a,0) ends at a: it shifts the span when a < start and keeps
//     a span that ends at a or earlier ("insertion directly after the span
//     keeps it"). A span that straddles an insertion point (start <= a < end)
//     keeps its bounds: no old line changed and the insertion does not end
//     before start, though part of the inserted text then sits inside the
//     mapped span.
//   - Any span line inside a changed old range (modified or deleted): the
//     dismissed code itself changed, so the dismissal no longer applies —
//     ok=false.
//   - File deleted at toRef: ok=false.
//
// The returned span is meaningful only when ok is true.
func (m lineRemaps) remapSpan(file string, start, end int) (newStart, newEnd int, ok bool) {
	rm, seen := m[file]
	if !seen {
		return start, end, true
	}
	if rm.deleted {
		return start, end, false
	}
	shift := 0
	for _, h := range rm.hunks {
		if h.oldCount == 0 {
			// Pure insertion after old line h.oldStart: shifts every line
			// after the insertion point, i.e. spans that start beyond it.
			if h.oldStart < start {
				shift += h.delta
			}
			continue
		}
		oldEnd := h.oldStart + h.oldCount - 1
		if start <= oldEnd && h.oldStart <= end {
			return start, end, false
		}
		if oldEnd < start {
			shift += h.delta
		}
	}
	return start + shift, end + shift, true
}

// parseLineRemaps is a pure parser over `git diff --no-color --no-renames
// -U0` output (it never runs git, so tests can feed fixture text). It returns
// one entry per file section that reaches a `+++` header; hunks attach to the
// section they follow. Hunk content is skipped while inHunk so a removed line
// whose text starts with "-- " (rendered `--- ...`) can never be mistaken for
// a file header. Malformed hunk headers are skipped: git emits well-formed
// headers, and the guard only defends against truncated output.
func parseLineRemaps(diffText string) lineRemaps {
	remaps := lineRemaps{}
	var (
		cur     *fileRemap
		oldPath string // `--- ` side: repo-relative path, or devNull for creations
		newPath string // `+++ ` side: repo-relative path, or devNull for deletions
		inHunk  bool
	)
	// beginFile registers the section once the +++ header names it: keyed by
	// the new path, except a deletion (`+++ /dev/null`) keys by the old path
	// and marks the file deleted.
	beginFile := func() {
		if cur != nil {
			return
		}
		if newPath == devNull {
			cur = &fileRemap{deleted: true}
			remaps[oldPath] = cur
			return
		}
		cur = &fileRemap{}
		remaps[newPath] = cur
	}
	for _, line := range strings.Split(diffText, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			cur, oldPath, newPath, inHunk = nil, "", "", false
		case strings.HasPrefix(line, "@@ "):
			inHunk = true // following +/- lines are content, never headers
			if h, ok := parseHunkHeader(line); ok && cur != nil {
				cur.hunks = append(cur.hunks, h)
			}
		case !inHunk && strings.HasPrefix(line, "--- "):
			oldPath = trimDiffSidePath(line)
		case !inHunk && strings.HasPrefix(line, "+++ "):
			newPath = trimDiffSidePath(line)
			beginFile()
		}
		// Everything else (index/mode lines, hunk content,
		// "\ No newline at end of file") is ignored.
	}
	return remaps
}

// parseHunkHeader parses one -U0 hunk header: `@@ -a[,b] +c[,d] @@ optional
// section heading`. git omits the count when it is 1 (`@@ -3 +3 @@`), so a
// missing count means one line; a count of 0 marks a pure insertion (`-a,0`)
// or deletion (`+c,0`) whose number names the line before the change point.
func parseHunkHeader(line string) (hunkRange, bool) {
	fields := strings.Fields(line)
	if len(fields) < 4 || fields[0] != "@@" || fields[3] != "@@" {
		return hunkRange{}, false
	}
	oldStart, oldCount, ok := parseRange(fields[1], '-')
	if !ok {
		return hunkRange{}, false
	}
	_, newCount, ok := parseRange(fields[2], '+')
	if !ok {
		return hunkRange{}, false
	}
	return hunkRange{oldStart: oldStart, oldCount: oldCount, delta: newCount - oldCount}, true
}

// parseRange parses the `-a[,b]` / `+c[,d]` field of a hunk header; sign
// selects '-' or '+'. The count defaults to 1 when omitted. Counts must be
// non-negative (0 is meaningful for pure insertions/deletions); anything else
// is a parse failure.
func parseRange(field string, sign byte) (start, count int, ok bool) {
	if len(field) < 2 || field[0] != sign {
		return 0, 0, false
	}
	nums := field[1:]
	count = 1
	if i := strings.IndexByte(nums, ','); i >= 0 {
		n, err := strconv.Atoi(nums[i+1:])
		if err != nil || n < 0 {
			return 0, 0, false
		}
		count = n
		nums = nums[:i]
	}
	start, err := strconv.Atoi(nums)
	if err != nil || start < 0 {
		return 0, 0, false
	}
	return start, count, true
}

// trimDiffSidePath extracts the repo-relative path from a `--- ` / `+++ `
// header line: it strips the a/ or b/ prefix and any trailing tab. "/dev/null"
// is kept verbatim as the creation/deletion sentinel. git C-quotes paths that
// contain special characters (embedded quotes, control bytes); those are out
// of scope here because callers pass plain repo-relative paths.
func trimDiffSidePath(line string) string {
	p := line[4:] // past the "--- " / "+++ " prefix
	if i := strings.IndexByte(p, '\t'); i >= 0 {
		p = p[:i]
	}
	if p == devNull {
		return p
	}
	if len(p) >= 2 && (p[0] == 'a' || p[0] == 'b') && p[1] == '/' {
		return p[2:]
	}
	return p
}

// gitErrDetail extracts git's stderr from an exec error for error context.
func gitErrDetail(err error) string {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(bytes.TrimSpace(exitErr.Stderr))
	}
	return ""
}
