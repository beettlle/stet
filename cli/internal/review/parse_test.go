package review

import (
	"strings"
	"testing"

	"stet/cli/internal/findings"
)

func TestParseFindingsResponse_validArray(t *testing.T) {
	jsonStr := `[{"file":"pkg/a.go","line":10,"severity":"warning","category":"style","message":"Use tabs"}]`
	list, err := ParseFindingsResponse(jsonStr)
	if err != nil {
		t.Fatalf("ParseFindingsResponse: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	if list[0].File != "pkg/a.go" || list[0].Line != 10 || list[0].Message != "Use tabs" {
		t.Errorf("finding: %+v", list[0])
	}
}

func TestParseFindingsResponse_wrapperObject(t *testing.T) {
	jsonStr := `{"findings":[{"file":"x.go","line":1,"severity":"error","category":"bug","message":"nil deref"}]}`
	list, err := ParseFindingsResponse(jsonStr)
	if err != nil {
		t.Fatalf("ParseFindingsResponse: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	if list[0].Severity != findings.SeverityError || list[0].Category != findings.CategoryBug {
		t.Errorf("finding: %+v", list[0])
	}
}

func TestParseFindingsResponse_emptyArray(t *testing.T) {
	list, err := ParseFindingsResponse("[]")
	if err != nil {
		t.Fatalf("ParseFindingsResponse: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("len(list) = %d, want 0", len(list))
	}
}

func TestParseFindingsResponse_malformed_returnsError(t *testing.T) {
	_, err := ParseFindingsResponse("not json")
	if err == nil {
		t.Fatal("ParseFindingsResponse: want error, got nil")
	}
}

func TestParseFindingsResponse_emptyString_returnsError(t *testing.T) {
	_, err := ParseFindingsResponse("")
	if err == nil {
		t.Fatal("ParseFindingsResponse: want error for empty, got nil")
	}
}

func TestParseFindingsResponse_singleObject(t *testing.T) {
	// Some models return a single finding object instead of an array or {"findings": [...]}.
	jsonStr := `{"file":".gitignore","line":119,"severity":"info","category":"style","confidence":0.95,"message":"Inconsistent indentation","suggestion":"Use consistent indentation throughout."}`
	list, err := ParseFindingsResponse(jsonStr)
	if err != nil {
		t.Fatalf("ParseFindingsResponse: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	f := list[0]
	if f.File != ".gitignore" || f.Line != 119 || f.Message != "Inconsistent indentation" {
		t.Errorf("finding: file=%q line=%d message=%q", f.File, f.Line, f.Message)
	}
	if f.Severity != findings.SeverityInfo || f.Category != findings.CategoryStyle {
		t.Errorf("finding: severity=%q category=%q", f.Severity, f.Category)
	}
	if f.Suggestion != "Use consistent indentation throughout." {
		t.Errorf("finding: suggestion=%q", f.Suggestion)
	}
}

func TestParseFindingsResponse_singleObject_invalid_returnsError(t *testing.T) {
	// Single object that fails Validate (e.g. empty message) returns error.
	jsonStr := `{"file":"a.go","line":1,"severity":"info","category":"style","message":""}`
	list, err := ParseFindingsResponse(jsonStr)
	if err == nil {
		t.Fatal("ParseFindingsResponse: want error for invalid single object, got nil")
	}
	if list != nil {
		t.Errorf("len(list) = %d, want nil on error", len(list))
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("error should mention validation failed: %v", err)
	}
}

func TestAssignFindingIDs_setsIDAndValidates(t *testing.T) {
	list := []findings.Finding{
		{File: "a.go", Line: 5, Severity: findings.SeverityWarning, Category: findings.CategoryStyle, Confidence: 1.0, Message: "msg"},
	}
	out, err := AssignFindingIDs(list, "fallback.go")
	if err != nil {
		t.Fatalf("AssignFindingIDs: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	if out[0].ID == "" {
		t.Error("ID should be set")
	}
	if err := out[0].Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestAssignFindingIDs_emptyFile_usesHunkPath(t *testing.T) {
	list := []findings.Finding{
		{File: "", Line: 1, Severity: findings.SeverityInfo, Category: findings.CategoryStyle, Confidence: 1.0, Message: "x"},
	}
	out, err := AssignFindingIDs(list, "hunk.go")
	if err != nil {
		t.Fatalf("AssignFindingIDs: %v", err)
	}
	// ID should be derived from hunk.go:1:message stem
	if out[0].ID == "" {
		t.Error("ID should be set using hunk path")
	}
}

func TestAssignFindingIDs_sameInput_sameID(t *testing.T) {
	list := []findings.Finding{
		{File: "f.go", Line: 10, Severity: findings.SeverityWarning, Category: findings.CategoryStyle, Confidence: 1.0, Message: "same"},
		{File: "f.go", Line: 10, Severity: findings.SeverityWarning, Category: findings.CategoryStyle, Confidence: 1.0, Message: "same"},
	}
	out, err := AssignFindingIDs(list, "")
	if err != nil {
		t.Fatalf("AssignFindingIDs: %v", err)
	}
	if out[0].ID != out[1].ID {
		t.Errorf("same file/line/message should yield same ID: %q vs %q", out[0].ID, out[1].ID)
	}
}

func TestAssignFindingIDs_differentMessage_differentID(t *testing.T) {
	list := []findings.Finding{
		{File: "f.go", Line: 10, Severity: findings.SeverityWarning, Category: findings.CategoryStyle, Confidence: 1.0, Message: "msg1"},
		{File: "f.go", Line: 10, Severity: findings.SeverityWarning, Category: findings.CategoryStyle, Confidence: 1.0, Message: "msg2"},
	}
	out, err := AssignFindingIDs(list, "")
	if err != nil {
		t.Fatalf("AssignFindingIDs: %v", err)
	}
	if out[0].ID == out[1].ID {
		t.Error("different message should yield different ID")
	}
}

func TestAssignFindingIDs_invalidFinding_returnsError(t *testing.T) {
	list := []findings.Finding{
		{File: "a.go", Line: 1, Severity: "invalid", Category: findings.CategoryBug, Confidence: 1.0, Message: "m"},
	}
	_, err := AssignFindingIDs(list, "")
	if err == nil {
		t.Fatal("AssignFindingIDs: want error for invalid severity, got nil")
	}
}

func TestAssignFindingIDs_withRange(t *testing.T) {
	list := []findings.Finding{
		{File: "a.go", Line: 0, Range: &findings.LineRange{Start: 10, End: 12}, Severity: findings.SeverityWarning, Category: findings.CategoryBug, Confidence: 1.0, Message: "range"},
	}
	out, err := AssignFindingIDs(list, "")
	if err != nil {
		t.Fatalf("AssignFindingIDs: %v", err)
	}
	if out[0].ID == "" {
		t.Error("ID should be set when using range")
	}
}
