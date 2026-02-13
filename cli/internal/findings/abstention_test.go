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
			got := FilterAbstention(tt.input, 0.8, 0.9)
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

func TestFilterAbstention_strictThresholds(t *testing.T) {
	t.Parallel()
	// strict: 0.6 keep, 0.7 maintainability
	tests := []struct {
		name   string
		input  []Finding
		output []Finding
	}{
		{
			name: "confidence_065_kept",
			input: []Finding{
				{File: "a.go", Line: 1, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.65, Message: "borderline"},
			},
			output: []Finding{
				{File: "a.go", Line: 1, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.65, Message: "borderline"},
			},
		},
		{
			name: "confidence_055_dropped",
			input: []Finding{
				{File: "a.go", Line: 1, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.55, Message: "low"},
			},
			output: nil,
		},
		{
			name: "maintainability_075_kept",
			input: []Finding{
				{File: "b.go", Line: 2, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 0.75, Message: "doc"},
			},
			output: []Finding{
				{File: "b.go", Line: 2, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 0.75, Message: "doc"},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FilterAbstention(tt.input, 0.6, 0.7)
			if len(got) != len(tt.output) {
				t.Errorf("FilterAbstention(0.6, 0.7): got %d findings, want %d", len(got), len(tt.output))
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

func TestFilterAbstention_lenientThresholds(t *testing.T) {
	t.Parallel()
	// lenient: 0.9 keep, 0.95 maintainability
	tests := []struct {
		name   string
		input  []Finding
		output []Finding
	}{
		{
			name: "confidence_085_dropped",
			input: []Finding{
				{File: "a.go", Line: 1, Severity: SeverityWarning, Category: CategorySecurity, Confidence: 0.85, Message: "borderline"},
			},
			output: nil,
		},
		{
			name: "maintainability_096_kept",
			input: []Finding{
				{File: "b.go", Line: 2, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 0.96, Message: "doc"},
			},
			output: []Finding{
				{File: "b.go", Line: 2, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 0.96, Message: "doc"},
			},
		},
		{
			name: "maintainability_094_dropped",
			input: []Finding{
				{File: "c.go", Line: 3, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 0.94, Message: "nit"},
			},
			output: nil,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FilterAbstention(tt.input, 0.9, 0.95)
			if len(got) != len(tt.output) {
				t.Errorf("FilterAbstention(0.9, 0.95): got %d findings, want %d", len(got), len(tt.output))
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
