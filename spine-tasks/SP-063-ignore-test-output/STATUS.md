# SP-063: Ignore `test_output.txt`; release check fails when one is tracked — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-22
**Review Level:** 0
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Ignore rule and release-check guard
**Status:** ✅ Complete

- [x] `.gitignore` rule
- [x] Release-check guard step and header line

---

### Step 2: Testing & Verification
**Status:** ✅ Complete

- [x] Contract `testCommand` passing
- [x] Guard fails with a tracked `test_output.txt` (scratch clone)
- [x] Guard passes on the lane worktree
- [x] Diff limited to the two files

**Evidence:**

- Contract `testCommand` (`bash -n` + both `git check-ignore --no-index` paths + `rg` for the step name): exit 0.
- Negative test in scratch clone `git clone -q . /tmp/sp063-check`, guard logic run verbatim with `git add -f test_output.txt` (root) and separately `cli/test_output.txt`:
  ```
  FAIL: test_output.txt must not be tracked in git.
  Tracked paths:
  test_output.txt
  Fix: git rm --cached <path>
  --- exit code: 1 ---
  ```
  Nested case printed `cli/test_output.txt` in Tracked paths, exit 1. Clone deleted afterwards.
- Lane worktree: `git ls-files` has no `test_output.txt`; guard logic prints PASS.
- `git diff HEAD~1 --name-only`: `.gitignore`, `scripts/release-check.sh` (+ this STATUS.md only).

---

## Completion Criteria

- [x] All steps complete
- [x] Contract `testCommand` exits 0

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| AGENTS.md check-if-affected: its "Release gate" sentence summarizes release-check.sh ("all of the above plus gofmt, go vet, and a cross-compile") without enumerating individual guard steps, so the new test-output guard does not make it incomplete. No edit needed (harness-maintained anyway). | No action | AGENTS.md §Testing |
| The guard matches basenames via `(^|/)test_output\.txt$`, so it catches root and nested occurrences (both historical: SP-052 root, 7020f31 cli/). | Noted | scripts/release-check.sh |
