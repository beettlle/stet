# Task: SP-067 — JS/TS import block reaches top-level and test-callback hunks

**Created:** 2026-10-24
**Size:** S

## Review Level: 1

**Assessment:** One function in `cli/internal/expand/expand_jsts.go` plus table tests; fail-open behavior must stay.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 0

## Mission

Closes #36

A reviewer reported "`createRulesFixture` uses `fs` without importing it" for a `.test.mjs` hunk that started at `@@ -4,102`, while `import fs from "node:fs"` sat on line 2. `expandJSTSHunk` (`cli/internal/expand/expand_jsts.go` ~line 188) is supposed to prepend a `## File imports` section whenever the file has top-level imports above the hunk.

Note: the issue's evidence (the import missing from the prompt shadow in `.review/session.json`) is not proof on its own. Prompt shadows store `p.Hunk.RawContent` only (`truncateForPromptContext` in `cli/internal/run/run.go`), never the expanded context. Establish the real behavior with tests first.

Write regression tests that drive `expandJSTSHunk` (or `ExpandHunk`) with the token budget the review pipeline uses (`contextLimit / 4`, capped at `maxExpandTokensCap` in `cli/internal/review/review.go`; use a realistic 32768 context) and assert the returned `Context` contains the `## File imports` section with the import line for:

1. A hunk in top-level code (no enclosing function) starting below the imports.
2. A hunk inside a test callback: `describe("x", () => { test("y", () => { ... }) })` in a `.test.mjs` file.
3. A hunk inside a long enclosing function whose source exceeds the function budget.
4. A file whose import block is longer than the import budget (the block is truncated, not dropped).

Fix every case that fails. Known suspects: the import budget dropping the block when `maxTokens / jstsImportTokenShare < minJSTSTruncateTokens`; the enclosing-scope path in case 2 or 3 losing the section; and `extractJSTSImports` missing a statement shape (`import fs from "node:fs"`, `import * as path from "node:path"`, `const x = require("y")`). When the budget is too small for the whole block, keep a truncated block rather than dropping it. Keep the fail-open contract: any read or parse failure returns the hunk unchanged.

## Dependencies

- **None**

## Context to Read First

**Tier 3 (reference docs):**

- `cli/internal/expand/expand_jsts.go` — `expandJSTSHunk`, `extractJSTSImports`, `findEnclosingJSTSScope`, `jstsImportTokenShare`, `minJSTSTruncateTokens`
- `cli/internal/expand/expand_jsts_test.go` — existing import-block tests (added by SP-046)
- `cli/internal/review/review.go` — `PrepareHunkPrompt` (~line 150) for how `maxExpand` is computed

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/expand/expand_jsts.go`
- `cli/internal/expand/expand_jsts_test.go`

## Documentation Requirements

**Must Update:** none (SP-076 updates `docs/review-process-internals.md`).

**Check If Affected:** `docs/review-process-internals.md` — leave unchanged; record any behavior change under Discoveries in STATUS.md for SP-076.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/expand/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/expand/expand_jsts_test.go` |
| fileScopeMustNotChange | `cli/internal/expand/expand.go`, `cli/cmd/**`, `extension/**`, `.review/**` |
| completionCriteria | Tests for all four Mission cases pass and assert the `## File imports` section contains the import line; a file with no imports still returns the original behavior (test); stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Regression tests for the four hunk positions

- [ ] Tests for top-level, test-callback, long-function, and long-import-block hunks; record in STATUS which fail before any fix
- [ ] `spine_review_step` after the step

### Step 2: Fix the failing cases

- [ ] Import block reaches the context in every case (truncated when over budget, never dropped while any budget remains)
- [ ] `spine_review_step` after the step

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Fix:** `fix(SP-067): description`
- **Tests:** `test(SP-067): description`

## Do NOT

- Change RAG, the review prompt, or `run.go`
- Change the Go, Python, or Rust expanders
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
