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

// TestVerifyFindingsSystemOne_everyQuestionCarriesShippedCriteria pins the
// exact criteria strings every System One question must carry. SP-080
// validated three rewordings and none passed the pre-registered rule
// (docs/research/critic-eval/criteria-sp080.md), so these remain the shipped
// strings; the literals are asserted verbatim so a silent criteria rewording
// cannot ship without updating this test (and re-running the criteria-sp080
// validation).
func TestVerifyFindingsSystemOne_everyQuestionCarriesShippedCriteria(t *testing.T) {
	const wantTrue = "The finding is correct and actionable for this code."
	const wantFalse = "The finding is incorrect, unsupported by this code, or not actionable."
	if systemOneCriteriaTrue != wantTrue || systemOneCriteriaFalse != wantFalse {
		t.Fatalf("SP-080 criteria changed; re-run the critic-eval validation before shipping:\n true = %q\n false = %q", systemOneCriteriaTrue, systemOneCriteriaFalse)
	}
	const n = systemOneMaxQuestionsPerRequest + 1
	batch := make([]findings.Finding, 0, n)
	for i := 0; i < n; i++ {
		batch = append(batch, sampleFinding(fmt.Sprintf("id-%d", i), "msg"))
	}
	fake := &fakeDecider{probs: map[string]float64{"f0": 0.9, "f1": 0.9}}
	if _, _, err := VerifyFindingsSystemOne(context.Background(), fake, "m", batch, "code", 0.5, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.requests) != 2 {
		t.Fatalf("expected 2 chunked requests, got %d", len(fake.requests))
	}
	for ri, req := range fake.requests {
		for key, q := range req.Questions {
			if q.Criteria.True != wantTrue {
				t.Errorf("request %d question %q criteria true = %q, want the SP-080 wording", ri, key, q.Criteria.True)
			}
			if q.Criteria.False != wantFalse {
				t.Errorf("request %d question %q criteria false = %q, want the SP-080 wording", ri, key, q.Criteria.False)
			}
		}
	}
}

