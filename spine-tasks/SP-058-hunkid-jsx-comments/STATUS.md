# SP-058: `.jsx` hunks get JS comment stripping in semantic hunk IDs — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-22
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 1
**Size:** S

---

### Step 1: Use jsts.IsExt in langFromPath
**Status:** ✅ Complete (commit af5eff5)

- [x] `langFromPath` uses `jsts.IsExt` on the lowercased extension

---

### Step 2: Testing & Verification
**Status:** ✅ Complete (commit dcd0ef2; triage completed in iteration 1)

- [x] `.jsx` comment-only and upper-case extension tests
- [x] `.jsx` literal only in `cli/internal/jsts/ext.go`
- [x] Contract `testCommand` passing (stet review exits 0)

**Verification evidence (2026-10-22, iteration 1):**

- `bash scripts/check-gofmt.sh` → clean; `go vet ./cli/...` → clean
- `go test ./cli/internal/hunkid/... -count=1 -cover` → `ok stet/cli/internal/hunkid` (93.3% coverage)
- `rg -n '"[.]jsx"' cli --glob '!*_test.go'` → only `cli/internal/jsts/ext.go:10`
- `bash scripts/spine-stet-contract-run.sh lenient` → first run this iteration found 8 active findings (exit 1); all 8 triaged and dismissed with verified reasons (see Discoveries); re-run → `0 finding(s)`, exit 0

**Note on earlier runs:** the previous session wrote "contract passing" before the contract
had actually run; it then failed twice (see `.reviews/contract-fail-*.log`, 3 hunkid findings
each) and the session ended before triage. Iteration 1 re-ran the contract from a fresh
session (recreated missing `.review/spine-stet-baseline.ref` = `700da3c`, parent of the
first SP-058 commit) and completed triage. Evidence above is from the passing re-run.

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|
| 1 | plan | 1 | APPROVE | `.reviews/1-20261022T234435.md` |

---

## Completion Criteria

- [x] All steps complete
- [x] All tests passing

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| Stet reviewer claims `stet/cli/internal/jsts` import is unused (be47528, ERROR). False: `jsts.IsExt(ext)` is called at hunkid.go:81; `go vet`/`go test` compile the package (Go rejects unused imports at compile time). Reviewer appears to reason only over the import block without checking usage elsewhere in the file. | false_positive (stet reviewer bug) | `cli/internal/hunkid/hunkid.go:14` |
| Stet reviewer claims case-sensitive `jsts.IsExt` breaks uppercase extensions (52d8925, ERROR). False: `langFromPath` lowercases the extension *before* calling `IsExt` (`.JSX` → `.jsx` matches); the finding itself quotes the lowercasing. `TestLangFromPath` asserts `file.JSX`/`file.TS` → `"js"` and passes. | false_positive (stet reviewer bug) | `cli/internal/hunkid/hunkid.go:81` |
| Stet reviewer claims `.jsx` is unhandled so the comment-only test must fail (6309bc4, ERROR). False: `.jsx` is in `jsts.Extensions`, so `langFromPath` returns `"js"` and comments are stripped; `TestSemanticHunkID_JSX` passes. Reviewer hypothesized from the `SemanticHunkID` doc comment without consulting `jsts/ext.go`. | false_positive (stet reviewer bug) | `cli/internal/hunkid/hunkid_test.go:145` |
| 5 additional findings in SP-060 files (`cli/internal/benchmark/**`: 56c5c0c, a555ea9, 66d313e, 9c429e2) and SP-063 file (`scripts/release-check.sh`: 361fbaa) are outside SP-058 file scope; spot-checked before dismissal: benchmark tests pass, `math` is used (lines 150/154/174), release-check uses `grep -E` so the pattern matches. Dismissed `out_of_scope`; owners may revisit via their own packets. | out_of_scope | `cli/internal/benchmark/**`, `scripts/release-check.sh` |

---
