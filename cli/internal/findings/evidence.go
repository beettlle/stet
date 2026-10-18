// Package findings defines the schema for code review findings. This file
// implements the evidence post-filters: FilterByAddedLines keeps findings on
// or near the hunk's added lines; FilterByHunkLines keeps findings within the
// hunk's whole new-file line range (used as the pure-deletion fallback).

package findings

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// addedLineWindow is how many new-file lines away from an added line a
// finding may sit and still count as evidence-backed.
const addedLineWindow = 3

// addedLinesHeaderRegex matches the unified diff hunk header and captures the
// new-file start line (same shape as expand.hunkHeaderRegex).
var addedLinesHeaderRegex = regexp.MustCompile(`@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// AddedLines returns the 1-based new-file line numbers of the added ("+")
// lines in a single hunk's raw content, in ascending order. The first line of
// rawContent must be the "@@ -a,b +c,d @@" header; nil is returned when it
// cannot be parsed or the hunk adds no lines. Context (" ") lines advance the
// new-file counter, removed ("-") lines do not, and "\ No newline at end of
// file" markers are ignored. Empty body lines are treated as context lines
// whose leading space was stripped.
func AddedLines(rawContent string) []int {
	lines := strings.Split(rawContent, "\n")
	matches := addedLinesHeaderRegex.FindStringSubmatch(lines[0])
	if matches == nil {
		return nil
	}
	newLine, err := strconv.Atoi(matches[1])
	if err != nil || newLine <= 0 {
		return nil
	}
	var added []int
	for _, l := range lines[1:] {
		switch {
		case strings.HasPrefix(l, "+"):
			added = append(added, newLine)
			newLine++
		case strings.HasPrefix(l, "-"), strings.HasPrefix(l, "\\"):
			// Removed lines and "\ No newline at end of file" don't exist in the new file.
		default:
			newLine++
		}
	}
	return added
}

// FilterByAddedLines keeps findings on or within addedLineWindow lines of an
// added line. added holds ascending 1-based new-file line numbers (see
// AddedLines); hunkStart and hunkEnd are the hunk's new-file range from
// expand.HunkLineRange. Returns a new slice; input is not modified. Order of
// kept findings is preserved.
//
// Rules: File mismatch (finding.File != filePath) → keep. File-only
// (Line == 0 and no Range) → keep. Range invalid (Start > End) → drop. Line or
// valid Range: keep iff some added line a satisfies
// a-addedLineWindow <= end && a+addedLineWindow >= start. When added is empty
// (pure-deletion hunk) the whole-hunk rule of FilterByHunkLines applies.
func FilterByAddedLines(list []Finding, filePath string, added []int, hunkStart, hunkEnd int) []Finding {
	if len(added) == 0 {
		return FilterByHunkLines(list, filePath, hunkStart, hunkEnd)
	}
	if len(list) == 0 {
		return nil
	}
	out := make([]Finding, 0, len(list))
	for _, f := range list {
		if f.File != filePath || (f.Line == 0 && f.Range == nil) {
			out = append(out, f)
			continue
		}
		start, end := f.Line, f.Line
		if f.Range != nil {
			if f.Range.Start > f.Range.End {
				continue
			}
			start, end = f.Range.Start, f.Range.End
		}
		// First added line that is not too far above the finding's start.
		i := sort.SearchInts(added, start-addedLineWindow)
		if i < len(added) && added[i] <= end+addedLineWindow {
			out = append(out, f)
		}
	}
	return out
}

// FilterByHunkLines drops findings whose reported location (Line or Range)
// does not fall within the hunk's line range in the new file. Caller passes
// filePath and 1-based inclusive hunkStart, hunkEnd from expand.HunkLineRange.
// If hunk range is invalid (hunkStart <= 0 or hunkEnd < hunkStart), the list
// is returned unchanged. Returns a new slice; input is not modified. Order
// of kept findings is preserved.
//
// Rules: File mismatch (finding.File != filePath) → keep. File-only
// (Line == 0 and no Range) → keep. Line only: keep iff in [hunkStart, hunkEnd].
// Range: invalid (Start > End) → drop; else keep iff range overlaps hunk
// (overlap: finding.Range.Start <= hunkEnd && finding.Range.End >= hunkStart).
// Ranges are inclusive; touching at a single line (e.g. finding.End == hunkStart) counts as overlap.
func FilterByHunkLines(list []Finding, filePath string, hunkStart, hunkEnd int) []Finding {
	if len(list) == 0 {
		return nil
	}
	if hunkStart <= 0 || hunkEnd < hunkStart {
		return list
	}
	out := make([]Finding, 0, len(list))
	for _, f := range list {
		if f.File != filePath {
			out = append(out, f)
			continue
		}
		if f.Line == 0 && f.Range == nil {
			out = append(out, f)
			continue
		}
		if f.Range != nil {
			if f.Range.Start > f.Range.End {
				continue
			}
			if f.Range.Start <= hunkEnd && f.Range.End >= hunkStart {
				out = append(out, f)
			}
			continue
		}
		if f.Line >= hunkStart && f.Line <= hunkEnd {
			out = append(out, f)
		}
	}
	return out
}
