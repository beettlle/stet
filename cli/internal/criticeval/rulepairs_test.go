package criticeval

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/systemone"
)

// rpDecider records every Decide request and answers question "qN" of call C
// with probs[C][N] (0 when that call or index has no entry). err, when set,
// is returned as the Decide error starting at call errCall (0-indexed);
// earlier calls succeed. dropAnswers, when >= 0, omits question "qN" of the
// response at call errCall.
type rpDecider struct {
	requests    []systemone.Request
	probs       [][]float64
	err         error
	errCall     int
	dropAnswers int
}

func (f *rpDecider) Decide(_ context.Context, req systemone.Request) (*systemone.Response, error) {
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
		key := fmt.Sprintf("q%d", i)
		if _, ok := req.Questions[key]; !ok {
			break
		}
		if f.dropAnswers >= 0 && call == f.errCall && i == f.dropAnswers {
			continue
		}
		p := 0.0
		if i < len(callProbs) {
			p = callProbs[i]
		}
		answers[key] = systemone.NoulAnswer{Noul: p}
	}
	return &systemone.Response{Model: req.Model, Answers: answers}, nil
}

// rpPair builds one rule pair with the given fields.
func rpPair(id, language, premiseID, premise, source, snippet string, label RulePairLabel) RulePair {
	return RulePair{
		ID:        id,
		Language:  language,
		PremiseID: premiseID,
		Premise:   premise,
		Source:    source,
		Snippet:   snippet,
		Label:     label,
		Origin:    "synthetic",
	}
}

// rpWriteTemp writes the given lines to a temp JSONL file and returns its path.
func rpWriteTemp(t *testing.T, lines []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pairs.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write temp pairs file: %v", err)
	}
	return path
}

