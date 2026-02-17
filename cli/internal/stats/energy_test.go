package stats

import (
	"encoding/json"
	"reflect"
	"testing"

	"stet/cli/internal/git"
)

func TestEnergy_twoNotesWithUsage(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	headSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	head1SHA := runOut(t, repo, "git", "rev-parse", "HEAD~1")
	// 1 hour + 0.5 hour = 1.5h total eval; 1M+500k prompt, 100k+50k completion
	note1 := `{"session_id":"s1","baseline_sha":"` + runOut(t, repo, "git", "rev-parse", "HEAD~2") + `","head_sha":"` + head1SHA + `","findings_count":0,"dismissals_count":0,"tool_version":"test","finished_at":"2025-01-01T00:00:00Z","eval_duration_ns":3600000000000,"prompt_tokens":1000000,"completion_tokens":100000}`
	note2 := `{"session_id":"s2","baseline_sha":"` + head1SHA + `","head_sha":"` + headSHA + `","findings_count":0,"dismissals_count":0,"tool_version":"test","finished_at":"2025-01-01T00:00:00Z","eval_duration_ns":1800000000000,"prompt_tokens":500000,"completion_tokens":50000}`
	if err := git.AddNote(repo, git.NotesRefStet, head1SHA, note1); err != nil {
		t.Fatalf("AddNote HEAD~1: %v", err)
	}
	if err := git.AddNote(repo, git.NotesRefStet, headSHA, note2); err != nil {
		t.Fatalf("AddNote HEAD: %v", err)
	}
	cloudModels := []CloudModel{
		{Name: "gpt-4o-mini", InPerMillion: 0.15, OutPerMillion: 0.60},
		{Name: "claude-sonnet", InPerMillion: 3, OutPerMillion: 15},
	}
	res, err := Energy(repo, "HEAD~2", "HEAD", 30, cloudModels)
	if err != nil {
		t.Fatalf("Energy: %v", err)
	}
	if res.SessionsCount != 2 {
		t.Errorf("SessionsCount: got %d, want 2", res.SessionsCount)
	}
	if res.TotalEvalDurationNs != 5400000000000 {
		t.Errorf("TotalEvalDurationNs: got %d, want 5400000000000", res.TotalEvalDurationNs)
	}
	if res.TotalPromptTokens != 1500000 || res.TotalCompletionTokens != 150000 {
		t.Errorf("TotalPromptTokens=%d TotalCompletionTokens=%d, want 1500000, 150000", res.TotalPromptTokens, res.TotalCompletionTokens)
	}
	// kWh = (5400 sec / 3600) * (30 / 1000) = 1.5 * 0.03 = 0.045
	wantKWh := 0.045
	if res.LocalEnergyKWh < wantKWh-0.001 || res.LocalEnergyKWh > wantKWh+0.001 {
		t.Errorf("LocalEnergyKWh: got %.4f, want %.4f", res.LocalEnergyKWh, wantKWh)
	}
	// gpt-4o-mini: 1.5*0.15 + 0.15*0.60 = 0.225 + 0.09 = 0.315
	if cost := res.CloudCostAvoided["gpt-4o-mini"]; cost < 0.31 || cost > 0.32 {
		t.Errorf("CloudCostAvoided[gpt-4o-mini]: got %.4f, want ~0.315", cost)
	}
	// claude-sonnet: 1.5*3 + 0.15*15 = 4.5 + 2.25 = 6.75
	if cost := res.CloudCostAvoided["claude-sonnet"]; cost < 6.7 || cost > 6.8 {
		t.Errorf("CloudCostAvoided[claude-sonnet]: got %.4f, want ~6.75", cost)
	}
}

func TestEnergy_emptyRange(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	res, err := Energy(repo, "HEAD", "HEAD", 30, nil)
	if err != nil {
		t.Fatalf("Energy: %v", err)
	}
	if res.SessionsCount != 0 || res.TotalEvalDurationNs != 0 || res.LocalEnergyKWh != 0 {
		t.Errorf("empty range: SessionsCount=%d TotalEvalDurationNs=%d LocalEnergyKWh=%.2f, want 0, 0, 0",
			res.SessionsCount, res.TotalEvalDurationNs, res.LocalEnergyKWh)
	}
}

func TestEnergy_noNotesInRange(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	res, err := Energy(repo, "HEAD~2", "HEAD", 30, nil)
	if err != nil {
		t.Fatalf("Energy: %v", err)
	}
	if res.SessionsCount != 0 || res.TotalEvalDurationNs != 0 || res.LocalEnergyKWh != 0 {
		t.Errorf("no notes: SessionsCount=%d TotalEvalDurationNs=%d LocalEnergyKWh=%.2f, want 0, 0, 0",
			res.SessionsCount, res.TotalEvalDurationNs, res.LocalEnergyKWh)
	}
}

