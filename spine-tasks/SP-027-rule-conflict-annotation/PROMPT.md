# Task: SP-027 — Rule-conflict annotation (library)

**Created:** 2026-10-08
**Size:** S

## Review Level: 1

**Assessment:** New annotation function in `review` plus an optional output field on findings; findings are never dropped.
**Score:** 3/8 — Blast radius: 1, Pattern novelty: 2, Security: 0, Reversibility: 0

## Mission

Build the library half of the SP-020 rules-compliance **annotation**: for each finding in a hunk, ask System One (choice protocol) whether applying the finding's suggestion would conflict with one of the applicable project guideline sections, and record the most likely conflicting section on the finding. Human-visible only — **never drop, reorder, or rescore findings**.

**Constraints from `docs/research/systemone-rules-compliance.md` (Recommendation):** choice protocol, not noul-per-premise; batch ≤ 32 questions per request; skip (do not degrade) when the question budget is exceeded; surface premise text + confidence; no automatic drops at any threshold (11% injection flip rate measured).

**API surface (adjust names to local style):**

```go
// cli/internal/findings/finding.go
type RuleConflict struct {
    Source      string  `json:"source"`      // rule file, e.g. "go-1-21-development-standards.mdc"
    Section     string  `json:"section"`     // section heading
    Probability float64 `json:"probability"` // P(choice) from System One
}
// Finding gains: RuleConflict *RuleConflict `json:"rule_conflict,omitempty"`

// cli/internal/review/rules_annotation.go
func AnnotateRuleConflicts(ctx context.Context, client rules.ChoiceDecider, model string, batch []findings.Finding, hunkContent string, sections []rules.Section) ([]findings.Finding, error)
```

- One choice question per finding (`q0…qN`), at most `annotateMaxQuestionsPerRequest` (32) questions per `DecideChoice` request.
- Options: at most `annotateMaxOptions` (32) sections as `s0…sK` (description `Source + ": " + Heading`) plus `none` ("Applying the suggestion conflicts with none of these guidelines.").
- State = hunk content (same prefix style as `review/critic_systemone.go`). Instructions contain the finding's message and suggestion (LLM-generated text, not raw diff) and ask which guideline applying the suggestion would conflict with.
- Annotate when answer `Choice != "none"` and its probability ≥ `annotateMinProbability` (0.5); otherwise leave `RuleConflict` nil.
- Skip with no request (return a copy of `batch` unchanged) when `client == nil`, `model == ""`, `len(sections) == 0`, `len(batch) == 0`, or `len(batch) > annotateMaxQuestions` (64, the SP-020 question budget).
- Returns a new slice of the same length and order; input slice not mutated. Decider errors wrapped with context; on error return `(nil, err)` — callers decide whether to warn and continue.
- `Finding` JSON round-trip unchanged when `RuleConflict` is nil (`omitempty`).

## Dependencies

- **Task:** SP-026 (`rules.Section`, `rules.ChoiceDecider`)
- **Task:** SP-032 (`systemone.ChoiceRequest` / `DecideChoice`)
- **Task:** SP-031 (repo-wide gofmt touches `findings/` and `review/`)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `docs/research/systemone-rules-compliance.md` — Q3, Q4, Q5 (annotation), Recommendation
- `cli/internal/review/critic_systemone.go` — batching and state pattern
- `cli/internal/rules/selector.go` — `Section`, `ChoiceDecider` (SP-026)
- `cli/internal/findings/finding.go`

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none (tests use a fake `rules.ChoiceDecider`)

## File Scope

- `cli/internal/review/rules_annotation.go` (new)
- `cli/internal/review/rules_annotation_test.go` (new)
- `cli/internal/findings/finding.go`
- `cli/internal/findings/finding_test.go`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go vet ./internal/review/... ./internal/findings/... && go test ./internal/review/... ./internal/findings/... -count=1 -cover` |
| fileScopeMustChange | `cli/internal/review/rules_annotation.go`, `cli/internal/review/rules_annotation_test.go`, `cli/internal/findings/finding.go` |
| fileScopeMustNotChange | `cli/internal/review/review.go`, `cli/internal/review/critic.go`, `cli/internal/review/critic_systemone.go`, `cli/internal/run/**`, `cli/internal/config/**`, `cli/cmd/**`, `extension/**` |
| completionCriteria | Annotation never changes finding count/order/content other than `RuleConflict`; batching, option cap, skip budget, and threshold behave per Mission; `rules_annotation.go` coverage ≥ 72% |

## Steps

### Step 1: Finding field

- [ ] `RuleConflict` type and optional `Finding.RuleConflict` with `omitempty`

### Step 2: Annotator

- [ ] `AnnotateRuleConflicts` per Mission (batching, option cap, skip budget, threshold, copy semantics)

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Fake decider: annotation set/unset at threshold; `none` never annotates; 40 findings → 2 requests; 65 findings → 0 requests and unchanged output; > 32 sections capped; input slice not mutated; output length/order equal input
- [ ] Error path wraps context; nil client / empty model / no sections skip
- [ ] JSON: nil `RuleConflict` omitted; populated round-trips
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Per-file coverage ≥ 72% for `rules_annotation.go` and `finding.go`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Implementation:** `feat(SP-027): description`
- **Tests:** `test(SP-027): description`

## Do NOT

- Drop, reorder, or rescore findings based on the annotation
- Wire the annotator into the pipeline, config, CLI, or extension (SP-034 does that)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
