package findings

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestFindingRoundtripJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		finding Finding
	}{
		{
			name: "minimal",
			finding: Finding{
				File:       "pkg/foo.go",
				Line:       10,
				Severity:   SeverityWarning,
				Category:   CategoryBug,
				Confidence: 1.0,
				Message:    "possible nil dereference",
			},
		},
		{
			name: "with_range",
			finding: Finding{
				ID:         "f1",
				File:       "bar.go",
				Range:      &LineRange{Start: 5, End: 8},
				Severity:   SeverityInfo,
				Category:   CategoryStyle,
				Confidence: 0.5,
				Message:    "formatting",
			},
		},
		{
			name: "with_suggestion_and_cursor_uri",
			finding: Finding{
				ID:         "f2",
				File:       "baz.go",
				Line:       42,
				Severity:   SeverityError,
				Category:   CategorySecurity,
				Confidence: 1.0,
				Message:    "use constant-time compare",
				Suggestion: "use subtle.ConstantTimeCompare",
				CursorURI:  "file:///abs/baz.go#L42",
			},
		},
		{
			name: "all_fields",
			finding: Finding{
				ID:         "f3",
				File:       "internal/x.go",
				Line:       1,
				Range:      &LineRange{Start: 1, End: 1},
				Severity:   SeverityNitpick,
				Category:   CategoryMaintainability,
				Confidence: 0.8,
				Message:    "doc comment",
				Suggestion: "Add package doc",
				CursorURI:  "file:///x.go#L1",
			},
		},
		{
			name: "with_evidence_lines",
			finding: Finding{
				File:          "p.go",
				Line:          5,
				Severity:      SeverityWarning,
				Category:      CategoryBug,
				Confidence:    1.0,
				Message:       "m",
				EvidenceLines: []int{10, 12},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data, err := json.Marshal(tt.finding)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var got Finding
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			roundtripCompare(t, &tt.finding, &got)
		})
	}
}

func roundtripCompare(t *testing.T, a, b *Finding) {
	t.Helper()
	if a.ID != b.ID {
		t.Errorf("ID: got %q want %q", b.ID, a.ID)
	}
	if a.File != b.File {
		t.Errorf("File: got %q want %q", b.File, a.File)
	}
	if a.Line != b.Line {
		t.Errorf("Line: got %d want %d", b.Line, a.Line)
	}
	if (a.Range == nil) != (b.Range == nil) {
		t.Errorf("Range: nil mismatch")
	}
	if a.Range != nil && b.Range != nil && (a.Range.Start != b.Range.Start || a.Range.End != b.Range.End) {
		t.Errorf("Range: got %+v want %+v", b.Range, a.Range)
	}
	if a.Severity != b.Severity {
		t.Errorf("Severity: got %q want %q", b.Severity, a.Severity)
	}
	if a.Category != b.Category {
		t.Errorf("Category: got %q want %q", b.Category, a.Category)
	}
	if a.Confidence != b.Confidence {
		t.Errorf("Confidence: got %g want %g", b.Confidence, a.Confidence)
	}
	if a.Message != b.Message {
		t.Errorf("Message: got %q want %q", b.Message, a.Message)
	}
	if a.Suggestion != b.Suggestion {
		t.Errorf("Suggestion: got %q want %q", b.Suggestion, a.Suggestion)
	}
	if a.CursorURI != b.CursorURI {
		t.Errorf("CursorURI: got %q want %q", b.CursorURI, a.CursorURI)
	}
	if !reflect.DeepEqual(a.EvidenceLines, b.EvidenceLines) {
		t.Errorf("EvidenceLines: got %v want %v", b.EvidenceLines, a.EvidenceLines)
	}
}

