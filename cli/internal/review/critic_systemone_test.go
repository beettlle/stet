package review

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"stet/cli/internal/findings"
	"stet/cli/internal/systemone"
)

// fakeDecider records every Decide request and answers each requested question
// with probs[key] (0 when the key is absent). err, when set, is returned as the
// Decide error. omit[key] drops an answer for key to simulate a bad server.
type fakeDecider struct {
	requests []systemone.Request
	probs    map[string]float64
	omit     map[string]bool
	err      error
}

func (f *fakeDecider) Decide(_ context.Context, req systemone.Request) (*systemone.Response, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return nil, f.err
	}
	answers := make(map[string]systemone.NoulAnswer, len(req.Questions))
	for key := range req.Questions {
		if f.omit[key] {
			continue
		}
		answers[key] = systemone.NoulAnswer{Noul: f.probs[key]}
	}
	return &systemone.Response{Model: req.Model, Answers: answers}, nil
}

func sampleFinding(id, msg string) findings.Finding {
	return findings.Finding{
		ID:       id,
		File:     "pkg/foo.go",
		Line:     10,
		Severity: findings.SeverityWarning,
		Category: findings.CategoryCorrectness,
		Message:  msg,
	}
}

func TestVerifyFindingsSystemOne_allKept(t *testing.T) {
	batch := []findings.Finding{
		sampleFinding("f-a", "leak"),
		sampleFinding("f-b", "nil deref"),
	}
	fake := &fakeDecider{probs: map[string]float64{"f0": 0.99, "f1": 0.87}}
	kept, scores, err := VerifyFindingsSystemOne(context.Background(), fake, "openjev-0.1", batch, "code", 0.5, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(kept) != 2 || kept[0].ID != "f-a" || kept[1].ID != "f-b" {
		t.Errorf("both findings should be kept in order, got: %+v", kept)
	}
	if len(scores) != 2 || scores["f-a"] != 0.99 || scores["f-b"] != 0.87 {
		t.Errorf("scores should map IDs to P(yes), got: %v", scores)
	}
	if len(fake.requests) != 1 {
		t.Fatalf("expected exactly 1 request, got %d", len(fake.requests))
	}
	req := fake.requests[0]
	if req.Model != "openjev-0.1" {
		t.Errorf("request model = %q, want openjev-0.1", req.Model)
	}
	if req.Samples != 3 {
		t.Errorf("request samples = %d, want 3", req.Samples)
	}
	if req.State != "Code under review:\ncode" {
		t.Errorf("state = %q, want prefixed hunk", req.State)
	}
	if len(req.Questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(req.Questions))
	}
	if _, ok := req.Questions["f0"]; !ok {
		t.Error("request should contain question key f0")
	}
	if _, ok := req.Questions["f1"]; !ok {
		t.Error("request should contain question key f1")
	}
}

