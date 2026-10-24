# SP-071: Line re-mapping library for dismissed findings — Status

**Current Step:** Complete
**Status:** ✅ Complete (all steps done, contract review exits 0)
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
**Status:** ✅ Complete

- [x] Tests for every Contract `completionCriteria` case (parser table tests incl. lines above/inside/below a modification, insertion/deletion shifts, insertion directly after the span, overlap `ok=false`, multi-hunk accumulation, absent-file identity, deleted file, plus temp-repo `loadLineRemaps` end to end)
- [x] Contract `testCommand` passing (stet review exits 0; one finding d8062e9 triaged as false_positive — stet diff-splitter bug, see Discoveries)

---

## Completion Criteria

- [x] All steps complete
- [x] All tests passing (`bash scripts/check-gofmt.sh`, `go vet ./cli/...`, `go test ./cli/internal/run/... -run 'Remap' -count=1`, full `go test ./cli/internal/run/... -count=1`; contract `testCommand` exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| Contract finding d8062e9 claimed `line_remap_test.go` is truncated at `remapDiffFixture` with a compilation error — false: `gofmt`/`go vet`/`go build`/`go test` all pass. Root cause is a stet review-pipeline bug: `splitByFileSections` (`cli/internal/diff/parse.go` ~55) splits at `"diff --git "` anywhere in the text, so the fixture's embedded `diff --git a/f.txt b/f.txt` line cuts the reviewer's file view mid-constant. Fixing the splitter is outside this packet's File Scope; logged in `spine-tasks/CONTEXT.md` tech-debt backlog. Finding dismissed `false_positive` with that evidence. | stet reviewer bug — dismissed false_positive, fix logged as tech debt | `cli/internal/diff/parse.go` `splitByFileSections` |
