# Task: SP-053 — Drop praise-only findings

**Created:** 2026-10-17
**Size:** S

## Review Level: 2

**Assessment:** New post-filter in `findings`, one prompt sentence, wiring in `cli/internal/run/run.go` (hot shared module).
**Score:** 4/8 — Blast radius: 2, Pattern novelty: 1, Security: 0, Reversibility: 1 — Level 2 for the hot module

## Mission

Partial #25

The reviewer sometimes emits confident `info` findings that only praise the change or confirm it is correct ("This correctly handles X"). Lenient strictness raises confidence thresholds only, and `FilterFPKillList` (`cli/internal/findings/fpkilllist.go`) bans hedging phrases only, so one praise finding fails the spine stet contract.

Implement #25 options 1 and 2 (option 3, `stet list --min-severity`, is a maintainer decision and out of scope):

1. **Prompt rule.** In `DefaultSystemPrompt` (`cli/internal/prompt/prompt.go`, Output step 4, ~line 48) add one sentence: do not report findings that only praise the change, confirm it is correct, or describe it without a required action.
2. **Deterministic backstop.** New file `cli/internal/findings/praise.go` with `FilterPraiseOnly(list []Finding) []Finding`: drop a finding when **all** hold — severity is `info` or `nitpick` (use the existing severity constants in `finding.go`), `Suggestion` is empty after trimming, and `Message` matches a praise pattern (case-insensitive; for example `correctly`, `properly handles`, `good `, `well done`, `nice `, `looks good`, `is correct`). Keep the pattern list in a named package-level slice compiled once, like `fpkilllist.go`. Return a new slice, preserve order, never mutate input.
3. **Wiring.** Apply it wherever `FilterFPKillList` is applied in `cli/internal/run/run.go` (~lines 493, 574, 1361), inside the same `ApplyFP` / `applyFP` guard so nitpicky mode and the `+` strictness presets skip it. Add a trace line `Praise filter: N -> M` next to the existing `FP kill list` trace line.

## Dependencies

- **None**

## Context to Read First

**Tier 3 (reference docs):**

- `cli/internal/findings/fpkilllist.go`, `fpkilllist_test.go` — pattern to mirror
- `cli/internal/findings/finding.go` — severity constants
- `cli/internal/run/run.go` — post-filter blocks (~lines 485–505, 570–585, 1355–1365)
- `cli/internal/prompt/prompt.go` — `DefaultSystemPrompt`

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/findings/praise.go`
- `cli/internal/findings/praise_test.go`
- `cli/internal/prompt/prompt.go`
- `cli/internal/prompt/prompt_test.go`
- `cli/internal/run/run.go`
- `cli/internal/run/run_test.go`

## Documentation Requirements

**Must Update:** none (SP-056 updates `docs/review-process-internals.md`)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/findings/... ./cli/internal/prompt/... ./cli/internal/run/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/findings/praise.go`, `cli/internal/findings/praise_test.go`, `cli/internal/prompt/prompt.go`, `cli/internal/run/run.go` |
| fileScopeMustNotChange | `cli/cmd/**`, `cli/internal/findings/evidence.go`, `extension/**`, `scripts/**`, `.review/**` |
| completionCriteria | A praise-only `info` finding with no suggestion is dropped (test); an `info` finding with a suggestion is kept (test); a `warning`/`error` praise-worded finding is kept (test); filter skipped when FP filtering is off; prompt contains the new rule; stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Filter and prompt rule

- [ ] `praise.go` `FilterPraiseOnly` with compiled patterns
- [ ] `DefaultSystemPrompt` Output step 4 sentence
- [ ] `spine_review_step` after the step

### Step 2: Wire into run

- [ ] All three `FilterFPKillList` sites also apply `FilterPraiseOnly` under the same guard; trace line added
- [ ] `spine_review_step` after the step

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] `praise_test.go`: drop (info, no suggestion, praise); keep (info with suggestion); keep (warning praise-worded); keep (info, no praise words); empty input → nil; input not mutated
- [ ] `prompt_test.go`: default system prompt contains the new sentence
- [ ] `run_test.go`: with FP filtering on, a canned/dry-run praise-only info finding is not stored; with nitpicky (FP off), it is (reuse existing dry-run test helpers)
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing; `praise.go` ≥ 72% line coverage

## Git Commit Convention

- **Feature:** `feat(SP-053): description`
- **Tests:** `test(SP-053): description`

## Do NOT

- Add `stet list --min-severity` or change the contract script's counting
- Change the evidence filter (SP-054 owns that)
- Drop `warning` or `error` findings in this filter
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
