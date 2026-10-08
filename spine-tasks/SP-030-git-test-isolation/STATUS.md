**Current Step:** Step 1: Not started
**Status:** Ready
**Last Updated:** 2026-10-08
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

## Step 1: `testutil` helpers

**Status:** Not Started

- [ ] `RunIsolatedGit` and `TempGitDir`
- [ ] Unit tests for env, gitconfig, retry success, final failure

## Step 2: Apply to git-heavy packages

**Status:** Not Started
> ⚠️ Hydrate: list each package as it is converted

- [ ] `TestMain` in every in-scope package
- [ ] `TempGitDir` for git repo temp dirs

## Step 3: Testing & Verification

**Status:** Not Started

- [ ] Contract `testCommand`
- [ ] 5 consecutive full-suite runs green
- [ ] Coverage gate (77% / 72%)

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| | | | |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-10-08 | Root cause: git-ai daemon (trace2 eventtarget) writes `.git/ai` + `refs/notes/ai` into test repos asynchronously | Races `t.TempDir` cleanup; fails ~50% of full runs |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-10-08 | Authored | From CONTEXT.md tech-debt backlog |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
