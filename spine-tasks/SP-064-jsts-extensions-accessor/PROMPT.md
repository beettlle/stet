# Task: SP-064 — `jsts.Extensions()` returns a copy; unexport the slice

**Created:** 2026-10-22
**Size:** S

## Review Level: 1

**Assessment:** Small API change in `cli/internal/jsts/ext.go` with two callers (`rag/js` `init()` and tests).
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 0, Security: 0, Reversibility: 1

## Mission

Closes #27

`cli/internal/jsts/ext.go` exports the JS/TS extension list as a mutable package variable:

```go
// Callers must not modify it.
var Extensions = []string{".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx"}
```

Nothing enforces "must not modify". Any package can overwrite an element or reslice it and change what both `expand` and `rag/js` treat as JS/TS.

Fix:

1. Rename the variable to an unexported `extensions` and add `func Extensions() []string` that returns `slices.Clone(extensions)`. Keep `IsExt(ext string) bool` with the same behavior (exact, case-sensitive), reading the unexported slice. Update the package and function doc comments.
2. Update callers: `cli/internal/rag/js/resolver.go` `init()` (~line 100) ranges over `jsts.Extensions()`; `cli/internal/jsts/ext_test.go` and `cli/internal/rag/js/resolver_test.go` (~line 87) use the accessor.
3. Add a test: mutating the slice returned by `Extensions()` (assign an element, append) does not change a later `Extensions()` result or `IsExt`.

Find every reference first: `rg -n 'jsts\.Extensions|\bExtensions\b' cli`.

## Dependencies

- **Task:** SP-059 (also edits `cli/internal/rag/js/resolver.go` and `resolver_test.go`; serialize to avoid merge conflicts)

## Context to Read First

**Tier 3 (reference docs):**

- `cli/internal/jsts/ext.go`, `cli/internal/jsts/ext_test.go`
- `cli/internal/rag/js/resolver.go` — `init()`
- `cli/internal/rag/js/resolver_test.go` — registry test (~line 87)

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/jsts/ext.go`
- `cli/internal/jsts/ext_test.go`
- `cli/internal/rag/js/resolver.go`
- `cli/internal/rag/js/resolver_test.go`

## Documentation Requirements

**Must Update:** none (SP-066 updates the `jsts.Extensions` mention in `docs/review-process-internals.md`).

**Check If Affected:** none.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/jsts/... ./cli/internal/rag/... ./cli/internal/expand/... ./cli/internal/hunkid/... -count=1 && bash scripts/spine-stet-contract-run.sh lenient` |
| fileScopeMustChange | `cli/internal/jsts/ext.go`, `cli/internal/jsts/ext_test.go`, `cli/internal/rag/js/resolver.go` |
| fileScopeMustNotChange | `cli/internal/expand/**`, `cli/internal/hunkid/**`, `extension/**`, `.review/**` |
| completionCriteria | No exported mutable `Extensions` variable remains; mutating the returned slice does not change later `Extensions()` or `IsExt` results (test); every JS/TS extension still registers a RAG resolver (existing registry test passes); `IsExt` behavior unchanged; stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh lenient`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Accessor and callers

- [ ] Unexported slice, `Extensions()` returns a clone, `IsExt` unchanged
- [ ] `rag/js` `init()` and tests use `jsts.Extensions()`
- [ ] `spine_review_step` after the step

### Step 2: Testing & Verification

> ZERO test failures allowed.

- [ ] Mutation-isolation test in `ext_test.go`
- [ ] `rg -n 'jsts\.Extensions\b[^(]' cli` returns nothing (all uses are calls)
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Refactor:** `refactor(SP-064): description`
- **Tests:** `test(SP-064): description`

## Do NOT

- Change the extension list contents or order
- Change `extractSymbols` or the built-in name sets (SP-059 owns them)
- Edit `expand` or `hunkid` (they only call `IsExt`)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
