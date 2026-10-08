package criticeval

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"stet/cli/internal/findings"
	"stet/cli/internal/review"
	"stet/cli/internal/systemone"
)

// sweepDecider records every Decide request and answers question "fN" of
// call C with probs[C][N] (0 when that call or index has no entry). err, when
// set, is returned as the Decide error starting at call errCall (0-indexed);
// earlier calls succeed.
type sweepDecider struct {
	requests []systemone.Request
	probs    [][]float64
	err      error
	errCall  int
}

func (f *sweepDecider) Decide(_ context.Context, req systemone.Request) (*systemone.Response, error) {
	f.requests = append(f.requests, req)
	call := len(f.requests) - 1
	if f.err != nil && call >= f.errCall {
		return nil, f.err
	}
	var callProbs []float64
	if call < len(f.probs) {
		callProbs = f.probs[call]
	}
	answers := make(map[string]systemone.NoulAnswer, len(req.Questions))
	for i := 0; ; i++ {
		key := fmt.Sprintf("f%d", i)
		if _, ok := req.Questions[key]; !ok {
			break
		}
		p := 0.0
		if i < len(callProbs) {
			p = callProbs[i]
		}
		answers[key] = systemone.NoulAnswer{Noul: p}
	}
	return &systemone.Response{Model: req.Model, Answers: answers}, nil
}

// sweepFinding builds a minimal finding with the given ID.
func sweepFinding(id string) findings.Finding {
	return findings.Finding{
		ID:       id,
		File:     "pkg/foo.go",
		Line:     10,
		Severity: findings.SeverityWarning,
		Category: findings.CategoryCorrectness,
		Message:  "issue in " + id,
	}
}

// ex builds one labeled example.
func ex(id string, label Label) Example {
	return Example{Finding: sweepFinding(id), Label: label, Source: SourceCurated}
}

// labeledGroup builds a hunk group holding the given keep and drop finding
// IDs (keeps first, matching the per-call prob order in the fake decider).
func labeledGroup(hunk string, keeps, drops []string) HunkGroup {
	g := HunkGroup{Hunk: hunk}
	for _, id := range keeps {
		g.Examples = append(g.Examples, ex(id, LabelKeep))
	}
	for _, id := range drops {
		g.Examples = append(g.Examples, ex(id, LabelDrop))
	}
	return g
}

// buildDataset assembles a Dataset with Counts computed from the groups.
func buildDataset(groups ...HunkGroup) *Dataset {
	ds := &Dataset{
		Counts:   map[Label]int{LabelKeep: 0, LabelDrop: 0},
		BySource: map[string]int{},
	}
	for _, g := range groups {
		ds.Groups = append(ds.Groups, g)
		for _, e := range g.Examples {
			ds.Counts[e.Label]++
		}
	}
	return ds
}

// The balanced dataset: 10 keeps and 10 drops across 3 hunk groups.
var balancedGroups = []HunkGroup{
	labeledGroup("hunk-0",
		[]string{"k00", "k01", "k02", "k03", "k04"},
		[]string{"d00", "d01", "d02"}),
	labeledGroup("hunk-1",
		[]string{"k05", "k06", "k07"},
		[]string{"d03", "d04", "d05", "d06"}),
	labeledGroup("hunk-2",
		[]string{"k08", "k09"},
		[]string{"d07", "d08", "d09"}),
}

// balancedScores maps every balanced finding ID to its P(yes): keeps score
// high (0.95 down to 0.55), drops score low (0.4 down to 0.05).
var balancedScores = map[string]float64{
	"k00": 0.95, "k01": 0.9, "k02": 0.85, "k03": 0.8, "k04": 0.8,
	"k05": 0.75, "k06": 0.7, "k07": 0.65, "k08": 0.6, "k09": 0.55,
	"d00": 0.4, "d01": 0.35, "d02": 0.3, "d03": 0.3, "d04": 0.25,
	"d05": 0.25, "d06": 0.2, "d07": 0.15, "d08": 0.1, "d09": 0.05,
}

// balancedProbs lists the P(yes) answers per group call, in group order.
var balancedProbs = [][]float64{
	{0.95, 0.9, 0.85, 0.8, 0.8, 0.4, 0.35, 0.3},
	{0.75, 0.7, 0.65, 0.3, 0.25, 0.25, 0.2},
	{0.6, 0.55, 0.15, 0.1, 0.05},
}