func TestEnergy_notesWithoutUsageFields(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	headSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	note := `{"session_id":"s1","baseline_sha":"` + runOut(t, repo, "git", "rev-parse", "HEAD~1") + `","head_sha":"` + headSHA + `","findings_count":0,"dismissals_count":0,"tool_version":"test","finished_at":"2025-01-01T00:00:00Z","hunks_reviewed":1}`
	if err := git.AddNote(repo, git.NotesRefStet, headSHA, note); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	res, err := Energy(repo, "HEAD~1", "HEAD", 30, []CloudModel{{Name: "gpt-4o-mini", InPerMillion: 0.15, OutPerMillion: 0.60}})
	if err != nil {
		t.Fatalf("Energy: %v", err)
	}
	if res.SessionsCount != 1 {
		t.Errorf("SessionsCount: got %d, want 1", res.SessionsCount)
	}
	if res.TotalEvalDurationNs != 0 || res.TotalPromptTokens != 0 || res.TotalCompletionTokens != 0 {
		t.Errorf("usage should be zero: TotalEvalDurationNs=%d TotalPromptTokens=%d TotalCompletionTokens=%d",
			res.TotalEvalDurationNs, res.TotalPromptTokens, res.TotalCompletionTokens)
	}
	if res.LocalEnergyKWh != 0 {
		t.Errorf("LocalEnergyKWh: got %.2f, want 0 (no usage data)", res.LocalEnergyKWh)
	}
	if cost := res.CloudCostAvoided["gpt-4o-mini"]; cost != 0 {
		t.Errorf("CloudCostAvoided[gpt-4o-mini]: got %.2f, want 0", cost)
	}
}

func TestEnergy_cloudModelPreset(t *testing.T) {
	t.Parallel()
	m, err := ParseCloudModel("gpt-4o-mini")
	if err != nil {
		t.Fatalf("ParseCloudModel: %v", err)
	}
	if m.Name != "gpt-4o-mini" || m.InPerMillion != 0.15 || m.OutPerMillion != 0.60 {
		t.Errorf("got %+v, want gpt-4o-mini $0.15/$0.60", m)
	}
	m2, err := ParseCloudModel("claude-sonnet")
	if err != nil {
		t.Fatalf("ParseCloudModel claude-sonnet: %v", err)
	}
	if m2.Name != "claude-sonnet" || m2.InPerMillion != 3 || m2.OutPerMillion != 15 {
		t.Errorf("got %+v, want claude-sonnet $3/$15", m2)
	}
}

func TestEnergy_cloudModelCustom(t *testing.T) {
	t.Parallel()
	m, err := ParseCloudModel("foo:1:2")
	if err != nil {
		t.Fatalf("ParseCloudModel: %v", err)
	}
	if m.Name != "foo" || m.InPerMillion != 1 || m.OutPerMillion != 2 {
		t.Errorf("got %+v, want foo $1/$2", m)
	}
	// Verify it produces correct cost when used
	repo := initRepo(t)
	headSHA := runOut(t, repo, "git", "rev-parse", "HEAD")
	note := `{"session_id":"s1","baseline_sha":"` + runOut(t, repo, "git", "rev-parse", "HEAD~1") + `","head_sha":"` + headSHA + `","eval_duration_ns":0,"prompt_tokens":1000000,"completion_tokens":1000000}`
	if err := git.AddNote(repo, git.NotesRefStet, headSHA, note); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	res, err := Energy(repo, "HEAD~1", "HEAD", 30, []CloudModel{m})
	if err != nil {
		t.Fatalf("Energy: %v", err)
	}
	// 1M*1 + 1M*2 = 3
	if cost := res.CloudCostAvoided["foo"]; cost < 2.99 || cost > 3.01 {
		t.Errorf("CloudCostAvoided[foo]: got %.2f, want 3", cost)
	}
}

func TestEnergy_parseCloudModelInvalid(t *testing.T) {
	t.Parallel()
	if _, err := ParseCloudModel(""); err == nil {
		t.Error("ParseCloudModel empty: expected error")
	}
	if _, err := ParseCloudModel("unknown-preset"); err == nil {
		t.Error("ParseCloudModel unknown preset: expected error")
	}
	if _, err := ParseCloudModel("bad:notanum:2"); err == nil {
		t.Error("ParseCloudModel bad in: expected error")
	}
	if _, err := ParseCloudModel("bad:1:notanum"); err == nil {
		t.Error("ParseCloudModel bad out: expected error")
	}
	if _, err := ParseCloudModel("only:two"); err == nil {
		t.Error("ParseCloudModel NAME:in (missing out): expected error")
	}
}

func TestEnergy_resultJSONRoundtrip(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	res, err := Energy(repo, "HEAD", "HEAD", 30, nil)
	if err != nil {
		t.Fatalf("Energy: %v", err)
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded EnergyResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.SessionsCount != res.SessionsCount || decoded.TotalEvalDurationNs != res.TotalEvalDurationNs ||
		decoded.LocalEnergyKWh != res.LocalEnergyKWh {
		t.Errorf("roundtrip: decoded %+v != original %+v", decoded, *res)
	}
	if decoded.CloudCostAvoided == nil {
		t.Error("roundtrip: CloudCostAvoided should be non-nil")
	}
	if !reflect.DeepEqual(decoded.CloudCostAvoided, res.CloudCostAvoided) {
		t.Errorf("roundtrip CloudCostAvoided: decoded %v != original %v", decoded.CloudCostAvoided, res.CloudCostAvoided)
	}
}
