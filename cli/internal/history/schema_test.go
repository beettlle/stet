package history

import (
	"encoding/json"
	"testing"

	"stet/cli/internal/findings"
)

func TestRecord_marshalUnmarshal_withoutDismissals(t *testing.T) {
	rec := Record{
		DiffRef: "HEAD~1",
		ReviewOutput: []findings.Finding{
			{ID: "f1", File: "a.go", Line: 10, Severity: findings.SeverityWarning, Category: findings.CategoryStyle, Message: "nit"},
		},
		UserAction: UserAction{
			DismissedIDs: []string{"f1"},
		},
	}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Record
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.DiffRef != rec.DiffRef {
		t.Errorf("diff_ref: got %q, want %q", decoded.DiffRef, rec.DiffRef)
	}
	if len(decoded.ReviewOutput) != 1 || decoded.ReviewOutput[0].ID != "f1" {
		t.Errorf("review_output: got %+v", decoded.ReviewOutput)
	}
	if len(decoded.UserAction.DismissedIDs) != 1 || decoded.UserAction.DismissedIDs[0] != "f1" {
		t.Errorf("dismissed_ids: got %v", decoded.UserAction.DismissedIDs)
	}
	if len(decoded.UserAction.Dismissals) != 0 {
		t.Errorf("dismissals should be empty; got %v", decoded.UserAction.Dismissals)
	}
}

func TestRecord_marshalUnmarshal_withDismissalReasons(t *testing.T) {
	rec := Record{
		DiffRef:     "main",
		ReviewOutput: []findings.Finding{},
		UserAction: UserAction{
			DismissedIDs: []string{"f1", "f2"},
			Dismissals: []Dismissal{
				{FindingID: "f1", Reason: ReasonFalsePositive},
				{FindingID: "f2", Reason: ReasonAlreadyCorrect},
			},
			FinishedAt: "2025-02-12T12:00:00Z",
		},
	}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Record
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.DiffRef != "main" {
		t.Errorf("diff_ref: got %q", decoded.DiffRef)
	}
	if len(decoded.UserAction.Dismissals) != 2 {
		t.Fatalf("dismissals: got %d, want 2", len(decoded.UserAction.Dismissals))
	}
	if decoded.UserAction.Dismissals[0].FindingID != "f1" || decoded.UserAction.Dismissals[0].Reason != ReasonFalsePositive {
		t.Errorf("dismissals[0]: got %+v", decoded.UserAction.Dismissals[0])
	}
	if decoded.UserAction.Dismissals[1].FindingID != "f2" || decoded.UserAction.Dismissals[1].Reason != ReasonAlreadyCorrect {
		t.Errorf("dismissals[1]: got %+v", decoded.UserAction.Dismissals[1])
	}
	if decoded.UserAction.FinishedAt != "2025-02-12T12:00:00Z" {
		t.Errorf("finished_at: got %q", decoded.UserAction.FinishedAt)
	}
}

func TestRecord_jsonShape_snake_case(t *testing.T) {
	rec := Record{
		DiffRef:     "abc",
		ReviewOutput: nil,
		UserAction:  UserAction{},
	}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	m := make(map[string]interface{})
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	if _, ok := m["diff_ref"]; !ok {
		t.Errorf("JSON should have diff_ref")
	}
	if _, ok := m["review_output"]; !ok {
		t.Errorf("JSON should have review_output")
	}
	if _, ok := m["user_action"]; !ok {
		t.Errorf("JSON should have user_action")
	}
	ua, _ := m["user_action"].(map[string]interface{})
	if ua != nil {
		if _, ok := ua["dismissed_ids"]; !ok {
			// omitempty may omit it when empty; that's fine
		}
	}
}

func TestValidReason(t *testing.T) {
	for _, c := range []struct {
		s    string
		want bool
	}{
		{ReasonFalsePositive, true},
		{ReasonAlreadyCorrect, true},
		{ReasonWrongSuggestion, true},
		{ReasonOutOfScope, true},
		{"", false},
		{"invalid", false},
		{"FALSE_POSITIVE", false},
	} {
		if got := ValidReason(c.s); got != c.want {
			t.Errorf("ValidReason(%q) = %v, want %v", c.s, got, c.want)
		}
	}
}
