# SP-064: `jsts.Extensions()` returns a copy; unexport the slice — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-23
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Accessor and callers
**Status:** ✅ Complete

- [x] Unexported slice; `Extensions()` returns a clone
- [x] Callers use the accessor

---

### Step 2: Testing & Verification
**Status:** ✅ Complete

- [x] Mutation-isolation test
- [x] No non-call uses of `jsts.Extensions` (`rg -n 'jsts\.Extensions\b[^(]' cli` → no matches)
- [x] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| Stet reviewer claimed `Extensions()` clone in `init()` is an unnecessary allocation; the clone is the mandated encapsulation (task mission) and runs once | dismissed `wrong_suggestion` | `cli/internal/rag/js/resolver.go:108` |
| Stet reviewer hallucinated a subtest race (duplicate `src/file.<ext>` paths, parallel t.Run). False on 3 counts: appended extensions are unique, each subtest uses its own `TempGitDir`, subtests are sequential (no `t.Parallel`) | dismissed `false_positive` | `cli/internal/rag/js/resolver_test.go:87` |
| Stet reviewer called `TestExtensions_mutationIsolation` flawed for mutating a copy; mutating the returned copy and then re-reading `Extensions()`/`IsExt` is exactly what the test must prove. Contract failed on this finding (2 attempts); after dismissal the contract re-run exits 0 | dismissed `false_positive` | `cli/internal/jsts/ext_test.go:15` |
