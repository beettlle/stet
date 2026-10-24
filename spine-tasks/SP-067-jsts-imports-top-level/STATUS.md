# SP-067: JS/TS import block reaches top-level and test-callback hunks — Status

**Current Step:** 3
**Status:** 🔄 In Progress
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
**Status:** ⬜ Not Started

- [ ] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
