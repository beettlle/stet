# Task: SP-055 — JS RAG: function references, import-bound names, no built-ins

**Created:** 2026-10-17
**Size:** S

## Review Level: 1

**Assessment:** Symbol extraction changes in one resolver; lookup unchanged.
**Score:** 3/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 1

## Mission

Partial #19

`extractSymbols` in `cli/internal/rag/js/resolver.go` (~line 79) collects names from three regexes: `function name(`, capitalized identifiers, and `name(` calls. It misses:

- functions passed by reference, e.g. `compareTaskIds` in `arr.sort(compareTaskIds)`;
- names bound by `import { a, b as c } from "..."`, `import d from "..."`, `import * as ns from "..."`, and `const { a } = require("...")` / `const x = require("...")` in the hunk;

and it spends candidate slots (`maxSymbolCandidates = 30`) on built-in method names such as `filter`, `map`, `sort`, `push`, `includes`, `has`, `forEach`, `then`.

Change `extractSymbols` so that:

1. Names bound by `import` / `require` statements in the hunk are extracted (local binding names; for `b as c` use `c`).
2. Identifiers passed as call arguments are extracted (bare identifiers only, e.g. `(compareTaskIds)` or `, handler)`; skip literals, member expressions, and keywords).
3. A named package-level set of common built-in / prototype method names is skipped before candidates are added.
4. Candidate order: import-bound names first, then the current regex order, then argument identifiers; dedupe; still capped at `maxSymbolCandidates`.

`lookupDefinitions` stays unchanged. Resolving an import-bound name in the `from` module (the rest of #19) is out of scope.

## Dependencies

- **Task:** SP-051 (also edits `cli/internal/rag/js/resolver.go`; serialize to avoid merge conflicts)

## Context to Read First

**Tier 3 (reference docs):**

- `cli/internal/rag/js/resolver.go`, `resolver_test.go`

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/rag/js/resolver.go`
- `cli/internal/rag/js/resolver_test.go`

## Documentation Requirements

**Must Update:** none

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/rag/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/rag/js/resolver.go`, `cli/internal/rag/js/resolver_test.go` |
| fileScopeMustNotChange | `cli/cmd/**`, `cli/internal/run/**`, `cli/internal/expand/**`, `cli/internal/jsts/**`, `extension/**`, `scripts/**`, `.review/**` |
| completionCriteria | A hunk with `arr.sort(compareTaskIds)` resolves `compareTaskIds` (test against a temp repo defining it); built-in method names are not candidates (test); `import { compareTaskIds } from "./discover.mjs";` yields `compareTaskIds` as a candidate (test); existing resolver tests pass; stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Extraction changes

- [ ] Import/require-bound names, argument identifiers, built-in skip set, candidate order
- [ ] `spine_review_step` after the step

### Step 2: Testing & Verification

> ZERO test failures allowed.

- [ ] `extractSymbols` unit tests: import forms (named, aliased, default, namespace, require destructuring), argument identifiers, built-ins skipped, cap respected
- [ ] `ResolveSymbols` test: temp repo with `export function compareTaskIds` in `discover.mjs`; hunk `arr.sort(compareTaskIds)` returns its definition
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Fix:** `fix(SP-055): description`
- **Tests:** `test(SP-055): description`

## Do NOT

- Change `lookupDefinitions`, `git grep` usage, or timeouts (#15)
- Change the registered extensions (SP-051 owns that)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
