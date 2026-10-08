# Task: SP-028 — Rules-compliance labeled pairs dataset and evaluator

**Created:** 2026-10-08
**Size:** S

## Review Level: 1

**Assessment:** New file in an existing evaluation package plus a committed test dataset; evaluator sends synthetic snippets to the configured System One endpoint.
**Score:** 2/8 — Blast radius: 0, Pattern novelty: 1, Security: 1, Reversibility: 0

## Mission

Give stet a reusable labeled set of (code snippet, project-rule premise, label) pairs and an evaluator that measures how well System One judges rule compliance. This is the "validate before productizing" condition from `docs/research/systemone-rules-compliance.md` (Recommendation item 5): the SP-020 set had only 24 Go pairs written by one author.

**Seed:** `spine-tasks/SP-028-rule-pairs-dataset/seed/sp020-labeled-set.json` — the SP-020 set: `premises` (12, each `{id, source, rule}`) and `pairs` (24, each `{id, premise, label: "violating"|"compliant", code}`), all Go. Convert all 24 seed pairs into the new format and extend.

**Dataset requirements (`cli/internal/criticeval/testdata/rulepairs/pairs.jsonl`):**

- ≥ 100 pairs; ≥ 2 languages (Go and TypeScript at minimum, ≥ 30 pairs each); ≥ 20 distinct premises.
- Every premise has ≥ 1 violating and ≥ 1 compliant pair; overall violating share between 40% and 60%.
- Snippets are synthetic, self-contained, 3–15 lines; no real user code, secrets, or URLs to private hosts.
- Premises are short, atomic, code-checkable statements (style of the seed premises), each with a `source` naming the rule area it paraphrases (e.g. `go-1-21-development-standards`, `general-llm-anti-patterns`).
- Include at least 10 "hard negatives": compliant snippets that superficially resemble a violation (e.g. a goroutine *with* WaitGroup tracking).

**API surface (adjust names to local style):**

```go
type RulePairLabel string // "violating" | "compliant"
type RulePair struct {
    ID        string        `json:"id"`
    Language  string        `json:"language"`
    PremiseID string        `json:"premise_id"`
    Premise   string        `json:"premise"`
    Source    string        `json:"source"`
    Snippet   string        `json:"snippet"`
    Label     RulePairLabel `json:"label"`
    Origin    string        `json:"origin"` // "sp020-seed" | "synthetic"
}
func LoadRulePairs(path string) ([]RulePair, error)
type RulePairReport struct {
    Total, Violating, Compliant int
    Accuracy float64            // at threshold
    AUROC    float64            // rank-based Mann–Whitney; NaN-free: 0 when a class is empty
    ByLanguage map[string]float64 // accuracy per language
    Misses   []string           // pair IDs misclassified
}
func EvaluateRulePairs(ctx context.Context, client review.SystemOneDecider, model string, pairs []RulePair, threshold float64) (RulePairReport, error)
```

- `LoadRulePairs`: JSON Lines; rejects unknown labels, empty required fields, duplicate IDs; errors name the line number.
- `EvaluateRulePairs` uses the noul protocol (SP-020 Protocol A): state = snippet; instructions = `"Project rule: " + premise + "\n\nDoes this code violate the rule?"`; criteria true = "The code violates the stated project rule.", false = "The code complies with the rule or the rule does not apply to this code." P(yes) ≥ threshold ⇒ predicted violating. Pairs sharing an identical snippet are batched into one request (≤ 32 questions per request, named constant); errors wrap the pair ID. The premise text goes in instructions, the snippet only in state.

## Dependencies

- **None**

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `docs/research/systemone-rules-compliance.md` — Method (Q3/Q4), Risks, Recommendation
- `cli/internal/criticeval/dataset.go`, `cli/internal/criticeval/sweep.go` — package style, AUROC/percentile helpers to reuse if present
- `cli/internal/review/critic_systemone.go` — `SystemOneDecider`

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none (tests use a fake `review.SystemOneDecider`)

## File Scope

- `cli/internal/criticeval/rulepairs.go` (new)
- `cli/internal/criticeval/rulepairs_test.go` (new)
- `cli/internal/criticeval/testdata/rulepairs/pairs.jsonl` (new)
- `cli/internal/criticeval/testdata/rulepairs/*.jsonl` (new, invalid-input fixtures)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go vet ./internal/criticeval/... && go test ./internal/criticeval/... -count=1 -cover` |
| fileScopeMustChange | `cli/internal/criticeval/rulepairs.go`, `cli/internal/criticeval/rulepairs_test.go`, `cli/internal/criticeval/testdata/rulepairs/pairs.jsonl` |
| fileScopeMustNotChange | `cli/internal/criticeval/dataset.go`, `cli/internal/criticeval/sweep.go`, `cli/internal/review/**`, `cli/cmd/**`, `extension/**` |
| completionCriteria | A test loads the committed `pairs.jsonl` and asserts every dataset requirement in Mission; loader and evaluator tests pass with exact expected metrics; `rulepairs.go` coverage ≥ 72% |

## Steps

### Step 1: Format, loader, evaluator

- [ ] `RulePair` types, `LoadRulePairs` with validation
- [ ] `EvaluateRulePairs` (noul protocol, snippet batching, accuracy, AUROC, per-language accuracy, misses)

### Step 2: Dataset

- [ ] Convert the 24 seed pairs (`origin: "sp020-seed"`)
- [ ] Add ≥ 76 synthetic pairs meeting every Mission requirement (Go + TypeScript, ≥ 20 premises, hard negatives)

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Dataset-invariant test over the committed `pairs.jsonl` (counts, languages, premise coverage, label balance, unique IDs, snippet line bounds)
- [ ] Loader error cases with line numbers (bad JSON, unknown label, empty field, duplicate ID)
- [ ] Evaluator with fake decider: exact accuracy/AUROC/misses; batching call count; decider error surfaces pair ID; empty input
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Per-file coverage ≥ 72% for `rulepairs.go`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Implementation:** `feat(SP-028): description`
- **Dataset:** `test(SP-028): description`

## Do NOT

- Modify the existing critic-eval dataset loader or sweep (`dataset.go`, `sweep.go`)
- Add a CLI command or flag (log "expose rule-pair eval via `stet critic-eval`" as backlog in `spine-tasks/CONTEXT.md` if worthwhile)
- Copy real repository code, secrets, or private URLs into the dataset
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)

- 2026-10-08 (worker): Synthetic pairs (78 of 102) were authored by a single agent in one session, so the SP-020 independence condition (multiple authors) remains only partially met — noted in STATUS Discoveries. Mitigations: premises span 4 rule-area sources and two languages, and 13 hard negatives target the specific false-positive modes flagged in the research doc.
- 2026-10-08 (worker): Seed pair V04 snippet padded from 2 to 5 lines (same hardcoded-key code wrapped in a function) so every snippet meets the dataset-wide 3–15 line bound; all other 23 seed snippets converted verbatim. Seed premise `source` values normalized by dropping the `.mdc` extension to match the PROMPT examples.
