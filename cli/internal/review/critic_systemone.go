// Package review provides the per-hunk review pipeline and the optional
// critic (second-pass verification). This file implements the batch System
// One verifier: one decision-server request per hunk instead of one LLM
// generation per finding.
package review

import (
	"context"
	"fmt"
	"regexp"
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

// Callee-return claim detection (issue #39 finding 2). calleeReturnsRegex
// matches messages that assert a callee's return value or return behavior:
// "`callee` returns X", "`callee` returning X", "`callee` will/would return
// X". The span between the identifier and the return verb must stay on one
// line and must not cross another backticked span. calleeReturnOfRegex
// matches the reversed form: "the return value of `callee`".
var (
	calleeReturnsRegex  = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_]*)`[^`\n]{0,120}?\\b(?:returns?|returning|will\\s+return|would\\s+return)\\b")
	calleeReturnOfRegex = regexp.MustCompile("\\breturn(?:s|ing)?(?:\\s+value)?\\s+of\\s+`([A-Za-z_][A-Za-z0-9_]*)`")
)

// claimedReturnCallees returns the backticked identifiers whose return value
// or return behavior the message asserts, in first-match order, duplicates
// included.
func claimedReturnCallees(msg string) []string {
	var idents []string
	for _, re := range []*regexp.Regexp{calleeReturnsRegex, calleeReturnOfRegex} {
		for _, m := range re.FindAllStringSubmatch(msg, -1) {
			idents = append(idents, m[1])
		}
	}
	return idents
}

// demoteUnsupportedCalleeReturnErrors downgrades ERROR findings whose message
// asserts a return value or return behavior for a callee that is not declared
// in context (issue #39 finding 2): when the callee's body is not in the
// critic context — e.g. the callee lives in an imported file — the claim is
// speculation, so the critic must not vouch for ERROR severity. Claims naming
// at least one callee declared in the context keep their severity (the body
// may support the claim); non-ERROR findings pass through untouched. Returns
// a new slice; the input is not modified.
func demoteUnsupportedCalleeReturnErrors(batch []findings.Finding, context string) []findings.Finding {
	out := make([]findings.Finding, len(batch))
	copy(out, batch)
	for i := range out {
		f := out[i]
		if f.Severity != findings.SeverityError {
			continue
		}
		claimed := claimedReturnCallees(f.Message)
		if len(claimed) == 0 {
			continue
		}
		supported := false
		for _, id := range claimed {
			if findings.IdentDeclared(context, id) {
				supported = true
				break
			}
		}
		if supported {
			continue
		}
		out[i].Severity = findings.SeverityWarning
	}
	return out
}

// Out-of-view existence-claim rejection (issue #20). The System One decider
// judges each finding against the shown hunk only, and the SP-080 validation
// (docs/research/critic-eval/criteria-sp080.md) measured that it treats the
// excerpt as the whole world: messages claiming a symbol is undefined, not
// imported, unused, or not exported score 0.79–0.999 — above any usable
// min_keep — because the code that decides the claim (an import above the
// hunk, a use elsewhere in the file) is never shown. Three criteria rewordings
// failed to flip those rows without also suppressing in-view keeps, so the
// model-facing criteria stay pinned and the rejection is deterministic:
// a message claiming a backticked identifier is undefined / not defined /
// not imported / not declared / missing an import / ReferenceError / unused /
// not exported asserts a file-level fact a hunk excerpt can never verify, so
// the critic rejects the finding before scoring regardless of what the model
// would answer — the deterministic backstop for the reviewer-prompt ban that
// issue #20 proposed. Detection mirrors findings.FilterExistenceClaims
// (issues #33/#39) with two deliberate differences: value-uses of
// "undefined" ("returns undefined", including backticked forms like
// "returns `null` or `undefined`") are stripped first so return-value
// hypotheses are not mistaken for existence claims, and module mentions
// ("`src/cli/deps.mjs`", "`./utils`") do not exempt the message —
// FilterExistenceClaims bails on them because it can never disprove a claim
// about a file it does not read, while this guard rejects claims the shown
// hunk cannot verify, and issue #20 explicitly targets claims about imported
// modules and unshown files. In-view claims without an identifier anchor
// ("missing a separator", "no size bound on this read") are untouched.
var (
	// criticBacktickSpanRegex extracts `...` spans from a finding message.
	criticBacktickSpanRegex = regexp.MustCompile("`([^`\n]+)`")

	// criticValidIdentifierRegex reports whether a backticked span is a plain
	// identifier (no dots, slashes, or other qualification).
	criticValidIdentifierRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

	// criticValueUndefinedUseRegex matches value-uses of "undefined" (a
	// function returning / staying undefined is not an existence claim about a
	// named symbol); matched spans are stripped before claim detection. Unlike
	// the findings-package variant this tolerates backticked filler words
	// ("returns `null` or `undefined`").
	criticValueUndefinedUseRegex = regexp.MustCompile(`(?i)\b(?:returns?|returning|yields?|yielding|produces?|gives?|stays?|remains?|leaves?|left|sets?|becomes?)\s+(?:[\w` + "`" + `]+\s+){0,2}` + "`" + `?undefined\b`)

	// criticMissingClaimRegex matches claims that a symbol does not exist in
	// the file: undefined, not/never defined, not/never imported, not/never
	// declared, missing import(s), "without importing", or a would-be
	// ReferenceError.
	criticMissingClaimRegex = regexp.MustCompile(`(?i)\b(?:undefined|not\s+defined|never\s+defined|not\s+imported|never\s+imported|not\s+declared|never\s+declared|missing\s+imports?|without\s+importing|referenceerror)\b`)

	// criticUnusedClaimRegex matches claims that a symbol is unused, including
	// the "the diff does not show it being used" phrasing from issue #39.
	criticUnusedClaimRegex = regexp.MustCompile(`(?i)\b(?:unused|not\s+used|never\s+used|(?:does\s+not|doesn'?t|did\s+not|didn'?t)\s+(?:show|shown|showing)\s+(?:that\s+|it\s+|is\s+|was\s+|being\s+)*used)\b`)

	// criticNotExportedClaimRegex matches claims that a symbol is not exported
	// by its module ("not exported", "does not export").
	criticNotExportedClaimRegex = regexp.MustCompile(`(?i)\b(?:not\s+exported|(?:does\s+not|doesn'?t)\s+export)\b`)
)

