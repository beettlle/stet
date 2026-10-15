# SP-046: Include JS/TS imports in expanded hunk context — Status

**Current Step:** 3
**Status:** 🟡 In Progress
**Last Updated:** 2026-10-15
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Import block extraction
**Status:** ✅ Complete

- [x] Top-level import scanner with stop rule and 60-line cap

---

### Step 2: Wire into `expandJSTSHunk`
**Status:** ✅ Complete

- [x] `## File imports` section; token split; no-scope and overlap cases

---

### Step 3: Testing & Verification
**Status:** ⬜ Not Started

- [ ] #10 fixture test; import-form tests; no-imports unchanged; caps
- [ ] Contract `testCommand` passing (stet review exits 0)

---

## Plan

**Step 1 — Import block extraction (`expand_jsts.go`)**
- `jstsImports{block, endLine, found}` + `extractJSTSImports(src []byte)` scanning top-of-file lines.
- Skip set: blank, `//` / `/*` / `*` comments, `#!` shebang, `"use strict"` directive; stop at first other line.
- Import starts: `import `/`import{`, re-exports (`export * from`, `export * as ns from`, `export { a } from`), `const|let|var x = require(...)`, TS `import x = require(...)` (covered by `import ` prefix).
- Multi-line statements consumed whole via significant-brace depth (`skipJSTSToken` skips comments/strings); single-line `export { a };` without `from` is not an import and stops the scan.
- Cap block at `maxJSTSImportLines = 60`; `endLine` = 1-based source line of last import (uncapped) for overlap checks.

**Step 2 — Wire into `expandJSTSHunk` (`expand_jsts.go`)**
- `showImports := imports.found && hunkStart > imports.endLine` (overlap → no import section).
- With imports + scope: `## File imports` fence, then `## Enclosing function context` fence, then `## Diff hunk`.
- With imports + no scope: `## File imports` fence, then `## Diff hunk` (new; was unchanged).
- `maxTokens > 0` + imports shown: `importBudget = maxTokens/4` (imports truncated via `truncateToTokens`), function context gets `maxTokens - importBudget`; budget too small (<1) → drop imports, keep today's behavior. No imports → byte-identical output.

**Step 3 — Tests & verification (`expand_jsts_test.go`)**
- #10 fixture (import line 2 + helper using `fs.mkdirSync`); multi-line named import, re-export, `require`, TS `import =`; shebang + `"use strict"`; no-imports byte-identical to pre-change format; overlap → no section; 61+ lines → capped at 60; tight `maxTokens` → imports ≤ quarter.
- Contract: `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/expand/... ./cli/internal/review/... -count=1 && bash scripts/spine-stet-contract-run.sh default`

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-10-14 | Task staged | PROMPT.md and STATUS.md created (release v1.1.0) |
| 2026-10-15 | Lane setup | Ran `stet start HEAD --allow-dirty` + wrote `.review/spine-stet-baseline.ref` (worktreeSetupHook had not run; services LM Studio + OpenJev verified up) |
| 2026-10-15 | Step 1 + 2 | Scanner + wiring implemented; gofmt, `go vet ./cli/...`, existing expand tests green |

---

## Blockers

*None*
