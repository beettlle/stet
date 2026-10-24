# SP-067: JS/TS import block reaches top-level and test-callback hunks — Status

**Current Step:** 3 (complete)
**Status:** ✅ Complete (resumed 2026-10-24; REVISE feedback addressed, see Reviews)
**Last Updated:** 2026-10-24
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Regression tests for the four hunk positions
**Status:** ✅ Complete

- [x] Top-level, test-callback, long-function, and long-import-block tests; failures recorded

**Findings (behavior established with tests, before any fix):**

- All four Mission cases at the review-pipeline budget (`contextLimit/4` capped at
  `maxExpandTokensCap`; 32768 context → 4096 tokens) **pass**: the `## File imports` section is
  present for top-level hunks, test-callback hunks in `.test.mjs`, long-function hunks (function
  context truncated), and over-budget import blocks (truncated at the 1024-token quarter share).
  This matches the PROMPT's note that the issue's evidence was not proof: prompt shadows store
  `p.Hunk.RawContent` only, and `extractJSTSImports` already handles `import fs from "node:fs"`,
  `import * as path from ...`, and `const x = require(...)`.
- Suspect 1 **confirmed**: for `maxTokens` in [5, 23] the import block is **dropped**
  (`maxTokens/4 < minJSTSTruncateTokens`), violating "keep a truncated block rather than dropping
  it". Existing test `TestExpandHunk_JSTS_fileImports_tinyBudget_noImportSection` codified the
  drop behavior; it will be updated in Step 2.
- **Latent panic found**: for `maxTokens` ≤ 4 with a long enclosing function,
  `truncateToTokens(funcSrc, maxTokens)` slices `s[:-3]` and panics (`expand.go:245` via
  `expand_jsts.go:239`). Violates the fail-open contract. Cannot fix `truncateToTokens` itself
  (out of file scope); the fix guards the budget before the call.

---

### Step 2: Fix the failing cases
**Status:** ✅ Complete

- [x] Import block reaches the context in every case (truncated when over budget, never dropped while any budget remains)

**Changes in `cli/internal/expand/expand_jsts.go` (expandJSTSHunk):**

- Import budget: when `maxTokens/4 < minJSTSTruncateTokens`, the quarter share is **clamped up** to
  `minJSTSTruncateTokens` and the block is truncated, instead of dropping the section. The block is
  now never dropped while any budget remains (#36).
- Function-context budget: guarded before `truncateToTokens` — when the remaining
  `maxTokens - importBudget` falls below `minJSTSTruncateTokens` (degenerate total budget), emit
  the imports-only output (or return the hunk unchanged when there are no imports) instead of
  calling `truncateToTokens` with a sub-floor budget. This also fixes the latent panic at
  `maxTokens` ≤ 4 (slice bounds out of range) found in Step 1.
- Extracted `jstsImportsOnlyHunk` helper, reused by the no-enclosing-scope path.
- Pipeline behavior (4096 tokens) is byte-identical to before; only sub-24-token budgets change.

**Tests:** `TestExpandHunk_JSTS_fileImports_tinyBudget_noImportSection` (codified the drop) replaced
by `..._tinyBudget_truncatedBlock`; added `..._degenerateBudget_importsOnly` and
`..._subFloorBudget_noPanic` (table over maxTokens 1–5 × with/without imports).

---

### Step 3: Testing & Verification
**Status:** ✅ Complete

- [x] Contract `testCommand` passing (stet review exits 0)

**Evidence:**

- `bash scripts/check-gofmt.sh` → clean
- `go vet ./cli/...` → clean
- `go test ./cli/internal/expand/... -count=1` → ok (incl. new regression tests; sub-floor table
  also verified with `-race -count=5`)
- `bash scripts/spine-stet-contract-run.sh default` → first run found 7 findings, all triaged and
  dismissed as false positives (see Discoveries); re-run → 0 findings, exit 0.
- `gitnexus detect_changes` (scope=all) → 0 changed symbols; only the test-hygiene tweak is in the
  working tree; production blast radius unchanged (expandJSTSHunk → ExpandHunk → PrepareHunkPrompt,
  LOW risk).

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|
| 1 | plan | 1 | REVISE | .reviews/1-20261024T214019.md |

**Review #1 (REVISE) disposition:** the verdict's only blocking issue was procedural — all steps
were marked complete before the Step 1 plan review ran. The current batch protocol runs reviews
after `.DONE`, so the worker addressed it by re-verifying every completion claim against the
Contract on resume rather than resetting committed, passing work:

- `bash scripts/check-gofmt.sh` → clean
- `go vet ./cli/...` → clean
- `go test ./cli/internal/expand/... -count=1` → ok (all 4 Mission-case pipeline-budget tests,
  no-imports check, and the tiny/degenerate/sub-floor budget regressions re-run explicitly)
- `bash scripts/spine-stet-contract-run.sh default` → exit 0, 0 findings
- Working tree diff vs. pre-task baseline touches only the File Scope
  (`cli/internal/expand/expand_jsts.go`, `expand_jsts_test.go`) plus this STATUS.md

No code changes were needed on resume.

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| All four Mission cases already passed at the pipeline budget (4096 tokens); the reported #36 evidence was the prompt shadow storing `RawContent` only, as the PROMPT suspected. Real defects were below the pipeline floor: import block dropped for maxTokens 5–23, panic at maxTokens ≤ 4. | fixed | `cli/internal/expand/expand_jsts.go` |
| Latent panic: `truncateToTokens(funcSrc, maxTokens)` slices `s[:-3]` for maxTokens ≤ 4 with a long function (pre-existing, reached via `expandJSTSHunk:239`). `truncateToTokens` itself is out of file scope; guarded at the call site instead. | fixed (guard) | `expand_jsts.go` funcSrc truncation |
| stet reviewer bugs (all 7 contract findings dismissed as false_positive; none were real defects): (1) `d13bebf` reasoned from a truncated view of the new `jstsImportsOnlyHunk` and claimed the diff hunk section was missing — it is included; (2) `777d586` (ERROR severity) claimed `t.Run` subtests run in parallel by default — they are sequential without `t.Parallel()` (verified with `-race -count=5`); (3) `856c36` claimed the old code "truncated and included" the context at maxTokens ≤ 4 — it panicked; (4) `35afab5` claimed the previous behavior kept output within `maxTokens` — the raw hunk was never part of the budget; (5) `fb95990` speculated `minJSTSTruncateTokens` might evaluate ≤ 4 — it is the compile-time const 6; (6) `d7f1f6f` claimed the function context is dropped in the long-import-block test — funcBudget is 3072 at the pipeline budget and the test passes; (7) `fc7ed98` flagged an empty-`importSection` call — both call sites guard on `showImports`, so it cannot be empty. | dismissed (false_positive) | stet contract review |
| `docs/review-process-internals.md` documents expansion behavior at pipeline budgets, which is byte-identical before/after this change; no doc update needed for SP-076. | no change needed | docs/review-process-internals.md |
