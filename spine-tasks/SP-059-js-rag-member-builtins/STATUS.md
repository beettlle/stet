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

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| stet 10e0544 (ERROR): claimed leftover `jsBuiltinNames` references cause compile errors | false_positive — `rg` shows zero references; `go build`/`go vet`/tests pass; same diff updated the only call site (`extractSymbols`) | cli/internal/rag/js/resolver.go |
| stet b5b5dc7 (WARNING): claimed `isMemberCall` stops at newlines | false_positive — skip set explicitly includes `'\r'`/`'\n'`, so `obj.\n  method(` is detected as a member call | cli/internal/rag/js/resolver.go:193 |
| stet d03ebcc (WARNING): claimed `Promise.all(ps)` leaks `Promise` into candidates | false_positive — `add()` checks `jsBuiltinGlobals` unconditionally regardless of member flag; test passes | cli/internal/rag/js/resolver_test.go:310 |

## Completion Criteria

- [x] All steps complete
- [x] All tests passing

**Verification evidence:** `bash scripts/check-gofmt.sh` ✅ · `go vet ./cli/...` ✅ · `go test ./cli/internal/rag/... -count=1` ✅ (7 packages ok) · `bash scripts/spine-stet-contract-run.sh default` exit 0 (0 findings after triage).