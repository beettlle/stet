# Task: SP-018 — stet doctor System One check

**Created:** 2026-09-30
**Size:** S

## Review Level: 1

**Assessment:** Small additive check in an existing diagnostic command; reuses the SP-015 client.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 0

## Mission

Extend `stet doctor` so users who enabled the System One critic find out before a review whether the server is reachable and the configured model exists.

**Background (System One / OpenJev):** OpenJev ([razorback16/openjev](https://github.com/razorback16/openjev)) exposes `GET /health` (`{"status":"ok"}`) and `GET /v1/models` (`{"models":[{"name":"openjev-latest",...},{"name":"openjev-0.1",...}]}`), plus `POST /v1/systemone` for decisions. Same wire API as hosted Jev. SP-015 provides `systemone.NewClient` and sentinel errors; SP-021 adds `Health(ctx)` and `Models(ctx) ([]string, error)`; errors include (`ErrUnreachable`, `ErrUnauthorized`, ...).

**Behaviour** in `runDoctor` (`cli/cmd/stet/main.go`), after the existing LLM checks succeed, and only when `cfg.CriticEnabled && cfg.CriticBackend == "systemone"`:

- Build `systemone.NewClient(cfg.SystemOneBaseURL, cfg.SystemOneAPIKey, nil)`.
- `Health` fails with `ErrUnreachable` → print `System One server unreachable at <base_url>` to stderr, exit **2** (same code as the existing LLM-unreachable path).
- `Health`/`Models` fails with `ErrUnauthorized` → print a hint to set `STET_SYSTEMONE_API_KEY`, exit **2**.
- Model not in `Models` → print `System One model "<model>" not found at <base_url>` and the available names, exit **1**.
- Success → print `System One OK` and `System One model: <model>` to stdout.
- When the critic is disabled or backend is `llm`, output is unchanged.
- Never print the API key.

## Dependencies

- **Task:** SP-021 (`systemone.Client.Health` / `Models`)
- **Task:** SP-023 (shares `cli/cmd/stet/main.go`; also provides the wired config)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `cli/cmd/stet/main.go` — `newDoctorCmd`, `runDoctor`, `errExit`
- `cli/cmd/stet/main_test.go` — `TestRunCLI_doctor*` patterns
- `cli/internal/systemone/client.go` (SP-015, SP-021)
- `AGENTS.md` — coverage gates

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none (tests use `httptest.Server`)

## File Scope

- `cli/cmd/stet/main.go`
- `cli/cmd/stet/main_test.go`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./cmd/stet/... -count=1 -cover -run doctor` |
| fileScopeMustChange | `cli/cmd/stet/main.go`, `cli/cmd/stet/main_test.go` |
| fileScopeMustNotChange | `cli/internal/**`, `extension/**` |
| completionCriteria | Doctor reports System One OK / unreachable (2) / unauthorized (2) / model missing (1) when enabled; unchanged otherwise; tests pass |

## Steps

### Step 1: Implement doctor check

- [ ] Add System One check to `runDoctor` per Mission (extract a small helper if `runDoctor` grows past readability)
- [ ] Map sentinel errors to messages and exit codes

### Step 2: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] `TestRunCLI_doctorSystemOne*` tests with `httptest.Server` (for both Ollama check and System One): OK, unreachable, 401, model missing, critic disabled (no System One request made)
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Per-file coverage ≥ 72% for `main.go`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing
- [ ] Existing doctor tests unchanged

## Git Commit Convention

- **Implementation:** `feat(SP-018): description`
- **Tests:** `test(SP-018): description`

## Do NOT

- Send a `POST /v1/systemone` request from doctor
- Change existing doctor exit codes or messages
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
