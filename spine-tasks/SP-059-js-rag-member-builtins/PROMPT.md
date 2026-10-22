# Task: SP-059 — JS RAG skips built-in method names only as member calls

**Created:** 2026-10-22
**Size:** S

## Review Level: 1

**Assessment:** Candidate filtering change inside `cli/internal/rag/js/resolver.go` plus tests; one package.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 0

## Mission

Closes #28

`extractSymbols` in `cli/internal/rag/js/resolver.go` (~line 138) drops every candidate whose name is in `jsBuiltinNames` (~line 45). That set mixes two kinds of names:

- **Prototype/static method names** (`filter`, `map`, `sort`, `get`, `set`, `add`, `has`, `clear`, `find`, `match`, `replace`, `split`, `trim`, `log`, `warn`, `error`, `info`, `debug`, `call`, `apply`, `bind`, ...). They are built-ins only when called as members (`arr.filter(`, `m.get(`, `console.log(`).
- **True globals** (`console`, `JSON`, `Math`, `Promise`, `Object`, `require`, `parseInt`, `setTimeout`, ...).

Because the skip is by name only, a bare call to a repo function with a method-like name is never looked up:

```js
import { get } from "./cache.mjs";
const v = get(key); // `get` never reaches RAG, so "get is not defined" false positives return (#19)
```

Fix:

1. Split `jsBuiltinNames` into a globals set (always skipped, as today) and a member-method set (skipped only when the occurrence is a member call, that is, the name is immediately preceded by `.` with optional whitespace).
2. For call-site candidates (`callIdent` matches), check the character before the match: skip a member-method name only for `.name(`; keep bare `name(` calls.
3. Never skip names returned by `importBoundNames` (import/require bindings), whatever their spelling, unless they are true globals or keywords.
4. Bare identifiers from `callArgumentIdents` and `function name(` declarations are not member accesses, so do not drop them for being in the member-method set.

Keep the candidate order, deduplication, and the `maxSymbolCandidates` cap unchanged.

## Dependencies

- **None**

## Context to Read First

**Tier 3 (reference docs):**

- `cli/internal/rag/js/resolver.go` — `jsBuiltinNames`, `extractSymbols`, `importBoundNames`, `callArgumentIdents`, `callIdent`
- `cli/internal/rag/js/resolver_test.go` — `TestExtractSymbols_skipsBuiltinMethodNames` (~line 300), `initGitRepo` (~line 401) for temp-repo lookups

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/rag/js/resolver.go`
- `cli/internal/rag/js/resolver_test.go`

## Documentation Requirements

**Must Update:** none (SP-066 updates `docs/review-process-internals.md`).

**Check If Affected:** none.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/rag/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/rag/js/resolver.go`, `cli/internal/rag/js/resolver_test.go` |
| fileScopeMustNotChange | `cli/internal/jsts/**`, `cli/internal/expand/**`, `extension/**`, `.review/**` |
| completionCriteria | `import { get } from "./cache.mjs"; get(key)` resolves `get` from a temp repo that defines it (test); `arr.filter(fn)` / `m.get(k)` / `console.log(x)` add none of `filter`, `get`, `log`, `console` (test); a bare `log(x)` call to a repo function is a candidate (test); existing `rag/js` tests pass; stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Split globals from member-method names

- [ ] Two sets: globals (always skipped) and member-method names (skipped only as `.name(` calls)
- [ ] Import/require-bound names, bare call arguments, and `function name(` declarations are never dropped for the member-method set
- [ ] `spine_review_step` after the step

### Step 2: Testing & Verification

> ZERO test failures allowed.

- [ ] Import-bound `get` resolved from a temp repo (uses `ResolveSymbols`, not only `extractSymbols`)
- [ ] Member calls `arr.filter(fn)`, `m.get(k)`, `console.log(x)` add no built-in candidates; `fn` still added
- [ ] Bare `log(x)` call is a candidate
- [ ] Update `TestExtractSymbols_skipsBuiltinMethodNames` only where its assertion assumed name-only skipping
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Fix:** `fix(SP-059): description`
- **Tests:** `test(SP-059): description`

## Do NOT

- Change `lookupDefinitions`, the grep strategy, or `maxSymbolCandidates` (#15 is separate)
- Resolve definitions in the `from` module (remaining part of #19; out of scope)
- Touch `jsts` or the `init()` registration loop (SP-064 owns those lines)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
