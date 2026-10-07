# Task: SP-023 — --critic-backend flag, client wiring, and docs

**Created:** 2026-09-30
**Size:** S

## Review Level: 2

**Assessment:** Makes the System One critic user-visible: new CLI flag, client built from config with an env-only API key, and user docs.
**Score:** 4/8 — Blast radius: 1, Pattern novelty: 1, Security: 1, Reversibility: 1

## Mission

Let users turn on the System One critic from the CLI and config: `critic_enabled = true` (or `--verify`) plus `critic_backend = "systemone"` (or `--critic-backend systemone`) on `stet start`, `stet run`, and `stet rerun`.

**Background (System One / OpenJev):** OpenJev ([razorback16/openjev](https://github.com/razorback16/openjev)) exposes `POST /v1/systemone`; a `noul` question returns P(yes). Same wire API as hosted Jev, so `systemone_base_url` may point at a local server (default `http://127.0.0.1:8080`) or a hosted one with `STET_SYSTEMONE_API_KEY`. Warm latency ~100 ms per hunk.

**Prerequisites delivered by earlier tasks:**

- SP-014: config keys `critic_backend`, `systemone_base_url`, `systemone_model`, `systemone_min_keep`, `systemone_samples`, env-only `SystemOneAPIKey`, and `config.Overrides.CriticBackend`.
- SP-015: `systemone.NewClient(baseURL, apiKey, httpClient)`.
- SP-022: `run.StartOptions` / `run.RunOptions` fields `CriticBackend`, `SystemOneClient`, `SystemOneBaseURL`, `SystemOneModel`, `SystemOneMinKeep`, `SystemOneSamples`.

**Required changes:**

1. **Flag (`cli/cmd/stet/main.go`):** add `--critic-backend` (string; `llm` or `systemone`) next to `--verify` in `newStartCmd` and `addRunLikeFlags`. Wire it into `config.Overrides.CriticBackend` following the existing `verifyChanged` pattern (only set when the flag changed; include it in the "any flag changed" check).
2. **Client wiring:** at every place that sets `CriticEnabled: cfg.CriticEnabled` on `run.StartOptions` / `run.RunOptions` (start, run, rerun), also set the System One fields. Build `systemone.NewClient(cfg.SystemOneBaseURL, cfg.SystemOneAPIKey, nil)` only when `cfg.CriticEnabled && cfg.CriticBackend == "systemone"`; otherwise leave `SystemOneClient` nil. Extract a small helper if the three call sites would otherwise duplicate the logic.
3. **Docs:**
   - `docs/cli-extension-contract.md`: add the five config keys, their env vars, defaults, and validation; `STET_SYSTEMONE_API_KEY` (env only, never in config files); the `--critic-backend` flag.
   - `docs/review-process-internals.md` §7.10 critic bullet: describe the `systemone` backend (one request per hunk, keep when P(yes) ≥ `systemone_min_keep`, errors abort the run, off by default).
   - `README.md`: one sentence near the existing `--verify`/critic or configuration overview pointing to the contract doc.

## Dependencies

- **Task:** SP-014 (config keys and override)
- **Task:** SP-022 (run options; covers the client, verifier, and `applyCritic` chain)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `cli/cmd/stet/main.go` — `newStartCmd`, `addRunLikeFlags`, `verifyChanged` override handling, `CriticEnabled: cfg.CriticEnabled` call sites
- `cli/cmd/stet/main_test.go` — `--verify` override test pattern
- `cli/internal/config/config.go` (SP-014 keys)
- `docs/cli-extension-contract.md`, `docs/review-process-internals.md` §7.10, `README.md`
- `AGENTS.md` — coverage gates

## Environment

- **Workspace:** `cli/` Go module and `docs/`
- **Services required:** none (tests use `httptest.Server` or `--dry-run`)

## File Scope

- `cli/cmd/stet/main.go`
- `cli/cmd/stet/main_test.go`
- `docs/cli-extension-contract.md`
- `docs/review-process-internals.md`
- `README.md`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./cmd/stet/... -count=1 -cover` |
| fileScopeMustChange | `cli/cmd/stet/main.go`, `cli/cmd/stet/main_test.go`, `docs/cli-extension-contract.md` |
| fileScopeMustNotChange | `cli/internal/**`, `extension/**` |
| completionCriteria | `--critic-backend` on start/run/rerun sets the override; System One client built from config when enabled; docs describe keys, env, flag; tests pass with ≥72% coverage on `main.go` |

## Steps

### Step 1: Flag and client wiring

- [ ] `--critic-backend` on start/run/rerun wired to `Overrides.CriticBackend`
- [ ] System One fields and client set at every `CriticEnabled` call site (helper if needed)

### Step 2: Docs

- [ ] `docs/cli-extension-contract.md` keys, env, flag
- [ ] `docs/review-process-internals.md` §7.10
- [ ] `README.md` one-line pointer

### Step 3: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] `main_test.go`: `--critic-backend systemone` sets the override; invalid value returns an error; client is nil when critic disabled or backend `llm`
- [ ] End-to-end: `stet start` with `--verify --critic-backend systemone` against an `httptest.Server` System One and a mock LLM drops a low-scored finding
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Per-file coverage ≥ 72% for `main.go`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing
- [ ] Default behaviour unchanged when the flag and keys are unset

## Git Commit Convention

- **Implementation:** `feat(SP-023): description`
- **Tests:** `test(SP-023): description`
- **Docs:** `docs(SP-023): description`

## Do NOT

- Change `stet doctor` (SP-018)
- Add an evaluation command (SP-025)
- Print or log the API key
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
