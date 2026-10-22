# SP-058: `.jsx` hunks get JS comment stripping in semantic hunk IDs — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-22
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Use jsts.IsExt in langFromPath
**Status:** ✅ Complete

- [x] `langFromPath` uses `jsts.IsExt` on the lowercased extension

---

### Step 2: Testing & Verification
**Status:** ✅ Complete

- [x] `.jsx` comment-only and upper-case extension tests
- [x] `.jsx` literal only in `cli/internal/jsts/ext.go`
- [x] Contract `testCommand` passing (stet review exits 0)

**Verification evidence (2026-10-22):**

- `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/hunkid/... -count=1` → `ok stet/cli/internal/hunkid` (93.3% coverage with `-cover`)
- `rg -n '"[.]jsx"' cli --glob '!*_test.go'` → only `cli/internal/jsts/ext.go:10`
- `bash scripts/spine-stet-contract-run.sh lenient` → `0 finding(s).`, exit 0

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|
| 1 | plan | 1 | skipped (engine-owned; nested reviewer spawn blocked in pi worker session, SP-195) | `.reviews/1-20261022T234314.md` |

---

## Completion Criteria

- [x] All steps complete
- [x] All tests passing

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
