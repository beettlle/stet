# Task: SP-046 — Include JS/TS imports in expanded hunk context

**Created:** 2026-10-14
**Size:** S

## Review Level: 1

**Assessment:** Adds a section to JS/TS hunk expansion; fail-open path unchanged.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 0

## Mission

Partial #10

The reviewer reports "`fs` is not imported" and "missing `await`" for code it cannot see. `expandJSTSHunk` (`cli/internal/expand/expand_jsts.go` ~line 50) adds the enclosing function or method around a JS/TS hunk, but not the file's imports. With a hunk inside a helper, the model sees `fs.mkdirSync(...)` with no import in view and infers that it is missing. Observed: `tests/batch/worker-prompt-rules.test.mjs:22` flagged although line 2 is `import fs from "node:fs";`.

**Behaviour:**

- Collect the file's top-level import statements from the loaded source: lines starting (after optional whitespace) with `import ` or `import{`, `export ... from`, `const|let|var <binding> = require(...)`, and TypeScript `import x = require(...)`. Multi-line imports (`import {\n a,\n b\n} from "x";`) are included whole. Stop scanning at the first line that is not blank, not a comment, not a directive (`"use strict"`, shebang), and not an import.
- Cap the block at `maxJSTSImportLines = 60` lines. When `maxTokens > 0`, the import block counts against it: truncate imports to at most a quarter of `maxTokens` with `truncateToTokens`, and give the function context what remains.
- Output format when imports exist:

  ````text
  ## File imports

  ```<lang>
  <import block>
  ```

  ## Enclosing function context
  ...
  ````

  When no enclosing scope is found but imports exist and the hunk starts after the import block, return `## File imports` followed by `## Diff hunk` (today such hunks come back unchanged). When the hunk overlaps the import block, add no import section.
- No imports → output identical to today. Python expansion (`expand_py.go`) is out of scope.

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Dependencies

- **None**

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md` — "Stet in spine (dogfooding)" section

**Tier 3 (reference docs):**

- `cli/internal/expand/expand_jsts.go` — `expandJSTSHunk`, `extractLineRange`, `skipJSTSToken`
- `cli/internal/expand/expand.go` — dispatch (~line 78), `truncateToTokens`, `loadRepoSourceFile`
- `cli/internal/expand/expand_jsts_test.go` — fixture style

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/expand/expand_jsts.go`
- `cli/internal/expand/expand_jsts_test.go`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/expand/... ./cli/internal/review/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/expand/expand_jsts.go`, `cli/internal/expand/expand_jsts_test.go` |
| fileScopeMustNotChange | `cli/internal/prompt/**`, `cli/internal/run/**`, `cli/cmd/**`, `extension/**`, `scripts/**`, `.review/**`, `go.mod`, `go.sum` |
| completionCriteria | A JS/TS hunk inside a function whose module is imported above the hunk gets a `## File imports` section containing that import; files without imports expand exactly as before; caps respected; stet contract review exits 0 |

## Steps

### Step 1: Import block extraction

- [ ] Top-level import scanner (ESM, re-exports, `require`, TS `import =`, multi-line), stop rule, 60-line cap

### Step 2: Wire into `expandJSTSHunk`

- [ ] `## File imports` section before function context; token split; no-scope case; overlap case

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Fixture from #10: `import fs from "node:fs";` on line 2, hunk inside a helper using `fs.mkdirSync` → context contains the import line under `## File imports`
- [ ] Multi-line named import, `require`, TS `import x = require()`, shebang + `"use strict"` before imports
- [ ] No imports → output byte-identical to the current behavior (assert against the pre-change format)
- [ ] Hunk overlapping the import block → no import section; 61+ import lines → capped; tight `maxTokens` → imports truncated to ≤ a quarter
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in Mission)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Implementation:** `fix(SP-046): description`
- **Tests:** `test(SP-046): description`

## Do NOT

- Change Python or Go expansion
- Change the system prompt (SP-047 owns that)
- Add RAG or callee-definition lookup
- Edit `.review/config.toml` or `scripts/**`
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
