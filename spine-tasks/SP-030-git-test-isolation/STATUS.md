**Current Step:** Step 3: Testing & Verification
**Status:** In Progress
**Last Updated:** 2026-10-08
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

## Step 1: `testutil` helpers

**Status:** Complete

- [x] `RunIsolatedGit` and `TempGitDir`
- [x] Unit tests for env, gitconfig, retry success, final failure

## Step 2: Apply to git-heavy packages

**Status:** Complete
> Converted: `cmd/stet` (TestMain + initRepo), `internal/diff` (TestMain + initRepoDiff), `internal/git` (TestMain in repo_test.go + initRepo in worktree_test.go), `internal/rag/{go,java,js,python,rust,swift}` (TestMain + 3 repo fixtures each; go also initCallGraphRepo), `internal/review` (TestMain + 2 initGitRepo tests), `internal/run` (TestMain + initRepo), `internal/scope` (TestMain + initRepoScope/initRepoGoComment), `internal/stats` (TestMain + initRepo). Non-git `t.TempDir()` dirs (state dirs, non-repo error paths, findings/skill/dataset outputs) intentionally unchanged.

- [x] `TestMain` in every in-scope package (13/13, exactly one per package)
- [x] `TempGitDir` for git repo temp dirs

> Amendment during Step 2: `TempGitDir` prefix changed `stet-git-test-*` → `gitrepo-test-*` because `git worktree list` includes the repo root and `internal/run`/`cmd/stet` tests assert no path contains the `stet-` substring (worktree-name sentinel).

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
| 2026-10-08 | Production `minimalEnv()` (internal/git) already strips GIT_CONFIG_GLOBAL; flakiness comes from test-helper git subprocesses that inherit env | TestMain `os.Setenv` isolation targets exactly the right surface |
| 2026-10-08 | `internal/rag/*` and `internal/review` runGit helpers append `GIT_CONFIG_GLOBAL=/dev/null` to child env; left untouched (isolation holds, test logic unchanged) | No action |
| 2026-10-08 | testutil coverage 79.5% (gate 72%); uncovered lines are RunIsolatedGit wrapper (needs real `*testing.M`) and write-error branch | Passes gate |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-10-08 | Authored | From CONTEXT.md tech-debt backlog |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
