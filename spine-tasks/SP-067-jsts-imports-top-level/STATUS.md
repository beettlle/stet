# SP-067: JS/TS import block reaches top-level and test-callback hunks — Status

**Current Step:** 2
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
**Status:** ⬜ Not Started
> ⚠️ Hydrate: Expand based on which Step 1 cases fail

- [ ] Import block reaches the context in every case

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
