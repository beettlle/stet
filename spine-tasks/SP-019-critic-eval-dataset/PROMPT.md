# Task: SP-019 — Critic threshold evaluation harness

**Created:** 2026-09-30
**Size:** M

## Review Level: 1

**Assessment:** New read-only analysis command; reads local review history and sends code to the configured System One endpoint.
**Score:** 3/8 — Blast radius: 0, Pattern novelty: 2, Security: 1, Reversibility: 0

## Mission

Give users a data-driven way to choose `systemone_min_keep`. Add `stet critic-eval`, which builds a labeled set of past findings, scores each with the System One critic, and reports how many false positives would be dropped versus how many real findings would be lost at each threshold.

**Background (System One / OpenJev):** OpenJev ([razorback16/openjev](https://github.com/razorback16/openjev)) exposes `POST /v1/systemone`; a `noul` question returns P(yes). SP-016 provides `review.VerifyFindingsSystemOne(ctx, client, model, batch, hunkContent, minKeep, samples)` returning per-finding scores; SP-015 provides `systemone.NewClient`; SP-014 provides the `systemone_*` config keys. Warm latency ~100 ms per request.

**Label sources** (confirm against real data in Step 0):

- **Negatives** (critic should drop): from `history.ReadRecords(stateDir)` (`cli/internal/history/append.go`), dismissals whose `reason` is `false_positive` or `wrong_suggestion` and that have a non-empty `prompt_context`; the finding itself comes from the same record's `review_output` by `finding_id`.
- **Positives** (weak labels, critic should keep): active findings in `.review/session.json` that are not in `dismissed_ids` and have an entry in `finding_prompt_context`. History does not store hunk content for undismissed findings, so history alone cannot supply positives.
- **Optional curated set:** `--dataset <path.jsonl>` with lines `{"finding": {...}, "hunk": "...", "label": "keep"|"drop"}`; when given, it is used instead of history/session.
- **Excluded:** `already_correct` (auto-dismiss reuses it for fixed issues, so it is ambiguous) and `out_of_scope`.

**Output** (human table by default; `--json` for machine output):

- Counts per label and source; warn when either class has fewer than 10 examples ("recommendation unreliable").
- For thresholds 0.1, 0.2, …, 0.9: false positives dropped (%), real findings lost (%), findings kept.
- p50/p95 request latency.
- Recommended threshold: the highest threshold where real findings lost ≤ 10% (named constant), or "none" if no threshold qualifies.

Requests are grouped per hunk (same `prompt_context`) so the verifier batches them; no per-finding request loop.

## Dependencies

- **Task:** SP-018 (shares `cli/cmd/stet/main.go`; transitively SP-014–SP-017)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `cli/internal/history/schema.go` — `Record`, `Dismissal`, reason constants
- `cli/internal/history/append.go` — `ReadRecords`
- `cli/internal/session/session.go` — `DismissedIDs`, `FindingPromptContext`, `Findings`
- `cli/internal/review/critic_systemone.go` (SP-016)
- `cli/cmd/stet/main.go` — subcommand registration pattern (e.g. `newBenchmarkCmd`)
- `AGENTS.md` — coverage gates

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none for tests (fake decider / `httptest.Server`); a real OpenJev at `http://127.0.0.1:8080` for manual runs

## File Scope

- `cli/internal/criticeval/criticeval.go` (new)
- `cli/internal/criticeval/criticeval_test.go` (new)
- `cli/internal/criticeval/testdata/**` (new fixtures)
- `cli/cmd/stet/main.go`
- `cli/cmd/stet/main_test.go`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./internal/criticeval/... ./cmd/stet/... -count=1 -cover` |
| fileScopeMustChange | `cli/internal/criticeval/criticeval.go`, `cli/cmd/stet/main.go` |
| fileScopeMustNotChange | `cli/internal/history/**`, `cli/internal/session/**`, `cli/internal/review/**`, `extension/**` |
| completionCriteria | `stet critic-eval` builds labeled set, sweeps thresholds, reports metrics and recommendation; read-only on `.review/`; tests pass with ≥72% per changed file |

## Steps

### Step 0: Confirm labeling against real data

- [ ] Inspect a real `.review/history.jsonl` and `session.json` (schema only; do not commit user data) and confirm the label rules above
- [ ] Record any rule changes in STATUS Discoveries

> ⚠️ Hydrate: adjust Step 1 checkboxes if the label sources differ from the PROMPT.

### Step 1: Dataset and scoring (`criticeval` package)

- [ ] Load negatives/positives (or `--dataset`), grouped by hunk
- [ ] Score via `VerifyFindingsSystemOne` with `minKeep = 0` to collect raw scores; measure per-request latency
- [ ] Compute threshold sweep, latency percentiles, recommendation

### Step 2: `stet critic-eval` subcommand

- [ ] Register command; flags `--dataset`, `--json`; uses config `systemone_*` keys and env API key
- [ ] Human table and JSON output; clear error when no labeled examples exist

### Step 3: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Fixture history/session/dataset files under `testdata/`; fake decider with fixed scores → exact expected sweep and recommendation
- [ ] Edge cases: empty data, one class only (warning), excluded reasons ignored, decider error surfaces
- [ ] `main_test.go`: command runs against `httptest.Server`, `--json` output parses
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Per-file coverage ≥ 72% for `criticeval.go` and `main.go`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing
- [ ] Command never writes to `.review/`

## Git Commit Convention

- **Implementation:** `feat(SP-019): description`
- **Tests:** `test(SP-019): description`

## Do NOT

- Change critic defaults automatically based on results
- Modify history or session schemas
- Commit real user history or code as fixtures
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
