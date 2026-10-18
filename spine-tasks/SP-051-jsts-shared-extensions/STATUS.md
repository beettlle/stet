# SP-051: One JS/TS extension list for expand and RAG — Status

**Current Step:** Step 3
**Status:** 🟨 In Progress
**Last Updated:** 2026-10-18
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Shared list
**Status:** ✅ Complete

- [x] `cli/internal/jsts/ext.go` with `Extensions` and `IsExt`; unit test

---

### Step 2: Wire both consumers
**Status:** ✅ Complete

- [x] `expand` uses `jsts.IsExt` (`isJSTSExt` delegates; `expand.go` caller is outside File Scope)
- [x] `rag/js` registers all extensions (adds `.jsx`)

---

### Step 3: Testing & Verification
**Status:** 🟨 In Progress

- [x] `.jsx` RAG test (`TestRegistry_everyJSTSExtension_resolvesDefinition`; fails on `.jsx` against old init)
- [x] `.jsx` expand test (existing `arrow_function_jsx` case in `TestExpandHunk_JSTS_enclosingFunction`)
- [x] `.mjs`/`.cjs` still resolve (same registry test)
- [ ] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| `hunkid.langFromPath` keeps its own JS list (`.js .mjs .cjs .ts .tsx`, no `.jsx`) for comment stripping | Out of scope (file not in File Scope); candidate follow-up to use `jsts.IsExt` | `cli/internal/hunkid/hunkid.go:84` |
| Stet reviewer bug: ERROR 4760d8c claimed removed `jsExt`/`tsxExt` constants still have usages ("compilation errors"); vet/build/tests pass and `rg` finds none — reviewer cannot see whole file on refactor diffs (same class as SP-040 hazard) | Dismissed `false_positive` | `cli/internal/expand/expand_jsts.go:10` |
| Stet reviewer bug: 5c92fa8 claimed `t.Fatalf` runs inside `t.Run` subtest; it is in the parent test body before `t.Run` | Dismissed `false_positive` | `cli/internal/rag/js/resolver_test.go:78` |
| Other stet findings: 78f709c (`.ts`/`.tsx` literals in `jstsFenceLang` — restoring constants defeats single list), f8cd8f4 (mutable exported slice — PROMPT mandates exported `Extensions`; Go has no const slices), 4fdfc9b (`.jsx.bak` case tests exact-match semantics of exported `IsExt`), 2b0970e (`.jsx` registration is the task's intent, #16) | Dismissed `wrong_suggestion`/`wrong_suggestion`/`false_positive`/`already_correct` | stet session |
| Stet contract run exceeded 30 min tool timeout while other lanes shared the LLM backend; rerun in background | Environment contention, not a code issue | `scripts/spine-stet-contract-run.sh` |
