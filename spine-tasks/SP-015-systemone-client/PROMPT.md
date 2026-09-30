# Task: SP-015 — System One HTTP client core (Decide)

**Created:** 2026-09-30
**Size:** S

## Review Level: 2

**Assessment:** New outbound HTTP client carrying an optional bearer token; must redact secrets and bound retries and response sizes.
**Score:** 4/8 — Blast radius: 1, Pattern novelty: 1, Security: 2, Reversibility: 0

## Mission

Create `cli/internal/systemone`, a small Go client for System One decision servers, with the `Decide` call stet needs to verify findings. `Health`/`Models` for `stet doctor` are a separate task (SP-021).

**Background (System One / OpenJev):** OpenJev ([razorback16/openjev](https://github.com/razorback16/openjev)) is a self-hosted decision server using the same wire API as TypeSafe's hosted Jev (so the same client works against `https://api.codiv.ai` or hosted Jev with an API key).

- `POST /v1/systemone` — body `{"model": "...", "state": "...", "questions": {"<id>": {"type": "noul", "instructions": "...", "criteria": {"true": "...", "false": "..."}}}, "samples": 1}`; response `{"model":"openjev-0.1","answers":{"<id>":{"type":"noul","noul":0.9997}},"usage":{"input_tokens":246,"output_tokens":0}}`. `noul` is P(yes).
- Errors: `400` plain-text reason or `api_usage_error`; `422` FastAPI validation list; `401`/`403` `{"detail":{"error_type","message"}}`; `429` rate limit; `529` overloaded.
- Warm latency is about 100 ms per request; a cold first request took ~3.7 s.

Use only the Go standard library (`net/http`, `encoding/json`); there is no Go SDK and none is needed. Follow the conventions of `cli/internal/openaicompat/client.go`: `NewClient(baseURL, ..., httpClient *http.Client)` with a default timeout when `httpClient` is nil, a bounded retry loop with capped exponential backoff, a response-size cap via `io.LimitReader`, and truncated HTTP error bodies.

**API surface (this task):**

```go
func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client
func (c *Client) Decide(ctx context.Context, req Request) (*Response, error)
```

- `Request` has `Model`, `State`, `Questions map[string]NoulQuestion`, `Samples int` (sent explicitly when > 0). Only the `noul` question type is modelled (YAGNI).
- `Response` has `Model`, `Answers map[string]NoulAnswer{Noul float64}`, `Usage{InputTokens, OutputTokens}`.
- Sentinel errors: `ErrUnreachable` (dial/timeout), `ErrBadRequest` (400, 422), `ErrUnauthorized` (401, 403), `ErrOverloaded` (429, 529 after retries exhausted). Wrap with `%w` and include status code.
- Put the request/retry/error-mapping logic in an unexported helper (e.g. `do(ctx, method, path, body, out)`) so SP-021 can reuse it for GET endpoints.
- `Authorization: Bearer <key>` only when `apiKey != ""`. The key must never appear in any error string, including wrapped `url.Error` values.
- Validate the response: every requested question ID has an answer, and each `noul` is within [0, 1]; otherwise return an error.

## Dependencies

- **None**

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `cli/internal/openaicompat/client.go` — client, retry, and error-body conventions to mirror
- `cli/internal/openaicompat/client_test.go` — `httptest.Server` test patterns
- `AGENTS.md` — coverage gates

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none (tests use `httptest.Server`; a real OpenJev on `http://127.0.0.1:8080` is optional for manual smoke tests only)

## File Scope

- `cli/internal/systemone/client.go` (new)
- `cli/internal/systemone/client_test.go` (new)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./internal/systemone/... -count=1 -cover` |
| fileScopeMustChange | `cli/internal/systemone/client.go`, `cli/internal/systemone/client_test.go` |
| fileScopeMustNotChange | `cli/internal/config/**`, `cli/internal/run/**`, `cli/cmd/**`, `extension/**` |
| completionCriteria | `Decide` implemented with sentinel errors, bounded retries, bearer-token redaction, response validation; tests pass with ≥72% file coverage |

## Steps

### Step 1: Types, shared request helper, Decide

- [ ] Define `Request`, `NoulQuestion`, `NoulCriteria`, `Response`, `NoulAnswer`, `Usage`
- [ ] Unexported request helper: bearer header when key set, status → sentinel error mapping, truncated error bodies, size cap, bounded retry with capped backoff for `429`/`529`/transient network errors, `ctx` honoured between attempts, key redaction
- [ ] `Decide`: JSON POST to `/v1/systemone` (accept base URLs with or without trailing `/v1`) plus response validation

### Step 2: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] `httptest.Server` tests using the real response shape above: success, missing answer, out-of-range `noul`, 400, 401, 422, 429-then-200, 529 exhausted, oversized body, cancelled context, unreachable host
- [ ] Assert the `Authorization` header is present only when a key is set; assert no error string contains the key
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Per-file coverage ≥ 72% for `client.go`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing
- [ ] No new third-party dependencies in `cli/go.mod`

## Git Commit Convention

- **Implementation:** `feat(SP-015): description`
- **Tests:** `test(SP-015): description`

## Do NOT

- Add `Health` or `Models` (SP-021)
- Add `choice` or `score` question types
- Import a Jev/TypeSafe SDK or any new module
- Log request bodies (they contain source code) or the API key
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
