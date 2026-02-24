package review

import (
	"context"
	"errors"
	"strings"
	"testing"

	"stet/cli/internal/findings"
	"stet/cli/internal/ollama"
)

func TestBuildCriticPrompt_containsFindingAndHunk(t *testing.T) {
	f := findings.Finding{
		File:     "pkg/foo.go",
		Line:     42,
		Severity: findings.SeverityWarning,
		Category: findings.CategoryCorrectness,
		Message:  "Possible nil dereference",
		Suggestion: "Add a nil check before use",
	}
	hunk := "func Foo() {\n\tx := bar()\n\tprintln(x.Y)\n}\n"
	prompt := BuildCriticPrompt(f, hunk, 0)
	if !strings.Contains(prompt, "pkg/foo.go") {
		t.Error("prompt should contain file path")
	}
	if !strings.Contains(prompt, "line 42") {
		t.Error("prompt should contain line")
	}
	if !strings.Contains(prompt, "warning") {
		t.Error("prompt should contain severity")
	}
	if !strings.Contains(prompt, "correctness") {
		t.Error("prompt should contain category")
	}
	if !strings.Contains(prompt, "Possible nil dereference") {
		t.Error("prompt should contain message")
	}
	if !strings.Contains(prompt, "Add a nil check before use") {
		t.Error("prompt should contain suggestion")
	}
	if !strings.Contains(prompt, "func Foo()") {
		t.Error("prompt should contain hunk content")
	}
	if !strings.Contains(prompt, "Is this finding correct and actionable") {
		t.Error("prompt should contain instruction")
	}
}

func TestBuildCriticPrompt_rangeLocation(t *testing.T) {
	f := findings.Finding{
		File:   "a.go",
		Line:   10,
		Range:  &findings.LineRange{Start: 10, End: 15},
	}
	prompt := BuildCriticPrompt(f, "code", 0)
	if !strings.Contains(prompt, "lines 10-15") {
		t.Errorf("prompt should contain range location, got: %s", prompt)
	}
}

func TestBuildCriticPrompt_truncatesLongHunk(t *testing.T) {
	f := findings.Finding{File: "x", Line: 1, Severity: findings.SeverityInfo, Category: findings.CategoryStyle, Message: "msg"}
	hunk := strings.Repeat("x", 5000)
	maxLen := 100
	prompt := BuildCriticPrompt(f, hunk, maxLen)
	if !strings.Contains(prompt, "[truncated]") {
		t.Error("prompt should contain [truncated] when hunk exceeds max")
	}
	if len(hunk) <= maxLen {
		t.Error("test hunk should be longer than maxLen")
	}
	// Prompt should have at most maxLen chars of hunk plus "[truncated]"
	idx := strings.Index(prompt, "```")
	if idx >= 0 {
		codeBlock := prompt[idx:]
		if len(codeBlock) > maxLen+20 && !strings.Contains(codeBlock, "[truncated]") {
			t.Error("code block should be truncated")
		}
	}
}

func TestParseCriticVerdict_yes(t *testing.T) {
	if !ParseCriticVerdict(`{"verdict":"yes"}`) {
		t.Error("verdict yes should keep")
	}
	if !ParseCriticVerdict(`{"verdict":"yes","reason":"valid"}`) {
		t.Error("verdict yes with reason should keep")
	}
	if !ParseCriticVerdict(`  {"verdict": "YES"}  `) {
		t.Error("verdict YES (case-insensitive) should keep")
	}
}

func TestParseCriticVerdict_no(t *testing.T) {
	if ParseCriticVerdict(`{"verdict":"no"}`) {
		t.Error("verdict no should drop")
	}
	if ParseCriticVerdict(`{"verdict":"no","reason":"not actionable"}`) {
		t.Error("verdict no with reason should drop")
	}
	if ParseCriticVerdict(`{"verdict":"NO"}`) {
		t.Error("verdict NO should drop")
	}
}

func TestParseCriticVerdict_invalidOrEmpty(t *testing.T) {
	if ParseCriticVerdict("") {
		t.Error("empty should drop")
	}
	if ParseCriticVerdict("not json") {
		t.Error("invalid JSON should drop")
	}
	if ParseCriticVerdict(`{}`) {
		t.Error("missing verdict should drop")
	}
	if ParseCriticVerdict(`{"reason":"only"}`) {
		t.Error("missing verdict key should drop")
	}
}

// fakeCriticClient returns a fixed response for testing.
type fakeCriticClient struct {
	response string
	err     error
}

func (f *fakeCriticClient) Generate(ctx context.Context, model, systemPrompt, userPrompt string, opts *ollama.GenerateOptions) (*ollama.GenerateResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &ollama.GenerateResult{Response: f.response}, nil
}

