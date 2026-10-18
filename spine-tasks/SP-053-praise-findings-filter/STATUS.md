# SP-053: Drop praise-only findings — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-17
**Review Level:** 2
**Review Counter:** 2
**Iteration:** 1
**Size:** S

---

### Step 1: Filter and prompt rule
**Status:** ✅ Complete

- [x] `FilterPraiseOnly` (`cli/internal/findings/praise.go`; patterns compiled once; caveat/negation words such as "not", "but", "missing" keep the finding)
- [x] Prompt sentence (`DefaultSystemPrompt` Output step 4)

---

### Step 2: Wire into run
**Status:** ✅ Complete

- [x] Call sites under the FP guard; trace line `Praise filter: N -> M` (both `runReviewPipeline` branches plus **both** dry-run sites in `Start` and `Run`. That makes four sites; PROMPT listed three)

---

### Step 3: Testing & Verification
**Status:** ✅ Complete

- [x] `praise_test.go` cases (drop info/nitpick praise; keep with suggestion, warning, error, no-praise, "incorrectly", negated, caveat; nil/empty → nil; no mutation). `praise.go` coverage 100%
- [x] Prompt test (`TestDefaultSystemPrompt_forbidsPraiseOnlyFindings`)
- [x] Run test (FP on vs nitpicky): `TestStart_praiseFilter_dropsPraiseOnlyInfo`, `TestStart_nitpicky_keepsPraiseOnlyInfo`, `TestRunReviewPipeline_praiseFilter` (checks the trace line)
- [x] Contract `testCommand` passing (stet review exits 0; `0 finding(s).`)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|
| 1 | plan | 1 | skipped (engine-owned) | .reviews/1-20261018T002046.md |
| 2 | plan | 2 | skipped (engine-owned) | .reviews/2-20261018T002059.md |

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| `.review/spine-stet-baseline.ref` was missing at contract time. The lane's prior task (SP-050) finished its session and the ref was gone, so the contract script exited 1. I restored it with this task's pre-worker HEAD `0bdf026`. | Harness gap; I restored the ref manually. The ref is gitignored, so no tracked change. | `scripts/spine-worktree-setup.sh` runs only once per lane |
| Stet reviewer false positives on first contract run (8): claimed `n't\b` is invalid RE2, which is wrong because it compiles and the tests pass; claimed `SeverityNitpick` undefined and the prompt sentence missing (out-of-hunk definitions); claimed the second pipeline branch and the non-dry-run path lack `FilterPraiseOnly` (all four sites have it); argued nitpicky should also filter praise (against the spec); claimed serial subtests under a parallel parent are invalid. | Dismissed: 3c340c6, 7b8b72f, c5f861e, 6f04cf1 false_positive; 0b69c01, 4185471, d717ca3 already_correct; 5961b80 wrong_suggestion. Re-run: 0 findings. | stet reviewer (prompt: out-of-hunk evidence, sibling-site reasoning) |