func TestVerifyFindingsSystemOne_allDropped(t *testing.T) {
	batch := []findings.Finding{sampleFinding("f-a", "x"), sampleFinding("f-b", "y")}
	fake := &fakeDecider{probs: map[string]float64{"f0": 0.01, "f1": 0.2}}
	kept, scores, err := VerifyFindingsSystemOne(context.Background(), fake, "m", batch, "code", 0.5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(kept) != 0 {
		t.Errorf("no findings should be kept, got: %+v", kept)
	}
	if len(scores) != 2 {
		t.Errorf("scores should still record both IDs, got: %v", scores)
	}
}

func TestVerifyFindingsSystemOne_thresholdBoundaryKeeps(t *testing.T) {
	// probs: exactly minKeep keeps, just below drops; order preserved in kept.
	batch := []findings.Finding{
		sampleFinding("keep-at-boundary", "a"),
		sampleFinding("drop-below", "b"),
		sampleFinding("keep-above", "c"),
	}
	fake := &fakeDecider{probs: map[string]float64{"f0": 0.5, "f1": 0.4999, "f2": 0.9}}
	kept, _, err := VerifyFindingsSystemOne(context.Background(), fake, "m", batch, "code", 0.5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(kept) != 2 {
		t.Fatalf("expected 2 kept findings, got %d: %+v", len(kept), kept)
	}
	if kept[0].ID != "keep-at-boundary" || kept[1].ID != "keep-above" {
		t.Errorf("P(yes) == minKeep must keep, and input order must be preserved, got: %s, %s", kept[0].ID, kept[1].ID)
	}
}

func TestVerifyFindingsSystemOne_chunksAt32Questions(t *testing.T) {
	const n = 33
	batch := make([]findings.Finding, 0, n)
	for i := 0; i < n; i++ {
		batch = append(batch, sampleFinding(fmt.Sprintf("id-%d", i), "msg"))
	}
	fake := &fakeDecider{probs: map[string]float64{"f32": 0.8}}
	for i := 0; i < 32; i++ {
		fake.probs[systemOneQuestionKey(i)] = 0.8
	}
	kept, _, err := VerifyFindingsSystemOne(context.Background(), fake, "m", batch, "code", 0.5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.requests) != 2 {
		t.Fatalf("33 findings should produce 2 requests, got %d", len(fake.requests))
	}
	if len(fake.requests[0].Questions) != systemOneMaxQuestionsPerRequest {
		t.Errorf("first request should carry %d questions, got %d", systemOneMaxQuestionsPerRequest, len(fake.requests[0].Questions))
	}
	if len(fake.requests[1].Questions) != 1 {
		t.Errorf("second request should carry 1 question, got %d", len(fake.requests[1].Questions))
	}
	if _, ok := fake.requests[1].Questions["f32"]; !ok {
		t.Error("second request should key its finding by batch index f32")
	}
	if len(kept) != n {
		t.Errorf("all findings above threshold should be kept, got %d", len(kept))
	}
	if kept[0].ID != "id-0" || kept[n-1].ID != "id-32" {
		t.Error("kept must preserve input order across chunks")
	}
}

func TestVerifyFindingsSystemOne_duplicateIDsNoCollision(t *testing.T) {
	batch := []findings.Finding{
		sampleFinding("dup", "first claim"),
		sampleFinding("dup", "second claim"),
	}
	fake := &fakeDecider{probs: map[string]float64{"f0": 0.9, "f1": 0.1}}
	kept, scores, err := VerifyFindingsSystemOne(context.Background(), fake, "m", batch, "code", 0.5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := fake.requests[0]
	if len(req.Questions) != 2 {
		t.Fatalf("duplicate IDs must produce two distinct questions, got %d", len(req.Questions))
	}
	if _, ok := req.Questions["f0"]; !ok {
		t.Error("question keys must use batch index, not finding ID")
	}
	if _, ok := req.Questions["f1"]; !ok {
		t.Error("question keys must use batch index, not finding ID")
	}
	// Each finding is judged on its own probability: first kept, second dropped.
	if len(kept) != 1 || kept[0].Message != "first claim" {
		t.Errorf("only the first duplicate should be kept, got: %+v", kept)
	}
	if len(scores) != 1 {
		t.Fatalf("scores keyed by ID collapses duplicates to one entry, got: %v", scores)
	}
	if scores["dup"] != 0.1 {
		t.Errorf("scores[dup] should hold the last duplicate's P(yes), got %v", scores["dup"])
	}
}

func TestVerifyFindingsSystemOne_noIDOmittedFromScores(t *testing.T) {
	f := sampleFinding("", "no id")
	fake := &fakeDecider{probs: map[string]float64{"f0": 0.9}}
	kept, scores, err := VerifyFindingsSystemOne(context.Background(), fake, "m", []findings.Finding{f}, "code", 0.5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(kept) != 1 {
		t.Fatalf("finding should be kept, got: %+v", kept)
	}
	if len(scores) != 0 {
		t.Errorf("findings without an ID must be omitted from scores, got: %v", scores)
	}
}

func TestVerifyFindingsSystemOne_emptyBatchNilClientEmptyModelNoRequest(t *testing.T) {
	batch := []findings.Finding{sampleFinding("f-a", "x")}

	// Empty batch: no request even with a live client.
	fake := &fakeDecider{}
	kept, scores, err := VerifyFindingsSystemOne(context.Background(), fake, "m", nil, "code", 0.5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kept != nil || len(kept) != 0 {
		t.Errorf("empty batch should return the batch unchanged, got: %+v", kept)
	}
	if len(scores) != 0 {
		t.Errorf("empty batch should return no scores, got: %v", scores)
	}
	if len(fake.requests) != 0 {
		t.Errorf("empty batch must not issue a request, got %d", len(fake.requests))
	}

	// Nil client: batch returned unchanged, nothing to call.
	kept, scores, err = VerifyFindingsSystemOne(context.Background(), nil, "m", batch, "code", 0.5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(kept) != 1 || kept[0].ID != "f-a" {
		t.Errorf("nil client should return the batch unchanged, got: %+v", kept)
	}
	if len(scores) != 0 {
		t.Errorf("nil client should return no scores, got: %v", scores)
	}

	// Empty model: no request.
	fake2 := &fakeDecider{}
	kept, _, err = VerifyFindingsSystemOne(context.Background(), fake2, "", batch, "code", 0.5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(kept) != 1 || kept[0].ID != "f-a" {
		t.Errorf("empty model should return the batch unchanged, got: %+v", kept)
	}
	if len(fake2.requests) != 0 {
		t.Errorf("empty model must not issue a request, got %d", len(fake2.requests))
	}
}

func TestVerifyFindingsSystemOne_decideErrorPropagates(t *testing.T) {
	sentinel := errors.New("system one server unreachable")
	fake := &fakeDecider{err: sentinel}
	kept, scores, err := VerifyFindingsSystemOne(context.Background(), fake, "m", []findings.Finding{sampleFinding("f-a", "x")}, "code", 0.5, 0)
	if err == nil {
		t.Fatal("expected Decide error to propagate")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error should wrap the Decide error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "system one verify") {
		t.Errorf("error should carry verifier context, got: %v", err)
	}
	if kept != nil || scores != nil {
		t.Errorf("on error, kept and scores must be nil, got: %+v, %v", kept, scores)
	}
}

func TestVerifyFindingsSystemOne_missingAnswerIsError(t *testing.T) {
	fake := &fakeDecider{omit: map[string]bool{"f1": true}}
	kept, scores, err := VerifyFindingsSystemOne(context.Background(), fake, "m", []findings.Finding{sampleFinding("a", "x"), sampleFinding("b", "y")}, "code", 0.5, 0)
	if err == nil {
		t.Fatal("missing answer must be an error, not a silent skip")
	}
	if !strings.Contains(err.Error(), `missing answer for question "f1"`) {
		t.Errorf("error should name the missing question, got: %v", err)
	}
	if kept != nil || scores != nil {
		t.Errorf("on error, kept and scores must be nil, got: %+v, %v", kept, scores)
	}
}

func TestVerifyFindingsSystemOne_truncatesLongHunk(t *testing.T) {
	hunk := strings.Repeat("x", defaultCriticContextCap+1000)
	fake := &fakeDecider{probs: map[string]float64{"f0": 0.9}}
	_, _, err := VerifyFindingsSystemOne(context.Background(), fake, "m", []findings.Finding{sampleFinding("f-a", "x")}, hunk, 0.5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	state := fake.requests[0].State
	want := "Code under review:\n" + strings.Repeat("x", defaultCriticContextCap) + "\n[truncated]"
	if state != want {
		t.Errorf("state should truncate the critic input at %d chars with the [truncated] marker (len %d, want %d)", defaultCriticContextCap, len(state), len(want))
	}
}

func TestVerifyFindingsSystemOne_keepsContextUnderDefaultCap(t *testing.T) {
	// Content at or under defaultCriticContextCap must reach the request state
	// untruncated: the critic may see everything the reviewer saw.
	hunk := strings.Repeat("x", defaultCriticContextCap)
	fake := &fakeDecider{probs: map[string]float64{"f0": 0.9}}
	_, _, err := VerifyFindingsSystemOne(context.Background(), fake, "m", []findings.Finding{sampleFinding("f-a", "x")}, hunk, 0.5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Code under review:\n" + hunk
	if fake.requests[0].State != want {
		t.Errorf("state should carry the full %d-char input untruncated (len %d)", defaultCriticContextCap, len(fake.requests[0].State))
	}
}

func TestVerifyFindingsSystemOne_instructionsCarryFindingFieldsNotHunk(t *testing.T) {
	f := findings.Finding{
		ID:         "f-a",
		File:       "pkg/foo.go",
		Line:       7,
		Range:      &findings.LineRange{Start: 7, End: 9},
		Severity:   findings.SeverityError,
		Category:   findings.CategoryBug,
		Message:    "fd leak",
		Suggestion: "close the file",
	}
	const hunk = "SECRET_HUNK_BODY := open()"
	fake := &fakeDecider{probs: map[string]float64{"f0": 0.9}}
	_, _, err := VerifyFindingsSystemOne(context.Background(), fake, "m", []findings.Finding{f}, hunk, 0.5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := fake.requests[0]
	if !strings.Contains(req.State, hunk) {
		t.Error("hunk content belongs in the request state")
	}
	q := req.Questions["f0"]
	instructions := q.Instructions
	if !strings.HasPrefix(instructions, "Finding claimed by a code reviewer:") {
		t.Errorf("instructions should start with the finding prefix, got: %q", instructions)
	}
	for _, want := range []string{"pkg/foo.go", "lines 7-9", "error", "bug", "fd leak", "close the file"} {
		if !strings.Contains(instructions, want) {
			t.Errorf("instructions should contain finding field %q, got: %q", want, instructions)
		}
	}
	if strings.Contains(instructions, hunk) {
		t.Error("instructions must not echo hunk/diff content (prompt-injection surface)")
	}
	if q.Criteria.True != systemOneCriteriaTrue {
		t.Errorf("criteria true = %q, want %q", q.Criteria.True, systemOneCriteriaTrue)
	}
	if q.Criteria.False != systemOneCriteriaFalse {
		t.Errorf("criteria false = %q, want %q", q.Criteria.False, systemOneCriteriaFalse)
	}
}

func TestVerifyFindingsSystemOne_samplesPassedToEveryRequest(t *testing.T) {
	batch := make([]findings.Finding, 0, systemOneMaxQuestionsPerRequest+1)
	for i := 0; i < systemOneMaxQuestionsPerRequest+1; i++ {
		batch = append(batch, sampleFinding(fmt.Sprintf("id-%d", i), "msg"))
	}
	fake := &fakeDecider{probs: map[string]float64{"f32": 0.9}}
	for i := 0; i < systemOneMaxQuestionsPerRequest; i++ {
		fake.probs[systemOneQuestionKey(i)] = 0.9
	}
	if _, _, err := VerifyFindingsSystemOne(context.Background(), fake, "m", batch, "code", 0.5, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(fake.requests))
	}
	for i, req := range fake.requests {
		if req.Samples != 5 {
			t.Errorf("request %d samples = %d, want 5", i, req.Samples)
		}
	}
}
