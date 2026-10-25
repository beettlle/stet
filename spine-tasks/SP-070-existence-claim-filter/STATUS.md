# SP-070: Drop "unused / undefined / not imported" findings the file disproves — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-24
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

### Step 1: `FilterExistenceClaims` library and table tests
**Status:** ✅ Complete

- [x] Claim detection, identifier extraction, proof rules, conservative keeps

---

### Step 2: Wire into both pipeline branches
**Status:** ✅ Complete

- [x] Runs after the evidence filter and before the critic in both branches; trace line

---

### Step 3: Testing & Verification
**Status:** ✅ Complete

- [x] #33 fixture tests and run-package ordering test
- [x] Contract `testCommand` passing (stet review exits 0)

---

## Completion Criteria

- [x] All steps complete
- [x] All tests passing — `check-gofmt` ✓, `go vet ./cli/...` ✓, `go test ./cli/internal/findings/... ./cli/internal/run/... -count=1` ✓, stet contract review exits 0 (0 active findings)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| Contract finding 8596293: "Unused import 'path/filepath' will cause a compilation error" — demonstrably false: `filepath.Join` is called in both pipeline branches and the build/vet pass (unused imports are compile errors in Go). Stet reviewer bug. | Dismissed `false_positive` | `cli/internal/run/run.go:19` |
| Contract findings beb1e0d + 9584059: "path traversal" via `filepath.Join(opts.RepoRoot, p.Hunk.FilePath)` — false positive in this codebase's trust model: `Hunk.FilePath` originates from the repo's own `git diff` (git cannot track `..`/absolute paths), and the identical join already exists at `expand.go:134` plus every RAG resolver — the same file content already reaches the prompt context, so the new read adds no exposure. | Dismissed `false_positive` | `cli/internal/run/run.go:513`, `cli/internal/run.go:611` |
| Contract finding 7264a37: claims the immutability test's `reflect.DeepEqual` is "brittle" over the `Range *LineRange` pointer — false positive: fixtures leave `Range` nil, and `FilterExistenceClaims` never dereferences or writes any finding field (it appends value copies to a new slice), so no in-place mutation path exists. | Dismissed `false_positive` | `cli/internal/findings/existence_test.go:393` |
| Contract finding b5c7846 (O(N·M) regex scans in `identUsedTwiceOutsideDecl`) was a legitimate performance warning — fixed by `combinedPattern`, which folds the per-pattern scans into one alternation regex (flag scopes preserved via passive groups) for all existence proofs. | Fixed | `cli/internal/findings/existence.go` |
| Doc check per PROMPT: `docs/cli-extension-contract.md` does not list individual trace post-filter lines (only high-level strictness/FP behavior), so no doc edit required for the new `Existence claims: N -> M` trace. SP-076 owns doc updates. | No action | `docs/cli-extension-contract.md` |
| Mixed-claim messages (e.g. "`fs` is unused and `x` is not defined") require every extracted identifier to satisfy every present claim's proof before dropping — the strict reading of "every identifier is proven", biased to conservative keeps. | By design | `cli/internal/findings/existence.go` (`existenceClaimDisproven`) |
