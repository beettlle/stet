package findings

import (
	"testing"
)

const evidenceTestPath = "a.go"
const evidenceHunkStart, evidenceHunkEnd = 1, 5

func TestFilterByHunkLines(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		input      []Finding
		filePath   string
		hunkStart  int
		hunkEnd    int
		wantOutput []Finding
	}{
		{
			name:       "h_empty_list",
			input:      nil,
			filePath:   evidenceTestPath,
			hunkStart:  evidenceHunkStart,
			hunkEnd:    evidenceHunkEnd,
			wantOutput: nil,
		},
		{
			name:       "h_empty_slice",
			input:      []Finding{},
			filePath:   evidenceTestPath,
			hunkStart:  evidenceHunkStart,
			hunkEnd:    evidenceHunkEnd,
			wantOutput: nil,
		},
		{
			name: "a_all_inside_hunk_kept",
			input: []Finding{
				{File: evidenceTestPath, Line: 2, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "m1"},
				{File: evidenceTestPath, Line: 4, Severity: SeverityInfo, Category: CategoryStyle, Confidence: 0.85, Message: "m2"},
			},
			filePath:   evidenceTestPath,
			hunkStart:  evidenceHunkStart,
			hunkEnd:    evidenceHunkEnd,
			wantOutput: []Finding{
				{File: evidenceTestPath, Line: 2, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "m1"},
				{File: evidenceTestPath, Line: 4, Severity: SeverityInfo, Category: CategoryStyle, Confidence: 0.85, Message: "m2"},
			},
		},
		{
			name: "b_line_before_hunk_start_dropped",
			input: []Finding{
				{File: evidenceTestPath, Line: 2, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "before"},
			},
			filePath:   evidenceTestPath,
			hunkStart:  5,
			hunkEnd:    10,
			wantOutput: nil,
		},
		{
			name: "c_line_after_hunk_end_dropped",
			input: []Finding{
				{File: evidenceTestPath, Line: 10, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "after"},
			},
			filePath:   evidenceTestPath,
			hunkStart:  evidenceHunkStart,
			hunkEnd:    evidenceHunkEnd,
			wantOutput: nil,
		},
		{
			name: "d_range_entirely_outside_dropped",
			input: []Finding{
				{File: evidenceTestPath, Line: 10, Range: &LineRange{Start: 10, End: 12}, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "outside"},
			},
			filePath:   evidenceTestPath,
			hunkStart:  evidenceHunkStart,
			hunkEnd:    evidenceHunkEnd,
			wantOutput: nil,
		},
		{
			name: "e_range_overlapping_hunk_kept",
			input: []Finding{
				{File: evidenceTestPath, Line: 3, Range: &LineRange{Start: 3, End: 7}, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "overlap"},
			},
			filePath:   evidenceTestPath,
			hunkStart:  evidenceHunkStart,
			hunkEnd:    evidenceHunkEnd,
			wantOutput: []Finding{
				{File: evidenceTestPath, Line: 3, Range: &LineRange{Start: 3, End: 7}, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "overlap"},
			},
		},
		{
			name: "e2_range_overlap_low_end_zero_start_kept",
			input: []Finding{
				{File: evidenceTestPath, Line: 1, Range: &LineRange{Start: 0, End: 2}, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "overlap low"},
			},
			filePath:   evidenceTestPath,
			hunkStart:  evidenceHunkStart,
			hunkEnd:    evidenceHunkEnd,
			wantOutput: []Finding{
				{File: evidenceTestPath, Line: 1, Range: &LineRange{Start: 0, End: 2}, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "overlap low"},
			},
		},
		{
			name: "f_file_only_line_zero_no_range_kept",
			input: []Finding{
				{File: evidenceTestPath, Line: 0, Severity: SeverityInfo, Category: CategoryDocumentation, Confidence: 0.9, Message: "file-only"},
			},
			filePath:   evidenceTestPath,
			hunkStart:  evidenceHunkStart,
			hunkEnd:    evidenceHunkEnd,
			wantOutput: []Finding{
				{File: evidenceTestPath, Line: 0, Severity: SeverityInfo, Category: CategoryDocumentation, Confidence: 0.9, Message: "file-only"},
			},
		},
		{
			name: "g_invalid_hunk_range_list_unchanged",
			input: []Finding{
				{File: evidenceTestPath, Line: 99, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "would drop"},
			},
			filePath:   evidenceTestPath,
			hunkStart:  0,
			hunkEnd:    0,
			wantOutput: []Finding{
				{File: evidenceTestPath, Line: 99, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "would drop"},
			},
		},
		{
			name: "g_invalid_hunk_end_less_than_start_unchanged",
			input: []Finding{
				{File: evidenceTestPath, Line: 1, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "keep"},
			},
			filePath:   evidenceTestPath,
			hunkStart:  5,
			hunkEnd:    1,
			wantOutput: []Finding{
				{File: evidenceTestPath, Line: 1, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "keep"},
			},
		},
		{
			name: "i_range_start_gt_end_dropped",
			input: []Finding{
				{File: evidenceTestPath, Line: 3, Range: &LineRange{Start: 5, End: 3}, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "invalid range"},
			},
			filePath:   evidenceTestPath,
			hunkStart:  evidenceHunkStart,
			hunkEnd:    evidenceHunkEnd,
			wantOutput: nil,
		},
		{
			name: "j_file_mismatch_kept",
			input: []Finding{
				{File: "other.go", Line: 100, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "different file"},
			},
			filePath:   evidenceTestPath,
			hunkStart:  evidenceHunkStart,
			hunkEnd:    evidenceHunkEnd,
			wantOutput: []Finding{
				{File: "other.go", Line: 100, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "different file"},
			},
		},
		{
			name: "boundary_line_one_kept",
			input: []Finding{
				{File: evidenceTestPath, Line: 1, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "line 1"},
			},
			filePath:   evidenceTestPath,
			hunkStart:  1,
			hunkEnd:    5,
			wantOutput: []Finding{
				{File: evidenceTestPath, Line: 1, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "line 1"},
			},
		},
		{
			name: "boundary_line_five_kept",
			input: []Finding{
				{File: evidenceTestPath, Line: 5, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "line 5"},
			},
			filePath:   evidenceTestPath,
			hunkStart:  1,
			hunkEnd:    5,
			wantOutput: []Finding{
				{File: evidenceTestPath, Line: 5, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "line 5"},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FilterByHunkLines(tt.input, tt.filePath, tt.hunkStart, tt.hunkEnd)
			if len(got) != len(tt.wantOutput) {
				t.Errorf("FilterByHunkLines: got %d findings, want %d", len(got), len(tt.wantOutput))
				return
			}
			for i := range got {
				if got[i].File != tt.wantOutput[i].File || got[i].Line != tt.wantOutput[i].Line ||
					got[i].Message != tt.wantOutput[i].Message {
					t.Errorf("finding[%d]: got File=%q Line=%d Message=%q, want File=%q Line=%d Message=%q",
						i, got[i].File, got[i].Line, got[i].Message,
						tt.wantOutput[i].File, tt.wantOutput[i].Line, tt.wantOutput[i].Message)
				}
				if (got[i].Range != nil) != (tt.wantOutput[i].Range != nil) {
					t.Errorf("finding[%d]: Range presence mismatch", i)
				}
				if got[i].Range != nil && tt.wantOutput[i].Range != nil &&
					(got[i].Range.Start != tt.wantOutput[i].Range.Start || got[i].Range.End != tt.wantOutput[i].Range.End) {
					t.Errorf("finding[%d]: Range got %+v, want %+v", i, *got[i].Range, *tt.wantOutput[i].Range)
				}
			}
		})
	}
}