// balancedWant is the exact expected sweep for the balanced dataset.
var balancedWant = []ThresholdResult{
	{Threshold: 0.1, DropsCaughtPct: 10, KeepsLostPct: 0, Kept: 19},
	{Threshold: 0.2, DropsCaughtPct: 30, KeepsLostPct: 0, Kept: 17},
	{Threshold: 0.3, DropsCaughtPct: 60, KeepsLostPct: 0, Kept: 14},
	{Threshold: 0.4, DropsCaughtPct: 90, KeepsLostPct: 0, Kept: 11},
	{Threshold: 0.5, DropsCaughtPct: 100, KeepsLostPct: 0, Kept: 10},
	{Threshold: 0.6, DropsCaughtPct: 100, KeepsLostPct: 10, Kept: 9},
	{Threshold: 0.7, DropsCaughtPct: 100, KeepsLostPct: 30, Kept: 7},
	{Threshold: 0.8, DropsCaughtPct: 100, KeepsLostPct: 50, Kept: 5},
	{Threshold: 0.9, DropsCaughtPct: 100, KeepsLostPct: 80, Kept: 2},
}

func TestScore_batchedPerGroupAndScores(t *testing.T) {
	t.Parallel()
	ds := buildDataset(balancedGroups...)
	fake := &sweepDecider{probs: balancedProbs}
	scores, latencies, err := Score(context.Background(), fake, "openjev-0.1", 3, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.requests) != len(ds.Groups) {
		t.Fatalf("expected exactly %d requests (one per hunk group), got %d", len(ds.Groups), len(fake.requests))
	}
	for i, req := range fake.requests {
		want := len(ds.Groups[i].Examples)
		if len(req.Questions) != want {
			t.Errorf("request %d: questions = %d, want %d", i, len(req.Questions), want)
		}
		if req.Model != "openjev-0.1" {
			t.Errorf("request %d: model = %q, want openjev-0.1", i, req.Model)
		}
		if req.Samples != 3 {
			t.Errorf("request %d: samples = %d, want 3", i, req.Samples)
		}
		if !strings.Contains(req.State, ds.Groups[i].Hunk) {
			t.Errorf("request %d: state %q should contain hunk %q", i, req.State, ds.Groups[i].Hunk)
		}
	}
	if len(scores) != len(balancedScores) {
		t.Fatalf("scores has %d entries, want %d", len(scores), len(balancedScores))
	}
	for id, want := range balancedScores {
		if got := scores[id]; got != want {
			t.Errorf("scores[%s] = %v, want %v", id, got, want)
		}
	}
	if len(latencies) != len(ds.Groups) {
		t.Fatalf("latencies has %d entries, want %d (one per group call)", len(latencies), len(ds.Groups))
	}
	for i, l := range latencies {
		if l < 0 {
			t.Errorf("latencies[%d] = %v, want >= 0", i, l)
		}
	}
}

func TestScore_deciderErrorNamesGroupIndex(t *testing.T) {
	t.Parallel()
	ds := buildDataset(
		labeledGroup("hunk-0", []string{"k0"}, nil),
		labeledGroup("hunk-1", nil, []string{"d0"}),
	)
	cause := errors.New("connection refused")
	fake := &sweepDecider{probs: [][]float64{{0.9}}, err: cause, errCall: 1}
	scores, latencies, err := Score(context.Background(), fake, "openjev-0.1", 1, ds)
	if err == nil {
		t.Fatal("expected an error from the failing decider")
	}
	if !errors.Is(err, cause) {
		t.Errorf("error should wrap the decider cause, got: %v", err)
	}
	if !strings.Contains(err.Error(), "hunk group 1") {
		t.Errorf("error should name the failing hunk group index 1, got: %v", err)
	}
	if scores != nil || latencies != nil {
		t.Errorf("failed scoring should return nil results, got scores=%v latencies=%v", scores, latencies)
	}
	if len(fake.requests) != 2 {
		t.Errorf("the failing group's request should still have been sent, got %d requests", len(fake.requests))
	}
}

func TestScore_inputValidation(t *testing.T) {
	t.Parallel()
	ds := buildDataset(labeledGroup("h", []string{"k0"}, nil))
	fake := &sweepDecider{}
	tests := []struct {
		name    string
		client  review.SystemOneDecider
		model   string
		ds      *Dataset
		wantErr string
	}{
		{"nil client", nil, "openjev-0.1", ds, "nil System One client"},
		{"empty model", fake, "", ds, "empty model"},
		{"nil dataset", fake, "openjev-0.1", nil, "nil dataset"},
	}
	for _, tt := range tests {
		scores, latencies, err := Score(context.Background(), tt.client, tt.model, 1, tt.ds)
		if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("%s: err = %v, want it to contain %q", tt.name, err, tt.wantErr)
		}
		if scores != nil || latencies != nil {
			t.Errorf("%s: failed scoring should return nil results", tt.name)
		}
	}
	if len(fake.requests) != 0 {
		t.Errorf("validation failures must not send requests, got %d", len(fake.requests))
	}
}

