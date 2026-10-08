// Tests for the rules-compliance annotator: threshold behaviour, batching,
// option cap, skip budget, copy semantics, and error paths.
package review

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"stet/cli/internal/findings"
	"stet/cli/internal/rules"
	"stet/cli/internal/systemone"
)

// fakeChoiceDecider records DecideChoice requests and returns a canned
// response, a per-request respond func, or an error.
type fakeChoiceDecider struct {
	reqs    []systemone.ChoiceRequest
	resp    *systemone.ChoiceResponse
	respond func(req systemone.ChoiceRequest) *systemone.ChoiceResponse
	err     error
}

func (f *fakeChoiceDecider) DecideChoice(_ context.Context, req systemone.ChoiceRequest) (*systemone.ChoiceResponse, error) {
	f.reqs = append(f.reqs, req)
	if f.err != nil {
		return nil, f.err
	}
	if f.respond != nil {
		return f.respond(req), nil
	}
	return f.resp, nil
}

// annotateSections builds n distinct test sections.
func annotateSections(n int) []rules.Section {
	sections := make([]rules.Section, n)
	for i := range sections {
		sections[i] = rules.Section{
			Source:  fmt.Sprintf("rule-%02d.mdc", i),
			Heading: fmt.Sprintf("Section %02d", i),
			Content: fmt.Sprintf("## Section %02d\n\nbody %d", i, i),
		}
	}
	return sections
}

// annotateAnswer builds a ChoiceResponse with one answer per question in req.
func annotateAnswer(fn func(key string) (choice string, prob float64)) func(systemone.ChoiceRequest) *systemone.ChoiceResponse {
	return func(req systemone.ChoiceRequest) *systemone.ChoiceResponse {
		resp := &systemone.ChoiceResponse{Answers: make(map[string]systemone.ChoiceAnswer, len(req.Questions))}
		for key := range req.Questions {
			choice, prob := fn(key)
			resp.Answers[key] = systemone.ChoiceAnswer{
				Choice:        choice,
				Probabilities: map[string]float64{choice: prob},
			}
		}
		return resp
	}
}

func TestAnnotateRuleConflicts_setsConflictAboveThreshold(t *testing.T) {
	sections := annotateSections(3)
	var batch []findings.Finding
	for i := 0; i < 3; i++ {
		batch = append(batch, findings.Finding{
			ID:         fmt.Sprintf("f%d", i),
			File:       "a.go",
			Line:       i + 1,
			Severity:   findings.SeverityWarning,
			Category:   findings.CategoryBug,
			Confidence: 1.0,
			Message:    fmt.Sprintf("message %d", i),
			Suggestion: fmt.Sprintf("suggestion %d", i),
		})
	}
	fake := &fakeChoiceDecider{respond: annotateAnswer(func(key string) (string, float64) {
		switch key {
		case "q0":
			return "s2", 0.9
		case "q1":
			return "s0", annotateMinProbability // exactly at threshold annotates
		case "q2":
			return "s1", annotateMinProbability - 0.01 // below threshold does not
		}
		return annotateNoneOption, 1.0
	})}

	out, err := AnnotateRuleConflicts(context.Background(), fake, "test-model", batch, "hunk", sections)
	if err != nil {
		t.Fatalf("AnnotateRuleConflicts: %v", err)
	}
	if len(out) != len(batch) {
		t.Fatalf("len(out)=%d want %d", len(out), len(batch))
	}
	if out[0].RuleConflict == nil {
		t.Fatalf("finding 0 should be annotated")
	}
	want := findings.RuleConflict{Source: sections[2].Source, Section: sections[2].Heading, Probability: 0.9}
	if *out[0].RuleConflict != want {
		t.Errorf("finding 0 conflict = %+v, want %+v", *out[0].RuleConflict, want)
	}
	if out[1].RuleConflict == nil {
		t.Fatalf("finding 1 at exactly the threshold should be annotated")
	}
	if out[1].RuleConflict.Probability != annotateMinProbability || out[1].RuleConflict.Section != sections[0].Heading {
		t.Errorf("finding 1 conflict = %+v, want section %q at %g", *out[1].RuleConflict, sections[0].Heading, annotateMinProbability)
	}
	if out[2].RuleConflict != nil {
		t.Errorf("finding 2 below threshold should not be annotated, got %+v", *out[2].RuleConflict)
	}
}

