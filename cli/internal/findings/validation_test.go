package findings

import (
	"testing"
)

func TestFindingNormalize_validUnchanged(t *testing.T) {
	t.Parallel()
	f := Finding{
		File:       "a.go",
		Line:       1,
		Severity:   SeverityWarning,
		Category:   CategoryBug,
		Confidence: 1.0,
		Message:    "msg",
	}
	f.Normalize()
	if f.Severity != SeverityWarning || f.Category != CategoryBug {
		t.Errorf("valid finding changed: severity=%q category=%q", f.Severity, f.Category)
	}
}

func TestFindingNormalize_invalidSeverity_coercedToWarning(t *testing.T) {
	t.Parallel()
	f := Finding{
		File:       "a.go",
		Line:       1,
		Severity:   "critical",
		Category:   CategoryStyle,
		Confidence: 1.0,
		Message:    "msg",
	}
	f.Normalize()
	if f.Severity != SeverityWarning {
		t.Errorf("severity = %q, want %q", f.Severity, SeverityWarning)
	}
	if f.Category != CategoryStyle {
		t.Errorf("category should be unchanged: got %q", f.Category)
	}
	if err := f.Validate(); err != nil {
		t.Errorf("after Normalize, Validate should pass: %v", err)
	}
}

func TestFindingNormalize_invalidCategory_coercedToBug(t *testing.T) {
	t.Parallel()
	f := Finding{
		File:       "b.go",
		Line:       2,
		Severity:   SeverityInfo,
		Category:   "typo",
		Confidence: 0.9,
		Message:    "fix",
	}
	f.Normalize()
	if f.Category != CategoryBug {
		t.Errorf("category = %q, want %q", f.Category, CategoryBug)
	}
	if f.Severity != SeverityInfo {
		t.Errorf("severity should be unchanged: got %q", f.Severity)
	}
	if err := f.Validate(); err != nil {
		t.Errorf("after Normalize, Validate should pass: %v", err)
	}
}

func TestFindingNormalize_nilNoop(t *testing.T) {
	t.Parallel()
	var f *Finding
	f.Normalize() // must not panic
}
