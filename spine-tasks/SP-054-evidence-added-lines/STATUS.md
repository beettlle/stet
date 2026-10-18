# SP-054: Evidence filter keeps only findings near added lines — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-18
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Added-line extraction and filter
**Status:** ✅ Complete

- [x] Added-line helper (`findings.AddedLines`)
- [x] Filter with window 3 and deletion fallback (`findings.FilterByAddedLines`)

---

### Step 2: Wire into run
**Status:** ✅ Complete

- [x] Live call sites; dry-run decision noted; trace label
  - Both live `runReviewPipeline` sites call `FilterByAddedLines(..., AddedLines(p.Hunk.RawContent), hunkStart, hunkEnd)`; trace label is `Evidence (added lines ±3)`.
  - Dry-run (`Start` and `Rerun` DryRun loops) keeps `FilterByHunkLines`: `cannedFindingsForHunks` places each finding on the hunk's first new-file line (the header start), which is normally a context line and can be more than 3 lines from the first `+` line (e.g. larger unified context or a leading deletion block). The added-line rule would make dry-run drop canned findings nondeterministically, breaking the one-finding-per-hunk CI contract. Code comment added at both sites.

---

### Step 3: Testing & Verification
**Status:** ✅ Complete

- [x] Helper tests (`TestAddedLines`: mixed body, `\ No newline` marker, `@@ -1 +1 @@`, pure deletion, unparsable header, empty line)
- [x] Filter tests (`TestFilterByAddedLines`: 4 away dropped both sides, exactly 3 away kept both sides, range overlapping window kept, file mismatch/file-only kept, invalid range dropped, pure-deletion fallback; plus `TestRunReviewPipeline_evidenceAddedLines` in run). Existing `TestFilterByHunkLines` unchanged and passing (function kept for fallback + dry-run).
- [x] Contract `testCommand` passing: gofmt, `go vet ./cli/...`, findings/expand/run tests ok; stet contract review exit 0 after triage (4 findings dismissed, see Discoveries). `evidence.go` coverage 98.1%.

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|
| 1 | plan | 1 | skipped (engine-owned) | .reviews/1-20261018T182508.md |
| 2 | plan | 2 | skipped (engine-owned) | .reviews/2-20261018T182621.md |

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| Dry-run keeps `FilterByHunkLines`: canned findings sit on hunk start line (usually context, may be >3 lines from first `+`) | Kept old filter, commented in code | `cli/internal/run/run.go` DryRun loops |
| stet run was very slow (~75 min) with 3 concurrent lanes sharing LM Studio; first contract attempt hit the 1h tool timeout | Re-ran in background | contract step |
| stet reviewer FP: claimed `reflect`/`testing` imports unsorted (they are sorted) — 4860066 | dismissed already_correct | `evidence_test.go:4` |
| stet reviewer FP: claimed `@@ -1,2 +1,2 @@` header inconsistent with body (it is consistent) — 9bc1caf | dismissed false_positive | `evidence_test.go` |
| stet reviewer FP: `AddedLines` per hunk called "redundant" though each hunk is parsed once — c80ca4c | dismissed false_positive | `run.go:505` |
| stet reviewer FP: rambling ERROR about `Context: raw` including header; real pipeline sets Context == RawContent (`diff.go:45`) — 3173caf | dismissed false_positive | `run_test.go` |
