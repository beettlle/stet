# Task: SP-030 — Isolate git-heavy tests from host git hooks and daemons

**Created:** 2026-10-08
**Size:** M

## Review Level: 1

**Assessment:** Test-only change across many packages; new shared test helper package. No production behaviour change.
**Score:** 3/8 — Blast radius: 2, Pattern novelty: 1, Security: 0, Reversibility: 0

## Mission

Eliminate the intermittent `TempDir RemoveAll cleanup: unlinkat …/.git: directory not empty` failures in git-heavy Go tests so `cd cli && go test ./... -count=1` is deterministic on developer machines.

**Root cause (diagnosed 2026-10-08):** the developer's global git config sets `trace2.eventtarget=af_unix:…/.git-ai/internal/daemon/trace2.sock`. A background `git-ai` daemon receives trace2 events from every `git` command and, shortly after commits, asynchronously writes `.git/ai/working_logs/…`, `refs/notes/ai`, and `logs/refs/notes/ai` into that repository. Tests create repos with `t.TempDir()`; when the daemon writes during `t.TempDir` cleanup, `os.RemoveAll` fails and the test is marked failed even though every assertion passed. Observed in `internal/run`, `internal/stats`, `internal/scope`, `internal/git` (and potentially every package that runs `git init` in tests). Reproduction: `cd cli && for i in 1 2 3 4 5; do go test ./internal/run/ ./internal/stats/ ./internal/git/ -count=1 2>&1 | tail -1; done` fails roughly half the runs. Setting `GIT_CONFIG_GLOBAL=/dev/null` reduces but does not fully eliminate failures, so both mitigations below are required.

**Required fix (both parts):**

1. **Git environment isolation per package.** New package `cli/internal/testutil` with `RunIsolatedGit(m *testing.M) int`: writes a temporary global gitconfig containing only `[init] defaultBranch = main` (tests may rely on `main`), sets `GIT_CONFIG_GLOBAL` to it and `GIT_CONFIG_NOSYSTEM=1`, runs `m.Run()`, removes the temp file, and returns the exit code. Each git-heavy test package gets `func TestMain(m *testing.M) { os.Exit(testutil.RunIsolatedGit(m)) }`. Use `TestMain` + `os.Setenv`, **not** `t.Setenv` — `t.Setenv` panics in tests that call `t.Parallel()` (`internal/run` has 57).
2. **Robust cleanup.** `testutil.TempGitDir(t testing.TB) string`: creates the directory with `os.MkdirTemp` and registers `t.Cleanup` that retries `os.RemoveAll` a bounded number of times (named constants, e.g. 10 attempts with 50 ms linear backoff) and reports `t.Errorf` only if the final attempt fails. Replace `t.TempDir()` with `testutil.TempGitDir(t)` wherever the directory becomes (or contains) a git repository.

**Packages in scope** (every `_test.go` that runs git against a `t.TempDir()`): `cmd/stet`, `internal/diff`, `internal/git`, `internal/rag/{go,java,js,python,rust,swift}`, `internal/review`, `internal/run`, `internal/scope`, `internal/stats`.

## Dependencies

- **None**

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md` — tech-debt row "Flaky git-subprocess tests"

**Tier 3 (reference docs):**

- `cli/internal/run/run_test.go` — `initRepo` / `runGit` helpers (typical pattern)
- `AGENTS.md` — coverage gates (77% project, 72% per file)

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none

## File Scope

- `cli/internal/testutil/gitenv.go` (new)
- `cli/internal/testutil/gitenv_test.go` (new)
- `cli/cmd/stet/main_test.go`
- `cli/internal/diff/*_test.go`
- `cli/internal/git/*_test.go`
- `cli/internal/rag/**/*_test.go`
- `cli/internal/review/review_test.go`
- `cli/internal/run/*_test.go`
- `cli/internal/scope/*_test.go`
- `cli/internal/stats/*_test.go`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go vet ./... && go test ./internal/run/... ./internal/stats/... ./internal/git/... ./internal/scope/... ./internal/testutil/... -count=3` |
| fileScopeMustChange | `cli/internal/testutil/gitenv.go`, `cli/internal/testutil/gitenv_test.go`, `cli/internal/run/run_test.go`, `cli/internal/stats/stats_test.go` |
| fileScopeMustNotChange | `cli/internal/rules/**`, `cli/internal/systemone/**`, `extension/**` |
| completionCriteria | No production (non-`_test.go`) file changes outside `cli/internal/testutil/`; every in-scope package has the `TestMain`; git repos in tests use `TempGitDir`; 5 consecutive `go test ./... -count=1` runs pass; `testutil` coverage ≥ 72% |

## Steps

### Step 1: `testutil` helpers

- [ ] `RunIsolatedGit(m)` and `TempGitDir(t)` with named retry constants and doc comments explaining the git-ai/trace2 race
- [ ] Unit tests: env vars set during `m.Run` equivalent (exercise via an internal helper that takes a run func), gitconfig content, retry succeeds after a transient failure (inject a remove func), final failure reported via a fake `testing.TB`

### Step 2: Apply to git-heavy packages

- [ ] Add `TestMain` to each in-scope package (one per package; create a small `main_test.go`-style file only if the package has no single obvious test file — keep it within the File Scope globs)
- [ ] Replace `t.TempDir()` with `testutil.TempGitDir(t)` for directories that hold git repos
- [ ] Do not change assertions or test logic

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Run Contract `testCommand`
- [ ] Run `cd cli && go test ./... -count=1` five times in a row; all five pass (record results in STATUS)
- [ ] Run `cd cli && go test ./... -coverprofile=../coverage.out -count=1 && cd .. && bash scripts/check-coverage.sh coverage.out` (then `git checkout -- coverage.out`)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing, 5/5 consecutive full-suite runs green

## Git Commit Convention

- **Helpers:** `test(SP-030): description`
- **Package updates:** `test(SP-030): description`

## Do NOT

- Change production code outside `cli/internal/testutil/`
- Use `t.Setenv` in packages with parallel tests
- Disable `t.Parallel()` or add `-p 1` as the fix
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`, or the developer's real `~/.gitconfig`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
