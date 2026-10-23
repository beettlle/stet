# SP-061: `stet benchmark` loads the repo `.review/config.toml` — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-22
**Review Level:** 2
**Review Counter:** 2 (plan APPROVE; code review REVISE — resolved by dismissing false positive aaee18d, contract re-run exit 0)
**Iteration:** 1
**Size:** S

---

### Step 1: Resolve repo root in runBenchmark
**Status:** ✅ Complete

- [x] Repo root resolved like `runDoctor` and passed to `config.Load`

`runBenchmark` now resolves the repo root exactly like `runDoctor`: `os.Getwd` (with the same
`erruser.New("Could not determine current directory.", err)` on failure), then `git.RepoRoot(cwd)`
with fallback to `""` outside a git repo. Nothing else in `runBenchmark` changed.
Commit: 04b3092 `feat(SP-061): complete Step 1 — resolve repo root in runBenchmark`.

---

### Step 2: Testing & Verification
**Status:** ✅ Complete

- [x] Temp-repo config test with mock OpenAI server
- [x] Contract `testCommand` passing (stet review exits 0)

New tests in `cli/cmd/stet/main_test.go`:

- `TestRunCLI_benchmarkUsesRepoConfig` — temp git repo with `.review/config.toml`
  (`provider = "openai"`, `openai_base_url` → httptest server, `model = "repo-config-model"`);
  `STET_PROVIDER` / `STET_OPENAI_BASE_URL` / `STET_MODEL` cleared with `t.Setenv` (empty values are
  ignored by `applyEnv`); `XDG_CONFIG_HOME` pointed at a fresh temp dir so no host global config
  leaks in (the CLI `config.Load` call has no `GlobalConfigPath` override). Asserts exit 0 and that
  every `/v1/chat/completions` request carries the repo-configured model. Verified it FAILS without
  the Step 1 fix (falls back to default Ollama URL, exits 2) and passes with it. Not parallel
  (changes cwd).
- `TestRunCLI_benchmarkOutsideGitRepoStillLoadsConfig` — plain temp dir (not a git repo): env-config
  ollama mock, benchmark exits 0, proving the `repoRoot=""` fallback preserves existing behavior.
  Not parallel (changes cwd).

Verification evidence:

- `bash scripts/check-gofmt.sh` — clean
- `go vet ./cli/...` — clean
- `go test ./cli/cmd/stet/... -count=1` — ok (cli/cmd/stet ~65-107s)
- `bash scripts/spine-stet-contract-run.sh default` — 0 findings, exit 0 (after triage, see below)

---

## Revision 1 (code review REVISE, resolved)

Code review (post-Step-2) returned REVISE: stet finding `aaee18d` was active, making the contract
gate exit non-zero. The finding claims the new cwd-changing tests can interleave with parallel
tests and must call `t.Parallel()`. Dismissed as **false_positive** — Go's `testing.runTests` runs
sequential top-level tests to completion before resuming parallel tests, so `os.Chdir` in a
non-parallel test never interleaves with a parallel one; PROMPT Step 2 explicitly forbids parallel
here ("Do not run the test in parallel (it changes cwd)"), and the tests match the file's
established `os.Chdir` pattern. Re-ran the full Contract `testCommand` after dismissal: gofmt/vet
clean, `go test ./cli/cmd/stet/... -count=1` ok (64.9s), `spine-stet-contract-run.sh default`
0 findings, exit 0. No code changes required.

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|
| 1 | plan | 1 | APPROVE | `.reviews/1-20261023T000806.md` |
| 2 | code | 2 | REVISE → resolved (dismissed false positive aaee18d; contract re-run exit 0) | `.reviews/2-20261023T000826.md` |

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| Stet reviewer: critic claims `console` import/member tests "will fail" if built-in globals are filtered — but filtering built-in globals even when import-bound or used as member receivers is SP-059's specified behavior (resolver.go:76 comment) and both tests pass | Dismissed eb892f3, 5740146 as false_positive (counterfactual failure predictions; 5740146 misreads `if set[n] { t.Errorf }` as wanting presence) | contract run 1 |
| Stet reviewer: flags discarded `git.RepoRoot` error in `runBenchmark` | Dismissed 9bd4837 as already_correct — the discard is the task contract: mirror `runDoctor`'s fallback to global-only config outside a git repo so benchmark works anywhere; warning would change benchmark output (out of scope) | contract run 1 |
| Stet reviewer: claims cwd-changing tests must call `t.Parallel()` or risk interleaving with parallel tests | Dismissed aaee18d as false_positive — sequential top-level tests complete before parallel tests resume in Go, so no interleaving is possible; PROMPT forbids parallel for these tests; matches existing `os.Chdir` pattern | code review (revision 1) |
| Existing benchmark tests all set `STET_PROVIDER` + base URL/model env vars, so env overrides keep them green now that benchmark reads the repo's own `.review/config.toml` (openai/LM Studio) during tests run from inside the stet worktree | No action needed — noted for future benchmark tests | contract run 1 |

---

## Completion Criteria

- [x] All steps complete
- [x] All tests passing (full `go test ./cli/cmd/stet/... -count=1` ok; contract stet review exit 0)
