# Task: SP-024 — Critic evaluation scoring and threshold sweep

**Created:** 2026-09-30
**Size:** S

## Review Level: 1

**Assessment:** Pure computation over System One scores plus batched scoring calls; sends past code to the configured endpoint.
**Score:** 3/8 — Blast radius: 0, Pattern novelty: 2, Security: 1, Reversibility: 0

## Mission

Score a labeled dataset with the System One critic and compute, for each candidate threshold, how many false positives would be dropped and how many real findings would be lost. This produces the recommendation that `stet critic-eval` (SP-025) prints.

**Background (System One / OpenJev):** OpenJev ([razorback16/openjev](https://github.com/razorback16/openjev)) exposes `POST /v1/systemone`; a `noul` question returns P(yes). Same wire API as hosted Jev. Warm latency ~100 ms per request. SP-016 provides `review.VerifyFindingsSystemOne(ctx, client, model, batch, hunkContent, minKeep, samples) (kept, scores, err)` and the `review.SystemOneDecider` interface; SP-019 provides `criticeval.Dataset` with examples grouped by hunk.

**API surface (adjust names to local style):**

```go
type ThresholdResult struct {
    Threshold       float64
    DropsCaughtPct  float64 // share of "drop" examples with score < threshold
    KeepsLostPct    float64 // share of "keep" examples with score < threshold
    Kept            int
}
type Report struct {
    Counts         map[criticeval.Label]int
    Thresholds     []ThresholdResult
    LatencyP50, LatencyP95 time.Duration
    Recommended    *float64 // nil when no threshold qualifies
    Warnings       []string
}

func Score(ctx context.Context, client review.SystemOneDecider, model string, samples int, ds *Dataset) (map[string]float64, []time.Duration, error)
func Evaluate(scores map[string]float64, latencies []time.Duration, ds *Dataset) Report
```

- `Score` sends **one `VerifyFindingsSystemOne` call per hunk group** (with `minKeep = 0` so all scores come back) and records each call's wall time. No per-finding request loop.
- `Evaluate` sweeps thresholds 0.1, 0.2, …, 0.9 (named constants).
- Recommended threshold: the highest threshold whose `KeepsLostPct` ≤ 10% (named constant `maxKeepsLostPct`); nil if none.
- Warning when either label has fewer than 10 examples (named constant): "recommendation unreliable".
- Percentiles use nearest-rank on sorted latencies; empty latencies → zero.

## Dependencies

- **Task:** SP-016 (`VerifyFindingsSystemOne`)
- **Task:** SP-019 (`Dataset` types; shares `cli/internal/criticeval`)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `cli/internal/criticeval/dataset.go` (SP-019)
- `cli/internal/review/critic_systemone.go` (SP-016)
- `AGENTS.md` — coverage gates

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none (tests use a fake `review.SystemOneDecider`)

## File Scope

- `cli/internal/criticeval/sweep.go` (new)
- `cli/internal/criticeval/sweep_test.go` (new)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./internal/criticeval/... -count=1 -cover` |
| fileScopeMustChange | `cli/internal/criticeval/sweep.go`, `cli/internal/criticeval/sweep_test.go` |
| fileScopeMustNotChange | `cli/internal/criticeval/dataset.go`, `cli/internal/review/**`, `cli/cmd/**`, `extension/**` |
| completionCriteria | One scoring call per hunk group; exact sweep metrics, percentiles, recommendation, and warnings; tests pass with ≥72% coverage on `sweep.go` |

## Steps

### Step 1: Scoring and evaluation

- [ ] `Score`: one batched call per hunk group, latencies recorded, errors wrapped with hunk index
- [ ] `Evaluate`: sweep, percentiles, recommendation, warnings

### Step 2: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Fake decider with fixed scores → exact expected `ThresholdResult` values and recommendation
- [ ] Cases: no qualifying threshold (nil), one label only (warning), empty dataset, decider error surfaces, call count equals number of hunk groups
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Per-file coverage ≥ 72% for `sweep.go`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Implementation:** `feat(SP-024): description`
- **Tests:** `test(SP-024): description`

## Do NOT

- Modify the dataset loader (SP-019)
- Add CLI output or a command (SP-025)
- Change critic defaults based on results
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
