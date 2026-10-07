# Task: SP-025 — stet critic-eval command

**Created:** 2026-09-30
**Size:** S

## Review Level: 1

**Assessment:** New read-only CLI subcommand over existing packages; sends past code to the configured System One endpoint.
**Score:** 3/8 — Blast radius: 1, Pattern novelty: 1, Security: 1, Reversibility: 0

## Mission

Add `stet critic-eval`, which tells users what `systemone_min_keep` value to use by scoring their own past findings with the System One critic.

**Background (System One / OpenJev):** stet's optional System One critic (OpenJev, [razorback16/openjev](https://github.com/razorback16/openjev); same wire API as hosted Jev) keeps a finding when P(yes) ≥ `systemone_min_keep`. Prerequisites:

- SP-014: config keys `systemone_base_url`, `systemone_model`, `systemone_samples`, env-only `SystemOneAPIKey`.
- SP-015: `systemone.NewClient(baseURL, apiKey, httpClient)`.
- SP-019: `criticeval.LoadFromState(stateDir)`, `criticeval.LoadDataset(path)`.
- SP-024: `criticeval.Score(...)`, `criticeval.Evaluate(...) Report`.

**Command behaviour:**

- `stet critic-eval [--dataset path.jsonl] [--json]`, registered like `newBenchmarkCmd` in `cli/cmd/stet/main.go`.
- Loads config (same as other commands), resolves the state dir, loads the dataset (`--dataset` replaces history/session), scores, evaluates.
- Works whether or not `critic_enabled` is set (it is an evaluation tool), but always uses the `systemone_*` keys.
- **Human output:** counts per label and source; a table with columns `threshold`, `false positives dropped`, `real findings lost`, `kept`; p50/p95 latency; the recommendation (or "no threshold keeps real-finding loss ≤ 10%"); any warnings.
- **`--json`:** the `Report` as JSON on stdout.
- No labeled examples → clear error explaining how to create them (dismiss findings with `false_positive` / `wrong_suggestion`, or pass `--dataset`), exit 1.
- System One unreachable → message with the base URL, exit 2 (matches `stet doctor`). Never print the API key.
- Read-only: never writes to `.review/`.

## Dependencies

- **Task:** SP-024 (scoring and sweep; covers the dataset loader and verifier)
- **Task:** SP-018 (shares `cli/cmd/stet/main.go`; covers config, client, and flag wiring)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `cli/cmd/stet/main.go` — `newBenchmarkCmd`, `runDoctor`, `errExit`, config loading
- `cli/cmd/stet/main_test.go` — `runCLI` test helper patterns
- `cli/internal/criticeval/dataset.go`, `cli/internal/criticeval/sweep.go` (SP-019, SP-024)
- `AGENTS.md` — coverage gates

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none for tests (`httptest.Server`); a real OpenJev at `http://127.0.0.1:8080` for manual runs

## File Scope

- `cli/cmd/stet/main.go`
- `cli/cmd/stet/main_test.go`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./cmd/stet/... -count=1 -cover` |
| fileScopeMustChange | `cli/cmd/stet/main.go`, `cli/cmd/stet/main_test.go` |
| fileScopeMustNotChange | `cli/internal/**`, `extension/**` |
| completionCriteria | `stet critic-eval` prints human or JSON report with recommendation; exit 1 on no data, 2 on unreachable; read-only; tests pass with ≥72% coverage on `main.go` |

## Steps

### Step 1: Implement command

- [ ] Register `critic-eval` with `--dataset` and `--json`
- [ ] Wire config → client → dataset → score → evaluate
- [ ] Human table, JSON output, error messages and exit codes

### Step 2: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] `TestRunCLI_criticEval*`: curated dataset against `httptest.Server` System One (human and `--json`, JSON parses to the report shape); empty state dir → exit 1; unreachable → exit 2; output never contains a test API key
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Per-file coverage ≥ 72% for `main.go`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing
- [ ] Command never writes to `.review/`

## Git Commit Convention

- **Implementation:** `feat(SP-025): description`
- **Tests:** `test(SP-025): description`

## Do NOT

- Change `criticeval` package logic (SP-019, SP-024)
- Change critic defaults or config automatically based on results
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
