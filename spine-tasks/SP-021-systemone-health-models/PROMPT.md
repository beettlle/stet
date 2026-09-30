# Task: SP-021 — System One client Health and Models

**Created:** 2026-09-30
**Size:** S

## Review Level: 1

**Assessment:** Two small GET methods reusing the SP-015 request helper; no new auth or retry logic.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 0, Security: 1, Reversibility: 0

## Mission

Add `Health` and `Models` to `cli/internal/systemone.Client` so `stet doctor` (SP-018) can confirm a System One server is reachable and serves the configured model.

**Background (System One / OpenJev):** OpenJev ([razorback16/openjev](https://github.com/razorback16/openjev)) uses the same wire API as hosted Jev. Endpoints used here:

- `GET /health` → `{"status":"ok"}`
- `GET /v1/models` → `{"models":[{"name":"openjev-latest","description":"Alias for the newest OpenJev release. Currently openjev-0.1.","release_date":"2026-09-18"},{"name":"openjev-0.1",...},{"name":"diffusiongemma-26b",...}]}`

SP-015 provides `NewClient`, sentinel errors (`ErrUnreachable`, `ErrBadRequest`, `ErrUnauthorized`, `ErrOverloaded`), and an unexported request helper handling auth, retries, size cap, and key redaction. Reuse that helper; do not duplicate it.

**API surface:**

```go
func (c *Client) Health(ctx context.Context) error
func (c *Client) Models(ctx context.Context) ([]string, error)
```

- `Health` returns nil only on 200 with `status == "ok"`; any other body is an error.
- `Models` returns model names in server order. Base URLs with or without a trailing `/v1` both work.

## Dependencies

- **Task:** SP-015 (client core and request helper)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `cli/internal/systemone/client.go` (SP-015)
- `AGENTS.md` — coverage gates

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none (tests use `httptest.Server`)

## File Scope

- `cli/internal/systemone/client.go`
- `cli/internal/systemone/client_test.go`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./internal/systemone/... -count=1 -cover` |
| fileScopeMustChange | `cli/internal/systemone/client.go`, `cli/internal/systemone/client_test.go` |
| fileScopeMustNotChange | `cli/internal/config/**`, `cli/internal/run/**`, `cli/internal/review/**`, `cli/cmd/**`, `extension/**` |
| completionCriteria | `Health` and `Models` implemented via the shared helper; tests pass with ≥72% file coverage |

## Steps

### Step 1: Implement Health and Models

- [ ] `Health` (`GET /health`) with `status == "ok"` check
- [ ] `Models` (`GET /v1/models`) returning names
- [ ] Both use the SP-015 request helper (auth, retries, error mapping)

### Step 2: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] `httptest.Server` tests: health ok, health wrong body, health 500, models list (real shape above), models 401 → `ErrUnauthorized`, unreachable → `ErrUnreachable`, base URL with and without `/v1`
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Per-file coverage ≥ 72% for `client.go`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing
- [ ] `Decide` behaviour unchanged

## Git Commit Convention

- **Implementation:** `feat(SP-021): description`
- **Tests:** `test(SP-021): description`

## Do NOT

- Change `Decide` or the request helper's behaviour
- Wire into `stet doctor` (SP-018)
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