func TestScore_emptyDataset(t *testing.T) {
	t.Parallel()
	fake := &sweepDecider{probs: [][]float64{{0.5}}}
	scores, latencies, err := Score(context.Background(), fake, "openjev-0.1", 1, buildDataset())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scores == nil || len(scores) != 0 {
		t.Errorf("scores = %v, want empty non-nil map", scores)
	}
	if latencies == nil || len(latencies) != 0 {
		t.Errorf("latencies = %v, want empty non-nil slice", latencies)
	}
	if len(fake.requests) != 0 {
		t.Errorf("empty dataset must not send requests, got %d", len(fake.requests))
	}
}

func TestEvaluate_exactSweepAndRecommendation(t *testing.T) {
	t.Parallel()
	ds := buildDataset(balancedGroups...)
	rep := Evaluate(balancedScores, []time.Duration{time.Millisecond}, ds)
	if rep.Counts[LabelKeep] != 10 || rep.Counts[LabelDrop] != 10 {
		t.Errorf("Counts = %v, want keep=10 drop=10", rep.Counts)
	}
	if len(rep.Thresholds) != len(balancedWant) {
		t.Fatalf("got %d threshold rows, want %d", len(rep.Thresholds), len(balancedWant))
	}
	for i, want := range balancedWant {
		if got := rep.Thresholds[i]; got != want {
			t.Errorf("Thresholds[%d] = %+v, want %+v", i, got, want)
		}
	}
	if rep.Recommended == nil || *rep.Recommended != 0.6 {
		t.Errorf("Recommended = %v, want 0.6 (0.7 loses 30%% of keeps)", rep.Recommended)
	}
	if len(rep.Warnings) != 0 {
		t.Errorf("balanced dataset should produce no warnings, got %v", rep.Warnings)
	}
}

func TestEvaluate_noQualifyingThreshold(t *testing.T) {
	t.Parallel()
	keeps := make([]string, 10)
	for i := range keeps {
		keeps[i] = fmt.Sprintf("k%02d", i)
	}
	drops := make([]string, 10)
	for i := range drops {
		drops[i] = fmt.Sprintf("d%02d", i)
	}
	ds := buildDataset(labeledGroup("h", keeps, drops))
	scores := map[string]float64{}
	for _, id := range keeps {
		scores[id] = 0.05 // Every keep lost at every threshold.
	}
	for _, id := range drops {
		scores[id] = 0.95
	}
	rep := Evaluate(scores, nil, ds)
	if rep.Recommended != nil {
		t.Errorf("Recommended = %v, want nil when keeps are lost at every threshold", rep.Recommended)
	}
	first := rep.Thresholds[0]
	want := ThresholdResult{Threshold: 0.1, DropsCaughtPct: 0, KeepsLostPct: 100, Kept: 10}
	if first != want {
		t.Errorf("Thresholds[0] = %+v, want %+v", first, want)
	}
	if len(rep.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none (both labels have 10 examples)", rep.Warnings)
	}
}

func TestEvaluate_oneLabelOnlyWarning(t *testing.T) {
	t.Parallel()
	ds := buildDataset(labeledGroup("h", nil, []string{"d0", "d1", "d2"}))
	scores := map[string]float64{"d0": 0.05, "d1": 0.5, "d2": 0.85}
	rep := Evaluate(scores, nil, ds)
	if rep.Counts[LabelKeep] != 0 || rep.Counts[LabelDrop] != 3 {
		t.Errorf("Counts = %v, want keep=0 drop=3", rep.Counts)
	}
	// No keeps: KeepsLostPct is 0 at every threshold, so the highest
	// threshold always qualifies.
	if rep.Recommended == nil || *rep.Recommended != 0.9 {
		t.Errorf("Recommended = %v, want 0.9", rep.Recommended)
	}
	last := rep.Thresholds[len(rep.Thresholds)-1]
	want := ThresholdResult{Threshold: 0.9, DropsCaughtPct: 100, KeepsLostPct: 0, Kept: 0}
	if last != want {
		t.Errorf("last threshold row = %+v, want %+v", last, want)
	}
	if len(rep.Warnings) != 1 || rep.Warnings[0] != "recommendation unreliable" {
		t.Errorf("Warnings = %v, want [recommendation unreliable]", rep.Warnings)
	}
}

