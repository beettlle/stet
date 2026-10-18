# SP-050: JS/TS expand: plain functions with arrow callbacks keep their full scope — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-17
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Arrow branch only before the body brace
**Status:** ✅ Complete

- [x] Body-brace path preferred when it precedes `=>`

---

### Step 2: Testing & Verification
**Status:** ✅ Complete

- [x] Issue example (`.mjs`) expands to 3–6
- [x] `.ts` variant test
- [x] Existing arrow tests passing
- [x] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| `.review/spine-stet-baseline.ref` was missing at worker start (worktreeSetupHook output absent); contract script exited 1. Recreated the session with `stet start <pre-worker HEAD 5d673a3>` and wrote the ref (gitignored). | Worked around; check that the hook runs for reused lanes | `scripts/spine-worktree-setup.sh` |
| stet reviewer finding 4377b4e claimed `findFunctionBodyOpenBrace` returns a chunk-relative offset; it returns `chunkStartOffset + rel` (absolute). | Dismissed `false_positive`. Stet reviewer bug: it misread the function contract | `cli/internal/expand/expand_jsts.go` |
| stet reviewer finding e2d39b1 suggested keeping arrow-scope for callbacks inside a function body, which is the bug this task fixes (#24). | Dismissed `wrong_suggestion` | `cli/internal/expand/expand_jsts.go` |
| Refinement: the body-brace path is taken only when the body brace opens before `=>` **and** closes after it, so TS arrow return types like `(x): { a: number } => ...` keep the arrow scope (covered by a test). | Implemented | `arrowInsideFunctionBody` |
| `docs/review-process-internals.md` JS/TS expand bullet could mention that arrow callbacks inside a `function` body no longer truncate the scope. | For SP-056 | docs |
