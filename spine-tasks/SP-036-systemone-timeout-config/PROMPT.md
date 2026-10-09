# Task: SP-036 — Configurable System One request timeout (`systemone_timeout`)

**Created:** 2026-10-09
**Size:** S

## Review Level: 1

**Assessment:** One new validated config key plus a client-construction helper used at three call sites; default behaviour unchanged.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 0, Security: 0, Reversibility: 1

## Mission

Every System One client in `cli/cmd/stet/main.go` is built with `systemone.NewClient(baseURL, apiKey, nil)`, which falls back to a hardcoded 10s `http.Client` timeout (`cli/internal/systemone/client.go`). A local OpenJev server takes 7.5–51s per request, so the critic fails with `context deadline exceeded` and stet's own spine contract has to run with `critic_enabled = false`. Make the timeout configurable.

**Behaviour:**

- **Config (`cli/internal/config/config.go`):** `SystemOneTimeout time.Duration`, TOML `systemone_timeout` / env `STET_SYSTEMONE_TIMEOUT`. Same duration syntax as `timeout` / `STET_TIMEOUT` (Go duration string or integer seconds, via the existing `parseDuration`). Default `10s` (current behaviour). Valid range `[1s, 30m]`; out of range or unparseable → error naming the key/env var, following `validateSystemOneSamples`. Precedence like the other `systemone_*` keys (defaults < file < env). No CLI flag.
- **CLI (`main.go`):** one helper, e.g. `newSystemOneClient(cfg *config.Config) *systemone.Client`, returning `systemone.NewClient(cfg.SystemOneBaseURL, cfg.SystemOneAPIKey, &http.Client{Timeout: cfg.SystemOneTimeout})`. Use it at all three call sites: `systemOneRunOptions`, `checkSystemOne` (doctor), and `runCriticEval`. No other `systemone.NewClient` calls may remain in `main.go`.
- **Docs (`docs/cli-extension-contract.md`):** add a `systemone_timeout` / `STET_SYSTEMONE_TIMEOUT` row next to `systemone_samples` (default `10s`, range, per-request timeout, applies to the critic, rules selector, rules annotator, doctor, and `critic-eval`).

**Stet-gated:** the contract ends with `scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record any stet reviewer bug under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Dependencies

- **Task:** SP-035 (same `main.go` System One code; serialize to avoid conflicts)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md` — "Stet in spine (dogfooding)" section

**Tier 3 (reference docs):**

- `cli/internal/config/config.go` — `systemone_samples` (struct field, file struct, env const, `validateSystemOneSamples`, env parsing) as the pattern; `parseDuration` and `STET_TIMEOUT` handling for durations
- `cli/internal/systemone/client.go` — `NewClient` (accepts a custom `*http.Client`; do not change it)
- `cli/cmd/stet/main.go` — `systemOneRunOptions` (~line 711), `checkSystemOne` (~line 1921), `runCriticEval` (~line 2005)

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** LM Studio at `http://127.0.0.1:1234` for the stet contract step only (unit tests use fakes)

## File Scope

- `cli/internal/config/config.go`
- `cli/internal/config/config_test.go`
- `cli/cmd/stet/main.go`
- `cli/cmd/stet/main_test.go`
- `docs/cli-extension-contract.md`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./internal/config/... ./internal/systemone/... ./cmd/stet/... -count=1 && go vet ./... && cd .. && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/config/config.go`, `cli/internal/config/config_test.go`, `cli/cmd/stet/main.go`, `docs/cli-extension-contract.md` |
| fileScopeMustNotChange | `extension/**`, `cli/internal/systemone/**`, `cli/go.mod`, `cli/go.sum`, `.review/**` |
| completionCriteria | Default stays 10s; TOML and env set the timeout with env winning; out-of-range and unparseable values error naming the key; all three call sites use the helper; a slow fake server succeeds with a raised timeout and fails with a low one; stet contract review exits 0; per-file coverage ≥ 72% for changed Go files |

## Steps

### Step 1: Config

- [ ] `SystemOneTimeout` field, TOML key, env var, default 10s, `[1s, 30m]` validation
- [ ] Table tests: default, TOML, env overrides TOML, integer seconds, `0s`/`31m`/garbage rejected (TOML and env)

### Step 2: CLI wiring and docs

- [ ] `newSystemOneClient` helper used by `systemOneRunOptions`, `checkSystemOne`, `runCriticEval`
- [ ] `docs/cli-extension-contract.md` row

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Helper test: a fake server that sleeps longer than a 1s timeout fails as unreachable (doctor exit 2); with the timeout raised above the sleep it succeeds. Keep sleeps short (≤ 2s total per test)
- [ ] Run Contract `testCommand` (includes the stet review; triage findings as described in Mission)
- [ ] Coverage gate: `cd cli && go test ./... -coverprofile=../coverage.out -count=1 && cd .. && bash scripts/check-coverage.sh coverage.out` (then `git checkout -- coverage.out`)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Implementation:** `feat(SP-036): description`
- **Tests:** `test(SP-036): description`
- **Docs:** `docs(SP-036): description`

## Do NOT

- Change `cli/internal/systemone/**` (the client already accepts an `*http.Client`)
- Change the default timeout or add a CLI flag
- Edit `.review/config.toml` (the operator re-enables the critic after this lands)
- Print or log the API key
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
