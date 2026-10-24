# Task: SP-070 — Drop "unused / undefined / not imported" findings the file disproves

**Created:** 2026-10-24
**Size:** M

## Review Level: 2

**Assessment:** New pure filter in `cli/internal/findings` plus wiring in `cli/internal/run/run.go` (hot shared module). Must stay conservative: a dropped true positive is worse than a kept false positive.
**Score:** 4/8 — Blast radius: 2, Pattern novelty: 1, Security: 0, Reversibility: 1 — Level 2 for the hot module

## Mission

Closes #33

The largest class of false positives in the pi-spine release was the reviewer claiming a symbol is missing, unused, undefined, not imported, or not declared when the file proves otherwise (SP-829: "`createRulesFixture` uses `fs` without importing it" while `import fs from "node:fs"` is line 2; SP-831: ~19 findings claiming `compareTaskIds` is missing).

Add a deterministic post-filter next to `FilterFPKillList`, `FilterPraiseOnly`, and `FilterByAddedLines`:

1. **Library** — new `cli/internal/findings/existence.go` with `FilterExistenceClaims(batch []Finding, fileContent string) []Finding` (name may differ; keep it pure, no I/O). For each finding:
   - Detect an existence claim in the message: unused, undefined, not defined, not imported, not declared, missing import, ReferenceError, "is not exported" about the file's own symbol. Match case-insensitively.
   - Extract backticked identifiers from the message (`` `fs` ``, `` `compareTaskIds` ``). Keep only valid identifiers.
   - Drop the finding only when **every** extracted identifier is proven by `fileContent`: for "not imported / undefined / not defined / not declared / ReferenceError", the identifier is imported, required, declared (`function`, `const`, `let`, `var`, `class`, Go `func`/`var`/`const`/`type`, Python `def`/`class`/`import`), or a parameter of a declaration in the file; for "unused", the identifier occurs at least twice as a whole word outside its declaration line; for "not exported", the file exports it.
   - Keep the finding when the message has no existence claim, no identifier can be extracted, `fileContent` is empty, or the claim is about another module ("not exported from `./x`").
2. **Wiring** — in `runReviewPipeline` (`run.go`), apply the filter in **both** post-filter blocks (the pipelined branch ~line 491 and the drain branch ~line 577), after `FilterByAddedLines` and before `applyCritic`, so the critic never spends a request on these. Read the post-change file once per hunk from `filepath.Join(opts.RepoRoot, p.Hunk.FilePath)`; on a read error pass an empty string (filter keeps everything). Trace `Existence claims: N -> M` like the neighboring filters. Apply it in every strictness mode.

## Dependencies

- **Task:** SP-069 (both edit `cli/internal/run/run.go` `runReviewPipeline`)

## Context to Read First

**Tier 3 (reference docs):**

- `cli/internal/findings/fpkilllist.go`, `cli/internal/findings/praise.go`, `cli/internal/findings/evidence.go` — filter shape, naming, and tests
- `cli/internal/run/run.go` — `runReviewPipeline` post-filter blocks
- GitHub issue #33 body (`gh issue view 33 --repo beettlle/stet`) — the pi-spine examples to turn into fixtures

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/findings/existence.go`
- `cli/internal/findings/existence_test.go`
- `cli/internal/run/run.go`
- `cli/internal/run/run_test.go`

## Documentation Requirements

**Must Update:** none (SP-076 updates `docs/review-process-internals.md`).

**Check If Affected:** `docs/cli-extension-contract.md` — only if it lists trace post-filter lines; record under Discoveries rather than editing.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/findings/... ./cli/internal/run/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/findings/existence.go`, `cli/internal/findings/existence_test.go`, `cli/internal/run/run.go` |
| fileScopeMustNotChange | `cli/internal/review/**`, `cli/internal/run/restore_dismissals.go`, `cli/internal/expand/**`, `cli/internal/rag/**`, `extension/**`, `.review/**` |
| completionCriteria | Table tests per claim type with a dropped case (symbol present) and a kept case (symbol genuinely absent); fixtures reproducing the #33 examples (SP-829 `fs` import, SP-831 `compareTaskIds`, used import flagged unused, declared parameter flagged undefined) are dropped; messages without backticked identifiers, cross-module "not exported" claims, and empty file content are kept; a run-package test shows the filter runs before the critic (critic fake receives no dropped finding); stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: `FilterExistenceClaims` library and table tests

- [ ] Claim detection, identifier extraction, and per-claim proof rules; conservative keeps
- [ ] `spine_review_step` after the step

### Step 2: Wire into both pipeline branches

- [ ] Filter runs after the evidence filter and before the critic in both branches; trace line added
- [ ] `spine_review_step` after the step

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] #33 fixture tests and the run-package ordering test
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Feature:** `feat(SP-070): description`
- **Tests:** `test(SP-070): description`

## Do NOT

- Read other modules to verify cross-module "not exported" claims
- Change existing filters, critic behavior, or prompts
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
