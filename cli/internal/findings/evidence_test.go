package findings

import (
	"reflect"
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
			filePath:  evidenceTestPath,
			hunkStart: evidenceHunkStart,
			hunkEnd:   evidenceHunkEnd,
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
			filePath:  evidenceTestPath,
			hunkStart: evidenceHunkStart,
			hunkEnd:   evidenceHunkEnd,
			wantOutput: []Finding{
				{File: evidenceTestPath, Line: 3, Range: &LineRange{Start: 3, End: 7}, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "overlap"},
			},
		},
		{
			name: "e2_range_overlap_low_end_zero_start_kept",
			input: []Finding{
				{File: evidenceTestPath, Line: 1, Range: &LineRange{Start: 0, End: 2}, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "overlap low"},
			},
			filePath:  evidenceTestPath,
			hunkStart: evidenceHunkStart,
			hunkEnd:   evidenceHunkEnd,
			wantOutput: []Finding{
				{File: evidenceTestPath, Line: 1, Range: &LineRange{Start: 0, End: 2}, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "overlap low"},
			},
		},
		{
			name: "f_file_only_line_zero_no_range_kept",
			input: []Finding{
				{File: evidenceTestPath, Line: 0, Severity: SeverityInfo, Category: CategoryDocumentation, Confidence: 0.9, Message: "file-only"},
			},
			filePath:  evidenceTestPath,
			hunkStart: evidenceHunkStart,
			hunkEnd:   evidenceHunkEnd,
			wantOutput: []Finding{
				{File: evidenceTestPath, Line: 0, Severity: SeverityInfo, Category: CategoryDocumentation, Confidence: 0.9, Message: "file-only"},
			},
		},
		{
			name: "g_invalid_hunk_range_list_unchanged",
			input: []Finding{
				{File: evidenceTestPath, Line: 99, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "would drop"},
			},
			filePath:  evidenceTestPath,
			hunkStart: 0,
			hunkEnd:   0,
			wantOutput: []Finding{
				{File: evidenceTestPath, Line: 99, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "would drop"},
			},
		},
		{
			name: "g_invalid_hunk_end_less_than_start_unchanged",
			input: []Finding{
				{File: evidenceTestPath, Line: 1, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "keep"},
			},
			filePath:  evidenceTestPath,
			hunkStart: 5,
			hunkEnd:   1,
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
			filePath:  evidenceTestPath,
			hunkStart: evidenceHunkStart,
			hunkEnd:   evidenceHunkEnd,
			wantOutput: []Finding{
				{File: "other.go", Line: 100, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "different file"},
			},
		},
		{
			name: "boundary_line_one_kept",
			input: []Finding{
				{File: evidenceTestPath, Line: 1, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "line 1"},
			},
			filePath:  evidenceTestPath,
			hunkStart: 1,
			hunkEnd:   5,
			wantOutput: []Finding{
				{File: evidenceTestPath, Line: 1, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "line 1"},
			},
		},
		{
			name: "boundary_line_five_kept",
			input: []Finding{
				{File: evidenceTestPath, Line: 5, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "line 5"},
			},
			filePath:  evidenceTestPath,
			hunkStart: 1,
			hunkEnd:   5,
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

func TestAddedLines(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want []int
	}{
		{
			// New-file lines: 10 ctx, 11 +, (old - skipped), 12 ctx, 13 +, 14 +.
			name: "mixed_body",
			raw:  "@@ -10,4 +10,5 @@ func f() {\n ctx\n+add1\n-del\n ctx2\n+add2\n+add3\n",
			want: []int{11, 13, 14},
		},
		{
			name: "no_newline_marker_ignored",
			raw:  "@@ -1,2 +1,2 @@\n a\n-b\n\\ No newline at end of file\n+c\n\\ No newline at end of file\n",
			want: []int{2},
		},
		{
			name: "missing_count_in_header",
			raw:  "@@ -1 +1 @@\n-old\n+new\n",
			want: []int{1},
		},
		{
			name: "pure_deletion",
			raw:  "@@ -5,3 +5,1 @@\n ctx\n-a\n-b\n",
			want: nil,
		},
		{
			name: "unparsable_header",
			raw:  "not a header\n+added\n",
			want: nil,
		},
		{
			name: "empty_line_counts_as_context",
			raw:  "@@ -1,3 +1,4 @@\n a\n\n b\n+c\n",
			want: []int{4},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := AddedLines(tt.raw); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AddedLines() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterByAddedLines(t *testing.T) {
	t.Parallel()
	const hunkStart, hunkEnd = 1, 20
	added := []int{10}
	line := func(n int, msg string) Finding {
		return Finding{File: evidenceTestPath, Line: n, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: msg}
	}
	rng := func(start, end int, msg string) Finding {
		f := line(start, msg)
		f.Range = &LineRange{Start: start, End: end}
		return f
	}
	tests := []struct {
		name    string
		input   []Finding
		added   []int
		wantMsg []string
	}{
		{name: "empty_list", input: nil, added: added, wantMsg: nil},
		{name: "on_added_line_kept", input: []Finding{line(10, "on")}, added: added, wantMsg: []string{"on"}},
		{name: "exactly_3_above_kept", input: []Finding{line(7, "above3")}, added: added, wantMsg: []string{"above3"}},
		{name: "exactly_3_below_kept", input: []Finding{line(13, "below3")}, added: added, wantMsg: []string{"below3"}},
		{name: "context_4_above_dropped", input: []Finding{line(6, "above4")}, added: added, wantMsg: nil},
		{name: "context_4_below_dropped", input: []Finding{line(14, "below4")}, added: added, wantMsg: nil},
		{name: "range_overlapping_window_kept", input: []Finding{rng(1, 7, "r")}, added: added, wantMsg: []string{"r"}},
		{name: "range_spanning_added_kept", input: []Finding{rng(2, 18, "span")}, added: added, wantMsg: []string{"span"}},
		{name: "range_outside_window_dropped", input: []Finding{rng(14, 18, "far")}, added: added, wantMsg: nil},
		{name: "range_invalid_dropped", input: []Finding{rng(11, 9, "bad")}, added: added, wantMsg: nil},
		{
			name:    "file_mismatch_kept",
			input:   []Finding{{File: "other.go", Line: 1, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "other"}},
			added:   added,
			wantMsg: []string{"other"},
		},
		{name: "file_only_kept", input: []Finding{line(0, "fileonly")}, added: added, wantMsg: []string{"fileonly"}},
		{
			name:    "multiple_added_lines_order_preserved",
			input:   []Finding{line(19, "nearSecond"), line(14, "gap"), line(2, "nearFirst")},
			added:   []int{4, 18},
			wantMsg: []string{"nearSecond", "nearFirst"},
		},
		{
			// Pure deletion: no added lines, so the whole-hunk rule keeps line 2 and drops line 25.
			name:    "pure_deletion_falls_back_to_hunk_range",
			input:   []Finding{line(2, "inHunk"), line(25, "outside")},
			added:   nil,
			wantMsg: []string{"inHunk"},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FilterByAddedLines(tt.input, evidenceTestPath, tt.added, hunkStart, hunkEnd)
			var gotMsg []string
			for _, f := range got {
				gotMsg = append(gotMsg, f.Message)
			}
			if !reflect.DeepEqual(gotMsg, tt.wantMsg) {
				t.Errorf("FilterByAddedLines() messages = %v, want %v", gotMsg, tt.wantMsg)
			}
		})
	}
}

func TestFilterByAddedLines_doesNotMutateInput(t *testing.T) {
	t.Parallel()
	input := []Finding{
		{File: evidenceTestPath, Line: 1, Message: "far"},
		{File: evidenceTestPath, Line: 10, Message: "near"},
	}
	got := FilterByAddedLines(input, evidenceTestPath, []int{10}, 1, 20)
	if len(got) != 1 || got[0].Message != "near" {
		t.Errorf("FilterByAddedLines() = %+v, want only \"near\"", got)
	}
	if input[0].Message != "far" || input[1].Message != "near" || len(input) != 2 {
		t.Errorf("input mutated: %+v", input)
	}
}
