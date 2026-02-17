package stats

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"stet/cli/internal/findings"
	"stet/cli/internal/history"
)

func int64Ptr(n int64) *int64 { return &n }

func TestQuality_fixtureHistory(t *testing.T) {
	t.Parallel()
	stateDir := t.TempDir()
	// Session 1: 5 findings, 2 dismissed (false_positive, already_correct); has tokens.
	rec1 := history.Record{
		DiffRef: "HEAD",
		ReviewOutput: []findings.Finding{
			{ID: "f1", File: "a.go", Line: 1, Severity: findings.SeverityWarning, Category: findings.CategoryMaintainability, Confidence: 1.0, Message: "m1"},
			{ID: "f2", File: "a.go", Line: 2, Severity: findings.SeverityInfo, Category: findings.CategoryStyle, Confidence: 0.9, Message: "m2"},
			{ID: "f3", File: "b.go", Line: 1, Severity: findings.SeverityError, Category: findings.CategoryCorrectness, Confidence: 1.0, Message: "m3"},
			{ID: "f4", File: "b.go", Line: 2, Severity: findings.SeverityWarning, Category: findings.CategoryMaintainability, Confidence: 0.8, Message: "m4"},
			{ID: "f5", File: "c.go", Line: 1, Severity: findings.SeverityInfo, Category: findings.CategoryBestPractice, Confidence: 0.7, Message: "m5"},
		},
		UserAction: history.UserAction{
			DismissedIDs: []string{"f1", "f2"},
			Dismissals: []history.Dismissal{
				{FindingID: "f1", Reason: history.ReasonFalsePositive},
				{FindingID: "f2", Reason: history.ReasonAlreadyCorrect},
			},
		},
		PromptTokens:     int64Ptr(1000),
		CompletionTokens: int64Ptr(200),
	}
	// Session 2: 0 findings (clean commit).
	rec2 := history.Record{
		DiffRef:      "HEAD~1",
		ReviewOutput: nil,
		UserAction:   history.UserAction{},
	}
	if err := history.Append(stateDir, rec1, 0); err != nil {
		t.Fatalf("Append rec1: %v", err)
	}
	if err := history.Append(stateDir, rec2, 0); err != nil {
		t.Fatalf("Append rec2: %v", err)
	}
	res, err := Quality(stateDir)
	if err != nil {
		t.Fatalf("Quality: %v", err)
	}
	if res.SessionsCount != 2 {
		t.Errorf("SessionsCount: got %d, want 2", res.SessionsCount)
	}
	if res.TotalFindings != 5 {
		t.Errorf("TotalFindings: got %d, want 5", res.TotalFindings)
	}
	if res.TotalDismissed != 2 {
		t.Errorf("TotalDismissed: got %d, want 2", res.TotalDismissed)
	}
	// Dismissal rate = 2/5 = 0.4
	if res.DismissalRate != 0.4 {
		t.Errorf("DismissalRate: got %.2f, want 0.40", res.DismissalRate)
	}
	// Acceptance rate = 3/5 = 0.6
	if res.AcceptanceRate != 0.6 {
		t.Errorf("AcceptanceRate: got %.2f, want 0.60", res.AcceptanceRate)
	}
	// False positive rate = 1/5 = 0.2
	if res.FalsePositiveRate != 0.2 {
		t.Errorf("FalsePositiveRate: got %.2f, want 0.20", res.FalsePositiveRate)
	}
	// Actionability = already_correct / dismissed = 1/2 = 0.5
	if res.Actionability != 0.5 {
		t.Errorf("Actionability: got %.2f, want 0.50", res.Actionability)
	}
	// Clean commit rate = 1/2 = 0.5
	if res.CleanCommitRate != 0.5 {
		t.Errorf("CleanCommitRate: got %.2f, want 0.50", res.CleanCommitRate)
	}
	// Finding density = 5 / (1200/1000) = 5/1.2 ≈ 4.167
	if res.FindingDensity < 4.1 || res.FindingDensity > 4.2 {
		t.Errorf("FindingDensity: got %.2f, want ~4.17", res.FindingDensity)
	}
	if got := res.DismissalsByReason[history.ReasonFalsePositive]; got != 1 {
		t.Errorf("DismissalsByReason[false_positive]: got %d, want 1", got)
	}
	if got := res.DismissalsByReason[history.ReasonAlreadyCorrect]; got != 1 {
		t.Errorf("DismissalsByReason[already_correct]: got %d, want 1", got)
	}
	wantCategories := map[string]int{
		"maintainability": 2,
		"style":           1,
		"correctness":     1,
		"best_practice":   1,
	}
	for cat, want := range wantCategories {
		if got := res.CategoryBreakdown[cat]; got != want {
			t.Errorf("CategoryBreakdown[%q]: got %d, want %d", cat, got, want)
		}
	}
}

