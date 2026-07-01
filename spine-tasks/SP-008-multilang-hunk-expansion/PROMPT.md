# Task: SP-008 — Multi-language hunk expansion (JS/TS and Python)

**Created:** 2026-06-30
**Size:** L

## Review Level: 2

**Assessment:** Extends Phase 6.4 beyond Go; parser choice and token caps need careful fail-open behavior.
**Score:** 5/8 — Blast radius: 2, Pattern novelty: 2, Security: 0, Reversibility: 1

## Mission

Close the **partial implementation** gap for hunk expansion: today `expand.ExpandHunk` only enriches **Go** files with enclosing-function context; non-Go hunks are unchanged. Add enclosing-scope expansion for **JavaScript/TypeScript** and **Python** using lightweight parsing (tree-sitter or stdlib-equivalent) with the same fail-open and token-truncation semantics as Go.

Out of scope: full tree-sitter for all languages, Rust/Java expansion (future tasks).

## Dependencies

- **None**

## Context to Read First

- `cli/internal/expand/expand.go` — Go `ExpandHunk` reference implementation
- `docs/implementation-plan.md` Phase 6.4
- `docs/context-enrichment-research.md` Tier 2
- `docs/rust-support-implementation.md` (Phase 3 expansion — not this task)

## Environment

- **Workspace:** `cli/` Go module

## File Scope

- `cli/internal/expand/expand.go`
- `cli/internal/expand/expand_test.go`
- New files under `cli/internal/expand/` if split by language (e.g. `expand_js.go`, `expand_py.go`)
- `cli/internal/review/review.go` (only if expand call site needs language hook)
- `docs/review-process-internals.md` (expansion section)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./internal/expand/... ./internal/review/... -count=1` |
| fileScopeMustChange | `cli/internal/expand/expand.go` |
| fileScopeMustNotChange | `extension/**`, `scripts/**`, `spine-tasks/**` |
| completionCriteria | JS/TS and Python hunks inside a function receive expanded context; fail-open on parse errors; token truncation matches Go behavior |

## Steps

### Step 0: Design parser approach

- [ ] Evaluate: tree-sitter via Go binding vs regex/line-based fallback for v1
- [ ] Record choice in STATUS Discoveries; prefer fail-open like Go path
- [ ] Define file extensions: `.js`, `.jsx`, `.ts`, `.tsx`, `.py`

### Step 1: JS/TS enclosing function expansion

- [ ] Implement expansion for JS/TS (function/class method enclosing hunk line range)
- [ ] Unit tests with fixture files (hunk mid-function, hunk outside function → unchanged)

### Step 2: Python enclosing function expansion

- [ ] Implement expansion for `.py` (def/class method enclosing range)
- [ ] Unit tests with fixtures

### Step 3: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Run Contract `testCommand`
- [ ] Document supported languages in `docs/review-process-internals.md`
- [ ] Verify per-file coverage ≥ 72% on changed files

## Completion Criteria

- [ ] All steps complete
- [ ] JS/TS and Python expansion with Go-parity fail-open semantics
- [ ] Tests pass with coverage thresholds

## Git Commit Convention

- **Implementation:** `feat(SP-008): description`

## Do NOT

- Add OpenCode backend
- Implement cross-file impact (roadmap 10.1)
- Block review on parse failure

---

## Amendments (Added During Execution)

**Note:** If this task exceeds stall budget, split follow-up as SP-009 for additional languages (Rust/Java).
