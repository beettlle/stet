# SP-071: Line re-mapping library for dismissed findings — Status

**Current Step:** 3
**Status:** 🔄 Step 3 in progress (contract verification)
**Last Updated:** 2026-10-24
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Diff parser and `remapSpan`
**Status:** ✅ Complete

- [x] Pure `-U0` parser and span remapping
  - `cli/internal/run/line_remap.go`: `parseLineRemaps` (pure parser over diff text), `parseHunkHeader`/`parseRange`/`trimDiffSidePath` helpers, `remapSpan` with the Mission rules (identity for absent files, delta shift for hunks ending before start, `ok=false` on overlap/deleted file, insertion/deletion `-U0` conventions)
  - Verified against real git output (probe repo): single-line counts omitted (`@@ -3 +3 @@`), insertion `-a,0`, deletion `+c,0`, `+++ /dev/null` for deletions
  - In-hunk guard: a removed line rendered `--- ...` is treated as hunk content, never a file header

---

### Step 2: `loadLineRemaps` git call
**Status:** ✅ Complete

- [x] One `git diff` per ref pair; empty file list makes no call
  - Single `exec.CommandContext` with `git.MinimalEnv()` and `cmd.Dir = repoRoot`, mirroring `changedFilesSince`; result filtered to requested files; errors wrap stderr
  - Tests: `TestLoadLineRemaps_endToEnd` (temp repo, all span cases), `TestLoadLineRemaps_emptyFilesMakesNoCall` (non-repo dir proves no git call), `TestLoadLineRemaps_invalidRefReturnsError`, `TestLoadLineRemaps_limitsToRequestedFiles`

---

### Step 3: Testing & Verification
**Status:** 🔄 In progress

- [x] Tests for every Contract `completionCriteria` case
- [ ] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
