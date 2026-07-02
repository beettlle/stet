# Task: SP-013 — Guard against extension/coverage re-tracking

**Created:** 2026-07-01
**Size:** S

## Review Level: 1

**Assessment:** Small script + doc hook; prevents repeat of spine DirtyWorktree failures.
**Score:** 2/8 — Blast radius: 0, Pattern novelty: 1, Security: 0, Reversibility: 1

## Mission

Prevent regression of commit `175c1a4` (`chore: stop tracking extension/coverage artifacts`): **`extension/coverage/` must not be tracked in git** even though Vitest writes there during `npm test`.

Add a repo check script and wire it into the project's standard verification path so spine batches and CI do not fail with DirtyWorktree after extension tests.

## Dependencies

- **None**

## Context to Read First

- `.gitignore` — `extension/coverage` entry
- Commit `175c1a4` — untracked coverage artifacts
- [pi-spine #73](https://github.com/beettlle/pi-spine/issues/73) — DirtyWorktree on tracked coverage files
- `AGENTS.md` — build/test commands

## Environment

- **Workspace:** `scripts/`, `AGENTS.md`

## File Scope

- `scripts/check-untracked-coverage.sh` (new)
- `AGENTS.md` (verification section only)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-untracked-coverage.sh && cd extension && npm test && bash ../scripts/check-untracked-coverage.sh` |
| fileScopeMustChange | `scripts/check-untracked-coverage.sh` |
| fileScopeMustNotChange | `cli/**`, `extension/src/**` |
| completionCriteria | Script fails if `git ls-files extension/coverage` is non-empty; passes after extension npm test |

## Steps

### Step 0: Define check behavior

- [ ] Fail if any path under `extension/coverage/` is tracked (`git ls-files`)
- [ ] Optionally warn on dirty untracked coverage after tests (informational only; do not fail on untracked)
- [ ] Record behavior in STATUS

### Step 1: Implement script and AGENTS.md hook

- [ ] Add `scripts/check-untracked-coverage.sh` with clear PASS/FAIL messages
- [ ] Document when to run it in `AGENTS.md` Testing section (after extension `npm test`, before spine batches)

### Step 2: Testing and verification

- [ ] Run Contract `testCommand`
- [ ] Confirm script would have caught pre-`175c1a4` state (mental check or temporary `git add -f` on fixture path in test branch — do not commit tracked coverage)

## Completion Criteria

- [ ] All steps complete
- [ ] Script exits 0 on current `main`
- [ ] AGENTS.md mentions the guard

## Git Commit Convention

- **Implementation:** `chore(SP-013): description`

## Do NOT

- Re-add `extension/coverage/` to git
- Change Vitest coverage output directory without updating `.gitignore` and this check
