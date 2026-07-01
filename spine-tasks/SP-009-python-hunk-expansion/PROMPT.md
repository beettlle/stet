# Task: SP-009 — Python hunk expansion

**Created:** 2026-06-30
**Size:** M

## Review Level: 2

**Assessment:** Python enclosing-scope parsing; depends on SP-008 expand dispatch pattern.
**Score:** 4/8 — Blast radius: 1, Pattern novelty: 2, Security: 0, Reversibility: 1

## Mission

Add enclosing-scope expansion for **Python** (`.py`) hunks in `cli/internal/expand/`, matching Go/JS/TS fail-open and token-truncation semantics.

## Dependencies

- **SP-008**

## Context to Read First

- `cli/internal/expand/expand.go` — dispatch after SP-008
- `cli/internal/expand/expand_jsts.go` — SP-008 pattern reference
- `docs/review-process-internals.md`

## Environment

- **Workspace:** `cli/` Go module

## File Scope

- `cli/internal/expand/expand.go` (Python dispatch hook only)
- `cli/internal/expand/expand_py.go` (new)
- `cli/internal/expand/expand_test.go` (Python fixtures only)
- `docs/review-process-internals.md` (Python expansion note)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./internal/expand/... -count=1 -run Python` |
| fileScopeMustChange | `cli/internal/expand/expand_py.go` |
| fileScopeMustNotChange | `extension/**`, `scripts/**`, `cli/internal/expand/expand_jsts.go` |
| completionCriteria | Python hunks inside def/class method receive expanded context; fail-open on parse errors |

## Steps

### Step 0: Design Python parser approach

- [ ] Choose ast-based vs line-based approach; record in STATUS Discoveries

### Step 1: Implement Python expansion

- [ ] Add `expand_py.go` with def/class-method enclosing range
- [ ] Wire dispatch from `expand.go` for `.py` only
- [ ] Unit tests with fixtures

### Step 2: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Run Contract `testCommand`
- [ ] Document Python in `docs/review-process-internals.md`
- [ ] Per-file coverage ≥ 72% on changed expand files

## Completion Criteria

- [ ] All steps complete
- [ ] Python expansion with Go-parity fail-open semantics
- [ ] Tests pass

## Git Commit Convention

- **Implementation:** `feat(SP-009): description`

## Do NOT

- Modify JS/TS expansion (SP-008)
- Add Rust/Java expansion

---

## Amendments (Added During Execution)
