# Task: SP-069 — Critic sees the reviewer's expanded context and RAG definitions

**Created:** 2026-10-24
**Size:** M

## Review Level: 2

**Assessment:** Changes what both critic backends receive; touches `cli/internal/run/run.go` (hot shared module) and `cli/internal/review/critic*.go`.
**Score:** 4/8 — Blast radius: 2, Pattern novelty: 1, Security: 0, Reversibility: 1 — Level 2 for the hot module

## Mission

Partial #18

Both critic backends judge findings against `p.Hunk.RawContent` only (`applyCritic` and `applyCriticSystemOne` in `cli/internal/run/run.go` ~lines 626–693), capped at 4096 characters (`defaultCriticMaxHunkLen` in `cli/internal/review/critic.go`). The reviewer saw more: `review.PrepareHunkPrompt` builds the user prompt from the expanded hunk (file imports, enclosing function) plus the RAG symbol-definitions and call-graph block. Suppression examples go into the **system** prompt, so the user prompt holds only code context. The critic therefore cannot refute "X is not defined / not imported / not exported" claims whose evidence the reviewer had.

Change:

1. In `run.go`, pass the prepared user prompt (`p.User`) to both critics instead of `p.Hunk.RawContent`, in `applyCritic` (LLM, `review.VerifyFinding`) and `applyCriticSystemOne` (`review.VerifyFindingsSystemOne`). When `p.User` is empty (defensive), fall back to `p.Hunk.RawContent`.
2. In `cli/internal/review/`, add a named constant for the critic context cap (16384 characters) and use it as the default in `BuildCriticPrompt` / `VerifyFinding` and in the System One critic state. The rules-annotation request (`rules_annotation.go`) must keep its current 4096-character cap: if it shares `buildSystemOneState`, give the critic its own cap parameter rather than changing annotation behavior. Existing truncation-marker behavior (`\n[truncated]`) stays.
3. When tracing is enabled, print the critic input length once per hunk (`Critic input: N chars`) next to the existing `Critic:` line.

## Dependencies

- **None**

## Context to Read First

**Tier 3 (reference docs):**

- `cli/internal/run/run.go` — `preparedPrompt` (~line 152), `runReviewPipeline` (~line 335), `applyCritic`, `applyCriticSystemOne`
- `cli/internal/review/review.go` — `PrepareHunkPrompt` (~line 106): what goes into `user` versus `system`
- `cli/internal/review/critic.go`, `cli/internal/review/critic_systemone.go`, `cli/internal/review/rules_annotation.go`
- `cli/internal/review/critic_test.go`, `cli/internal/review/critic_systemone_test.go`, `cli/internal/run/run_test.go` — existing critic tests and fakes

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/run/run.go`
- `cli/internal/run/run_test.go`
- `cli/internal/review/critic.go`
- `cli/internal/review/critic_systemone.go`
- `cli/internal/review/critic_test.go`
- `cli/internal/review/critic_systemone_test.go`

## Documentation Requirements

**Must Update:** none (SP-076 updates `docs/review-process-internals.md`).

**Check If Affected:** `docs/cli-extension-contract.md` — only if it states what the critic receives; record under Discoveries rather than editing.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/review/... ./cli/internal/run/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/run/run.go`, `cli/internal/review/critic.go`, `cli/internal/review/critic_systemone.go` |
| fileScopeMustNotChange | `cli/internal/review/rules_annotation.go`, `cli/internal/review/review.go`, `cli/internal/run/restore_dismissals.go`, `cli/cmd/**`, `extension/**`, `.review/**` |
| completionCriteria | A run-package test with a fake LLM critic and a fake System One decider asserts each critic input contains the hunk's expanded-context section and a RAG definition from the prepared user prompt (test per backend); critic input over the new cap is truncated with `[truncated]` (test); the rules-annotation 4096-character truncation test still passes unchanged; stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Critic context cap in the review package

- [ ] Named critic context cap (16384) used by the LLM critic and the System One critic state; annotation cap unchanged
- [ ] `spine_review_step` after the step

### Step 2: Pass the prepared user prompt to both critics

- [ ] `applyCritic` and `applyCriticSystemOne` use `p.User` (fallback `p.Hunk.RawContent`); trace prints critic input length
- [ ] `spine_review_step` after the step

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Tests for every case in Contract `completionCriteria`
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Feature:** `feat(SP-069): description`
- **Tests:** `test(SP-069): description`

## Do NOT

- Change critic prompts, criteria, thresholds (`systemone_min_keep`), or the reviewer prompt
- Refactor the duplicated post-filter blocks in `runReviewPipeline` (SP-070 adds a filter there next)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
