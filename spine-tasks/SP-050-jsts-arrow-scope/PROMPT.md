# Task: SP-050 — JS/TS expand: plain functions with arrow callbacks keep their full scope

**Created:** 2026-10-17
**Size:** S

## Review Level: 1

**Assessment:** Branch-order fix in one function of the expand package.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 0

## Mission

Closes #24

`jstsDeclLineRange` in `cli/internal/expand/expand_jsts.go` (~line 331) searches the next 12 lines for `=>` (`findSignificantToken(chunk, "=>")`, ~line 344) before looking for the function body brace. For a plain `function` declaration whose body contains an arrow callback with an expression body, the arrow branch wins and the scope ends at the first line containing `;`.

Example (`pending.mjs`, lines 3–6) — currently reported as 3–4, must be 3–6:

```js
export function filterPendingTaskIds(ids, done) {
  const pending = ids.filter((id) => !done.has(id));
  return pending.sort(compareTaskIds);
}
```

Fix: take the arrow branch only when the declaration's `=>` comes **before** the declaration's own body brace. Compute the body brace with `findFunctionBodyOpenBrace(chunk, declStart)` (relative offset) and compare with `relArrow`; if the body brace exists and precedes `=>`, use the body-brace path. Keep the existing arrow paths (block body and expression body) for `const/let/var x = (...) =>` declarations and class-field arrows.

## Dependencies

- **None**

## Context to Read First

**Tier 3 (reference docs):**

- `cli/internal/expand/expand_jsts.go` — `jstsDeclLineRange`, `findFunctionBodyOpenBrace`, `findSignificantToken`
- `cli/internal/expand/expand_jsts_test.go` — existing arrow and expression-body tests

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/expand/expand_jsts.go`
- `cli/internal/expand/expand_jsts_test.go`

## Documentation Requirements

**Must Update:** none

**Check If Affected:**

- `docs/review-process-internals.md` — JS/TS expand bullet (SP-056 updates this doc; log anything it needs in STATUS Discoveries)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/expand/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/expand/expand_jsts.go`, `cli/internal/expand/expand_jsts_test.go` |
| fileScopeMustNotChange | `cli/cmd/**`, `cli/internal/run/**`, `cli/internal/rag/**`, `extension/**`, `scripts/**`, `.review/**` |
| completionCriteria | The issue example expands to lines 3–6; existing arrow-function and expression-body arrow tests pass; stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Arrow branch only before the body brace

- [ ] `jstsDeclLineRange` prefers the body-brace path when the declaration's body brace precedes `=>`
- [ ] `spine_review_step` after the step

### Step 2: Testing & Verification

> ZERO test failures allowed.

- [ ] Test: the issue example (`.mjs` file) — hunk on lines 4–5 gets enclosing scope 3–6
- [ ] Test: same shape in `.ts` (function with `arr.map((x) => x * 2)` in the body)
- [ ] Existing arrow block-body and expression-body tests unchanged and passing
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Fix:** `fix(SP-050): description`
- **Tests:** `test(SP-050): description`

## Do NOT

- Change `isJSTSExt` or the extension list (SP-051 owns that)
- Rewrite the JS/TS parser or change token truncation
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