func TestAnnotateRuleConflicts_noneNeverAnnotates(t *testing.T) {
	fake := &fakeChoiceDecider{respond: annotateAnswer(func(string) (string, float64) {
		return annotateNoneOption, 1.0
	})}
	batch := []findings.Finding{{File: "a.go", Severity: findings.SeverityInfo, Category: findings.CategoryStyle, Confidence: 1, Message: "m"}}
	out, err := AnnotateRuleConflicts(context.Background(), fake, "m", batch, "hunk", annotateSections(2))
	if err != nil {
		t.Fatalf("AnnotateRuleConflicts: %v", err)
	}
	if len(fake.reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(fake.reqs))
	}
	if out[0].RuleConflict != nil {
		t.Errorf("none answer must never annotate, got %+v", *out[0].RuleConflict)
	}
}

func TestAnnotateRuleConflicts_unknownOptionIDNeverAnnotates(t *testing.T) {
	fake := &fakeChoiceDecider{respond: annotateAnswer(func(string) (string, float64) {
		return "s99", 0.99 // outside the offered options
	})}
	batch := []findings.Finding{{File: "a.go", Severity: findings.SeverityInfo, Category: findings.CategoryStyle, Confidence: 1, Message: "m"}}
	out, err := AnnotateRuleConflicts(context.Background(), fake, "m", batch, "hunk", annotateSections(2))
	if err != nil {
		t.Fatalf("AnnotateRuleConflicts: %v", err)
	}
	if out[0].RuleConflict != nil {
		t.Errorf("unknown option ID must never annotate, got %+v", *out[0].RuleConflict)
	}
}

func TestAnnotateRuleConflicts_batchesAt32QuestionsPerRequest(t *testing.T) {
	const n = 40 // one over the per-request cap: 32 + 8
	batch := make([]findings.Finding, n)
	for i := range batch {
		batch[i] = findings.Finding{
			ID:         fmt.Sprintf("f%d", i),
			File:       "a.go",
			Severity:   findings.SeverityWarning,
			Category:   findings.CategoryBug,
			Confidence: 1,
			Message:    fmt.Sprintf("m%d", i),
		}
	}
	fake := &fakeChoiceDecider{respond: annotateAnswer(func(key string) (string, float64) {
		return annotateNoneOption, 1.0
	})}

	out, err := AnnotateRuleConflicts(context.Background(), fake, "m", batch, "hunk", annotateSections(2))
	if err != nil {
		t.Fatalf("AnnotateRuleConflicts: %v", err)
	}
	if len(fake.reqs) != 2 {
		t.Fatalf("requests = %d, want 2 (32 + 8)", len(fake.reqs))
	}
	if got := len(fake.reqs[0].Questions); got != annotateMaxQuestionsPerRequest {
		t.Errorf("first request questions = %d, want %d", got, annotateMaxQuestionsPerRequest)
	}
	if got := len(fake.reqs[1].Questions); got != n-annotateMaxQuestionsPerRequest {
		t.Errorf("second request questions = %d, want %d", got, n-annotateMaxQuestionsPerRequest)
	}
	// Keys use the batch index, so the second request continues at q32.
	if _, ok := fake.reqs[1].Questions["q32"]; !ok {
		t.Errorf("second request should carry question q32")
	}
	if _, ok := fake.reqs[0].Questions["q39"]; ok {
		t.Errorf("first request should not carry question q39")
	}
	if len(out) != n {
		t.Fatalf("len(out)=%d want %d", len(out), n)
	}
	for i := range out {
		if out[i].ID != batch[i].ID {
			t.Errorf("out[%d].ID=%q want %q (order changed)", i, out[i].ID, batch[i].ID)
		}
	}
}

