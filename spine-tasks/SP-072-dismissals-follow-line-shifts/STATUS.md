# SP-072: Dismissals follow line shifts in `stet start` and `stet run` — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-25
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Start-time restore re-maps instead of dropping
**Status:** ✅ Complete (commit 8a6d6fa)

- [x] Re-map per `diffRef`; edited spans dropped; `Start` call site updated

---

### Step 2: Run-time skip re-maps from `LastReviewedAt`
**Status:** ✅ Complete

- [x] Dismissed spans re-mapped before the overlap check; fallback when unavailable

---

### Step 3: Testing & Verification
**Status:** ✅ Complete (commits 1c8cf94, 54cae8e)

- [x] Tests (a)–(d)
- [x] Contract `testCommand` passing (stet review exits 0)

---

## Completion Criteria

- [x] All steps complete
- [x] All tests passing — `check-gofmt` + `go vet ./cli/...` clean; `go test ./cli/internal/run/... -count=1` ok; `spine-stet-contract-run.sh default` exit 0 (0 active findings)

### Contract criteria evidence

- **(a)** `TestStart_restoresDismissal_afterLineShift` (restore_dismissals_test.go): dismissal recorded at c3 with span [2,3]; two lines inserted above (shift +2) plus a second edit so the reviewed hunk's canned finding lands on line 4; remapped span [4,5] matches the reworded finding → restored, ActiveFindingCount 0.
- **(b)** `TestStart_noRestore_dismissedLinesEdited`: the dismissal's own line edited → not restored (DismissedIDs empty). Unit-level: `TestRemapRestorableDismissals_dropsEditedSpansDeletedFilesBadRefs` (edited span, deleted file, bad ref → dropped; survivor below kept).
- **(c)** `TestRun_skipsHunkAtShiftedDismissedLines` (discriminating: 16-line file, 7 unchanged lines keep two hunks separate): after a 2-line insertion above the dismissed line, `stet run` skips the hunk covering the shifted span (16 ∈ 16..20) and reviews only the other hunk — `HunksReviewed == 1` (old behavior would review both). Unit-level: `TestFilterHunksWithDismissedFindings_remapsDismissedSpans` (+ fallbacks: empty LastReviewedAt, equal refs, git failure) and `TestFilterHunksWithDismissedFindings_editedDismissedSpanReviewed`.
- **(d)** Same test: the genuinely new finding from the reviewed hunk stays active (not in DismissedIDs, ActiveFindingCount == 1).
- Existing restore/skip suites (`TestStart_restoresDismissals_sameBaseline`, `TestStart_traceAndVerboseReportRestoredCount`, `TestFilterHunksWithDismissedFindings` table, `TestRun_dismissedHunksNotSentToModel`, `TestRun_forceFullReview_skipsDismissedFilter`, …) all pass unchanged in behavior.

### Contract triage records

Round 1 (8 findings → 1 fixed, 7 dismissed):
- `270507c` **fixed**: equal-refs sub-case now stores a line inside the hunk so the assertion proves stored spans drive the skip.
- `a1a012f` false_positive: `d.line = newStart` is intentional — matching uses `dismissalSpan` (valid range wins); keeping `line` in sync with the remapped span start is tested (`shifted.line == 4`).
- `ac51c8f` already_correct: the suggested rename was already made (`TestStart_noRestore_dismissedLinesEdited`); reviewer read stale diff context.
- `69b5570` false_positive: body uses ctx/repoRoot (`remapDismissedLocations` call at run.go).
- `b452f87` false_positive: `LastReviewedAt` stores a commit SHA (`headSHA`), not a timestamp.
- `11adb8a` false_positive: no `s.DismissedLocations` field exists; `locs` is a local slice and returning it on error is the documented fallback.
- `c6ee314` already_correct: `remapRestorableDismissals` is defined in the same package (restore_dismissals.go); build passes.
- `91da39c` out_of_scope: forceFullReview redundancy predates this task (caller guard existed before); PROMPT prescribes adding only ctx/repoRoot/targetRef, and the guard is exercised directly by the table test.

Round 2 (2 findings → 1 fixed, 1 dismissed):
- `8eb0c88` **fixed**: `t.Fatalf` for the re-review findings-count assertion.
- `fd626ca` false_positive: demands the opposite of round-1 `270507c`; with LastReviewedAt == targetRef the re-map is identity by design, so no output can distinguish short-circuit from identity — the case pins the documented contract (stored spans apply).

Round 3 (1 finding → dismissed):
- `2a039c8` false_positive: claimed the test's diff merges into one hunk; verified with real git — `git diff HEAD~2..HEAD` emits `@@ -1,5 +1,7 @@` and `@@ -14,3 +16,5 @@` (7 unchanged lines > 2×U3 keep them separate); the test's assertions pass.

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Plan

- GitNexus impact (required by AGENTS.md): `filterUnchangedFiles` and `filterHunksWithDismissedFindings` both flag HIGH risk. Blast radius is contained: one direct caller each (`Start` → `stet start`; `Run` → `stet run`/`rerun`/`commit-msg`), and both call sites are in the PROMPT scope. Fallback paths keep today's behavior when re-mapping is unavailable.
- Step 1: replace `filterUnchangedFiles` with `remapRestorableDismissals` (group by diffRef, one `loadLineRemaps` per ref, `remapSpan` per dismissal, drop ok=false/unresolvable). Remove `changedFilesSince` (no remaining callers) and prune imports.
- Step 2: `filterHunksWithDismissedFindings(ctx, repoRoot, toReview, s, targetRef, forceFullReview)`; re-map dismissed spans from `s.LastReviewedAt` when non-empty, ≠ targetRef, and git succeeds; ok=false spans are dropped so edited dismissed code is re-reviewed; update the `Run` caller.
- Step 3 tests: unit tests pin shift/keep/drop + fallbacks; end-to-end (a) `Start` restore after a 2-line insertion above a ranged dismissal (finding lands inside the shifted span); (b) start restore dropped when the dismissed line itself is edited; (c) run skips hunk at shifted lines (discriminating 2-hunk geometry) + edited-dismissed-line hunk is re-reviewed.

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| `docs/cli-extension-contract.md` line 88 (`stet dismiss`) says the next `stet start` restores dismissals "for unchanged files" — no longer accurate after SP-072 (changed files are restored when their line span survives the shift). SP-076 should reword, e.g. "restores matching dismissals from recent history, re-mapping recorded line spans to the current HEAD; dismissals whose own lines changed are not restored". | For SP-076 | docs/cli-extension-contract.md |
| Stet reviewer bugs (not code defects): `2a039c8` miscomputed unified-diff hunk merging (claimed one merged hunk where git emits two; verified with real git — 7 unchanged lines > 2×U3 stay separate); `11adb8a` hallucinated a `s.DismissedLocations` session field; `b452f87` misread `LastReviewedAt` as a timestamp though it stores a commit SHA. | Recorded | STATUS.md triage records |
