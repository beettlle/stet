# SP-080: System One critic rejects out-of-view "missing / undefined" claims — Status

**Current Step:** Complete
**Status:** ✅ Complete (Mission: `Partial #20` per PROMPT.md Amendment 1)
**Last Updated:** 2026-11-02
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

### Step 1: Baseline with the current criteria
**Status:** ✅ Complete

- [x] `stet-old` critic-eval results recorded; out-of-view rows identified
  (`docs/research/critic-eval/criteria-sp080.md`: old = 36/78 drops, 2/11 keeps lost,
  7/19 out-of-view rows dropped; 19 OOV ids fixed by the pre-registered classifier)

---

### Step 2: New criteria wording
**Status:** ✅ Complete

- [x] New true/false criteria (wording v1 in `critic_systemone.go`; absence claims
  about code the shown context does not include count as unsupported)
- [x] Unit test (criteria pinned on every chunked request question; short + hunk-free
  guard; rules-annotation request unchanged)

---

### Step 3: Validate and document
**Status:** ✅ Complete

- [x] `stet-new` critic-eval; validation rule applied — wording v1 FAIL (loses keeps 7ac810c,
  bf03c29), v2 FAIL (loses 821-n7, 7ac810c, bf03c29; OOV 7→13), v3 FAIL (OOV 7→7, loses 7ac810c).
  Per the pre-registered rule: criteria stay; Mission → `Partial #20` (PROMPT.md Amendment 1)
- [x] Contract and internals docs updated (`--critic-backend` paragraph; §7.10 systemone bullet:
  shipped criteria quoted + SP-080 outcome)

---

### Step 4: Testing & Verification
**Status:** ✅ Complete

- [x] Completion-criteria tests (criteria pinned on every chunked question; short + hunk-free
  guard; annotation request unchanged — all in `critic_systemone_test.go`)
- [x] Contract `testCommand` passes: check-gofmt ✓, go vet ✓, review tests ✓, stet contract
  review exit 0 (1 finding dismissed `false_positive`, see Discoveries)

---

## Completion Criteria

- [x] All steps complete
- [x] All tests passing (contract testCommand exit 0; full `go test ./cli/...` green)
- [x] Decision follows the pre-registered validation rule (all three wordings fail → criteria stay; Mission `Partial #20`)

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

*Reviews run in the batch engine after .DONE.*

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| stet reviewer false positive `921b358` (criteria-pinning test "IDs 'id-0..id-N' don't match fake prob keys 'f0','f1' — test will fail"): question keys are batch-index based (`systemOneQuestionKey(i)`), same scheme as the existing `allKept`/`allDropped` tests; the test passes. Reviewer misread positional keying. | dismissed `false_positive`; no stet code change; if it recurs, consider a trace/doc note that question keys are positional | contract review round 1 |
| Knife-edge keep: `pi-spine-SP-822-0f88739` scores exactly 0.500 under shipped criteria (kept only because keep is `>= min_keep`). Any future criteria/threshold change re-tests it first. | recorded for SP-081 | `criteria-sp080.md` |
| v2 wording measured best-ever drop precision on this dataset (49/78, OOV 13/19) but fails no-real-loss; candidate for a future task pairing criteria change with reviewer-prompt work (out of SP-080 scope). | recorded for SP-081 / tech debt | `criteria-sp080.md`, spine-tasks/CONTEXT.md not modified (packet forbids) |
| Harness noise: OpenJev sampling is non-deterministic run-to-run; the rule was applied to single old/new pairs as pre-registered. | recorded | `criteria-sp080.md` |

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-11-02 | Step 1 | Built `stet-old` from unchanged main (`a90293d`); `critic-eval --json` + per-example harness run vs OpenJev (82 groups, 89 examples); baseline recorded in `criteria-sp080.md` with pre-registered OOV classifier |
| 2026-11-02 | Step 2 | Reworded critic criteria (wording v1); added criteria-pinning, hunk-free guard, and annotation-unchanged tests; `go test ./cli/...` green |
| 2026-11-02 | Step 3 | Validated wordings v1–v3 against the dataset (all FAIL the pre-registered rule); reverted criteria to shipped strings; recorded decision in `criteria-sp080.md`, PROMPT.md Amendment 1, and both docs |
| 2026-11-02 | Step 4 | Contract testCommand green after dismissing reviewer false positive `921b358` (`false_positive`); re-run exit 0 |

---

## Blockers

*None*