func TestAnnotateRuleConflicts_overBudgetSkipsWithNoRequest(t *testing.T) {
	batch := make([]findings.Finding, annotateMaxQuestions+1) // 65 findings
	for i := range batch {
		batch[i] = findings.Finding{ID: fmt.Sprintf("f%d", i), File: "a.go", Severity: findings.SeverityInfo, Category: findings.CategoryStyle, Confidence: 1, Message: fmt.Sprintf("m%d", i)}
	}
	fake := &fakeChoiceDecider{respond: annotateAnswer(func(string) (string, float64) {
		return annotateNoneOption, 1.0
	})}

	out, err := AnnotateRuleConflicts(context.Background(), fake, "m", batch, "hunk", annotateSections(2))
	if err != nil {
		t.Fatalf("AnnotateRuleConflicts: %v", err)
	}
	if len(fake.reqs) != 0 {
		t.Fatalf("requests = %d, want 0 for an over-budget batch", len(fake.reqs))
	}
	if len(out) != len(batch) {
		t.Fatalf("len(out)=%d want %d", len(out), len(batch))
	}
	for i := range out {
		if !reflect.DeepEqual(out[i], batch[i]) {
			t.Errorf("out[%d] = %+v, want %+v", i, out[i], batch[i])
		}
	}
}

func TestAnnotateRuleConflicts_capsOptionsAt32PlusNone(t *testing.T) {
	for _, n := range []int{1, annotateMaxOptions, annotateMaxOptions + 8} {
		sections := annotateSections(n)
		fake := &fakeChoiceDecider{respond: annotateAnswer(func(string) (string, float64) {
			return annotateNoneOption, 1.0
		})}
		batch := []findings.Finding{{File: "a.go", Severity: findings.SeverityInfo, Category: findings.CategoryStyle, Confidence: 1, Message: "m"}}
		if _, err := AnnotateRuleConflicts(context.Background(), fake, "m", batch, "hunk", sections); err != nil {
			t.Fatalf("AnnotateRuleConflicts(%d sections): %v", n, err)
		}
		wantOptions := n
		if wantOptions > annotateMaxOptions {
			wantOptions = annotateMaxOptions
		}
		options := fake.reqs[0].Questions["q0"].Criteria
		if len(options) != wantOptions+1 {
			t.Errorf("%d sections: options = %d, want %d (+none)", n, len(options), wantOptions)
		}
		if _, ok := options[annotateOptionKey(wantOptions)]; ok {
			t.Errorf("%d sections: option key s%d should be capped away", n, wantOptions)
		}
		if _, ok := options[annotateOptionKey(wantOptions-1)]; !ok {
			t.Errorf("%d sections: option key s%d missing", n, wantOptions-1)
		}
		if options[annotateNoneOption] != annotateNoneDescription {
			t.Errorf("%d sections: none option = %q, want %q", n, options[annotateNoneOption], annotateNoneDescription)
		}
		if wantOptions > 0 {
			want := sections[0].Source + ": " + sections[0].Heading
			if options["s0"] != want {
				t.Errorf("%d sections: s0 = %q, want %q", n, options["s0"], want)
			}
		}
	}
}

func TestAnnotateRuleConflicts_inputSliceNotMutated(t *testing.T) {
	batch := []findings.Finding{
		{ID: "a", File: "a.go", Severity: findings.SeverityWarning, Category: findings.CategoryBug, Confidence: 1, Message: "one"},
		{ID: "b", File: "b.go", Severity: findings.SeverityError, Category: findings.CategorySecurity, Confidence: 1, Message: "two"},
	}
	input := make([]findings.Finding, len(batch))
	copy(input, batch)
	fetchedSections := annotateSections(1)
	fake := &fakeChoiceDecider{respond: annotateAnswer(func(key string) (string, float64) {
		if key == "q0" {
			return "s0", 0.8
		}
		return annotateNoneOption, 1.0
	})}

	out, err := AnnotateRuleConflicts(context.Background(), fake, "m", batch, "hunk", fetchedSections)
	if err != nil {
		t.Fatalf("AnnotateRuleConflicts: %v", err)
	}
	for i := range batch {
		if !reflect.DeepEqual(batch[i], input[i]) {
			t.Errorf("input[%d] mutated: got %+v want %+v", i, batch[i], input[i])
		}
	}
	if out[0].RuleConflict == nil {
		t.Fatalf("out[0] should carry the annotation")
	}
	if out[1].RuleConflict != nil {
		t.Errorf("out[1] should stay unannotated")
	}
}

