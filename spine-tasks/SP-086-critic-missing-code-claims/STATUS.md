# SP-086: critic-missing-code-claims — Status

**Current Step:** Step 1
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
**Status:** ⬜ Not Started

- [ ] Evaluation shows improvement without regressing in-view claims

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
| All 12 SP-080 out-of-view rows the model scored 0.79–0.999 (uncatchable at any usable threshold) are deterministically rejected; dataset simulation: 22/78 drops caught, 11/11 keeps preserved | Verified | Scratch simulation + `critic_systemone_test.go` dataset test (Step 1) |

## Execution Log

| Date | Event | Notes |
|------|-------|-------|
| 2026-11-20 | Step 0 implemented | Guard + predicate + 3 unit/integration tests in `critic_systemone_test.go`; full review package tests pass |
