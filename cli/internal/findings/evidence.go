// Package findings defines the schema for code review findings. This file
// implements the evidence (hunk lines) post-filter: drop findings whose
// reported line or range falls outside the current diff hunk's line range
// in the new file.

package findings

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
