# Post-integrate regression gate

Operator-enforced full check on `main` after every land loop. Spine does **not** run it: contract verify runs only each task's scoped `testCommand` inside its lane, so it cannot see merged-tree breakage, coverage drops, or cross-package regressions.

## When required (blocking)

Run on current `main` after:

1. A standard land loop: `spine gate approve` → `spine integrate` → `spine batch complete`
2. Any manual integrate: `git merge orch/*`, conflict resolution, `spine batch complete --detect-manual-merge`
3. Any fix commit made on `main` during the release
4. Before every `git push origin main` during the release
5. Before the Phase 5 publish gate (re-run if `main` moved)

## Command

```bash
cd "$(git rev-parse --show-toplevel)"
set -o pipefail
scripts/release-check.sh 2>&1 | tee /tmp/stet-release-check-wave-${WAVE:-main}.log
echo "release-check exit: ${PIPESTATUS[0]}"
```

`scripts/release-check.sh` runs, in order: gofmt, `go vet ./cli/...`, `go test ./cli/...` with the coverage thresholds (77% project, 72% per file), extension `npm ci && npm test`, the coverage-artifact guard, and a cross-compile of all six release targets. It prints `PASS: <step>` per step and `FAIL: <step>` on the first failure.

## Reading a failure

Do not load the whole log (the Go and Vitest output is long):

```bash
rg -n '^(FAIL|PASS):|^--- FAIL|^FAIL\s' /tmp/stet-release-check-wave-${WAVE:-main}.log
rg -n -A 15 '^--- FAIL' /tmp/stet-release-check-wave-${WAVE:-main}.log    # context for failing Go tests
```

Coverage failures print `FAIL: project coverage X% < 77%` or `FAIL: <file> coverage X% < 72%`; add tests in the task's package rather than lowering thresholds.

**If exit is non-zero:** STOP. Fix on `main`, commit, re-run the gate. Do not start the next wave, push, or tag.

## Environment-sensitive tests

Go tests create git repositories under the system temp directory. Inside a sandboxed agent shell they can fail with `.git/hooks/: Operation not permitted`; that is a sandbox artifact, not a regression. Re-run the gate outside the sandbox before concluding anything.

Tests run from the repo, so the committed `.review/config.toml` is visible to them. A test that assumes a config default must pin it with an env var (see `boolEnvOrOff` in `cli/cmd/stet/main_test.go`).

## Verification discipline

| Do | Do not |
|----|--------|
| Check `${PIPESTATUS[0]}` (or run without a pipe and check `$?`) | Judge pass/fail from `tail` output — `tail` exits 0 when the gate failed |
| Record the log path and exit code in the manifest wave log | Treat a passing contract verify as proof the merged tree is green |
| Re-run after every fix commit on `main` | Lower coverage thresholds or skip steps to get green |
