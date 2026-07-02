**Current Step:** Complete
**Status:** Complete
**Last Updated:** 2026-07-02
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 0: Define check behavior

**Status:** Complete

- [x] Tracked-path check via `git ls-files extension/coverage` — FAIL with listed paths if non-empty
- [x] Untracked dirty policy — WARN only (informational); never fail on untracked coverage output
- [x] Record in Discoveries

## Step 1: Implement script and AGENTS.md hook

**Status:** Complete

- [x] scripts/check-untracked-coverage.sh
- [x] AGENTS.md Testing section

## Step 2: Testing and verification

**Status:** Complete

- [x] Contract testCommand PASS
- [x] Verified script fails when `extension/coverage/lcov.info` force-tracked (simulated, not committed)

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| | | | |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-07-01 | Tracked extension/coverage caused spine DirtyWorktree | Motivation for SP-013 |
| 2026-07-02 | Script uses `git ls-files extension/coverage` for tracked check; `git ls-files --others` / directory probe for untracked WARN | Contract behavior |
| 2026-07-02 | Gitignored coverage dirs do not appear in `git status --porcelain`; WARN uses `find` fallback | Post-test warn works |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-07-02 | Step 0–1 | Defined check behavior; added script and AGENTS.md hook |
| 2026-07-02 | Step 2 | Contract testCommand PASS (85 tests); fail-path verified with `git add -f` simulation |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |
