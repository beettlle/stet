package review

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"stet/cli/internal/diff"
	"stet/cli/internal/ollama"
	"stet/cli/internal/trace"
)

// scriptedClient returns queued results for continuation (GenerateWithMessages)
// and retry (Generate) calls and records the completion cap each call received.
type scriptedClient struct {
	continuations []*ollama.GenerateResult
	retries       []*ollama.GenerateResult
	contCaps      []int
	retryCaps     []int
}

func (c *scriptedClient) Check(ctx context.Context, model string) (*ollama.CheckResult, error) {
	return &ollama.CheckResult{Reachable: true, ModelPresent: true}, nil
}

func capOf(opts *ollama.GenerateOptions) int {
	if opts == nil {
		return -1
	}
	return opts.MaxCompletionTokens
}

func (c *scriptedClient) Generate(ctx context.Context, model, systemPrompt, userPrompt string, opts *ollama.GenerateOptions) (*ollama.GenerateResult, error) {
	c.retryCaps = append(c.retryCaps, capOf(opts))
	if len(c.retries) == 0 {
		return &ollama.GenerateResult{Response: "[]", DoneReason: "stop"}, nil
	}
	r := c.retries[0]
	c.retries = c.retries[1:]
	return r, nil
}

func (c *scriptedClient) GeneratePlain(ctx context.Context, model, systemPrompt, userPrompt string, opts *ollama.GenerateOptions) (*ollama.GenerateResult, error) {
	return c.Generate(ctx, model, systemPrompt, userPrompt, opts)
}

func (c *scriptedClient) GenerateWithMessages(ctx context.Context, model string, messages []ollama.Message, opts *ollama.GenerateOptions) (*ollama.GenerateResult, error) {
	c.contCaps = append(c.contCaps, capOf(opts))
	if len(c.continuations) == 0 {
		return &ollama.GenerateResult{Response: "", DoneReason: "length"}, nil
	}
	r := c.continuations[0]
	c.continuations = c.continuations[1:]
	return r, nil
}

const validFindingObject = `{"file":"a.go","line":3,"severity":"error","category":"bug","confidence":1.0,"message":"race on counts"}`

func countingLoop(from, to int) string {
	var b strings.Builder
	for n := from; n <= to; n++ {
		b.WriteString(strconv.Itoa(n))
		b.WriteByte(',')
	}
	return b.String()
}

func TestIsDegenerateTail(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"counting_loop", `[{"file":"a.go","evidence_lines":[` + countingLoop(44, 400), true},
		{"short_integer_list", `[{"file":"a.go","evidence_lines":[1,2,3,`, false},
		{"repeated_ngram", `[{"message":"` + strings.Repeat("the lock is ", 20), true},
		{"repeated_ngram_trailing_space", strings.Repeat("abcdefgh", 9) + "  \n", true},
		{"too_few_repeats", "xyz" + strings.Repeat("abcdefgh", 7), false},
		{"normal_prose_cutoff", `[{"file":"a.go","line":1,"severity":"warning","category":"bug","message":"the value may be nil when the caller`, false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isDegenerateTail(tt.in); got != tt.want {
				t.Errorf("isDegenerateTail = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDegenerateTail_onlyInspectsWindow(t *testing.T) {
	t.Parallel()
	loop := "[" + countingLoop(1, 2000)
	if !isDegenerateTail(loop) {
		t.Fatal("long counting loop should be degenerate")
	}
	if isDegenerateTail(loop + "the model recovered and wrote a normal closing sentence.") {
		t.Error("tail ending in prose should not be degenerate")
	}
	if isDegenerateTail(loop + strings.Repeat("-", degenerateTailWindowBytes/2) + " then a closing sentence that does not repeat.") {
		t.Error("loop pushed out of the window should not count")
	}
}

func TestSalvageTruncatedFindings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		in        string
		wantCount int
		wantOK    bool
	}{
		{"one_complete_then_loop", "```json\n[" + validFindingObject + `,{"file":"a.go","evidence_lines":[` + countingLoop(44, 200), 1, true},
		{"loop_inside_first_object", `[{"file":"a.go","line":50,"severity":"error","category":"bug","message":"race","evidence_lines":[` + countingLoop(44, 200), 0, false},
		{"two_complete", "[" + validFindingObject + ",\n  " + validFindingObject + ",{\"file\":", 2, true},
		{"string_with_braces", `[{"file":"a.go","line":1,"severity":"error","category":"bug","message":"use } and { \" carefully"},{"file"`, 1, true},
		{"prose_bracket_before_array", "Lines [1-3] show:\n[" + validFindingObject + ",{", 1, true},
		{"no_array", "I could not decide.", 0, false},
		{"empty_message_dropped", `[{"file":"a.go","line":1,"severity":"error","category":"bug","message":""},{"fi`, 0, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			list, ok := salvageTruncatedFindings(tt.in)
			if ok != tt.wantOK || len(list) != tt.wantCount {
				t.Fatalf("salvage = (%d findings, %v), want (%d, %v): %+v", len(list), ok, tt.wantCount, tt.wantOK, list)
			}
		})
	}
}

func TestProcessReviewResponse_degenerateTail_skipsContinuationAndSalvages(t *testing.T) {
	client := &scriptedClient{}
	first := &ollama.GenerateResult{
		Response:   "```json\n[" + validFindingObject + `,{"file":"a.go","evidence_lines":[` + countingLoop(44, 1865),
		DoneReason: "length",
		EvalCount:  8191,
	}
	hunk := diff.Hunk{FilePath: "a.go", RawContent: "code", Context: "code"}
	var traceBuf strings.Builder
	opts := &ollama.GenerateOptions{MaxCompletionTokens: 8192}
	list, usage, err := ProcessReviewResponse(context.Background(), first, hunk, client, "m", "sys", "user", opts, trace.New(&traceBuf))
	if err != nil {
		t.Fatalf("ProcessReviewResponse: %v", err)
	}
	if len(client.contCaps) != 0 || len(client.retryCaps) != 0 {
		t.Errorf("calls: continuations=%d retries=%d, want 0 and 0", len(client.contCaps), len(client.retryCaps))
	}
	if len(list) != 1 || list[0].Message != "race on counts" {
		t.Fatalf("list = %+v, want the salvaged finding", list)
	}
	if usage.EvalCount != 8191 {
		t.Errorf("usage.EvalCount = %d, want 8191", usage.EvalCount)
	}
	for _, want := range []string{"skipped: degenerate tail", "salvaged 1 complete finding"} {
		if !strings.Contains(traceBuf.String(), want) {
			t.Errorf("trace missing %q:\n%s", want, traceBuf.String())
		}
	}
}

func TestProcessReviewResponse_emptyContentLength_noContinuation(t *testing.T) {
	client := &scriptedClient{retries: []*ollama.GenerateResult{{Response: "[]", DoneReason: "stop", EvalCount: 2}}}
	first := &ollama.GenerateResult{Response: "  \n", DoneReason: "length", EvalCount: 4096}
	hunk := diff.Hunk{FilePath: "a.go", RawContent: "code", Context: "code"}
	opts := &ollama.GenerateOptions{MaxCompletionTokens: 4096}
	list, usage, err := ProcessReviewResponse(context.Background(), first, hunk, client, "m", "sys", "user", opts, nil)
	if err != nil {
		t.Fatalf("ProcessReviewResponse: %v", err)
	}
	if len(client.contCaps) != 0 {
		t.Errorf("continuations = %d, want 0 for empty content", len(client.contCaps))
	}
	if len(client.retryCaps) != 1 || client.retryCaps[0] != 4096 {
		t.Errorf("retry caps = %v, want [4096]", client.retryCaps)
	}
	if len(list) != 0 {
		t.Errorf("list = %+v, want empty", list)
	}
	if usage.EvalCount != 4096+2 {
		t.Errorf("usage.EvalCount = %d, want %d (first + retry)", usage.EvalCount, 4096+2)
	}
}

func TestProcessReviewResponse_budgetLimitsContinuationsAndRetry(t *testing.T) {
	const capTokens = 1000
	client := &scriptedClient{
		continuations: []*ollama.GenerateResult{
			{Response: `"line":1,"severity":"warning","category":"bug","message":"cut`, DoneReason: "length", EvalCount: capTokens},
		},
	}
	first := &ollama.GenerateResult{Response: `[{"file":"a.go",`, DoneReason: "length", EvalCount: capTokens}
	hunk := diff.Hunk{FilePath: "a.go", RawContent: "code", Context: "code"}
	opts := &ollama.GenerateOptions{MaxCompletionTokens: capTokens, Temperature: 0.1}
	_, _, err := ProcessReviewResponse(context.Background(), first, hunk, client, "m", "sys", "user", opts, nil)
	if err == nil {
		t.Fatal("want error once the per-hunk output budget is spent")
	}
	if !strings.Contains(err.Error(), "budget exhausted") {
		t.Errorf("error = %v, want budget exhausted", err)
	}
	if len(client.contCaps) != 1 || client.contCaps[0] != capTokens {
		t.Errorf("continuation caps = %v, want [%d]", client.contCaps, capTokens)
	}
	if len(client.retryCaps) != 0 {
		t.Errorf("retries = %d, want 0 (budget %d spent)", len(client.retryCaps), hunkOutputBudgetMultiplier*capTokens)
	}
	if opts.MaxCompletionTokens != capTokens {
		t.Errorf("caller opts mutated: MaxCompletionTokens = %d", opts.MaxCompletionTokens)
	}
}

func TestProcessReviewResponse_retryCapLimitedToRemainingBudget(t *testing.T) {
	const capTokens = 1000
	client := &scriptedClient{
		continuations: []*ollama.GenerateResult{{Response: "garbage", DoneReason: "stop", EvalCount: 400}},
		retries:       []*ollama.GenerateResult{{Response: "[" + validFindingObject + "]", DoneReason: "stop", EvalCount: 50}},
	}
	first := &ollama.GenerateResult{Response: `[{"file":"a.go",`, DoneReason: "length", EvalCount: capTokens}
	hunk := diff.Hunk{FilePath: "a.go", RawContent: "code", Context: "code"}
	opts := &ollama.GenerateOptions{MaxCompletionTokens: capTokens}
	list, usage, err := ProcessReviewResponse(context.Background(), first, hunk, client, "m", "sys", "user", opts, nil)
	if err != nil {
		t.Fatalf("ProcessReviewResponse: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list = %+v, want 1 finding from retry", list)
	}
	if len(client.retryCaps) != 1 || client.retryCaps[0] != 2*capTokens-capTokens-400 {
		t.Errorf("retry caps = %v, want [%d]", client.retryCaps, 2*capTokens-capTokens-400)
	}
	if usage.EvalCount != capTokens+400+50 {
		t.Errorf("usage.EvalCount = %d, want %d", usage.EvalCount, capTokens+400+50)
	}
}

func TestHunkOutputBudget_nilOptsStayNil(t *testing.T) {
	t.Parallel()
	b := newHunkOutputBudget(nil, 6000)
	if b.capTokens != defaultCompletionCapTokens {
		t.Errorf("capTokens = %d, want %d", b.capTokens, defaultCompletionCapTokens)
	}
	if got := b.followUpOptions(nil); got != nil {
		t.Errorf("followUpOptions(nil) = %+v, want nil", got)
	}
	if !b.canFollowUp() {
		t.Error("2192 tokens remain; follow-up should be allowed")
	}
	b.spend(2000)
	if b.canFollowUp() {
		t.Errorf("remaining %d < %d; follow-up should be refused", b.remaining(), minFollowUpTokens)
	}
}
