# Task: SP-051 — One JS/TS extension list for expand and RAG

**Created:** 2026-10-17
**Size:** S

## Review Level: 1

**Assessment:** Extract a shared constant list into a tiny package; two consumers.
**Score:** 3/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 1

## Mission

Closes #16

Two packages keep separate JS/TS extension lists:

- `isJSTSExt` in `cli/internal/expand/expand_jsts.go` (~line 179) now accepts `.js .jsx .mjs .cjs .ts .tsx` (`.mjs`/`.cjs` added in commit `bfa3b86`).
- `init()` in `cli/internal/rag/js/resolver.go` (~lines 44–49) registers `.js .mjs .cjs .ts .tsx` — **`.jsx` is missing**, so React files get no symbol definitions.

Create package `cli/internal/jsts` (file `cli/internal/jsts/ext.go`) exporting:

- `Extensions` — the ordered list `.js`, `.jsx`, `.mjs`, `.cjs`, `.ts`, `.tsx`
- `IsExt(ext string) bool` — exact, case-sensitive match against that list (same semantics as today's `isJSTSExt`)

Use it in both places: `isJSTSExt` delegates to (or is replaced by) `jsts.IsExt`, and `rag/js` `init()` loops over `jsts.Extensions` calling `rag.MustRegisterResolver`. Remove the per-package extension constants that become unused. Do not add `.mts`/`.cts` in this task.

## Dependencies

- **Task:** SP-050 (also edits `cli/internal/expand/expand_jsts.go`; serialize to avoid merge conflicts)

## Context to Read First

**Tier 3 (reference docs):**

- `cli/internal/expand/expand_jsts.go` — `isJSTSExt` and extension constants
- `cli/internal/rag/js/resolver.go` — `init()` registrations
- `cli/internal/rag/rag.go` — `MustRegisterResolver` (panics on duplicate registration)

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/jsts/ext.go`
- `cli/internal/jsts/ext_test.go`
- `cli/internal/expand/expand_jsts.go`
- `cli/internal/expand/expand_jsts_test.go`
- `cli/internal/rag/js/resolver.go`
- `cli/internal/rag/js/resolver_test.go`

## Documentation Requirements

**Must Update:** none (SP-056 updates `docs/review-process-internals.md`)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/jsts/... ./cli/internal/expand/... ./cli/internal/rag/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/jsts/ext.go`, `cli/internal/jsts/ext_test.go`, `cli/internal/expand/expand_jsts.go`, `cli/internal/rag/js/resolver.go` |
| fileScopeMustNotChange | `cli/cmd/**`, `cli/internal/run/**`, `extension/**`, `scripts/**`, `.review/**` |
| completionCriteria | `.jsx`, `.mjs`, `.cjs` hunks get both enclosing-function context and RAG definitions (a test per extension); only one JS/TS extension list exists (`rg -n '"[.]jsx"' cli --glob '!*_test.go'` matches only `cli/internal/jsts/ext.go`); stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Shared list

- [ ] `cli/internal/jsts/ext.go` with `Extensions` and `IsExt`; unit test
- [ ] `spine_review_step` after the step

### Step 2: Wire both consumers

- [ ] `expand` uses `jsts.IsExt`; unused constants removed
- [ ] `rag/js` registers every `jsts.Extensions` entry (adds `.jsx`)
- [ ] `spine_review_step` after the step

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] `rag/js` test: a `.jsx` file resolves a symbol definition
- [ ] `expand` test: a `.jsx` hunk inside a function gets enclosing context (add if missing; `.mjs`/`.cjs` tests already exist)
- [ ] `rag` test or assertion: `.mjs` and `.cjs` still resolve
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing; `cli/internal/jsts/ext.go` ≥ 72% line coverage

## Git Commit Convention

- **Implementation:** `fix(SP-051): description`
- **Tests:** `test(SP-051): description`

## Do NOT

- Change symbol extraction or lookup logic in `rag/js` (SP-055 owns that)
- Change `jstsDeclLineRange` (SP-050 owns that)
- Add `.mts`/`.cts`
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
