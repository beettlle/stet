// Package review provides the per-hunk review pipeline and the optional
// critic (second-pass verification). This file implements the batch System
// One verifier: one decision-server request per hunk instead of one LLM
// generation per finding.
package review

import (
	"context"
	"fmt"
	"strings"

	"stet/cli/internal/findings"
	"stet/cli/internal/systemone"
)

// systemOneMaxQuestionsPerRequest caps how many noul questions a single
// System One request carries so a large finding batch cannot produce an
// unbounded payload; the verifier loops over fixed-size chunks.
const systemOneMaxQuestionsPerRequest = 32

// systemOneFindingPrefix prefixes the per-question instructions for one finding.
const systemOneFindingPrefix = "Finding claimed by a code reviewer:"

// systemOneStatePrefix prefixes the request state that carries the code under review.
const systemOneStatePrefix = "Code under review:\n"

// Criteria text shared by every System One noul question.
const (
	systemOneCriteriaTrue  = "The finding is correct and actionable for this code."
	systemOneCriteriaFalse = "The finding is incorrect, unsupported by this code, or not actionable."
)

// SystemOneDecider is satisfied by *systemone.Client; tests use a fake.
type SystemOneDecider interface {
	Decide(ctx context.Context, req systemone.Request) (*systemone.Response, error)
}

// VerifyFindingsSystemOne verifies every finding in batch against hunkContent
// using System One decision-server calls, one request per at most
// systemOneMaxQuestionsPerRequest findings, and keeps only findings the model
// judges correct: kept holds the input findings with P(yes) >= minKeep in
// input order. scores maps finding ID to P(yes) for trace/eval use; findings
// without an ID are omitted. nil client, empty model, or empty batch returns
// the batch unchanged with no request. Any Decide error aborts with
// (nil, nil, err) wrapped with context; the caller decides abort behaviour.
//
// Security: hunk content is attacker-influenced input (e.g. diff comments), so
// it is passed only in the request state and never echoed into question
// instructions, but a crafted diff could still bias the verdict.
func VerifyFindingsSystemOne(ctx context.Context, client SystemOneDecider, model string, batch []findings.Finding, hunkContent string, minKeep float64, samples int) (kept []findings.Finding, scores map[string]float64, err error) {
	if client == nil || model == "" || len(batch) == 0 {
		return batch, map[string]float64{}, nil
	}

	state := buildSystemOneState(hunkContent)
	probs := make([]float64, len(batch))
	for start := 0; start < len(batch); start += systemOneMaxQuestionsPerRequest {
		end := start + systemOneMaxQuestionsPerRequest
		if end > len(batch) {
			end = len(batch)
		}
		req := systemone.Request{
			Model:     model,
			State:     state,
			Questions: make(map[string]systemone.NoulQuestion, end-start),
			Samples:   samples,
		}
		for i := start; i < end; i++ {
			req.Questions[systemOneQuestionKey(i)] = systemone.NoulQuestion{
				Instructions: buildSystemOneInstructions(batch[i]),
				Criteria: systemone.NoulCriteria{
					True:  systemOneCriteriaTrue,
					False: systemOneCriteriaFalse,
				},
			}
		}
		resp, decideErr := client.Decide(ctx, req)
		if decideErr != nil {
			return nil, nil, fmt.Errorf("system one verify: request for findings %d-%d: %w", start, end-1, decideErr)
		}
		for i := start; i < end; i++ {
			answer, ok := resp.Answers[systemOneQuestionKey(i)]
			if !ok {
				return nil, nil, fmt.Errorf("system one verify: response missing answer for question %q", systemOneQuestionKey(i))
			}
			probs[i] = answer.Noul
		}
	}

	kept = make([]findings.Finding, 0, len(batch))
	scores = make(map[string]float64, len(batch))
	for i, f := range batch {
		if f.ID != "" {
			scores[f.ID] = probs[i]
		}
		if probs[i] >= minKeep {
			kept = append(kept, f)
		}
	}
	return kept, scores, nil
}

// systemOneQuestionKey returns the question key for the finding at batch index
// i ("f0", "f1", ...). Keys use the batch index, never the finding ID, so
// duplicate finding IDs cannot collide within a request.
func systemOneQuestionKey(i int) string {
	return fmt.Sprintf("f%d", i)
}

// buildSystemOneState builds the request state: the code under review only,
// truncated with the same marker and cap as BuildCriticPrompt.
func buildSystemOneState(hunkContent string) string {
	if len(hunkContent) > defaultCriticMaxHunkLen {
		hunkContent = hunkContent[:defaultCriticMaxHunkLen] + "\n[truncated]"
	}
	return systemOneStatePrefix + hunkContent
}

// buildSystemOneInstructions formats one finding's reviewer-claimed fields.
// Only finding metadata appears here; the code under review stays in the
// request state and is never echoed into instructions.
func buildSystemOneInstructions(f findings.Finding) string {
	loc := fmt.Sprintf("line %d", f.Line)
	if f.Range != nil && f.Range.Start != f.Range.End {
		loc = fmt.Sprintf("lines %d-%d", f.Range.Start, f.Range.End)
	}
	var b strings.Builder
	b.WriteString(systemOneFindingPrefix)
	b.WriteString("\n- file: ")
	b.WriteString(f.File)
	b.WriteString("\n- location: ")
	b.WriteString(loc)
	b.WriteString("\n- severity: ")
	b.WriteString(string(f.Severity))
	b.WriteString("\n- category: ")
	b.WriteString(string(f.Category))
	b.WriteString("\n- message: ")
	b.WriteString(f.Message)
	if f.Suggestion != "" {
		b.WriteString("\n- suggestion: ")
		b.WriteString(f.Suggestion)
	}
	return b.String()
}
