package findings

import (
	"testing"
)

func TestFilterFPKillList(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  []Finding
		output []Finding
	}{
		{
			name:   "empty_list",
			input:  nil,
			output: nil,
		},
		{
			name:   "empty_slice",
			input:  []Finding{},
			output: nil,
		},
		{
			name: "consider_adding_comments_exact_dropped",
			input: []Finding{
				{File: "a.go", Line: 1, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 0.95, Message: "Consider adding comments"},
			},
			output: nil,
		},
		{
			name: "consider_adding_comments_with_context_dropped",
			input: []Finding{
				{File: "a.go", Line: 1, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 0.95, Message: "Consider adding comments for this function."},
			},
			output: nil,
		},
		{
			name: "ensure_that_dropped",
			input: []Finding{
				{File: "b.go", Line: 2, Severity: SeverityWarning, Category: CategoryBestPractice, Confidence: 0.9, Message: "Ensure that... input is validated."},
			},
			output: nil,
		},
		{
			name: "it_might_be_beneficial_dropped",
			input: []Finding{
				{File: "c.go", Line: 3, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 0.9, Message: "It might be beneficial to refactor here."},
			},
			output: nil,
		},
		{
			name: "non_banned_message_kept",
			input: []Finding{
				{File: "d.go", Line: 4, Severity: SeverityError, Category: CategorySecurity, Confidence: 0.9, Message: "Possible null dereference on line 10."},
			},
			output: []Finding{
				{File: "d.go", Line: 4, Severity: SeverityError, Category: CategorySecurity, Confidence: 0.9, Message: "Possible null dereference on line 10."},
			},
		},
		{
			name: "case_insensitive_dropped",
			input: []Finding{
				{File: "e.go", Line: 5, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 1.0, Message: "CONSIDER ADDING COMMENTS"},
			},
			output: nil,
		},
		{
			name: "dry_run_placeholder_not_matched_kept",
			input: []Finding{
				{File: "f.go", Line: 1, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 1.0, Message: "Dry-run placeholder (CI)"},
			},
			output: []Finding{
				{File: "f.go", Line: 1, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 1.0, Message: "Dry-run placeholder (CI)"},
			},
		},
		{
			name: "mixed_some_dropped_some_kept",
			input: []Finding{
				{File: "x.go", Line: 1, Severity: SeverityWarning, Category: CategoryCorrectness, Confidence: 0.9, Message: "Keep: potential off-by-one"},
				{File: "x.go", Line: 2, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 0.95, Message: "Consider adding comments"},
				{File: "x.go", Line: 3, Severity: SeverityError, Category: CategorySecurity, Confidence: 0.9, Message: "SQL injection risk"},
				{File: "x.go", Line: 4, Severity: SeverityInfo, Category: CategoryBestPractice, Confidence: 0.9, Message: "You might want to use a constant"},
			},
			output: []Finding{
				{File: "x.go", Line: 1, Severity: SeverityWarning, Category: CategoryCorrectness, Confidence: 0.9, Message: "Keep: potential off-by-one"},
				{File: "x.go", Line: 3, Severity: SeverityError, Category: CategorySecurity, Confidence: 0.9, Message: "SQL injection risk"},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FilterFPKillList(tt.input)
			if len(got) != len(tt.output) {
				t.Errorf("FilterFPKillList: got %d findings, want %d", len(got), len(tt.output))
				return
			}
			for i := range got {
				if got[i].File != tt.output[i].File || got[i].Line != tt.output[i].Line ||
					got[i].Message != tt.output[i].Message {
					t.Errorf("finding[%d]: got %+v, want %+v", i, got[i], tt.output[i])
				}
			}
		})
	}
}
