# Task: SP-017 — Extract applyCritic helper in review pipeline

**Created:** 2026-09-30
**Size:** S

## Review Level: 2

**Assessment:** Pure refactor of the core review loop; no behaviour change, but the loop is central and order-sensitive.
**Score:** 4/8 — Blast radius: 2, Pattern novelty: 0, Security: 0, Reversibility: 2

## Mission

`runReviewPipeline` in `cli/internal/run/run.go` contains two identical critic blocks, one in each branch of its main loop (the `if opts.CriticEnabled && opts.CriticModel != "" && len(batch) > 0 { ... }` sections, currently around lines 360–386 and 463–492). Extract them into one helper so the upcoming System One critic backend (SP-022) is added in one place instead of two.

Suggested shape (adjust names to local style):

```go
func applyCritic(ctx context.Context, opts reviewPipelineOpts, p *preparedPrompt, batch []findings.Finding) ([]findings.Finding, error)
```

Requirements:

- **No behaviour change.** Same guard, same `CriticOptions` construction (`RetryOnParseError`, `MaxCompletionTokens`, `KeepAlive` when critic model equals main model), same per-finding `review.VerifyFinding` loop, same trace lines (`Critic request failed: ...`, `Critic: N -> M`), same `erruser.New("Critic failed for "+path+".", err)` on error.
- Keep the existing comment about O(N) sequential calls on the helper.
- The two call sites become a single call each; everything around them (abstention, FP kill list, evidence filter, cursor URIs, streaming) is untouched.

This task has no dependency on the System One work and can run first.

## Dependencies

- **None**

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `cli/internal/run/run.go` — `reviewPipelineOpts`, `preparedPrompt`, `runReviewPipeline`
- `cli/internal/run/run_test.go` — existing critic tests
- `cli/internal/review/critic.go` — `VerifyFinding`, `CriticOptions`
- `AGENTS.md` — coverage gates

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none

## File Scope

- `cli/internal/run/run.go`
- `cli/internal/run/run_test.go`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./internal/run/... -count=1 -cover` |
| fileScopeMustChange | `cli/internal/run/run.go` |
| fileScopeMustNotChange | `cli/internal/review/**`, `cli/internal/config/**`, `cli/cmd/**`, `extension/**`, `docs/**` |
| completionCriteria | One `applyCritic` helper used by both loop branches; existing run tests pass unchanged; ≥72% coverage on `run.go` |

## Steps

### Step 1: Extract helper

- [ ] Add `applyCritic` with the exact current logic
- [ ] Replace both inline critic blocks with calls to it
- [ ] Confirm no other lines in `runReviewPipeline` changed (`git diff` review)

### Step 2: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Existing critic tests pass without modification
- [ ] Add a direct `applyCritic` unit test (keep, drop, error) with a fake `CriticClient` if coverage needs it
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Per-file coverage ≥ 72% for `run.go`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing
- [ ] Observable behaviour (findings, trace output, errors) identical

## Git Commit Convention

- **Refactor:** `refactor(SP-017): description`
- **Tests:** `test(SP-017): description`

## Do NOT

- Add the System One backend (SP-022)
- Change critic defaults, trace wording, or error messages
- Refactor unrelated parts of `runReviewPipeline`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
