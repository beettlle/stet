# Task: SP-029 — Rules `**` glob support

**Created:** 2026-10-08
**Size:** S

## Review Level: 1

**Assessment:** Single-package matching fix, but it changes which Cursor rules are injected into every review prompt.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 0

## Mission

Make `rules.FilterRules` honour recursive `**` globs so language rules (frontmatter like `globs: ["**/*.go"]`) actually apply to files under review.

**Bug (measured in `docs/research/systemone-rules-compliance.md`, Q2):** `FilterRules` in `cli/internal/rules/rules.go` calls `filepath.Match`, which has no `**` support. A pattern containing `/` is matched against the full slash path, so `**/*.go` behaves like `*/*.go`: it never matches `cli/internal/run/run.go` (three segments) and never matches a bare `main.go` (no slash). Today only `alwaysApply` rules are injected for every probed path.

**Required semantics** (gitignore / Cursor style, forward slashes, relative to repo root):

- `**` as a full path segment matches zero or more segments: `**/*.go` matches `main.go`, `cli/main.go`, `cli/internal/run/run.go`; `cli/**` matches everything under `cli/`; `cli/**/x_test.go` matches `cli/x_test.go` and `cli/a/b/x_test.go`.
- Patterns without `/` keep today's behaviour (match the base name): `*.ts` matches `extension/src/a.ts`.
- Patterns with `/` but no `**` keep today's `filepath.Match` behaviour against the full path.
- A malformed pattern (e.g. `[`) never matches and never panics.
- Implement with the standard library only (segment-wise matching with `path.Match`); do **not** add a third-party glob dependency.

## Dependencies

- **None**

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `cli/internal/rules/rules.go` — `FilterRules`, `descriptionGlobMap`, `InferGlobsFromDescription`
- `docs/research/systemone-rules-compliance.md` — Q2 (glob-semantics gap)

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none

## File Scope

- `cli/internal/rules/rules.go`
- `cli/internal/rules/rules_test.go`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go vet ./internal/rules/... && go test ./internal/rules/... ./internal/prompt/... -count=1 -cover` |
| fileScopeMustChange | `cli/internal/rules/rules.go`, `cli/internal/rules/rules_test.go` |
| fileScopeMustNotChange | `cli/internal/systemone/**`, `cli/internal/run/**`, `cli/cmd/**`, `extension/**` |
| completionCriteria | `**` patterns match zero or more path segments per Mission; non-`**` behaviour unchanged; table tests cover every Mission example plus malformed patterns; rules package coverage ≥ 72% |

## Steps

### Step 1: Recursive glob matching

- [ ] Add an unexported `matchGlob(pattern, target string) bool` used by `FilterRules` (segment-wise; `**` matches zero or more segments)
- [ ] Keep base-name matching for slash-free patterns and `filepath.Match` semantics for slash patterns without `**`
- [ ] Update the "no **" comments on `descriptionGlobMap` / `InferGlobsFromDescription` only if they become inaccurate

### Step 2: Testing & Verification

> ZERO test failures allowed.

- [ ] Table tests for every Mission example, plus: `**` alone, leading/trailing `**`, consecutive `**/**`, Windows-style input path (`filepath.ToSlash`), malformed pattern
- [ ] Existing `FilterRules` / `prompt` tests still pass unchanged (behaviour preserved for non-`**` patterns)
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Per-file coverage ≥ 72% for `rules.go`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Implementation:** `fix(SP-029): description`
- **Tests:** `test(SP-029): description`

## Do NOT

- Add a glob library dependency (`go.mod` must not change)
- Change rule loading, token budgets, or prompt assembly (`cli/internal/prompt`)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