func TestFindingValidate(t *testing.T) {
	t.Parallel()
	validFinding := func() Finding {
		return Finding{
			File:       "a.go",
			Line:       1,
			Severity:   SeverityWarning,
			Category:   CategoryBug,
			Confidence: 1.0,
			Message:    "msg",
		}
	}
	tests := []struct {
		name    string
		finding Finding
		wantErr string
	}{
		{"valid", validFinding(), ""},
		{"valid_confidence_zero", func() Finding { f := validFinding(); f.Confidence = 0; return f }(), ""},
		{"valid_confidence_half", func() Finding { f := validFinding(); f.Confidence = 0.5; return f }(), ""},
		{"valid_with_range", Finding{
			File: "a.go", Range: &LineRange{Start: 1, End: 5},
			Severity: SeverityInfo, Category: CategoryStyle, Confidence: 1.0, Message: "m",
		}, ""},
		{"valid_documentation", func() Finding { f := validFinding(); f.Category = CategoryDocumentation; return f }(), ""},
		{"valid_design", func() Finding { f := validFinding(); f.Category = CategoryDesign; return f }(), ""},
		{"nil_finding", Finding{}, "finding is nil"},
		{"missing_severity", func() Finding { f := validFinding(); f.Severity = ""; return f }(), "severity is required"},
		{"missing_category", func() Finding { f := validFinding(); f.Category = ""; return f }(), "category is required"},
		{"invalid_severity", func() Finding { f := validFinding(); f.Severity = "unknown"; return f }(), "invalid severity"},
		{"invalid_category", func() Finding { f := validFinding(); f.Category = "unknown"; return f }(), "invalid category"},
		{"invalid_confidence_negative", func() Finding { f := validFinding(); f.Confidence = -0.1; return f }(), "confidence"},
		{"invalid_confidence_over_one", func() Finding { f := validFinding(); f.Confidence = 1.1; return f }(), "confidence"},
		{"missing_file", func() Finding { f := validFinding(); f.File = ""; return f }(), "file is required"},
		{"missing_message", func() Finding { f := validFinding(); f.Message = ""; return f }(), "message is required"},
		{"valid_file_only", func() Finding {
			f := validFinding()
			f.Line = 0
			f.Range = nil
			return f
		}(), ""},
		{"range_start_gt_end", Finding{
			File: "a.go", Range: &LineRange{Start: 10, End: 3},
			Severity: SeverityWarning, Category: CategoryBug, Confidence: 1.0, Message: "m",
		}, "must be <="},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := tt.finding
			if tt.name == "nil_finding" {
				var nilFinding *Finding
				err := nilFinding.Validate()
				if err == nil {
					t.Fatal("expected error for nil finding")
				}
				if tt.wantErr != "" && !strings.Contains(err.Error(), "nil") {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			err := f.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() = %q, want error containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestFindingValidate_nilReceiver(t *testing.T) {
	t.Parallel()
	var f *Finding
	if err := f.Validate(); err == nil {
		t.Error("Validate() on nil receiver should return error")
	}
}

func TestUnmarshalFinding(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		jsonStr string
		wantErr bool
		check   func(t *testing.T, f *Finding)
	}{
		{
			name:    "valid_minimal",
			jsonStr: `{"file":"x.go","line":5,"severity":"error","category":"security","confidence":1.0,"message":"issue"}`,
			wantErr: false,
			check: func(t *testing.T, f *Finding) {
				t.Helper()
				if f.File != "x.go" || f.Line != 5 || f.Severity != SeverityError || f.Category != CategorySecurity || f.Confidence != 1.0 || f.Message != "issue" {
					t.Errorf("unexpected fields: %+v", f)
				}
			},
		},
		{
			name:    "valid_with_confidence",
			jsonStr: `{"file":"w.go","line":2,"severity":"info","category":"maintainability","confidence":0.8,"message":"check"}`,
			wantErr: false,
			check: func(t *testing.T, f *Finding) {
				t.Helper()
				if f.Confidence != 0.8 {
					t.Errorf("confidence = %g, want 0.8", f.Confidence)
				}
			},
		},
		{
			name:    "valid_with_range",
			jsonStr: `{"file":"y.go","range":{"start":1,"end":10},"severity":"warning","category":"bug","confidence":0.5,"message":"msg"}`,
			wantErr: false,
			check: func(t *testing.T, f *Finding) {
				t.Helper()
				if f.Range == nil || f.Range.Start != 1 || f.Range.End != 10 {
					t.Errorf("unexpected range: %+v", f.Range)
				}
			},
		},
		{
			name:    "invalid_json",
			jsonStr: `{`,
			wantErr: true,
		},
		{
			name:    "invalid_severity_validates",
			jsonStr: `{"file":"z.go","line":1,"severity":"critical","category":"style","confidence":1.0,"message":"m"}`,
			wantErr: false, // unmarshal succeeds; Validate() fails
			check: func(t *testing.T, f *Finding) {
				t.Helper()
				if err := f.Validate(); err == nil {
					t.Error("expected Validate() to fail for invalid severity")
				}
			},
		},
		{
			name:    "valid_with_evidence_lines",
			jsonStr: `{"file":"p.go","line":5,"severity":"warning","category":"bug","confidence":1.0,"message":"m","evidence_lines":[10,12]}`,
			wantErr: false,
			check: func(t *testing.T, f *Finding) {
				t.Helper()
				if len(f.EvidenceLines) != 2 || f.EvidenceLines[0] != 10 || f.EvidenceLines[1] != 12 {
					t.Errorf("evidence_lines = %v, want [10, 12]", f.EvidenceLines)
				}
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var f Finding
			err := json.Unmarshal([]byte(tt.jsonStr), &f)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected unmarshal error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if tt.check != nil {
				tt.check(t, &f)
			}
		})
	}
}

func TestMarshalFinding_omitempty(t *testing.T) {
	t.Parallel()
	f := Finding{
		File:       "a.go",
		Line:       2,
		Severity:   SeverityInfo,
		Category:   CategoryTesting,
		Confidence: 1.0,
		Message:    "test",
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "suggestion") || strings.Contains(s, "cursor_uri") {
		t.Errorf("optional empty fields should be omitted: %s", s)
	}
	if !strings.Contains(s, "line") || !strings.Contains(s, "file") || !strings.Contains(s, "confidence") {
		t.Errorf("required fields present: %s", s)
	}
}

func TestSliceFindingsRoundtrip(t *testing.T) {
	t.Parallel()
	list := []Finding{
		{File: "a.go", Line: 1, Severity: SeverityWarning, Category: CategoryBug, Confidence: 1.0, Message: "one"},
		{File: "b.go", Line: 2, Severity: SeverityError, Category: CategorySecurity, Confidence: 0.9, Message: "two"},
	}
	data, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	var got []Finding
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(list) {
		t.Fatalf("len(got)=%d want %d", len(got), len(list))
	}
	for i := range list {
		roundtripCompare(t, &list[i], &got[i])
	}
}