func TestAnnotateRuleConflicts_outputLengthOrderAndOnlyRuleConflictChanged(t *testing.T) {
	batch := make([]findings.Finding, 5)
	for i := range batch {
		batch[i] = findings.Finding{
			ID:         fmt.Sprintf("id-%d", i),
			File:       fmt.Sprintf("f%d.go", i),
			Line:       i,
			Severity:   findings.SeverityWarning,
			Category:   findings.CategoryBug,
			Confidence: float64(i) / 10,
			Message:    fmt.Sprintf("msg %d", i),
			Suggestion: fmt.Sprintf("fix %d", i),
		}
	}
	fetchedSections := annotateSections(1)
	fake := &fakeChoiceDecider{respond: annotateAnswer(func(key string) (string, float64) {
		if key == "q3" {
			return "s0", 0.7
		}
		return annotateNoneOption, 1.0
	})}

	out, err := AnnotateRuleConflicts(context.Background(), fake, "m", batch, "hunk", fetchedSections)
	if err != nil {
		t.Fatalf("AnnotateRuleConflicts: %v", err)
	}
	if len(out) != len(batch) {
		t.Fatalf("len(out)=%d want %d", len(out), len(batch))
	}
	for i := range batch {
		want := batch[i]
		if i == 3 {
			want.RuleConflict = &findings.RuleConflict{Source: fetchedSections[0].Source, Section: fetchedSections[0].Heading, Probability: 0.7}
		}
		if !reflect.DeepEqual(out[i], want) {
			t.Errorf("out[%d] = %+v, want %+v", i, out[i], want)
		}
	}
}

func TestAnnotateRuleConflicts_errorWrappedWithContext(t *testing.T) {
	sentinel := errors.New("decision server unreachable")
	fake := &fakeChoiceDecider{err: sentinel}
	batch := []findings.Finding{{File: "a.go", Severity: findings.SeverityInfo, Category: findings.CategoryStyle, Confidence: 1, Message: "m"}}

	out, err := AnnotateRuleConflicts(context.Background(), fake, "m", batch, "hunk", annotateSections(1))
	if out != nil {
		t.Fatalf("out = %+v, want nil on error", out)
	}
	if err == nil {
		t.Fatal("err = nil, want wrapped decider error")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("err should wrap the sentinel: %v", err)
	}
	if !strings.Contains(err.Error(), "rules annotate") {
		t.Errorf("err should carry annotator context: %v", err)
	}
}

func TestAnnotateRuleConflicts_missingAnswerIsError(t *testing.T) {
	fake := &fakeChoiceDecider{resp: &systemone.ChoiceResponse{Answers: map[string]systemone.ChoiceAnswer{}}}
	batch := []findings.Finding{{File: "a.go", Severity: findings.SeverityInfo, Category: findings.CategoryStyle, Confidence: 1, Message: "m"}}

	out, err := AnnotateRuleConflicts(context.Background(), fake, "m", batch, "hunk", annotateSections(1))
	if out != nil {
		t.Fatalf("out = %+v, want nil on error", out)
	}
	if err == nil || !strings.Contains(err.Error(), "missing answer") {
		t.Fatalf("err = %v, want missing-answer error", err)
	}
}

