package findings

import (
	"testing"
)

func TestFilterAbstention(t *testing.T) {
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
			name: "confidence_below_08_dropped",
			input: []Finding{
				{File: "a.go", Line: 1, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.7, Message: "low"},
			},
			output: nil,
		},
		{
			name: "confidence_08_kept_non_maintainability",
			input: []Finding{
				{File: "a.go", Line: 1, Severity: SeverityWarning, Category: CategorySecurity, Confidence: 0.8, Message: "ok"},
			},
			output: []Finding{
				{File: "a.go", Line: 1, Severity: SeverityWarning, Category: CategorySecurity, Confidence: 0.8, Message: "ok"},
			},
		},
		{
			name: "maintainability_085_dropped",
			input: []Finding{
				{File: "b.go", Line: 2, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 0.85, Message: "add comments"},
			},
			output: nil,
		},
		{
			name: "maintainability_09_kept",
			input: []Finding{
				{File: "c.go", Line: 3, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 0.9, Message: "doc"},
			},
			output: []Finding{
				{File: "c.go", Line: 3, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 0.9, Message: "doc"},
			},
		},
		{
			name: "security_09_kept",
			input: []Finding{
				{File: "d.go", Line: 4, Severity: SeverityError, Category: CategorySecurity, Confidence: 0.9, Message: "injection"},
			},
			output: []Finding{
				{File: "d.go", Line: 4, Severity: SeverityError, Category: CategorySecurity, Confidence: 0.9, Message: "injection"},
			},
		},
		{
			name: "mixed_some_kept_some_dropped",
			input: []Finding{
				{File: "x.go", Line: 1, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.7, Message: "drop low"},
				{File: "x.go", Line: 2, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 0.85, Message: "drop maint"},
				{File: "x.go", Line: 3, Severity: SeverityWarning, Category: CategoryCorrectness, Confidence: 0.85, Message: "keep"},
				{File: "x.go", Line: 4, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 0.95, Message: "keep maint"},
			},
			output: []Finding{
				{File: "x.go", Line: 3, Severity: SeverityWarning, Category: CategoryCorrectness, Confidence: 0.85, Message: "keep"},
				{File: "x.go", Line: 4, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 0.95, Message: "keep maint"},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FilterAbstention(tt.input)
			if len(got) != len(tt.output) {
				t.Errorf("FilterAbstention: got %d findings, want %d", len(got), len(tt.output))
				return
			}
			for i := range got {
				if got[i].File != tt.output[i].File || got[i].Line != tt.output[i].Line ||
					got[i].Category != tt.output[i].Category || got[i].Confidence != tt.output[i].Confidence ||
					got[i].Message != tt.output[i].Message {
					t.Errorf("finding[%d]: got %+v, want %+v", i, got[i], tt.output[i])
				}
			}
		})
	}
}