// outOfViewExistenceClaim reports whether msg asserts an identifier-anchored
// existence claim: one of the claim families above plus at least one
// backticked plain identifier. Such a claim is decidable only with the whole
// file, never from the hunk excerpt the critic sees. Message mentions of
// modules (`./utils`, `src/x.mjs`) are ignored rather than exempting the
// claim — see the guard comment above.
func outOfViewExistenceClaim(msg string) bool {
	msg = criticValueUndefinedUseRegex.ReplaceAllString(msg, "")
	claim := criticMissingClaimRegex.MatchString(msg) ||
		criticUnusedClaimRegex.MatchString(msg) ||
		criticNotExportedClaimRegex.MatchString(msg)
	if !claim {
		return false
	}
	for _, m := range criticBacktickSpanRegex.FindAllStringSubmatch(msg, -1) {
		if criticValidIdentifierRegex.MatchString(m[1]) {
			return true
		}
	}
	return false
}

// dropOutOfViewExistenceClaims removes findings whose message makes an
// identifier-anchored existence claim (issue #20): the critic context is a
// hunk excerpt, so the claim's deciding code is outside it and the finding is
// unverifiable. Dropped findings are excluded from the request and carry no
// score, so a threshold sweep counts them as rejected. Returns a new slice;
// the input is not modified. Order of kept findings is preserved.
func dropOutOfViewExistenceClaims(batch []findings.Finding) []findings.Finding {
	out := make([]findings.Finding, 0, len(batch))
	for _, f := range batch {
		if !outOfViewExistenceClaim(f.Message) {
			out = append(out, f)
		}
	}
	return out
}

// Criteria text shared by every System One noul question. SP-080 validated
// three rewordings against docs/research/critic-eval/pi-spine-v2-dataset.jsonl
// (see docs/research/critic-eval/criteria-sp080.md): none passed the
// pre-registered rule (each either dropped real findings the old criteria kept
// or failed to move the out-of-view absence claims), so these strings stay —
// the pinned-literal tests in critic_systemone_test.go guard them. The
// out-of-view existence-claim guard above carries the issue #20 rejection
// deterministically instead of through this wording.
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
// Before verification, findings whose message makes an identifier-anchored
// existence claim (undefined / not imported / unused / not exported, see
// dropOutOfViewExistenceClaims) are rejected outright (issue #20): the shown
// hunk cannot decide them and the model demonstrably agrees with them
// regardless of truth. ERROR findings asserting a return value or return
// behavior for a callee that is not declared in hunkContent are then demoted
// to warning (issue #39 finding 2): without the callee's body in context —
// e.g. an imported callee — the claim is speculation and the critic must not
// keep it at ERROR severity. The demoted severity rides in the request
// instructions and on the kept findings.
//
// Security: hunk content is attacker-influenced input (e.g. diff comments), so
// it is passed only in the request state and never echoed into question
// instructions, but a crafted diff could still bias the verdict.
func VerifyFindingsSystemOne(ctx context.Context, client SystemOneDecider, model string, batch []findings.Finding, hunkContent string, minKeep float64, samples int) (kept []findings.Finding, scores map[string]float64, err error) {
	if client == nil || model == "" || len(batch) == 0 {
		return batch, map[string]float64{}, nil
	}

	// Issue #20: reject out-of-view existence claims before verification —
	// the hunk cannot decide them, so they never reach the decider and carry
	// no score (a sweep counts a missing score as rejected at every threshold).
	batch = dropOutOfViewExistenceClaims(batch)

	// Issue #39 finding 2: demote ERROR callee-return claims with no callee
	// body in the critic context before verification, so the request
	// instructions and the kept findings both carry the honest severity.
	batch = demoteUnsupportedCalleeReturnErrors(batch, hunkContent)

	state := buildSystemOneStateCapped(hunkContent, defaultCriticContextCap)
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

// buildSystemOneState builds the rules-annotation request state: the code
// under review only, truncated with the same marker and 4096-character cap the
// annotation requests have always used (defaultCriticMaxHunkLen).
func buildSystemOneState(hunkContent string) string {
	return buildSystemOneStateCapped(hunkContent, defaultCriticMaxHunkLen)
}

// buildSystemOneStateCapped builds the request state at the given cap: the
// code context under review only, truncated with the same marker as
// BuildCriticPrompt. The critic calls it with defaultCriticContextCap; the
// rules-annotation requests keep their own smaller cap via buildSystemOneState.
func buildSystemOneStateCapped(hunkContent string, maxLen int) string {
	if len(hunkContent) > maxLen {
		hunkContent = hunkContent[:maxLen] + "\n[truncated]"
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
