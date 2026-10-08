# Task: SP-032 — System One client choice questions

**Created:** 2026-10-08
**Size:** S

## Review Level: 1

**Assessment:** Additive API in one package, reusing the existing transport, retry, and key-redaction code.
**Score:** 2/8 — Blast radius: 0, Pattern novelty: 1, Security: 1, Reversibility: 0

## Mission

Add `choice` question support to `cli/internal/systemone` so later tasks (SP-026 rule selector, SP-027 rule-conflict annotation) can ask one multiple-choice question per hunk instead of one yes/no question per option.

**Wire format (OpenJev `POST /v1/systemone`, verified in SP-020 research prototypes):**

Request question:

```json
{"q": {"type": "choice",
       "instructions": "Which rule does this code violate?",
       "criteria": {"goroutine_lifecycle": "Never fire-and-forget goroutines…",
                    "none": "The code violates none of the listed rules."}}}
```

Answer:

```json
{"answers": {"q": {"choice": "goroutine_lifecycle",
                   "probabilities": {"goroutine_lifecycle": 0.97, "none": 0.02}}}}
```

**API surface (additive; do not change `Request`, `Decide`, or noul types):**

```go
type ChoiceQuestion struct {
    Instructions string
    Criteria     map[string]string // option ID -> description; at least 2 options
}
type ChoiceRequest struct {
    Model     string
    State     string
    Questions map[string]ChoiceQuestion
    Samples   int
}
type ChoiceAnswer struct {
    Choice        string             `json:"choice"`
    Probabilities map[string]float64 `json:"probabilities"`
}
type ChoiceResponse struct {
    Model   string
    Answers map[string]ChoiceAnswer
    Usage   Usage
}
func (c *Client) DecideChoice(ctx context.Context, req ChoiceRequest) (*ChoiceResponse, error)
```

- `ChoiceQuestion` marshals with the `"type": "choice"` discriminator (mirror `NoulQuestion.MarshalJSON`).
- Validation (mirror `validateResponse`): every requested question has an answer; `Choice` is one of that question's criteria keys; every probability is in [0, 1] (NaN-safe); probability keys are a subset of the criteria keys. Validation errors go through `c.redact`.
- Client-side request validation before sending: non-empty `Questions`; each question has ≥ 2 criteria and no empty option ID. Return a descriptive error without contacting the server.
- Same endpoint, retries, error sentinels (`ErrUnreachable`, `ErrBadRequest`, `ErrUnauthorized`, `ErrOverloaded`), and API-key redaction as `Decide`.
- Update the package doc comment (it currently says only noul is modelled).

## Dependencies

- **None**

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `cli/internal/systemone/client.go` — `Request`, `NoulQuestion.MarshalJSON`, `Decide`, `validateResponse`, `doWithRetries`, `redact`
- `docs/research/systemone-rules-compliance.md` — Protocol B (choice)

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none (tests use `httptest.Server`)

## File Scope

- `cli/internal/systemone/client.go`
- `cli/internal/systemone/client_test.go`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go vet ./internal/systemone/... && go test ./internal/systemone/... ./internal/review/... -count=1 -cover` |
| fileScopeMustChange | `cli/internal/systemone/client.go`, `cli/internal/systemone/client_test.go` |
| fileScopeMustNotChange | `cli/internal/rules/**`, `cli/internal/run/**`, `cli/cmd/**`, `extension/**` |
| completionCriteria | `DecideChoice` sends the documented wire JSON and validates answers per Mission; existing noul API unchanged; API key never appears in any error string; `client.go` coverage ≥ 72% |

## Steps

### Step 1: Choice types and `DecideChoice`

- [ ] Types + `MarshalJSON` with `"type": "choice"`
- [ ] Client-side request validation, `DecideChoice`, response validation (reuse transport/retry/redaction)
- [ ] Package doc comment updated

### Step 2: Testing & Verification

> ZERO test failures allowed.

- [ ] `httptest` server asserts exact request JSON (type discriminator, criteria map, samples omitted when 0)
- [ ] Cases: valid answer; missing answer; choice not in criteria; probability out of range / NaN; unknown probability key; < 2 criteria; empty questions; 401/4xx/529 retry behaviour matches `Decide`; API key absent from error strings
- [ ] Existing noul tests unchanged and passing
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Per-file coverage ≥ 72% for `client.go`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Implementation:** `feat(SP-032): description`
- **Tests:** `test(SP-032): description`

## Do NOT

- Change the noul `Request`/`Response`/`Decide` API or behaviour
- Add callers outside `cli/internal/systemone` (SP-026/SP-027 do that)
- Log or print the API key
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