// TestVerifyFindingsSystemOne_criteriaStayShortAndHunkFree guards the criteria
// wording contract: both criteria strings stay short and carry no code/hunk
// text (the finding fields and hunk ride in the instructions/state instead).
func TestVerifyFindingsSystemOne_criteriaStayShortAndHunkFree(t *testing.T) {
	if len(systemOneCriteriaTrue) > 200 || len(systemOneCriteriaFalse) > 200 {
		t.Errorf("criteria strings must stay short (true %d, false %d chars, want <= 200)", len(systemOneCriteriaTrue), len(systemOneCriteriaFalse))
	}
	const hunk = "SECRET_HUNK_BODY := open()"
	fake := &fakeDecider{probs: map[string]float64{"f0": 0.9}}
	if _, _, err := VerifyFindingsSystemOne(context.Background(), fake, "m", []findings.Finding{sampleFinding("f-a", "x")}, hunk, 0.5, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for key, q := range fake.requests[0].Questions {
		if strings.Contains(q.Criteria.True, hunk) || strings.Contains(q.Criteria.False, hunk) {
			t.Errorf("question %q criteria must not echo hunk content", key)
		}
	}
}

// TestAnnotateRuleConflicts_criteriaUnchangedByCriticReword (SP-080) proves the
// rules-annotation request is untouched by the critic-criteria rewording: its
// choice questions still carry the section options plus the none option, and
// the shared annotate instructions suffix is unchanged.
func TestAnnotateRuleConflicts_criteriaUnchangedByCriticReword(t *testing.T) {
	if annotateNoneDescription != "Applying the suggestion conflicts with none of these guidelines." {
		t.Errorf("annotate none option description changed: %q", annotateNoneDescription)
	}
	if !strings.HasSuffix(annotateInstructionsSuffix, "Choose the one conflicting section, or none if applying the suggestion conflicts with none of the listed guidelines.") {
		t.Errorf("annotate instructions suffix changed: %q", annotateInstructionsSuffix)
	}
	sections := annotateSections(2)
	fake := &fakeChoiceDecider{respond: annotateAnswer(func(string) (string, float64) {
		return annotateNoneOption, 1.0
	})}
	batch := []findings.Finding{sampleFinding("f-a", "msg with a suggestion")}
	if _, err := AnnotateRuleConflicts(context.Background(), fake, "m", batch, "hunk", sections); err != nil {
		t.Fatalf("AnnotateRuleConflicts: %v", err)
	}
	if len(fake.reqs) != 1 {
		t.Fatalf("expected 1 annotate request, got %d", len(fake.reqs))
	}
	q := fake.reqs[0].Questions["q0"]
	want := map[string]string{
		"s0":               "rule-00.mdc: Section 00",
		"s1":               "rule-01.mdc: Section 01",
		annotateNoneOption: annotateNoneDescription,
	}
	if len(q.Criteria) != len(want) {
		t.Fatalf("annotate criteria options = %v, want %v", q.Criteria, want)
	}
	for key, desc := range want {
		if q.Criteria[key] != desc {
			t.Errorf("annotate criteria[%q] = %q, want %q", key, q.Criteria[key], desc)
		}
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

// sp39CalleeReturnMessage reproduces the ERROR message shape from issue #39
// finding 2: a specific claim about an imported callee's return value.
const sp39CalleeReturnMessage = "`buildStetVersionDoctorCheck` returns `ok: false` if the version is below the minimum. This is a logic error."

// sp39ImportedCalleeContext carries the import and a call site but NOT the
// callee's body: the callee lives in another module (src/doctor/stet-capacity.mjs).
const sp39ImportedCalleeContext = "import { buildStetVersionDoctorCheck } from \"./stet-capacity.mjs\";\n\nconst checks = [];\nchecks.push(buildStetVersionDoctorCheck());\n"

// TestVerifyFindingsSystemOne_demotesCalleeReturnErrorWithoutBodyInContext
// (issue #39 finding 2) asserts the critic demotes an ERROR finding that
// asserts an imported callee's return value when the callee's body is not in
// the critic context: the finding is kept (still reviewed) but at warning
// severity, and the request instructions carry the demoted severity.
func TestVerifyFindingsSystemOne_demotesCalleeReturnErrorWithoutBodyInContext(t *testing.T) {
	t.Parallel()
	f := sampleFinding("f-a", sp39CalleeReturnMessage)
	f.Severity = findings.SeverityError
	fake := &fakeDecider{probs: map[string]float64{"f0": 0.9}}
	kept, _, err := VerifyFindingsSystemOne(context.Background(), fake, "m", []findings.Finding{f}, sp39ImportedCalleeContext, 0.5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(kept) != 1 {
		t.Fatalf("finding should be kept (demoted, not dropped), got %+v", kept)
	}
	if kept[0].Severity != findings.SeverityWarning {
		t.Errorf("kept severity = %q, want warning: an ERROR claim about a callee whose body is not in context is speculation (issue #39)", kept[0].Severity)
	}
	if !strings.Contains(fake.requests[0].Questions["f0"].Instructions, "- severity: warning") {
		t.Errorf("request instructions should carry the demoted severity, got: %q", fake.requests[0].Questions["f0"].Instructions)
	}
}

// TestVerifyFindingsSystemOne_keepsErrorWhenCalleeDeclaredInContext asserts
// the demotion guard does not touch ERROR claims whose callee IS declared in
// the critic context: the body may support the claim.
func TestVerifyFindingsSystemOne_keepsErrorWhenCalleeDeclaredInContext(t *testing.T) {
	t.Parallel()
	f := sampleFinding("f-a", sp39CalleeReturnMessage)
	f.Severity = findings.SeverityError
	const hunk = "function buildStetVersionDoctorCheck() {\n  if (outdated) return { ok: false };\n  return { ok: true };\n}\nconst r = buildStetVersionDoctorCheck();\n"
	fake := &fakeDecider{probs: map[string]float64{"f0": 0.9}}
	kept, _, err := VerifyFindingsSystemOne(context.Background(), fake, "m", []findings.Finding{f}, hunk, 0.5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(kept) != 1 || kept[0].Severity != findings.SeverityError {
		t.Fatalf("severity should stay error when the callee is declared in context, got: %+v", kept)
	}
	if !strings.Contains(fake.requests[0].Questions["f0"].Instructions, "- severity: error") {
		t.Errorf("request instructions should carry the original error severity, got: %q", fake.requests[0].Questions["f0"].Instructions)
	}
}

// TestDemoteUnsupportedCalleeReturnErrors covers the guard directly: only
// ERROR findings with a callee-return claim and no in-context declaration are
// demoted, the reversed "return value of `x`" form is detected, and the input
// slice is not modified.
func TestDemoteUnsupportedCalleeReturnErrors(t *testing.T) {
	t.Parallel()
	const emptyCtx = "const checks = [];\n"
	tests := []struct {
		name    string
		sev     findings.Severity
		msg     string
		ctx     string
		wantSev findings.Severity
	}{
		{
			name:    "error_callee_return_no_body_demoted",
			sev:     findings.SeverityError,
			msg:     sp39CalleeReturnMessage,
			ctx:     sp39ImportedCalleeContext,
			wantSev: findings.SeverityWarning,
		},
		{
			name:    "error_return_value_of_form_demoted",
			sev:     findings.SeverityError,
			msg:     "The return value of `loadState` is never checked, so failures vanish.",
			ctx:     emptyCtx,
			wantSev: findings.SeverityWarning,
		},
		{
			name:    "error_will_return_form_demoted",
			sev:     findings.SeverityError,
			msg:     "`saveConfig` will return nil on failure and the caller cannot tell.",
			ctx:     emptyCtx,
			wantSev: findings.SeverityWarning,
		},
		{
			name:    "error_no_return_claim_untouched",
			sev:     findings.SeverityError,
			msg:     "`handleUpload` swallows the error, hiding failures.",
			ctx:     emptyCtx,
			wantSev: findings.SeverityError,
		},
		{
			name:    "warning_callee_return_untouched",
			sev:     findings.SeverityWarning,
			msg:     sp39CalleeReturnMessage,
			ctx:     emptyCtx,
			wantSev: findings.SeverityWarning,
		},
		{
			name:    "error_callee_declared_in_context_untouched",
			sev:     findings.SeverityError,
			msg:     sp39CalleeReturnMessage,
			ctx:     "function buildStetVersionDoctorCheck() { return { ok: false }; }\n",
			wantSev: findings.SeverityError,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := []findings.Finding{sampleFinding("f-a", tc.msg)}
			in[0].Severity = tc.sev
			inCopy := append([]findings.Finding(nil), in...)
			got := demoteUnsupportedCalleeReturnErrors(in, tc.ctx)
			if len(got) != 1 || got[0].Severity != tc.wantSev {
				t.Fatalf("severity = %q, want %q", got[0].Severity, tc.wantSev)
			}
			if in[0].Severity != inCopy[0].Severity {
				t.Errorf("input slice was modified: %q, want %q", in[0].Severity, inCopy[0].Severity)
			}
		})
	}
}

// agreeDecider answers every requested question with 0.99, simulating a
// System One model that agrees with every finding — the worst case for
// precision. SP-086 uses it to prove the out-of-view existence-claim guard
// rejects findings the model would keep (issue #20).
type agreeDecider struct {
	requests []systemone.Request
}

func (a *agreeDecider) Decide(_ context.Context, req systemone.Request) (*systemone.Response, error) {
	a.requests = append(a.requests, req)
	answers := make(map[string]systemone.NoulAnswer, len(req.Questions))
	for key := range req.Questions {
		answers[key] = systemone.NoulAnswer{Noul: 0.99}
	}
	return &systemone.Response{Model: req.Model, Answers: answers}, nil
}

// TestOutOfViewExistenceClaim covers the issue #20 predicate: identifier-
// anchored undefined / not-imported / unused / not-exported / ReferenceError
// claims are out-of-view, while value-uses of "undefined" (including the
// backticked "returns `null` or `undefined`" form), in-view claims without
// an identifier anchor, and messages with only module mentions stay. Message
// shapes are taken from the labeled dataset rows they reproduce.
func TestOutOfViewExistenceClaim(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{
			name: "used_without_importing",
			// pi-spine-SP-829-5f9106b: import was above the hunk start.
			msg:  "`createRulesFixture` uses `fs` without importing it; the review hunk started at `@@ -4,102`, so the import lines were outside the reviewer's context.",
			want: true,
		},
		{
			name: "not_defined_or_imported_with_module_mention",
			// pi-spine-SP-831-131e1ec: the module mention (`src/cli/deps.mjs`)
			// must not exempt the claim — the deciding code is still out of view.
			msg:  "The `compareTaskIds` function is used in the sort call but is not defined or imported within the visible context of `src/cli/deps.mjs`. This will cause a ReferenceError at runtime.",
			want: true,
		},
		{
			name: "import_unused_in_hunk",
			// pi-spine-SP-831-74dc631: usage may exist below the hunk.
			msg:  "The import of `compareTaskIds` is added but the variable is not used anywhere in the provided hunk. This will likely result in a linting error (unused variable).",
			want: true,
		},
		{
			name: "module_does_not_export",
			// pi-spine-SP-831-4594035: cross-module export claim.
			msg:  "Importing `compareTaskIds` from `../tasks/packet/discover.mjs` will cause a runtime error because that module does not export this function.",
			want: true,
		},
		{
			name: "reference_error_claim",
			// pi-spine-SP-824-348bd12: reviewer bug on stale hunks.
			msg:  "Finding claimed a `streakIo` ReferenceError in `approveIntegrateGate`; triage: reviewer bug on stale hunks — `streakIo` is declared before use and tests exercise the path green.",
			want: true,
		},
		{
			name: "may_be_undefined_not_validated",
			// pi-spine-SP-827-cc988af: undefined-as-existence claim.
			msg:  "The function destructures `reviewLevel` but does not validate its type. If `reviewLevel` is `undefined` or `null` (which is common for optional fields), the switch falls through.",
			want: true,
		},
		{
			name: "value_use_returns_undefined",
			msg:  "The helper returns undefined when the input array is empty, and the caller does not check for it.",
			want: false,
		},
		{
			name: "backticked_value_use_returns_null_or_undefined",
			// pi-spine-SP-831-7cfc4f4: a return-value hypothesis, not an
			// existence claim — must not be rejected.
			msg:  "The test asserts that `remainingWithDeps` is an empty array `[]` when there are no cycles. If `topoWaves` returns `null` or `undefined` for the empty case, `assert.deepEqual` will fail.",
			want: false,
		},
		{
			name: "in_view_missing_separator",
			// pi-spine-SP-838-8653ab9: labeled keep — punctuation claim about
			// code shown in the hunk.
			msg:  "The JSDoc description for `providerCode` is missing a separator between the z.ai and Kimi/Anthropic sources, making it read as a run-on phrase.",
			want: false,
		},
		{
			name: "in_view_missing_size_bound",
			msg:  "The read has no size bound, so a malformed file can exhaust memory.",
			want: false,
		},
		{
			name: "no_identifier_anchor",
			// pi-spine-SP-821-n1: claim keyword but only a module-ish span.
			msg:  "Finding claimed an `export { x } from` re-export leaves the symbol unbound or unused locally; triage: re-exports do not bind a local and lint passes.",
			want: false,
		},
		{
			name: "logic_claim_no_existence_keyword",
			msg:  "The sort comparator `compareTaskIds(a.from, b.from)` is incorrect because `0` is falsy in JavaScript.",
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := outOfViewExistenceClaim(tc.msg); got != tc.want {
				t.Errorf("outOfViewExistenceClaim = %v, want %v\nmsg: %s", got, tc.want, tc.msg)
			}
		})
	}
}

// TestDropOutOfViewExistenceClaims covers the batch guard: only existence-
// claim findings are removed, order is preserved, the input slice is not
// modified, and an all-clean batch is returned unchanged.
func TestDropOutOfViewExistenceClaims(t *testing.T) {
	t.Parallel()
	existence := sampleFinding("f-exist", "`fs` is used without importing it.")
	inView := sampleFinding("f-inview", "The JSDoc description for `providerCode` is missing a separator between the sources.")
	logic := sampleFinding("f-logic", "The comparator `compareTaskIds` treats equal IDs as unequal because `0` is falsy.")
	in := []findings.Finding{existence, inView, logic}
	inCopy := append([]findings.Finding(nil), in...)
	got := dropOutOfViewExistenceClaims(in)
	if len(got) != 2 || got[0].ID != "f-inview" || got[1].ID != "f-logic" {
		t.Fatalf("kept = %+v, want f-inview then f-logic in order", got)
	}
	for i := range in {
		if in[i].ID != inCopy[i].ID || in[i].Message != inCopy[i].Message {
			t.Errorf("input slice was modified at %d", i)
		}
	}
	if all := dropOutOfViewExistenceClaims([]findings.Finding{inView, logic}); len(all) != 2 {
		t.Errorf("clean batch should pass through, got %+v", all)
	}
}

// TestVerifyFindingsSystemOne_rejectsOutOfViewExistenceClaimDespiteHighScore
// (issue #20) asserts the end-to-end contract: an identifier-existence finding
// is rejected before the decider is called even when the model would score it
// 0.99, while a companion in-view finding on the same hunk is kept and
// scored.
func TestVerifyFindingsSystemOne_rejectsOutOfViewExistenceClaimDespiteHighScore(t *testing.T) {
	t.Parallel()
	exist := sampleFinding("f-exist", "The `compareTaskIds` function is used in the sort call but is not defined or imported within the visible context.")
	inView := sampleFinding("f-inview", "The JSDoc description for `providerCode` is missing a separator between the sources.")
	fake := &agreeDecider{}
	kept, scores, err := VerifyFindingsSystemOne(context.Background(), fake, "m", []findings.Finding{exist, inView}, "const sorted = tasks.sort(compareTaskIds);\n", 0.2, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(kept) != 1 || kept[0].ID != "f-inview" {
		t.Fatalf("kept = %+v, want only the in-view finding f-inview", kept)
	}
	if _, ok := scores["f-exist"]; ok {
		t.Errorf("rejected finding must carry no score, got %v", scores["f-exist"])
	}
	if scores["f-inview"] != 0.99 {
		t.Errorf("in-view score = %v, want 0.99", scores["f-inview"])
	}
	if len(fake.requests) != 1 || len(fake.requests[0].Questions) != 1 {
		t.Fatalf("expected one decider request carrying only the surviving finding, got %d requests", len(fake.requests))
	}
	if _, ok := fake.requests[0].Questions["f0"]; !ok {
		t.Errorf("surviving finding should be re-keyed to the guarded batch index f0, got keys %v", fake.requests[0].Questions)
	}
}
