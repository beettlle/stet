package rules

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"stet/cli/internal/systemone"
)

func TestSplitSections(t *testing.T) {
	tests := []struct {
		name string
		rule CursorRule
		want []Section
	}{
		{
			name: "preamble only before any heading",
			rule: CursorRule{Source: "rule.mdc", Content: "just text, no headings"},
			want: []Section{
				{Source: "rule.mdc", Heading: "rule.mdc", Content: "just text, no headings"},
			},
		},
		{
			name: "empty content yields no sections",
			rule: CursorRule{Source: "rule.mdc", Content: ""},
			want: nil,
		},
		{
			name: "level 2 and 3 headings split, preamble kept with Source heading",
			rule: CursorRule{
				Source:  "rule.mdc",
				Content: "intro text\n\n## First\n\nfirst body\n\n### Nested\n\nnested body",
			},
			want: []Section{
				{Source: "rule.mdc", Heading: "rule.mdc", Content: "intro text"},
				{Source: "rule.mdc", Heading: "First", Content: "## First\n\nfirst body"},
				{Source: "rule.mdc", Heading: "Nested", Content: "### Nested\n\nnested body"},
			},
		},
		{
			name: "empty preamble dropped",
			rule: CursorRule{
				Source:  "rule.mdc",
				Content: "## Only\n\nbody",
			},
			want: []Section{
				{Source: "rule.mdc", Heading: "Only", Content: "## Only\n\nbody"},
			},
		},
		{
			name: "heading inside code fence is not a split point",
			rule: CursorRule{
				Source:  "rule.mdc",
				Content: "## Real\n\n```go\n## Not a heading\nx := 1\n```\n\nafter fence",
			},
			want: []Section{
				{Source: "rule.mdc", Heading: "Real", Content: "## Real\n\n```go\n## Not a heading\nx := 1\n```\n\nafter fence"},
			},
		},
		{
			name: "level 4 heading stays in body",
			rule: CursorRule{
				Source:  "rule.mdc",
				Content: "## A\n\n#### sub stays in body\n\nmore",
			},
			want: []Section{
				{Source: "rule.mdc", Heading: "A", Content: "## A\n\n#### sub stays in body\n\nmore"},
			},
		},
		{
			name: "whitespace-only section dropped",
			rule: CursorRule{
				Source:  "rule.mdc",
				Content: "## A\n\nbody\n\n   \n\n## B\n\nb body",
			},
			want: []Section{
				{Source: "rule.mdc", Heading: "A", Content: "## A\n\nbody"},
				{Source: "rule.mdc", Heading: "B", Content: "## B\n\nb body"},
			},
		},
		{
			name: "heading with no body kept with heading line as content",
			rule: CursorRule{
				Source:  "rule.mdc",
				Content: "## A\nbody\n## B",
			},
			want: []Section{
				{Source: "rule.mdc", Heading: "A", Content: "## A\nbody"},
				{Source: "rule.mdc", Heading: "B", Content: "## B"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitSections(tt.rule)
			if len(got) != len(tt.want) {
				t.Fatalf("SplitSections returned %d sections, want %d: %+v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("section[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// fakeChoiceDecider records DecideChoice requests and returns a canned response.
type fakeChoiceDecider struct {
	reqs []systemone.ChoiceRequest
	resp *systemone.ChoiceResponse
	err  error
}

func (f *fakeChoiceDecider) DecideChoice(ctx context.Context, req systemone.ChoiceRequest) (*systemone.ChoiceResponse, error) {
	f.reqs = append(f.reqs, req)
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func TestSelectSections_singleRequestRanksByProbability(t *testing.T) {
	hunk := "func Add(a, b int) int { return a + b }"
	sections := []Section{
		{Source: "a.mdc", Heading: "Alpha", Content: strings.Repeat("a", 40)},
		{Source: "b.mdc", Heading: "Beta", Content: strings.Repeat("b", 40)},
	}
	fake := &fakeChoiceDecider{resp: &systemone.ChoiceResponse{Answers: map[string]systemone.ChoiceAnswer{
		"q0": {Choice: "s1", Probabilities: map[string]float64{"s0": 0.2, "s1": 0.8}},
	}}}
	got, err := SelectSections(context.Background(), fake, "test-model", hunk, sections, 1000)
	if err != nil {
		t.Fatalf("SelectSections: %v", err)
	}
	if len(fake.reqs) != 1 {
		t.Fatalf("DecideChoice called %d times, want exactly 1", len(fake.reqs))
	}
	req := fake.reqs[0]
	if req.Model != "test-model" {
		t.Errorf("Model = %q, want test-model", req.Model)
	}
	if req.State != "Code under review:\n"+hunk {
		t.Errorf("State = %q, want prefixed hunk content", req.State)
	}
	if len(req.Questions) != 1 {
		t.Fatalf("Questions count = %d, want 1", len(req.Questions))
	}
	q, ok := req.Questions["q0"]
	if !ok {
		t.Fatalf("Questions missing key %q", "q0")
	}
	if len(q.Criteria) != 2 {
		t.Fatalf("Criteria options = %d, want 2", len(q.Criteria))
	}
	if q.Criteria["s0"] != "a.mdc: Alpha" {
		t.Errorf("Criteria[s0] = %q, want %q", q.Criteria["s0"], "a.mdc: Alpha")
	}
	if q.Criteria["s1"] != "b.mdc: Beta" {
		t.Errorf("Criteria[s1] = %q, want %q", q.Criteria["s1"], "b.mdc: Beta")
	}
	// Security: hunk content must never appear in instructions or option descriptions.
	if strings.Contains(q.Instructions, hunk) {
		t.Errorf("Instructions contain hunk content")
	}
	for _, desc := range q.Criteria {
		if strings.Contains(desc, hunk) {
			t.Errorf("Option description %q contains hunk content", desc)
		}
	}
	if len(got) != 2 {
		t.Fatalf("SelectSections returned %d sections, want 2", len(got))
	}
	if got[0].Heading != "Beta" || got[1].Heading != "Alpha" {
		t.Errorf("ranked order = [%s, %s], want [Beta, Alpha]", got[0].Heading, got[1].Heading)
	}
}

func TestSelectSections_tiesBrokenByInputOrder(t *testing.T) {
	sections := []Section{
		{Source: "a.mdc", Heading: "S0", Content: "aaaa"},
		{Source: "a.mdc", Heading: "S1", Content: "bbbb"},
		{Source: "a.mdc", Heading: "S2", Content: "cccc"},
		{Source: "a.mdc", Heading: "S3", Content: "dddd"},
	}
	fake := &fakeChoiceDecider{resp: &systemone.ChoiceResponse{Answers: map[string]systemone.ChoiceAnswer{
		"q0": {Choice: "s1", Probabilities: map[string]float64{
			"s0": 0.5, "s1": 0.9, "s2": 0.5, "s3": 0.1,
		}},
	}}}
	got, err := SelectSections(context.Background(), fake, "m", "hunk", sections, 1000)
	if err != nil {
		t.Fatalf("SelectSections: %v", err)
	}
	var headings []string
	for _, s := range got {
		headings = append(headings, s.Heading)
	}
	want := []string{"S1", "S0", "S2", "S3"}
	if strings.Join(headings, ",") != strings.Join(want, ",") {
		t.Errorf("ranked order = %v, want %v", headings, want)
	}
}

func TestSelectSections_budgetSkipsOversizedAndContinues(t *testing.T) {
	sections := []Section{
		{Source: "a.mdc", Heading: "Big", Content: strings.Repeat("x", 80)},   // 20 tokens
		{Source: "a.mdc", Heading: "Mid", Content: strings.Repeat("y", 40)},   // 10 tokens
		{Source: "a.mdc", Heading: "Small", Content: strings.Repeat("z", 20)}, // 5 tokens
	}
	fake := &fakeChoiceDecider{resp: &systemone.ChoiceResponse{Answers: map[string]systemone.ChoiceAnswer{
		"q0": {Choice: "s0", Probabilities: map[string]float64{"s0": 0.9, "s1": 0.5, "s2": 0.1}},
	}}}
	// Budget 25: Big (20) fits; Mid (10) would exceed (30) and is skipped;
	// Small (5) still fits (25 <= 25).
	got, err := SelectSections(context.Background(), fake, "m", "hunk", sections, 25)
	if err != nil {
		t.Fatalf("SelectSections: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("SelectSections returned %d sections, want 2: %+v", len(got), got)
	}
	if got[0].Heading != "Big" || got[1].Heading != "Small" {
		t.Errorf("kept = [%s, %s], want [Big, Small]", got[0].Heading, got[1].Heading)
	}
}

func TestSelectSections_optionCapAt64(t *testing.T) {
	var sections []Section
	for i := 0; i < 70; i++ {
		sections = append(sections, Section{
			Source:  "many.mdc",
			Heading: fmt.Sprintf("S%d", i),
			Content: fmt.Sprintf("section %d", i),
		})
	}
	fake := &fakeChoiceDecider{resp: &systemone.ChoiceResponse{Answers: map[string]systemone.ChoiceAnswer{
		"q0": {Choice: "s0"},
	}}}
	got, err := SelectSections(context.Background(), fake, "m", "hunk", sections, 100000)
	if err != nil {
		t.Fatalf("SelectSections: %v", err)
	}
	if len(fake.reqs) != 1 {
		t.Fatalf("DecideChoice called %d times, want 1", len(fake.reqs))
	}
	criteria := fake.reqs[0].Questions["q0"].Criteria
	if len(criteria) != maxSelectorOptions {
		t.Fatalf("Criteria options = %d, want %d", len(criteria), maxSelectorOptions)
	}
	for i := 0; i < maxSelectorOptions; i++ {
		if _, ok := criteria[selectorOptionKey(i)]; !ok {
			t.Errorf("Criteria missing option %q", selectorOptionKey(i))
		}
	}
	if _, ok := criteria[selectorOptionKey(maxSelectorOptions)]; ok {
		t.Errorf("Criteria contains capped-out option %q", selectorOptionKey(maxSelectorOptions))
	}
	// Capped-out sections are not candidates, so they never appear in the result.
	for _, s := range got {
		if s.Heading == "S64" {
			t.Errorf("capped-out section S64 appeared in result")
		}
	}
}

func TestSelectSections_inputValidation(t *testing.T) {
	sections := []Section{{Source: "a.mdc", Heading: "A", Content: "aaaa"}}
	tests := []struct {
		name     string
		client   ChoiceDecider
		model    string
		sections []Section
		wantSub  string
	}{
		{name: "nil client", client: nil, model: "m", sections: sections, wantSub: "nil choice decider"},
		{name: "empty model", client: &fakeChoiceDecider{}, model: "", sections: sections, wantSub: "empty model"},
		{name: "no sections", client: &fakeChoiceDecider{}, model: "m", sections: nil, wantSub: "no sections"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SelectSections(context.Background(), tt.client, tt.model, "hunk", tt.sections, 1000)
			if err == nil {
				t.Fatal("SelectSections: want error")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantSub)
			}
			// The decider must never be invoked for invalid input.
			if f, ok := tt.client.(*fakeChoiceDecider); ok && len(f.reqs) != 0 {
				t.Errorf("DecideChoice called %d times for invalid input, want 0", len(f.reqs))
			}
		})
	}
}

func TestSelectSections_deciderErrorWrapped(t *testing.T) {
	sections := []Section{{Source: "a.mdc", Heading: "A", Content: "aaaa"}}
	fake := &fakeChoiceDecider{err: errors.New("boom")}
	_, err := SelectSections(context.Background(), fake, "m", "hunk", sections, 1000)
	if err == nil {
		t.Fatal("SelectSections: want error")
	}
	if !strings.Contains(err.Error(), "rules select") || !strings.Contains(err.Error(), "boom") {
		t.Errorf("error %q should wrap context and cause", err.Error())
	}
}

func TestSelectSections_missingAnswer(t *testing.T) {
	sections := []Section{{Source: "a.mdc", Heading: "A", Content: "aaaa"}}
	fake := &fakeChoiceDecider{resp: &systemone.ChoiceResponse{Answers: map[string]systemone.ChoiceAnswer{}}}
	_, err := SelectSections(context.Background(), fake, "m", "hunk", sections, 1000)
	if err == nil {
		t.Fatal("SelectSections: want error for missing answer")
	}
	if !strings.Contains(err.Error(), "missing answer") {
		t.Errorf("error %q should mention missing answer", err.Error())
	}
}

func TestSelectSections_stateTruncation(t *testing.T) {
	sections := []Section{{Source: "a.mdc", Heading: "A", Content: "aaaa"}}
	fake := &fakeChoiceDecider{resp: &systemone.ChoiceResponse{Answers: map[string]systemone.ChoiceAnswer{
		"q0": {Choice: "s0", Probabilities: map[string]float64{"s0": 1}},
	}}}
	hunk := strings.Repeat("x", selectorMaxHunkLen+100)
	if _, err := SelectSections(context.Background(), fake, "m", hunk, sections, 1000); err != nil {
		t.Fatalf("SelectSections: %v", err)
	}
	want := "Code under review:\n" + strings.Repeat("x", selectorMaxHunkLen) + "\n[truncated]"
	if fake.reqs[0].State != want {
		t.Errorf("State length = %d, want truncated state of length %d", len(fake.reqs[0].State), len(want))
	}
}

func TestSelectSections_zeroBudgetSelectsNothing(t *testing.T) {
	sections := []Section{{Source: "a.mdc", Heading: "A", Content: "aaaa"}}
	fake := &fakeChoiceDecider{resp: &systemone.ChoiceResponse{Answers: map[string]systemone.ChoiceAnswer{
		"q0": {Choice: "s0", Probabilities: map[string]float64{"s0": 1}},
	}}}
	got, err := SelectSections(context.Background(), fake, "m", "hunk", sections, 0)
	if err != nil {
		t.Fatalf("SelectSections: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("SelectSections returned %d sections with zero budget, want 0", len(got))
	}
}
