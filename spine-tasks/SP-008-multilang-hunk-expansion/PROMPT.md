# Task: SP-008 — JS/TS hunk expansion

**Created:** 2026-06-30
**Size:** M

## Review Level: 2

**Assessment:** Extends Phase 6.4 for JS/TS only; parser choice and fail-open behavior need care.
**Score:** 4/8 — Blast radius: 1, Pattern novelty: 2, Security: 0, Reversibility: 1

## Mission

Add enclosing-scope expansion for **JavaScript/TypeScript** hunks in `cli/internal/expand/`. Today only Go files get enclosing-function context. Implement JS/TS with the same fail-open and token-truncation semantics as Go.

**Out of scope:** Python (SP-009), Rust/Java, full tree-sitter for all languages.

## Dependencies

- **None**

## Context to Read First

- `cli/internal/expand/expand.go` — Go `ExpandHunk` reference
- `docs/implementation-plan.md` Phase 6.4
- `docs/context-enrichment-research.md` Tier 2

## Environment

- **Workspace:** `cli/` Go module

## File Scope

- `cli/internal/expand/expand.go` (dispatch hook for JS/TS only)
- `cli/internal/expand/expand_jsts.go` (new)
- `cli/internal/expand/expand_test.go` (JS/TS fixtures only)
- `docs/review-process-internals.md` (JS/TS expansion note)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./internal/expand/... -count=1 -run JSTS` |
| fileScopeMustChange | `cli/internal/expand/expand_jsts.go` |
| fileScopeMustNotChange | `extension/**`, `scripts/**`, `cli/internal/expand/expand_py.go` |
| completionCriteria | JS/TS hunks inside a function receive expanded context; fail-open on parse errors |

## Steps

### Step 0: Design parser approach

- [ ] Choose tree-sitter vs line-based fallback for v1; record in STATUS Discoveries
- [ ] Extensions: `.js`, `.jsx`, `.ts`, `.tsx`

### Step 1: Implement JS/TS expansion

- [ ] Add `expand_jsts.go` with enclosing function/class-method detection
- [ ] Wire dispatch from `expand.go` for JS/TS extensions only
- [ ] Unit tests: hunk mid-function, hunk outside function → unchanged

### Step 2: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Run Contract `testCommand` (or full `go test ./internal/expand/...` if `-run JSTS` not used)
- [ ] Document JS/TS in `docs/review-process-internals.md`
- [ ] Per-file coverage ≥ 72% on changed expand files

## Completion Criteria

- [ ] All steps complete
- [ ] JS/TS expansion with Go-parity fail-open semantics
- [ ] Tests pass

## Git Commit Convention

- **Implementation:** `feat(SP-008): description`

## Do NOT

- Implement Python (SP-009)
- Add OpenCode backend or cross-file impact

---

## Amendments (Added During Execution)

**2026-07-01:** Split from combined JS/TS+Python L task; Python moved to SP-009.