func TestVerifyFinding_keepWhenYes(t *testing.T) {
	ctx := context.Background()
	client := &fakeCriticClient{response: `{"verdict":"yes","reason":"ok"}`}
	f := findings.Finding{File: "a.go", Line: 1, Severity: findings.SeverityError, Category: findings.CategoryBug, Message: "bug"}
	keep, err := VerifyFinding(ctx, client, "model", f, "code", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !keep {
		t.Error("expected keep true when verdict yes")
	}
}

func TestVerifyFinding_dropWhenNo(t *testing.T) {
	ctx := context.Background()
	client := &fakeCriticClient{response: `{"verdict":"no","reason":"not actionable"}`}
	f := findings.Finding{File: "a.go", Line: 1, Severity: findings.SeverityWarning, Category: findings.CategoryStyle, Message: "style"}
	keep, err := VerifyFinding(ctx, client, "model", f, "code", nil)
	if err != nil {
		t.Fatal(err)
	}
	if keep {
		t.Error("expected keep false when verdict no")
	}
}

func TestVerifyFinding_nilClientOrEmptyModel(t *testing.T) {
	ctx := context.Background()
	f := findings.Finding{File: "a.go", Line: 1, Severity: findings.SeverityInfo, Category: findings.CategoryStyle, Message: "m"}
	keep, err := VerifyFinding(ctx, nil, "model", f, "code", nil)
	if err != nil {
		t.Fatal(err)
	}
	if keep {
		t.Error("nil client should return keep false")
	}
	client := &fakeCriticClient{response: `{"verdict":"yes"}`}
	keep, err = VerifyFinding(ctx, client, "", f, "code", nil)
	if err != nil {
		t.Fatal(err)
	}
	if keep {
		t.Error("empty model should return keep false")
	}
}

func TestVerifyFinding_clientError(t *testing.T) {
	ctx := context.Background()
	client := &fakeCriticClient{err: errors.New("network error")}
	f := findings.Finding{File: "a.go", Line: 1, Severity: findings.SeverityError, Category: findings.CategoryBug, Message: "m"}
	keep, err := VerifyFinding(ctx, client, "model", f, "code", nil)
	if err == nil {
		t.Fatal("expected error when client returns error")
	}
	if keep {
		t.Error("keep should be false on error")
	}
}

func TestVerifyFinding_parseFailureThenRetry(t *testing.T) {
	ctx := context.Background()
	client := &retryFakeClient{responses: []string{"garbage", `{"verdict":"yes"}`}}
	keep, err := VerifyFinding(ctx, client, "model", findings.Finding{File: "a.go", Line: 1, Severity: findings.SeverityError, Category: findings.CategoryBug, Message: "m"}, "code", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !keep {
		t.Error("after retry with yes expected keep true")
	}
	if client.callCount != 2 {
		t.Errorf("expected 2 calls (first parse fail, retry); got %d", client.callCount)
	}
}

type retryFakeClient struct {
	responses []string
	callCount int
}

func (r *retryFakeClient) Generate(ctx context.Context, model, systemPrompt, userPrompt string, opts *ollama.GenerateOptions) (*ollama.GenerateResult, error) {
	idx := r.callCount
	if idx >= len(r.responses) {
		idx = len(r.responses) - 1
	}
	r.callCount++
	return &ollama.GenerateResult{Response: r.responses[idx]}, nil
}

func TestVerifyFinding_noRetryWhenOptsDisableRetry(t *testing.T) {
	ctx := context.Background()
	client := &retryFakeClient{responses: []string{"invalid"}}
	opts := &CriticOptions{RetryOnParseError: false}
	keep, err := VerifyFinding(ctx, client, "model", findings.Finding{File: "a.go", Line: 1, Severity: findings.SeverityError, Category: findings.CategoryBug, Message: "m"}, "code", opts)
	if err != nil {
		t.Fatal(err)
	}
	if keep {
		t.Error("expected keep false when parse fails and retry disabled")
	}
	if client.callCount != 1 {
		t.Errorf("expected 1 call when retry disabled; got %d", client.callCount)
	}
}

// recordOptsClient records the GenerateOptions passed to Generate for tests.
type recordOptsClient struct {
	response string
	opts     *ollama.GenerateOptions
}

func (r *recordOptsClient) Generate(ctx context.Context, model, systemPrompt, userPrompt string, opts *ollama.GenerateOptions) (*ollama.GenerateResult, error) {
	r.opts = opts
	return &ollama.GenerateResult{Response: r.response}, nil
}

func TestVerifyFinding_passesKeepAliveWhenSet(t *testing.T) {
	ctx := context.Background()
	client := &recordOptsClient{response: `{"verdict":"yes"}`}
	opts := &CriticOptions{KeepAlive: -1}
	_, err := VerifyFinding(ctx, client, "model", findings.Finding{File: "a.go", Line: 1, Severity: findings.SeverityError, Category: findings.CategoryBug, Message: "m"}, "code", opts)
	if err != nil {
		t.Fatal(err)
	}
	if client.opts == nil || client.opts.KeepAlive != -1 {
		t.Errorf("expected KeepAlive -1 when opts.KeepAlive set; got opts=%v", client.opts)
	}
}
