# SP-044: Keep the review worktree out of `git add` — Status

**Current Step:** Complete
**Status:** 🟢 Complete — contract green (0 findings)
**Last Updated:** 2026-10-15
**Last Updated:** 2026-10-15
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Ignore helper and wiring
**Status:** ✅ Complete

- [x] `ignore.go` helper (inside-repo check, exclusive create, `*`)
- [x] `Create` calls it; failure warns only

---

### Step 2: Docs
**Status:** ✅ Complete

- [x] `docs/cli-extension-contract.md` `.review/` paragraph accurate

---

### Step 3: Testing & Verification
**Status:** ✅ Complete

- [x] `git add -A` regression test (no gitlink staged)
- [x] Existing file untouched; outside-repo root; read-only helper error
- [x] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|
| 1 | plan | 1 | skipped (engine-owned, SP-195) | `.reviews/1-20261015T101523.md` |

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| stet reviewer misunderstanding #1 (ac0a64c): claimed `.review/worktrees/.gitignore` "will not prevent the worktree directory itself from being staged" — per-directory gitignore files DO apply to the outer repo; disproven by `TestCreate_WorktreeRootNotStaged` (real `git add -A` + `git ls-files -s`) | dismissed `false_positive`; reviewer bug | session 2026-10-15, contract run 1 |
| stet reviewer misunderstanding #2 (291a6ce): claimed worktree-root `.gitignore` "only affects git operations run from within that worktree" — same false gitignore-scope model; suggested writing to `filepath.Dir(root)`, contradicting PROMPT-prescribed design | dismissed `false_positive`; reviewer bug | session 2026-10-15, contract run 2 |
| stet reviewer negation misread (0b4056e): claimed regression test is "tautological and will always pass" — mutation check (helper disabled) makes the test fail with `160000 … .review/worktrees/stet-*`, proving it catches the issue; explicit worktree-path-absence assertion added anyway | dismissed `false_positive`; test strengthened | session 2026-10-15, contract run 2 |
| Real defect found by stet (db9873b, fixed): `pathInsideRepo` treated `root == repoRoot` (`rel == "."`) as inside, which would drop a `*` `.gitignore` on the repo root under a degenerate `worktree_root` config; now guarded and unit-tested | fixed in `ignore.go` | session 2026-10-15, contract run 1 |

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-10-14 | Task staged | PROMPT.md and STATUS.md created (release v1.1.0) |
| 2026-10-15 | Step 1 | `ignore.go` (`ensureWorktreeRootIgnored` + `pathInsideRepo` with symlink-resolved containment) wired into `Create` after `MkdirAll`; failure → `warning: could not write <path>: <err>` on stderr. gofmt/vet clean; `go test ./cli/internal/git/...` pass. Plan review skipped (engine-owned, SP-195) |
| 2026-10-15 | Step 2 | `docs/cli-extension-contract.md` `.review/` paragraph replaced: stet writes `.review/worktrees/.gitignore`; state files not auto-ignored; recommended rules `.review/*`, `!.review/config.toml`, `!.review/prompt.md`. SP-043 already merged, no doc conflict |
| 2026-10-15 | Step 3 | 8 tests added in `ignore_test.go` (create/ErrExist idempotent, existing untouched, outside-repo, empty-args, repo-root guard, read-only helper error, Create-warns stderr capture, `git add -A` regression). `ignore.go` coverage 77.8% (≥72%) |
| 2026-10-15 | Contract run 1 | 2 findings: db9873b (real — fixed with `rel == "."` guard + test), ac0a64c (false — gitignore-scope misunderstanding; dismissed, logged) |
| 2026-10-15 | Contract run 2 | 2 findings: 291a6ce + 0b4056e (both false — same gitignore-scope error; negation misread disproven by mutation check); dismissed, logged; explicit path-absence assertion added to regression test |
| 2026-10-15 | Contract run 3 | **0 findings, exit 0.** Full testCommand green: check-gofmt ✓, go vet ✓, `go test ./cli/internal/git/...` ✓, stet contract ✓ |

---

## Blockers

*None*
