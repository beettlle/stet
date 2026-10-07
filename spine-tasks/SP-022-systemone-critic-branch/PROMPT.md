# Task: SP-022 — System One critic branch in review pipeline

**Created:** 2026-09-30
**Size:** S

## Review Level: 2

**Assessment:** Adds a new verification path inside the core review loop that decides which findings users see.
**Score:** 4/8 — Blast radius: 2, Pattern novelty: 1, Security: 1, Reversibility: 0

## Mission

Make the review pipeline able to verify findings with a System One server instead of the per-finding LLM critic. This task adds the backend branch and the run options that carry it; the CLI flag and client construction come in SP-023.

**Background (System One / OpenJev):** OpenJev ([razorback16/openjev](https://github.com/razorback16/openjev)) exposes `POST /v1/systemone` with `{state, model, questions}`; a `noul` question returns `{noul: P(yes)}`. Same wire API as hosted Jev. Verified local response: `{"model":"openjev-0.1","answers":{"id":{"type":"noul","noul":0.9997}},"usage":{"input_tokens":246,"output_tokens":0}}`; warm latency ~100 ms per request (one request per hunk).

**Prerequisites delivered by earlier tasks:**

- SP-016: `review.VerifyFindingsSystemOne(ctx, client, model, batch, hunkContent, minKeep, samples) (kept, scores, err)` and the `review.SystemOneDecider` interface.
- SP-017: a single `applyCritic` helper in `cli/internal/run/run.go` used by both branches of `runReviewPipeline`.

**Required changes (all in `cli/internal/run/run.go`):**

1. Add to `reviewPipelineOpts`, `StartOptions`, and `RunOptions`: `CriticBackend string`, `SystemOneClient review.SystemOneDecider`, `SystemOneBaseURL string` (for user-facing error messages only), `SystemOneModel string`, `SystemOneMinKeep float64`, `SystemOneSamples int`. Pass them through everywhere `Start`/`Run` build `reviewPipelineOpts` (follow how `CriticEnabled`/`CriticModel` flow today).
2. In `applyCritic`, when `CriticBackend == "systemone"`, call `VerifyFindingsSystemOne` with `p.Hunk.RawContent`; otherwise keep the existing LLM path unchanged. The `CriticModel != ""` guard applies only to the LLM backend; the System One branch requires a non-nil `SystemOneClient`.
3. **Errors:** match today's abort behaviour — `erruser.New("Critic failed for "+path+" (System One at "+SystemOneBaseURL+").", err)`. Never include an API key.
4. **Trace:** when tracing, print `Critic (systemone): N -> M` and one line per finding: `score=<P(yes), 4 decimals> id=<finding id>`. Never print request bodies.
5. **Dry run:** dry-run never reaches `applyCritic` with the System One backend (verify; add a guard if the LLM critic relies on an early return elsewhere).

## Dependencies

- **Task:** SP-016 (`VerifyFindingsSystemOne`)
- **Task:** SP-017 (`applyCritic` helper; shares `run.go`)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `cli/internal/run/run.go` — `reviewPipelineOpts`, `applyCritic`, `StartOptions`, `RunOptions`, `Start`, `Run`
- `cli/internal/run/run_test.go` — critic test patterns
- `cli/internal/review/critic_systemone.go` (SP-016)
- `AGENTS.md` — coverage gates

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none (tests use a fake `review.SystemOneDecider`)

## File Scope

- `cli/internal/run/run.go`
- `cli/internal/run/run_test.go`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./internal/run/... -count=1 -cover` |
| fileScopeMustChange | `cli/internal/run/run.go`, `cli/internal/run/run_test.go` |
| fileScopeMustNotChange | `cli/internal/review/**`, `cli/internal/systemone/**`, `cli/internal/config/**`, `cli/cmd/**`, `extension/**`, `docs/**` |
| completionCriteria | `applyCritic` routes to System One when `CriticBackend == "systemone"`; options flow through Start/Run; errors abort with user message; dry-run never calls it; tests pass with ≥72% coverage on `run.go` |

## Steps

### Step 1: Options and pass-through

- [ ] Add System One fields to `reviewPipelineOpts`, `StartOptions`, `RunOptions`
- [ ] Pass them through every place that builds `reviewPipelineOpts`

### Step 2: Backend branch

- [ ] Branch in `applyCritic`; LLM path untouched
- [ ] Error message and trace output per Mission
- [ ] Confirm dry-run never calls System One

### Step 3: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Fake decider: keeps/drops by score; decider error aborts with "Critic failed for"; `llm` backend still uses `VerifyFinding`; dry-run makes zero decider calls; trace lines present when tracing
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Per-file coverage ≥ 72% for `run.go`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing
- [ ] Default behaviour (`CriticBackend` empty or `llm`) unchanged

## Git Commit Convention

- **Implementation:** `feat(SP-022): description`
- **Tests:** `test(SP-022): description`

## Do NOT

- Add the `--critic-backend` flag or build the client in `main.go` (SP-023)
- Update docs (SP-023)
- Change LLM critic behaviour
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
