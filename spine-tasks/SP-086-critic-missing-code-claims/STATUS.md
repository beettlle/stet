# SP-086: critic-missing-code-claims — Status

**Current Step:** Step 2
**Status:** 🔄 In Progress
**Last Updated:** 2026-11-20
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

### Step 0: Tune System One criteria for out-of-view claims
**Status:** ✅ Complete

- [x] Update critic_systemone criteria to reject out-of-view missing/undefined claims

Decision record: SP-080 (docs/research/critic-eval/criteria-sp080.md) proved criteria *wording* cannot fix this — OpenJev treats the hunk excerpt as the whole world and scores out-of-view existence claims 0.79–0.999, and all 3 rewordings that moved those rows also dropped in-view keeps. The shipped criteria strings therefore stay pinned, and the rejection is deterministic instead: `dropOutOfViewExistenceClaims` in `cli/internal/review/critic_systemone.go` rejects findings whose message asserts a backticked identifier is undefined / not defined / not imported / not declared / missing an import / ReferenceError / unused / not exported before the decider is called (issue #20 proposal 4 — the "narrow filter" backstop). Mirrors `demoteUnsupportedCalleeReturnErrors` (issue #39) in placement and `findings.FilterExistenceClaims` (issues #33/#39) in claim detection, with two deliberate differences: value-uses of "undefined" (incl. backticked "returns `null` or `undefined`") are stripped, and module mentions do not exempt the claim (FilterExistenceClaims can never *disprove* a claim about an unread file; the critic *rejects as unverifiable* what its hunk cannot decide).

---

### Step 1: Evaluate against dataset
**Status:** ✅ Complete

- [x] Evaluation shows improvement without regressing in-view claims

`criticeval` machinery needed no changes: `Evaluate` already counts a missing score as rejected at every threshold, so pre-verification drops propagate through `Score`/`Evaluate` untouched. The evaluation ships as `TestVerifyFindingsSystemOne_datasetRejectsOutOfViewClaims` (review package — `criticeval` imports `review`, so the reverse import would cycle): it replays the frozen 89-example dataset (78 drop / 11 keep) grouped by hunk exactly like `criticeval.Score`, with a worst-case model scoring every finding 0.99 (the real SP-081 baseline caught only 29/78 drops at the shipped `min_keep` 0.2) and asserts:

- **No regression:** all 11 keep examples survive with score 0.99 (incl. the in-view `missing a separator` claim SP-080 nearly lost).
- **All 12 SP-080 out-of-view rows the model scored 0.79–0.999** — uncatchable at any usable threshold — are deterministically rejected.
- **22/78 drop examples rejected overall** under the adversarial model (baseline: 0), all labeled `drop`.

With the real SP-081 model scores, shipped `min_keep` 0.2 caught 29/78; the guard adds the 12 model-uncatchable rows plus other identifier-existence false positives → ≥ 41/78 caught, keeps still 0 lost.

---

### Step 2: Testing & Verification
**Status:** ⬜ Not Started

- [ ] Contract `testCommand` passes

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| SP-080 pre-registered 3 wording attempts, all failed; its record prescribes revisiting with non-wording changes | Guard implemented deterministically instead of criteria rewording | `cli/internal/review/critic_systemone.go` |
| All 12 SP-080 out-of-view rows the model scored 0.79–0.999 (uncatchable at any usable threshold) are deterministically rejected; dataset simulation: 22/78 drops caught, 11/11 keeps preserved | Verified | Scratch simulation + `TestVerifyFindingsSystemOne_datasetRejectsOutOfViewClaims` |
| `criticeval.Score`/`Evaluate` need no change: missing score already evaluates as rejected at every threshold | No action | `cli/internal/criticeval/sweep.go` |
| Guard rejects 6 additional drop rows beyond the SP-080 out-of-view list (SP-824-348bd12/f00ea66, SP-827-bdb1949/77a3d93, SP-831-4594035, SP-845-56c4b0f — all ReferenceError/export/unused claims on stale hunks) | Accepted (precision gain) | Dataset simulation log, Step 0 |

## Execution Log

| Date | Event | Notes |
|------|-------|-------|
| 2026-11-20 | Step 0 implemented | Guard + predicate + 3 unit/integration tests in `critic_systemone_test.go`; full review package tests pass |
| 2026-11-20 | Step 1 implemented | Dataset evaluation test added; contract `testCommand` passes (gofmt + vet + review/criticeval tests) |