func TestEvaluate_emptyDataset(t *testing.T) {
	t.Parallel()
	empty := buildDataset()
	for _, ds := range []*Dataset{empty, nil} {
		rep := Evaluate(map[string]float64{}, nil, ds)
		if rep.Counts[LabelKeep] != 0 || rep.Counts[LabelDrop] != 0 {
			t.Errorf("Counts = %v, want both zero", rep.Counts)
		}
		if len(rep.Thresholds) != 9 {
			t.Fatalf("got %d threshold rows, want 9", len(rep.Thresholds))
		}
		for i, row := range rep.Thresholds {
			want := ThresholdResult{Threshold: float64(i+1) / 10}
			if row != want {
				t.Errorf("ds=%v Thresholds[%d] = %+v, want %+v", ds != nil, i, row, want)
			}
		}
		// With no keeps, KeepsLostPct is 0 at every threshold, so the literal
		// rule qualifies every threshold and picks the highest one; the
		// warning is the signal that the recommendation is unreliable.
		if rep.Recommended == nil || *rep.Recommended != 0.9 {
			t.Errorf("Recommended = %v, want 0.9 (vacuously qualifies with no keeps)", rep.Recommended)
		}
		if len(rep.Warnings) != 1 || rep.Warnings[0] != "recommendation unreliable" {
			t.Errorf("Warnings = %v, want [recommendation unreliable]", rep.Warnings)
		}
		if rep.LatencyP50 != 0 || rep.LatencyP95 != 0 {
			t.Errorf("latency percentiles = %v/%v, want 0/0 for empty latencies", rep.LatencyP50, rep.LatencyP95)
		}
	}
}

func TestEvaluate_missingScoreTreatedAsZero(t *testing.T) {
	t.Parallel()
	ds := buildDataset(labeledGroup("h", []string{"k0"}, []string{"d0"}))
	// k0 has no score: it counts as dropped at every threshold.
	rep := Evaluate(map[string]float64{"d0": 0.5}, nil, ds)
	if rep.Recommended != nil {
		t.Errorf("Recommended = %v, want nil when a keep is lost at every threshold", rep.Recommended)
	}
	first := rep.Thresholds[0]
	want := ThresholdResult{Threshold: 0.1, DropsCaughtPct: 0, KeepsLostPct: 100, Kept: 1}
	if first != want {
		t.Errorf("Thresholds[0] = %+v, want %+v", first, want)
	}
	last := rep.Thresholds[len(rep.Thresholds)-1]
	wantLast := ThresholdResult{Threshold: 0.9, DropsCaughtPct: 100, KeepsLostPct: 100, Kept: 0}
	if last != wantLast {
		t.Errorf("Thresholds[8] = %+v, want %+v", last, wantLast)
	}
}

func TestEvaluate_latencyPercentiles(t *testing.T) {
	t.Parallel()
	ds := buildDataset(labeledGroup("h", []string{"k0"}, []string{"d0"}))
	scores := map[string]float64{"k0": 0.9, "d0": 0.1}
	latencies := []time.Duration{
		5 * time.Millisecond,
		1 * time.Millisecond,
		3 * time.Millisecond,
		2 * time.Millisecond,
	}
	rep := Evaluate(scores, latencies, ds)
	if rep.LatencyP50 != 2*time.Millisecond {
		t.Errorf("LatencyP50 = %v, want 2ms (nearest rank of [1 2 3 5]ms)", rep.LatencyP50)
	}
	if rep.LatencyP95 != 5*time.Millisecond {
		t.Errorf("LatencyP95 = %v, want 5ms (nearest rank of [1 2 3 5]ms)", rep.LatencyP95)
	}

	single := Evaluate(scores, []time.Duration{7 * time.Millisecond}, ds)
	if single.LatencyP50 != 7*time.Millisecond || single.LatencyP95 != 7*time.Millisecond {
		t.Errorf("single latency percentiles = %v/%v, want 7ms/7ms", single.LatencyP50, single.LatencyP95)
	}

	two := Evaluate(scores, []time.Duration{20 * time.Millisecond, 10 * time.Millisecond}, ds)
	if two.LatencyP50 != 10*time.Millisecond {
		t.Errorf("LatencyP50 = %v, want 10ms (rank ceil(0.5*2)=1)", two.LatencyP50)
	}
	if two.LatencyP95 != 20*time.Millisecond {
		t.Errorf("LatencyP95 = %v, want 20ms (rank ceil(0.95*2)=2)", two.LatencyP95)
	}
}
