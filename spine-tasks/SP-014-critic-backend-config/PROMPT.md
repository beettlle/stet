# Task: SP-014 — Critic backend and System One config keys

**Created:** 2026-09-30
**Size:** S

## Review Level: 1

**Assessment:** Additive config keys with validation; one secret read from env only.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 0, Security: 1, Reversibility: 0

## Mission

stet has an optional critic (`critic_enabled`, `--verify`) that re-checks each finding with a second LLM call. This task adds the **configuration** needed to select an alternative critic backend: a System One decision server.

**Background (System One / OpenJev):** OpenJev ([razorback16/openjev](https://github.com/razorback16/openjev)) is a self-hosted decision server using the same wire API as TypeSafe's hosted Jev. `POST /v1/systemone` takes `{state, model, questions}`; a `noul` (yes/no) question returns `{noul: P(yes)}` read from model probabilities (no text generation). It also exposes `GET /health` and `GET /v1/models`. Verified local response shape: `{"model":"openjev-0.1","answers":{"id":{"type":"noul","noul":0.9997}},"usage":{"input_tokens":246,"output_tokens":0}}`. Warm latency is about 100 ms per request.

Add these keys to `cli/internal/config/config.go` (struct field, default, TOML file key, env var, validation). Precedence follows the existing order: CLI overrides > env > repo config > global config > defaults.

| Key | Env | Default | Validation |
|-----|-----|---------|------------|
| `critic_backend` | `STET_CRITIC_BACKEND` | `llm` | one of `llm`, `systemone` (case-insensitive, stored lowercase) |
| `systemone_base_url` | `STET_SYSTEMONE_BASE_URL` | `http://127.0.0.1:8080` | parseable URL with `http` or `https` scheme and non-empty host |
| `systemone_model` | `STET_SYSTEMONE_MODEL` | `openjev-latest` | non-empty |
| `systemone_min_keep` | `STET_SYSTEMONE_MIN_KEEP` | `0.5` | float in [0, 1] |
| `systemone_samples` | `STET_SYSTEMONE_SAMPLES` | `1` | int in [1, 32] |

**API key:** `SystemOneAPIKey` is read **only** from env `STET_SYSTEMONE_API_KEY`. It must not have a `toml` tag, must not be loadable from any config file, and must never appear in error messages, trace output, or `String()`-style dumps.

Add `CriticBackend *string` to `Overrides` so a later task (SP-017) can wire a `--critic-backend` flag; `applyOverrides` must validate it the same way as env/TOML.

## Dependencies

- **None**

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `cli/internal/config/config.go` — `Config`, `Overrides`, `DefaultConfig`, `mergeFile`, `applyEnv`, `applyOverrides`, `validateStrictness` (validation pattern), existing `CriticEnabled` / `CriticModel` keys
- `cli/internal/config/config_test.go` — existing env/TOML/override test patterns
- `AGENTS.md` — coverage gates

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none

## File Scope

- `cli/internal/config/config.go`
- `cli/internal/config/config_test.go`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./internal/config/... -count=1 -cover` |
| fileScopeMustChange | `cli/internal/config/config.go`, `cli/internal/config/config_test.go` |
| fileScopeMustNotChange | `cli/internal/run/**`, `cli/cmd/**`, `extension/**` |
| completionCriteria | Five keys load from defaults/TOML/env/overrides with validation; API key env-only and never printed; config tests pass with ≥72% file coverage |

## Steps

### Step 1: Add fields, defaults, and TOML keys

- [ ] Add `CriticBackend`, `SystemOneBaseURL`, `SystemOneModel`, `SystemOneMinKeep`, `SystemOneSamples` to `Config` with `toml` tags and doc comments
- [ ] Add `SystemOneAPIKey` to `Config` with **no** `toml` tag (`toml:"-"`)
- [ ] Set defaults in `DefaultConfig`; load from repo/global TOML in `mergeFile`
- [ ] Add `CriticBackend *string` to `Overrides`

### Step 2: Env vars, overrides, and validation

- [ ] Parse the five `STET_*` env vars plus `STET_SYSTEMONE_API_KEY` in `applyEnv`
- [ ] Apply `CriticBackend` in `applyOverrides`
- [ ] Validate backend enum, URL scheme/host, min_keep range, samples range; invalid values return an error naming the key (never the API key value)

### Step 3: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Table tests: defaults, TOML, env, override precedence, each invalid value
- [ ] Test that a TOML file containing `systemone_api_key` does **not** set the key
- [ ] Test that validation errors never contain the API key value
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Per-file coverage ≥ 72% for `config.go`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing
- [ ] No behaviour change when the new keys are unset (`critic_backend` defaults to `llm`)

## Git Commit Convention

- **Implementation:** `feat(SP-014): description`
- **Tests:** `test(SP-014): description`

## Do NOT

- Wire the keys into `run.go` or `main.go` (SP-017)
- Add a `--critic-backend` flag (SP-017)
- Read the API key from any file
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
