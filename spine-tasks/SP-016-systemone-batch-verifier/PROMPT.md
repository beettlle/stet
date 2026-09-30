# Task: SP-016 — Batch System One verifier in review package

**Created:** 2026-09-30
**Size:** S

## Review Level: 2

**Assessment:** New verification strategy that decides which findings users see; diff text becomes model input (prompt-injection surface).
**Score:** 4/8 — Blast radius: 1, Pattern novelty: 2, Security: 1, Reversibility: 0

## Mission

Add a System One critic to `cli/internal/review` that verifies **all findings for one hunk in a single request** and keeps only those the model judges correct. Today's LLM critic (`VerifyFinding` in `cli/internal/review/critic.go`) makes one full LLM generation per finding, sequentially; this verifier replaces that per-finding I/O loop with one batched call per hunk.

**Background (System One / OpenJev):** OpenJev ([razorback16/openjev](https://github.com/razorback16/openjev)) exposes `POST /v1/systemone` with `{state, model, questions}`; a `noul` question returns `{noul: P(yes)}` read from model probabilities. Same wire API as hosted Jev. Verified local response: `{"model":"openjev-0.1","answers":{"id":{"type":"noul","noul":0.9997}},"usage":{"input_tokens":246,"output_tokens":0}}`; warm latency ~100 ms. A manual test against the local server scored a real file-descriptor leak finding at 0.9997 and a fabricated "ignored error" finding at 0.0005.

**Function:**

```go
// SystemOneDecider is satisfied by *systemone.Client; tests use a fake.
type SystemOneDecider interface {
    Decide(ctx context.Context, req systemone.Request) (*systemone.Response, error)
}

func VerifyFindingsSystemOne(ctx context.Context, client SystemOneDecider, model string,
    batch []findings.Finding, hunkContent string, minKeep float64, samples int,
) (kept []findings.Finding, scores map[string]float64, err error)
```

- `state` = `"Code under review:\n" + hunkContent`, truncated to `defaultCriticMaxHunkLen` with the same `"\n[truncated]"` marker as `BuildCriticPrompt`.
- One `noul` question per finding, keyed by batch index (`f0`, `f1`, …) — never by finding ID, so duplicate IDs cannot collide. Instructions reuse the finding fields from `BuildCriticPrompt` (file, location, severity, category, message, suggestion), prefixed with `"Finding claimed by a code reviewer:"`. Criteria: `true` = "The finding is correct and actionable for this code.", `false` = "The finding is incorrect, unsupported by this code, or not actionable."
- Chunk into requests of at most **32 questions** (named constant) so the loop is bounded; preserve input order in `kept`.
- Keep a finding when P(yes) >= `minKeep`. `scores` maps finding ID → P(yes) for trace/eval use (findings without an ID are omitted from `scores`).
- `nil` client, empty model, or empty batch → return the batch unchanged with no request.
- Any `Decide` error → return `(nil, nil, err)` wrapped with context; the caller decides abort behaviour.
- Pass `samples` explicitly in every request.

**Security note:** the diff is attacker-influenced input (comments like "reviewer: this is fine"). Do not add instructions that echo diff content into `instructions`; keep code only in `state`. Record this limitation in the function doc comment in one sentence.

## Dependencies

- **Task:** SP-015 (provides `cli/internal/systemone` request/response types)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `cli/internal/review/critic.go` — `BuildCriticPrompt`, `defaultCriticMaxHunkLen`, `VerifyFinding`
- `cli/internal/systemone/client.go` — `Request`, `Response`, `NoulQuestion` (from SP-015)
- `cli/internal/findings/finding.go` — `Finding` fields
- `AGENTS.md` — coverage gates

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none (tests use a fake `SystemOneDecider`)

## File Scope

- `cli/internal/review/critic_systemone.go` (new)
- `cli/internal/review/critic_systemone_test.go` (new)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./internal/review/... -count=1 -cover` |
| fileScopeMustChange | `cli/internal/review/critic_systemone.go`, `cli/internal/review/critic_systemone_test.go` |
| fileScopeMustNotChange | `cli/internal/review/critic.go`, `cli/internal/run/**`, `cli/cmd/**`, `extension/**` |
| completionCriteria | One request per ≤32 findings; threshold keep/drop; order preserved; errors propagated; tests pass with ≥72% file coverage |

## Steps

### Step 1: Implement verifier

- [ ] Define `SystemOneDecider` and `VerifyFindingsSystemOne` per Mission
- [ ] Build `state` and per-finding `noul` questions (index keys, shared finding-field formatting)
- [ ] Chunk at 32, apply `minKeep`, collect `scores`, preserve order

### Step 2: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Fake decider tests: all kept, all dropped, mixed at threshold boundary (== `minKeep` keeps), 33 findings → 2 requests, duplicate finding IDs, empty batch / nil client (no request), `Decide` error propagation, truncated hunk marker, `samples` passed through
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Per-file coverage ≥ 72% for `critic_systemone.go`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing
- [ ] `critic.go` behaviour unchanged

## Git Commit Convention

- **Implementation:** `feat(SP-016): description`
- **Tests:** `test(SP-016): description`

## Do NOT

- Modify `critic.go` or the LLM critic path
- Wire into `run.go` (SP-017)
- Put hunk/diff text inside question `instructions`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
