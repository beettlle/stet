# Task: SP-068 — JS RAG resolves imported names in the module they come from

**Created:** 2026-10-24
**Size:** M

## Review Level: 1

**Assessment:** One resolver file in `cli/internal/rag/js/` plus tests; adds a module-path lookup before the existing `git grep` fallback.
**Score:** 3/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 1

## Mission

Closes #19

SP-055 and SP-059 delivered the first two acceptance items of #19: function references passed as arguments are looked up, and built-in method names are skipped as member calls. One item remains: a hunk that edits `import { compareTaskIds } from "./discover.mjs";` must get the exported `compareTaskIds` signature **from `discover.mjs`**.

Today `lookupDefinitions` (`cli/internal/rag/js/resolver.go` ~line 370) takes the first `git grep` hit for every symbol (`gitGrepSymbol`), so an import-bound name can resolve to a same-named declaration in an unrelated file.

Change:

1. Extend import parsing so that, for each import/require-bound local name, the resolver also knows the module specifier and the imported (source) name. Cover the forms `importBoundNames` already handles: `import { a, b as c } from "m"`, `import d from "m"`, `import * as ns from "m"`, `const { a, b: c } = require("m")`, `const x = require("m")`. The import may sit in the hunk itself (the hunk edits the import line) or in the file's import block; read the file at `filePath` under `repoRoot` for the latter. Namespace imports (`ns`) need no module lookup; skip them.
2. For a **relative** specifier (`./` or `../`), resolve the module file relative to the importing file: the exact path, then the path plus each extension from `jsts.Extensions()`, then `index` plus each extension inside a directory. Reject any resolved path outside `repoRoot` (no `..` escapes, no symlink escapes: compare cleaned absolute paths).
3. In the resolved module, find the exported declaration of the source name (`export function name`, `export const name`, `export class name`, `export { name }`, `export default` for default imports, `module.exports.name =` / `exports.name =` for CommonJS). Return it as the `rag.Definition` (repo-relative `File`, 1-based `Line`, signature via `readSignatureAndDoc`).
4. Fall back to the existing `git grep` lookup when the specifier is a bare package name (`"node:fs"`, `"lodash"`), the module file is missing, or no export matches. Never fail the review: any error falls back silently.

Keep candidate order and the `maxDefs` cap unchanged.

## Dependencies

- **None**

## Context to Read First

**Tier 3 (reference docs):**

- `cli/internal/rag/js/resolver.go` — `extractSymbols`, `importBoundNames`, `bindingListNames`, `lookupDefinitions`, `gitGrepSymbol`, `readSignatureAndDoc`
- `cli/internal/rag/js/resolver_test.go` — existing import-bound and argument-reference tests (SP-055, SP-059)
- `cli/internal/jsts/ext.go` — `Extensions()`, `IsExt`

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/rag/js/resolver.go`
- `cli/internal/rag/js/resolver_test.go`

## Documentation Requirements

**Must Update:** none (SP-076 updates `docs/review-process-internals.md`).

**Check If Affected:** `docs/review-process-internals.md` — leave unchanged.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/rag/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/rag/js/resolver.go`, `cli/internal/rag/js/resolver_test.go` |
| fileScopeMustNotChange | `cli/internal/jsts/**`, `cli/cmd/**`, `extension/**`, `.review/**` |
| completionCriteria | In a temp git repo where `discover.mjs` exports `compareTaskIds` and another file declares an unrelated `compareTaskIds`, a hunk editing `import { compareTaskIds } from "./discover.mjs";` resolves to `discover.mjs` (test); `import { x as y }`, default import, CommonJS `require`, `index.js` directory import, and extensionless specifier each have a test; a bare package specifier falls back to `git grep` (test); a specifier that escapes the repo root is not read (test); stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Import bindings with module and source name

- [ ] Parse local name, source name, and module specifier for every supported form (hunk and file import block)
- [ ] `spine_review_step` after the step

### Step 2: Module-path resolution and export lookup

- [ ] Relative specifiers resolve to a file inside the repo root; exports matched by source name; fallback to `git grep` otherwise
- [ ] `spine_review_step` after the step

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Tests for every case in Contract `completionCriteria`
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Feature:** `feat(SP-068): description`
- **Tests:** `test(SP-068): description`

## Do NOT

- Change the candidate list, built-in skip sets, or the `git grep` batching (#15 is separate)
- Resolve bare package specifiers through `node_modules`
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
