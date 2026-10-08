# Task: SP-026 — Rule-section selector (library)

**Created:** 2026-10-08
**Size:** S

## Review Level: 1

**Assessment:** New selection logic in `rules` plus a prompt helper; sends hunk text to the configured System One endpoint (only when a later task wires it in).
**Score:** 3/8 — Blast radius: 1, Pattern novelty: 2, Security: 0, Reversibility: 0

## Mission

Build the library half of the SP-020 "pre-prompt rule selector": split applicable Cursor rules into sections and ask System One **one choice question per hunk** to rank which sections matter, then fill the `MaxRuleTokens` budget with the top-ranked sections instead of blindly truncating the tail.

**Why (measured in `docs/research/systemone-rules-compliance.md`, Q5):** the always-apply rules alone are ~88 K chars (~22 K tokens) against `rules.MaxRuleTokens = 1000`; `prompt.AppendCursorRules` concatenates and truncates from the end, so ~95.5% of applicable rule content never reaches the review model and what survives is positional, not relevant.

**API surface (adjust names to local style):**

```go
// cli/internal/rules/selector.go
type Section struct {
    Source  string // rule file name, e.g. "go-1-21-development-standards.mdc"
    Heading string // markdown heading text; Source for text before the first heading
    Content string // heading line + body, trimmed
}
func SplitSections(r CursorRule) []Section
type ChoiceDecider interface {
    DecideChoice(ctx context.Context, req systemone.ChoiceRequest) (*systemone.ChoiceResponse, error)
}
func SelectSections(ctx context.Context, client ChoiceDecider, model, hunkContent string, sections []Section, maxTokens int) ([]Section, error)

// cli/internal/prompt/prompt.go
func AppendRuleSections(systemPrompt string, sections []rules.Section, maxRuleTokens int) string
```

- `SplitSections`: split `Content` at markdown headings of level 2 or 3 (`## `, `### `), ignoring headings inside fenced code blocks; drop empty sections.
- `SelectSections`: one `DecideChoice` request with one question; state = hunk content (prefixed like `review/critic_systemone.go` does); options `s0…sN` for at most `maxSelectorOptions` sections (named constant, 64; extra sections beyond the cap are not candidates), option description = `Source + ": " + Heading` (never the section body); instructions ask which guideline section is most relevant to reviewing the change. Rank by returned probability (desc, ties by input order) and greedily keep sections whose cumulative `tokens.Estimate` fits `maxTokens`, skipping any section that alone exceeds the remaining budget. Returns sections in ranked order. `nil` client, empty model, or no sections → error (caller decides fallback). Decider errors are wrapped with context.
- `AppendRuleSections`: same `## Project review criteria` header as `AppendCursorRules`; appends section contents in the given order; still applies `truncateToTokenBudget` as a safety net; unchanged prompt when `sections` is empty.
- Security: hunk content is attacker-influenced; pass it only in state, never in instructions or option descriptions.

## Dependencies

- **Task:** SP-029 (`**` glob fix determines which rules are candidates; same package)
- **Task:** SP-032 (`systemone.ChoiceRequest` / `DecideChoice`)
- **Task:** SP-031 (repo-wide gofmt touches `prompt.go` and `rules/`)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `docs/research/systemone-rules-compliance.md` — Q5, Recommendation items 1–3
- `cli/internal/rules/rules.go`, `cli/internal/prompt/prompt.go` (`AppendCursorRules`, `truncateToTokenBudget`)
- `cli/internal/systemone/client.go` — choice types (SP-032)
- `cli/internal/review/critic_systemone.go` — state prefix and security note pattern

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none (tests use a fake `ChoiceDecider`)

## File Scope

- `cli/internal/rules/selector.go` (new)
- `cli/internal/rules/selector_test.go` (new)
- `cli/internal/prompt/prompt.go`
- `cli/internal/prompt/prompt_test.go`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go vet ./internal/rules/... ./internal/prompt/... && go test ./internal/rules/... ./internal/prompt/... ./internal/review/... -count=1 -cover` |
| fileScopeMustChange | `cli/internal/rules/selector.go`, `cli/internal/rules/selector_test.go`, `cli/internal/prompt/prompt.go`, `cli/internal/prompt/prompt_test.go` |
| fileScopeMustNotChange | `cli/internal/rules/rules.go`, `cli/internal/run/**`, `cli/internal/config/**`, `cli/internal/review/**`, `cli/cmd/**`, `extension/**` |
| completionCriteria | Splitting, one-request ranking, budget filling, option cap, and prompt assembly behave per Mission; hunk text never appears in instructions/options; `selector.go` and `prompt.go` coverage ≥ 72% |

## Steps

### Step 1: Sections and selection

- [ ] `Section`, `SplitSections` (code-fence aware)
- [ ] `ChoiceDecider`, `SelectSections` (single request, option cap, ranking, greedy budget fill)

### Step 2: Prompt assembly

- [ ] `AppendRuleSections` in `prompt.go`; `AppendCursorRules` unchanged

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] `SplitSections`: preamble, `##`/`###`, headings inside code fences ignored, empty sections dropped
- [ ] `SelectSections` with fake decider: exactly one request; request has no hunk text outside state; ranking and tie order; budget skip of oversized section; >64 sections capped; nil client / empty model / no sections / decider error
- [ ] `AppendRuleSections`: order preserved, header, empty input, safety truncation
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Per-file coverage ≥ 72% for `selector.go` and `prompt.go`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Implementation:** `feat(SP-026): description`
- **Tests:** `test(SP-026): description`

## Do NOT

- Wire the selector into the review pipeline, config, or CLI (SP-033 does that)
- Change `FilterRules`, `AppendCursorRules`, or `MaxRuleTokens`
- Put hunk content into question instructions or option descriptions
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