func TestQuality_zeroDenominators(t *testing.T) {
	t.Parallel()
	stateDir := t.TempDir()
	// One session with 0 findings, 0 dismissed.
	if err := history.Append(stateDir, history.Record{
		DiffRef:      "HEAD",
		ReviewOutput: nil,
		UserAction:   history.UserAction{},
	}, 0); err != nil {
		t.Fatalf("Append: %v", err)
	}
	res, err := Quality(stateDir)
	if err != nil {
		t.Fatalf("Quality: %v", err)
	}
	if res.SessionsCount != 1 || res.TotalFindings != 0 || res.TotalDismissed != 0 {
		t.Errorf("SessionsCount=%d TotalFindings=%d TotalDismissed=%d", res.SessionsCount, res.TotalFindings, res.TotalDismissed)
	}
	if res.DismissalRate != 0 || res.AcceptanceRate != 0 || res.FalsePositiveRate != 0 || res.Actionability != 0 {
		t.Errorf("rates should be 0: dismissal=%.2f acceptance=%.2f fp=%.2f action=%.2f",
			res.DismissalRate, res.AcceptanceRate, res.FalsePositiveRate, res.Actionability)
	}
	if res.CleanCommitRate != 1.0 {
		t.Errorf("CleanCommitRate: got %.2f, want 1.0", res.CleanCommitRate)
	}
	if res.FindingDensity != 0 {
		t.Errorf("FindingDensity: got %.2f, want 0 (no tokens)", res.FindingDensity)
	}
}

func TestQuality_emptyOrMissingHistory(t *testing.T) {
	t.Parallel()
	// Empty state dir (no history.jsonl): ReadRecords returns empty slice.
	stateDir := t.TempDir()
	res, err := Quality(stateDir)
	if err != nil {
		t.Fatalf("Quality(empty dir): %v", err)
	}
	if res.SessionsCount != 0 || res.TotalFindings != 0 || res.TotalDismissed != 0 {
		t.Errorf("empty dir: got SessionsCount=%d TotalFindings=%d TotalDismissed=%d",
			res.SessionsCount, res.TotalFindings, res.TotalDismissed)
	}
	if res.DismissalsByReason == nil || res.CategoryBreakdown == nil {
		t.Error("DismissalsByReason and CategoryBreakdown should be non-nil maps")
	}
	// State dir does not exist: return zeros, no error.
	nonexistent := filepath.Join(t.TempDir(), "nonexistent")
	res2, err := Quality(nonexistent)
	if err != nil {
		t.Fatalf("Quality(nonexistent): %v", err)
	}
	if res2.SessionsCount != 0 || res2.TotalFindings != 0 {
		t.Errorf("nonexistent dir: got SessionsCount=%d TotalFindings=%d", res2.SessionsCount, res2.TotalFindings)
	}
}

func TestQuality_resultJSONRoundtrip(t *testing.T) {
	t.Parallel()
	stateDir := t.TempDir()
	if err := history.Append(stateDir, history.Record{
		DiffRef: "HEAD",
		ReviewOutput: []findings.Finding{
			{File: "x.go", Severity: findings.SeverityWarning, Category: findings.CategorySecurity, Confidence: 1.0, Message: "m"},
		},
		UserAction: history.UserAction{DismissedIDs: []string{"id1"}},
	}, 0); err != nil {
		t.Fatalf("Append: %v", err)
	}
	res, err := Quality(stateDir)
	if err != nil {
		t.Fatalf("Quality: %v", err)
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded QualityResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.SessionsCount != res.SessionsCount || decoded.TotalFindings != res.TotalFindings ||
		decoded.TotalDismissed != res.TotalDismissed || decoded.DismissalRate != res.DismissalRate {
		t.Errorf("roundtrip: decoded %+v vs original %+v", decoded, res)
	}
	if decoded.DismissalsByReason == nil || decoded.CategoryBreakdown == nil {
		t.Error("roundtrip: maps should be non-nil")
	}
}
