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
			{ID: "f1", File: "a.go", Line: 10, Severity: findings.SeverityWarning, Category: findings.CategoryStyle, Confidence: 1.0, Message: "nit"},
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

func TestRecord_marshalUnmarshal_withoutUsage(t *testing.T) {
	rec := Record{
		DiffRef:      "HEAD",
		ReviewOutput: nil,
		UserAction:   UserAction{},
	}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Record
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Usage != nil {
		t.Errorf("Usage: got %+v, want nil", decoded.Usage)
	}
	if decoded.PromptTokens != nil || decoded.CompletionTokens != nil || decoded.EvalDurationNs != nil {
		t.Errorf("top-level usage fields should be nil; got %v %v %v", decoded.PromptTokens, decoded.CompletionTokens, decoded.EvalDurationNs)
	}
}

func TestRecord_marshalUnmarshal_withUsage(t *testing.T) {
	pt := int64(1000)
	ct := int64(200)
	ed := int64(5000000000)
	rec := Record{
		DiffRef:      "HEAD",
		ReviewOutput: nil,
		UserAction:   UserAction{},
		Usage: &Usage{
			PromptTokens:     &pt,
			CompletionTokens: &ct,
			EvalDurationNs:   &ed,
			Model:            "qwen2.5-coder:32b",
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
	if decoded.Usage == nil {
		t.Fatal("Usage: got nil, want non-nil")
	}
	if decoded.Usage.PromptTokens == nil || *decoded.Usage.PromptTokens != 1000 {
		t.Errorf("Usage.PromptTokens: got %v, want 1000", decoded.Usage.PromptTokens)
	}
	if decoded.Usage.CompletionTokens == nil || *decoded.Usage.CompletionTokens != 200 {
		t.Errorf("Usage.CompletionTokens: got %v, want 200", decoded.Usage.CompletionTokens)
	}
	if decoded.Usage.EvalDurationNs == nil || *decoded.Usage.EvalDurationNs != 5000000000 {
		t.Errorf("Usage.EvalDurationNs: got %v, want 5000000000", decoded.Usage.EvalDurationNs)
	}
	if decoded.Usage.Model != "qwen2.5-coder:32b" {
		t.Errorf("Usage.Model: got %q, want qwen2.5-coder:32b", decoded.Usage.Model)
	}
}

func TestRecord_marshalUnmarshal_withRunConfigAndUsage(t *testing.T) {
	pt := int64(1000)
	ct := int64(200)
	ed := int64(5000000000)
	rec := Record{
		DiffRef:          "HEAD",
		ReviewOutput:     nil,
		UserAction:       UserAction{},
		RunConfig:        NewRunConfigSnapshot("model", "default", 10, 0, false),
		PromptTokens:     &pt,
		CompletionTokens: &ct,
		EvalDurationNs:   &ed,
	}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Record
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.RunConfig == nil || decoded.RunConfig.Model != "model" || decoded.RunConfig.Strictness != "default" {
		t.Errorf("run_config: got %+v", decoded.RunConfig)
	}
	if decoded.PromptTokens == nil || *decoded.PromptTokens != 1000 {
		t.Errorf("prompt_tokens: got %v", decoded.PromptTokens)
	}
	if decoded.CompletionTokens == nil || *decoded.CompletionTokens != 200 {
		t.Errorf("completion_tokens: got %v", decoded.CompletionTokens)
	}
	if decoded.EvalDurationNs == nil || *decoded.EvalDurationNs != 5000000000 {
		t.Errorf("eval_duration_ns: got %v", decoded.EvalDurationNs)
	}
}

func TestRunConfigSnapshot_marshalUnmarshal(t *testing.T) {
	cfg := NewRunConfigSnapshot("m", "strict", 5, 100, true)
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded RunConfigSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Model != cfg.Model || decoded.Strictness != cfg.Strictness {
		t.Errorf("model/strictness: got %q / %q, want %q / %q", decoded.Model, decoded.Strictness, cfg.Model, cfg.Strictness)
	}
	if decoded.RAGSymbolMaxDefinitions != cfg.RAGSymbolMaxDefinitions || decoded.RAGSymbolMaxTokens != cfg.RAGSymbolMaxTokens {
		t.Errorf("rag: got %d/%d, want %d/%d", decoded.RAGSymbolMaxDefinitions, decoded.RAGSymbolMaxTokens, cfg.RAGSymbolMaxDefinitions, cfg.RAGSymbolMaxTokens)
	}
	if decoded.Nitpicky != cfg.Nitpicky {
		t.Errorf("nitpicky: got %v, want %v", decoded.Nitpicky, cfg.Nitpicky)
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
