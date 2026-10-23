# SP-059: JS RAG skips built-in method names only as member calls — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-22
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Split globals from member-method names
**Status:** ✅ Complete

- [x] Globals set and member-method set; member names skipped only as `.name(`
- [x] Import-bound names, bare arguments, and declarations kept

---

### Step 2: Testing & Verification
**Status:** ✅ Complete

- [x] Import-bound `get` resolved from temp repo
- [x] Member calls add no built-in candidates
- [x] Bare `log(x)` is a candidate
- [x] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|
| 1 | plan | 1 | skipped (engine-owned; SP-195) | .reviews/1-20261022T234625.md |
| 2 | code | final | 3 findings dismissed as false_positive; re-run exit 0 | stet session |
| 3 | code | resume re-verification | scoped re-run 700da3c..f54c931: 5 findings dismissed (4 false_positive, 1 out_of_scope); 0 active; finish exit 0 | temp worktree @ f54c931 |
| 4 | code | post-resume contract re-run | full testCommand exit 0; stet 0 findings (no new hunks since cfd8ff8) | lane worktree |

---

## Resume verification (2026-10-22)

Worker was interrupted after the original contract pass but before `.DONE`. On resume: re-ran the Contract testCommand from a clean state — `check-gofmt.sh` ✅ · `go vet ./cli/...` ✅ · `go test ./cli/internal/rag/... -count=1` ✅ (7 packages) · stet contract re-verified in a throwaway worktree pinned at `f54c931` with baseline `700da3c` (exactly this task's diff; later SP-061 lane commits excluded): `stet run` default → 5 findings → triaged → 0 active → `stet finish` exit 0.

## Post-resume contract re-run (2026-10-22; engine logs timestamped 2026-10-23T00:43Z)

After the resume commit `cfd8ff8`, the batch engine re-ran the Contract testCommand twice; both attempts failed at the stet step: `stet: no session and missing .review/spine-stet-baseline.ref (did worktreeSetupHook run?)`. Infrastructure ordering issue, not a code failure: gofmt, vet, and all 7 rag test packages passed in both engine logs (preserved in `.reviews/contract-fail-2026-10-23T00-43-*.log`). The worktree setup hook then created `.review/spine-stet-baseline.ref` (baseline `700da3c`) and the session; the session already reviewed the lane diff at HEAD `cfd8ff8` with 0 active findings (8 dismissed, see Discoveries). On worker resume the full Contract testCommand was re-run from the lane root: `check-gofmt.sh` OK, `go vet ./cli/...` OK, `go test ./cli/internal/rag/... -count=1` OK (7 packages), `spine-stet-contract-run.sh default` exit 0 (`0 finding(s).`, no new hunks since `last_reviewed_at: cfd8ff8`). No code changes were needed.

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| stet 10e0544 (ERROR): claimed leftover `jsBuiltinNames` references cause compile errors | false_positive — `rg` shows zero references; `go build`/`go vet`/tests pass; same diff updated the only call site (`extractSymbols`) | cli/internal/rag/js/resolver.go |
| stet b5b5dc7 (WARNING): claimed `isMemberCall` stops at newlines | false_positive — skip set explicitly includes `'\r'`/`'\n'`, so `obj.\n  method(` is detected as a member call | cli/internal/rag/js/resolver.go:193 |
| stet d03ebcc (WARNING): claimed `Promise.all(ps)` leaks `Promise` into candidates | false_positive — `add()` checks `jsBuiltinGlobals` unconditionally regardless of member flag; test passes | cli/internal/rag/js/resolver_test.go:310 |
| stet 3f6aee2 (WARNING, resume): claimed test :310 lacks positive assertions for `isOpen`/`toId`/`byId`/`visit`/`onOk`/`merge` | false_positive — those exact presence assertions exist in the same test at :317–321 | cli/internal/rag/js/resolver_test.go:317 |
| stet d0b8707 (WARNING, resume): claimed `import { console }` should yield `console` as a binding | false_positive — spec'd design (PROMPT fix #3: never skip import-bound names "unless they are true globals"); documented on `jsBuiltinGlobals` ("including when imported or required") | cli/internal/rag/js/resolver.go:99 |
| stet 6017fbe (WARNING, resume): claimed `console` receiver would be extracted, breaking the member-call test | false_positive — `jsBuiltinGlobals` skipped unconditionally of the member flag; test assertion passes (fresh `go test` evidence) | cli/internal/rag/js/resolver_test.go:340 |
| stet f84fc4b (ERROR, resume): claimed `initGitRepo` helpers unreachable from js package and import-path mismatch (`./cache.mjs` vs `lib/`) | false_positive — helpers defined in `js/resolver_test.go:497+`; lookup is repo-wide `git grep`, and import-path resolution in the `from` module is out of scope per PROMPT (remaining #19); test compiles and passes | cli/internal/rag/js/resolver_test.go:471 |
| stet 70b8bc5 (WARNING, resume): `obj?.method(` not treated as member call by `isMemberCall` | out_of_scope — PROMPT defines member call as "`.` with optional whitespace"; consequence is an extra grep candidate only (graceful). Logged as tech debt in spine-tasks/CONTEXT.md | cli/internal/rag/js/resolver.go:193 |

## Completion Criteria

- [x] All steps complete
- [x] All tests passing

**Verification evidence:** `bash scripts/check-gofmt.sh` ✅ · `go vet ./cli/...` ✅ · `go test ./cli/internal/rag/... -count=1` ✅ (7 packages ok) · `bash scripts/spine-stet-contract-run.sh default` exit 0 (0 findings after triage) — re-verified on resume 2026-10-22 with a scoped stet run over `700da3c..f54c931` (0 active after triage, `stet finish` exit 0), and again post-resume with the full testCommand in the lane worktree (exit 0, 0 findings; see Post-resume contract re-run).