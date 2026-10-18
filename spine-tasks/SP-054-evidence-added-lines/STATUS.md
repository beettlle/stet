# SP-054: Evidence filter keeps only findings near added lines — Status

**Current Step:** Step 3
**Status:** 🟡 In Progress
**Last Updated:** 2026-10-17
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
**Status:** 🟡 In Progress

- [ ] Helper tests
- [ ] Filter tests
- [ ] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|
| 1 | plan | 1 | skipped (engine-owned) | .reviews/1-20261018T182508.md |
| 2 | plan | 2 | skipped (engine-owned) | .reviews/ |

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