func TestLoadRulePairsValid(t *testing.T) {
	path := rpWriteTemp(t, []string{
		`{"id":"V01","language":"go","premise_id":"goroutine_lifecycle","premise":"Never fire-and-forget goroutines.","source":"go-1-21-development-standards","snippet":"func f() {\n\tgo g()\n}","label":"violating","origin":"sp020-seed"}`,
		`{"id":"C01","language":"typescript","premise_id":"ts_no_any","premise":"Do not use any.","source":"javascript-3-development-standards","snippet":"const x: unknown = 1;\nvoid x;\nvoid x;","label":"compliant","origin":"synthetic"}`,
	})
	pairs, err := LoadRulePairs(path)
	if err != nil {
		t.Fatalf("LoadRulePairs() error = %v, want nil", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("len(pairs) = %d, want 2", len(pairs))
	}
	want := RulePair{
		ID:        "V01",
		Language:  "go",
		PremiseID: "goroutine_lifecycle",
		Premise:   "Never fire-and-forget goroutines.",
		Source:    "go-1-21-development-standards",
		Snippet:   "func f() {\n\tgo g()\n}",
		Label:     RulePairViolating,
		Origin:    "sp020-seed",
	}
	if pairs[0] != want {
		t.Errorf("pairs[0] = %+v, want %+v", pairs[0], want)
	}
	if pairs[1].Label != RulePairCompliant || pairs[1].Language != "typescript" {
		t.Errorf("pairs[1] = %+v, want compliant typescript pair", pairs[1])
	}
}

func TestLoadRulePairsEmptyLinesSkipped(t *testing.T) {
	path := rpWriteTemp(t, []string{
		`{"id":"V01","language":"go","premise_id":"p","premise":"rule","source":"s","snippet":"a\nb\nc","label":"violating","origin":"synthetic"}`,
		"",
	})
	pairs, err := LoadRulePairs(path)
	if err != nil {
		t.Fatalf("LoadRulePairs() error = %v, want nil", err)
	}
	if len(pairs) != 1 || pairs[0].ID != "V01" {
		t.Errorf("pairs = %+v, want single pair V01", pairs)
	}
}

func TestLoadRulePairsMissingFile(t *testing.T) {
	_, err := LoadRulePairs(filepath.Join(t.TempDir(), "absent.jsonl"))
	if err == nil {
		t.Fatal("LoadRulePairs() error = nil, want open error for missing file")
	}
	if !strings.Contains(err.Error(), "open") {
		t.Errorf("error = %v, want it to mention the failed open", err)
	}
}

func TestLoadRulePairsLineNumbers(t *testing.T) {
	valid := `{"id":"F%d","language":"go","premise_id":"p","premise":"rule","source":"s","snippet":"a\nb\nc","label":"violating","origin":"synthetic"}`
	tests := []struct {
		name     string
		lines    []string
		wantLine int
		wantSub  string
	}{
		{
			name:     "bad JSON on line 2",
			lines:    []string{fmt.Sprintf(valid, 1), `{"id": "broken"`},
			wantLine: 2,
			wantSub:  "invalid JSON",
		},
		{
			name: "unknown label on line 2",
			lines: []string{
				fmt.Sprintf(valid, 1),
				`{"id":"F2","language":"go","premise_id":"p","premise":"rule","source":"s","snippet":"a\nb\nc","label":"maybe","origin":"synthetic"}`,
			},
			wantLine: 2,
			wantSub:  `unknown label "maybe"`,
		},
		{
			name: "empty field on line 3",
			lines: []string{
				fmt.Sprintf(valid, 1),
				fmt.Sprintf(valid, 2),
				`{"id":"F3","language":"go","premise_id":"p","premise":"rule","source":"","snippet":"a\nb\nc","label":"compliant","origin":"synthetic"}`,
			},
			wantLine: 3,
			wantSub:  `empty field "source"`,
		},
		{
			name: "duplicate id on line 3",
			lines: []string{
				fmt.Sprintf(valid, 1),
				fmt.Sprintf(valid, 2),
				fmt.Sprintf(valid, 2),
			},
			wantLine: 3,
			wantSub:  `duplicate id "F2"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := rpWriteTemp(t, tt.lines)
			_, err := LoadRulePairs(path)
			if err == nil {
				t.Fatalf("LoadRulePairs() error = nil, want error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error = %v, want substring %q", err, tt.wantSub)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("line %d", tt.wantLine)) {
				t.Errorf("error = %v, want it to name line %d", err, tt.wantLine)
			}
		})
	}
}

func TestLoadRulePairsLineTooLong(t *testing.T) {
	long := strings.Repeat("x", 128*1024)
	line := fmt.Sprintf(`{"id":"F1","language":"go","premise_id":"p","premise":"rule","source":"%s","snippet":"a\nb\nc","label":"violating","origin":"synthetic"}`, long)
	path := rpWriteTemp(t, []string{line})
	_, err := LoadRulePairs(path)
	if err == nil {
		t.Fatal("LoadRulePairs() error = nil, want scanner error for oversized line")
	}
	if !strings.Contains(err.Error(), "read") {
		t.Errorf("error = %v, want it to mention the read failure", err)
	}
}

// evaluateFixture builds four pairs over three distinct snippets: P1 and P2
// share snippet S1 (one request, two questions); P3 and P4 get their own.
func evaluateFixture() []RulePair {
	return []RulePair{
		rpPair("P1", "go", "io_in_loops", "No I/O operations inside loops.", "go-1-21-brutal-audit", "S1", RulePairViolating),
		rpPair("P2", "typescript", "io_in_loops", "No I/O operations inside loops.", "go-1-21-brutal-audit", "S1", RulePairCompliant),
		rpPair("P3", "go", "preallocate_slices", "Preallocate slices when the final size is known.", "go-1-21-development-standards", "S2", RulePairViolating),
		rpPair("P4", "typescript", "preallocate_slices", "Preallocate slices when the final size is known.", "go-1-21-development-standards", "S3", RulePairCompliant),
	}
}

func TestEvaluateRulePairsMetrics(t *testing.T) {
	fake := &rpDecider{probs: [][]float64{{0.9, 0.9}, {0.2}, {0.1}}, dropAnswers: -1}
	rep, err := EvaluateRulePairs(context.Background(), fake, "test-model", evaluateFixture(), 0.5)
	if err != nil {
		t.Fatalf("EvaluateRulePairs() error = %v, want nil", err)
	}
	if rep.Total != 4 || rep.Violating != 2 || rep.Compliant != 2 {
		t.Errorf("counts = (%d, %d, %d), want (4, 2, 2)", rep.Total, rep.Violating, rep.Compliant)
	}
	if rep.Accuracy != 0.5 {
		t.Errorf("Accuracy = %v, want 0.5", rep.Accuracy)
	}
	if rep.AUROC != 0.625 {
		t.Errorf("AUROC = %v, want 0.625", rep.AUROC)
	}
	if got := rep.ByLanguage["go"]; got != 0.5 {
		t.Errorf("ByLanguage[go] = %v, want 0.5", got)
	}
	if got := rep.ByLanguage["typescript"]; got != 0.5 {
		t.Errorf("ByLanguage[typescript] = %v, want 0.5", got)
	}
	if len(rep.ByLanguage) != 2 {
		t.Errorf("len(ByLanguage) = %d, want 2", len(rep.ByLanguage))
	}
	if want := []string{"P2", "P3"}; len(rep.Misses) != 2 || rep.Misses[0] != want[0] || rep.Misses[1] != want[1] {
		t.Errorf("Misses = %v, want [P2 P3]", rep.Misses)
	}
}

func TestEvaluateRulePairsProtocolAndBatching(t *testing.T) {
	fake := &rpDecider{probs: [][]float64{{0.9, 0.9}, {0.2}, {0.1}}, dropAnswers: -1}
	if _, err := EvaluateRulePairs(context.Background(), fake, "test-model", evaluateFixture(), 0.5); err != nil {
		t.Fatalf("EvaluateRulePairs() error = %v, want nil", err)
	}
	if len(fake.requests) != 3 {
		t.Fatalf("len(requests) = %d, want 3 (shared snippet batched into one request)", len(fake.requests))
	}
	first := fake.requests[0]
	if first.Model != "test-model" {
		t.Errorf("Model = %q, want test-model", first.Model)
	}
	if first.State != "S1" {
		t.Errorf("State = %q, want the shared snippet S1", first.State)
	}
	if strings.Contains(first.State, "No I/O") {
		t.Errorf("State = %q, want the premise text to stay out of the state", first.State)
	}
	if len(first.Questions) != 2 {
		t.Fatalf("len(Questions) = %d, want 2 questions for the shared snippet", len(first.Questions))
	}
	for _, i := range []int{0, 1} {
		q, ok := first.Questions[rulePairQuestionKey(i)]
		if !ok {
			t.Fatalf("request 0 missing question %q", rulePairQuestionKey(i))
		}
		wantInstr := "Project rule: No I/O operations inside loops.\n\nDoes this code violate the rule?"
		if q.Instructions != wantInstr {
			t.Errorf("question %d Instructions = %q, want %q", i, q.Instructions, wantInstr)
		}
		if q.Criteria.True != rulePairCriteriaTrue {
			t.Errorf("question %d criteria true = %q, want %q", i, q.Criteria.True, rulePairCriteriaTrue)
		}
		if q.Criteria.False != rulePairCriteriaFalse {
			t.Errorf("question %d criteria false = %q, want %q", i, q.Criteria.False, rulePairCriteriaFalse)
		}
		if strings.Contains(q.Instructions, "S1") {
			t.Errorf("question %d Instructions = %q, want the snippet to stay out of the instructions", i, q.Instructions)
		}
	}
}

func TestEvaluateRulePairsChunkingAt32Questions(t *testing.T) {
	var pairs []RulePair
	for i := 0; i < rulePairMaxQuestionsPerRequest+1; i++ {
		id := fmt.Sprintf("V%02d", i)
		label := RulePairViolating
		if i == rulePairMaxQuestionsPerRequest {
			id = "C01"
			label = RulePairCompliant
		}
		pairs = append(pairs, rpPair(id, "go", "p", "rule", "s", "shared snippet", label))
	}
	fake := &rpDecider{probs: [][]float64{make([]float64, rulePairMaxQuestionsPerRequest), {1.0}}, dropAnswers: -1}
	for i := range fake.probs[0] {
		fake.probs[0][i] = 1.0
	}
	rep, err := EvaluateRulePairs(context.Background(), fake, "m", pairs, 0.5)
	if err != nil {
		t.Fatalf("EvaluateRulePairs() error = %v, want nil", err)
	}
	if len(fake.requests) != 2 {
		t.Fatalf("len(requests) = %d, want 2 requests for 33 same-snippet pairs", len(fake.requests))
	}
	if len(fake.requests[0].Questions) != rulePairMaxQuestionsPerRequest {
		t.Errorf("first request questions = %d, want %d", len(fake.requests[0].Questions), rulePairMaxQuestionsPerRequest)
	}
	if len(fake.requests[1].Questions) != 1 {
		t.Errorf("second request questions = %d, want 1", len(fake.requests[1].Questions))
	}
	if rep.Accuracy != float64(rulePairMaxQuestionsPerRequest)/float64(rulePairMaxQuestionsPerRequest+1) {
		t.Errorf("Accuracy = %v, want %v", rep.Accuracy, float64(rulePairMaxQuestionsPerRequest)/float64(rulePairMaxQuestionsPerRequest+1))
	}
	if rep.AUROC != 0.5 {
		t.Errorf("AUROC = %v, want 0.5 (all scores tied)", rep.AUROC)
	}
	if want := []string{"C01"}; len(rep.Misses) != 1 || rep.Misses[0] != want[0] {
		t.Errorf("Misses = %v, want [C01]", rep.Misses)
	}
}

func TestEvaluateRulePairsThresholdBoundary(t *testing.T) {
	pairs := []RulePair{
		rpPair("PV", "go", "p", "rule", "s", "sv", RulePairViolating),
		rpPair("PC", "go", "p", "rule", "s", "sc", RulePairCompliant),
	}
	fake := &rpDecider{probs: [][]float64{{0.5}, {0.5}}, dropAnswers: -1}
	rep, err := EvaluateRulePairs(context.Background(), fake, "m", pairs, 0.5)
	if err != nil {
		t.Fatalf("EvaluateRulePairs() error = %v, want nil", err)
	}
	if rep.Accuracy != 0.5 {
		t.Errorf("Accuracy = %v, want 0.5 (P(yes) == threshold predicts violating)", rep.Accuracy)
	}
	if len(rep.Misses) != 1 || rep.Misses[0] != "PC" {
		t.Errorf("Misses = %v, want [PC]", rep.Misses)
	}
}

func TestEvaluateRulePairsDeciderErrorWrapsPairID(t *testing.T) {
	fake := &rpDecider{probs: [][]float64{{0.9, 0.9}}, err: errors.New("decider offline"), errCall: 1, dropAnswers: -1}
	_, err := EvaluateRulePairs(context.Background(), fake, "m", evaluateFixture(), 0.5)
	if err == nil {
		t.Fatal("EvaluateRulePairs() error = nil, want decider error")
	}
	if !errors.Is(err, fake.err) {
		t.Errorf("error = %v, want it to wrap the decider error", err)
	}
	// The failing call is the second request: pairs P3 (own snippet S2).
	if !strings.Contains(err.Error(), "P3") {
		t.Errorf("error = %v, want it to name the evaluated pair P3", err)
	}
}

func TestEvaluateRulePairsMissingAnswerNamesPairID(t *testing.T) {
	fake := &rpDecider{probs: [][]float64{{0.9}}, errCall: 0, dropAnswers: 1}
	// Single shared snippet, two pairs; question q1 is missing from the answer map.
	pairs := []RulePair{
		rpPair("PA", "go", "p", "rule", "s", "shared", RulePairViolating),
		rpPair("PB", "go", "p", "rule", "s", "shared", RulePairCompliant),
	}
	_, err := EvaluateRulePairs(context.Background(), fake, "m", pairs, 0.5)
	if err == nil {
		t.Fatal("EvaluateRulePairs() error = nil, want missing-answer error")
	}
	if !strings.Contains(err.Error(), "PB") {
		t.Errorf("error = %v, want it to name pair PB", err)
	}
	if !strings.Contains(err.Error(), `question "q1"`) {
		t.Errorf("error = %v, want it to name question q1", err)
	}
}

func TestEvaluateRulePairsEmptyInput(t *testing.T) {
	fake := &rpDecider{err: errors.New("must not be called"), dropAnswers: -1}
	rep, err := EvaluateRulePairs(context.Background(), fake, "m", nil, 0.5)
	if err != nil {
		t.Fatalf("EvaluateRulePairs() error = %v, want nil", err)
	}
	if len(fake.requests) != 0 {
		t.Errorf("requests sent = %d, want 0 for empty input", len(fake.requests))
	}
	if rep.Total != 0 || rep.Violating != 0 || rep.Compliant != 0 || rep.Accuracy != 0 || rep.AUROC != 0 {
		t.Errorf("report = %+v, want zero-valued counts and metrics", rep)
	}
	if rep.ByLanguage == nil || len(rep.ByLanguage) != 0 {
		t.Errorf("ByLanguage = %v, want empty non-nil map", rep.ByLanguage)
	}
	if len(rep.Misses) != 0 {
		t.Errorf("Misses = %v, want empty", rep.Misses)
	}
}

func TestEvaluateRulePairsInvalidClients(t *testing.T) {
	fake := &rpDecider{err: errors.New("must not be called"), dropAnswers: -1}
	if _, err := EvaluateRulePairs(context.Background(), nil, "m", evaluateFixture(), 0.5); err == nil {
		t.Error("EvaluateRulePairs(nil client) error = nil, want error")
	} else if !strings.Contains(err.Error(), "nil System One client") {
		t.Errorf("error = %v, want it to name the nil client", err)
	}
	if _, err := EvaluateRulePairs(context.Background(), fake, "", evaluateFixture(), 0.5); err == nil {
		t.Error("EvaluateRulePairs(empty model) error = nil, want error")
	} else if !strings.Contains(err.Error(), "empty model") {
		t.Errorf("error = %v, want it to name the empty model", err)
	}
	if len(fake.requests) != 0 {
		t.Errorf("requests sent = %d, want 0 on validation failure", len(fake.requests))
	}
}

func TestRulePairAUROCKnown(t *testing.T) {
	tests := []struct {
		name                 string
		positives, negatives []float64
		want                 float64
	}{
		{"perfect separation", []float64{0.9, 0.8}, []float64{0.1, 0.2}, 1.0},
		{"perfectly reversed", []float64{0.1, 0.2}, []float64{0.9, 0.8}, 0.0},
		{"all tied", []float64{0.5, 0.5}, []float64{0.5, 0.5}, 0.5},
		{"empty positives", nil, []float64{0.1}, 0.0},
		{"empty negatives", []float64{0.9}, nil, 0.0},
		{"both empty", nil, nil, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rulePairAUROC(tt.positives, tt.negatives); got != tt.want {
				t.Errorf("rulePairAUROC(%v, %v) = %v, want %v", tt.positives, tt.negatives, got, tt.want)
			}
		})
	}
}