func TestAnnotateRuleConflicts_skipConditionsReturnUnchangedCopy(t *testing.T) {
	batch := []findings.Finding{
		{ID: "a", File: "a.go", Severity: findings.SeverityWarning, Category: findings.CategoryBug, Confidence: 1, Message: "one"},
		{ID: "b", File: "b.go", Severity: findings.SeverityError, Category: findings.CategorySecurity, Confidence: 1, Message: "two"},
	}
	sections := annotateSections(2)

	tests := []struct {
		name    string
		client  rules.ChoiceDecider
		model   string
		batch   []findings.Finding
		section []rules.Section
	}{
		{name: "nil client", client: nil, model: "m", batch: batch, section: sections},
		{name: "empty model", client: &fakeChoiceDecider{}, model: "", batch: batch, section: sections},
		{name: "no sections", client: &fakeChoiceDecider{}, model: "m", batch: batch, section: nil},
		{name: "empty batch", client: &fakeChoiceDecider{}, model: "m", batch: nil, section: sections},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := AnnotateRuleConflicts(context.Background(), tt.client, tt.model, tt.batch, "hunk", tt.section)
			if err != nil {
				t.Fatalf("AnnotateRuleConflicts: %v", err)
			}
			if len(out) != len(tt.batch) {
				t.Fatalf("len(out)=%d want %d", len(out), len(tt.batch))
			}
			for i := range out {
				if !reflect.DeepEqual(out[i], tt.batch[i]) {
					t.Errorf("out[%d] = %+v, want %+v", i, out[i], tt.batch[i])
				}
			}
			if len(out) > 0 {
				out[0].Message = "mutated"
				if batch[0].Message == "mutated" {
					t.Errorf("skip path must return a copy, not the input slice")
				}
			}
		})
	}
	// No skip path may have issued a request.
	if decider, ok := tests[1].client.(*fakeChoiceDecider); ok && len(decider.reqs) != 0 {
		t.Errorf("empty-model skip issued %d requests, want 0", len(decider.reqs))
	}
}

func TestAnnotateRuleConflicts_stateAndInstructionsShape(t *testing.T) {
	hunk := "func main() {\n\tfmt.Println()\n}"
	batch := []findings.Finding{{
		ID:         "f0",
		File:       "a.go",
		Severity:   findings.SeverityWarning,
		Category:   findings.CategoryPerformance,
		Confidence: 1,
		Message:    "I/O in loop",
		Suggestion: "Batch the reads",
	}}
	fake := &fakeChoiceDecider{respond: annotateAnswer(func(string) (string, float64) {
		return annotateNoneOption, 1.0
	})}

	if _, err := AnnotateRuleConflicts(context.Background(), fake, "m", batch, hunk, annotateSections(1)); err != nil {
		t.Fatalf("AnnotateRuleConflicts: %v", err)
	}
	req := fake.reqs[0]
	if req.Model != "m" {
		t.Errorf("model = %q, want m", req.Model)
	}
	wantState := "Code under review:\n" + hunk
	if req.State != wantState {
		t.Errorf("state = %q, want %q", req.State, wantState)
	}
	// Hunk content must never leak into instructions.
	q := req.Questions["q0"]
	if strings.Contains(q.Instructions, "func main") {
		t.Errorf("instructions must not carry hunk content: %q", q.Instructions)
	}
	if !strings.Contains(q.Instructions, "I/O in loop") || !strings.Contains(q.Instructions, "Batch the reads") {
		t.Errorf("instructions should carry the finding message and suggestion: %q", q.Instructions)
	}
}

func TestAnnotateRuleConflicts_longHunkTruncatedInState(t *testing.T) {
	hunk := strings.Repeat("x", defaultCriticMaxHunkLen+1000)
	batch := []findings.Finding{{File: "a.go", Severity: findings.SeverityInfo, Category: findings.CategoryStyle, Confidence: 1, Message: "m"}}
	fake := &fakeChoiceDecider{respond: annotateAnswer(func(string) (string, float64) {
		return annotateNoneOption, 1.0
	})}

	if _, err := AnnotateRuleConflicts(context.Background(), fake, "m", batch, hunk, annotateSections(1)); err != nil {
		t.Fatalf("AnnotateRuleConflicts: %v", err)
	}
	want := "Code under review:\n" + strings.Repeat("x", defaultCriticMaxHunkLen) + "\n[truncated]"
	if fake.reqs[0].State != want {
		t.Errorf("state should truncate the hunk at %d chars with the [truncated] marker (len %d, want %d)", defaultCriticMaxHunkLen, len(fake.reqs[0].State), len(want))
	}
}
