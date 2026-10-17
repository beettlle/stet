# Task: SP-052 — Benchmark and run summary show throughput for OpenAI-compatible providers

**Created:** 2026-10-17
**Size:** S

## Review Level: 2

**Assessment:** Client timing in `openaicompat`, a fallback in `benchmark`, and a label in `cli/cmd/stet/main.go` (hot shared module).
**Score:** 4/8 — Blast radius: 2, Pattern novelty: 1, Security: 0, Reversibility: 1 — Level 2 for the hot module

## Mission

Closes #4

`stet benchmark` against LM Studio (`provider = "openai"`) prints `Eval rate: 0.0 t/s (generation)` and `Total: 0.0s` even though token counts arrive. The OpenAI-compatible client (`cli/internal/openaicompat/client.go`, ~lines 311–314 and ~415–418) always sets `PromptEvalDuration`, `EvalDuration`, and `TotalDuration` to 0 because Chat Completions responses don't include them. `resultToBenchmark` in `cli/internal/benchmark/benchmark.go` (~line 80) only computes a rate when `EvalDuration > 0`, and `printRunSummary` in `cli/cmd/stet/main.go` (~line 187) only prints "tokens/sec" when `EvalDurationNs > 0`.

Fix:

1. **`openaicompat`:** time each request on the client (wall clock from just before the HTTP request to after the body is read) and set `TotalDuration` and `EvalDuration` to that value in nanoseconds, on both result paths. Leave `PromptEvalDuration` at 0. This makes the existing run-summary guard print tokens/sec with no change to `printRunSummary`.
2. **`benchmark`:** add a `BenchmarkResult` field (for example `EndToEndRate bool`) set when `PromptEvalDurationNs == 0` and `EvalDurationNs > 0`, meaning the rate covers prefill plus generation. Keep the Ollama path unchanged.
3. **Label:** in the benchmark printer in `cli/cmd/stet/main.go` (~line 1876), print `(end-to-end)` instead of `(generation)` when `EndToEndRate` is set.

Optional server timing fields (LM Studio `stats`, llama.cpp `timings`) are out of scope.

## Dependencies

- **None**

## Context to Read First

**Tier 3 (reference docs):**

- `cli/internal/openaicompat/client.go` and its tests (existing `httptest` servers)
- `cli/internal/benchmark/benchmark.go`, `benchmark_test.go`
- `cli/cmd/stet/main.go` — `printRunSummary` (~line 171) and the benchmark printer (~line 1870)

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only (unit tests use `httptest`)

## File Scope

- `cli/internal/openaicompat/client.go`
- `cli/internal/openaicompat/client_test.go`
- `cli/internal/benchmark/benchmark.go`
- `cli/internal/benchmark/benchmark_test.go`
- `cli/cmd/stet/main.go`
- `cli/cmd/stet/main_test.go`

## Documentation Requirements

**Must Update:** none

**Check If Affected:**

- `README.md` / `docs/cli-extension-contract.md` — if either documents `stet benchmark` output for OpenAI-compatible providers, log the needed change in STATUS Discoveries

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/openaicompat/... ./cli/internal/benchmark/... ./cli/cmd/stet/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/openaicompat/client.go`, `cli/internal/openaicompat/client_test.go`, `cli/internal/benchmark/benchmark.go`, `cli/internal/benchmark/benchmark_test.go`, `cli/cmd/stet/main.go` |
| fileScopeMustNotChange | `cli/internal/run/**`, `cli/internal/ollama/**`, `extension/**`, `scripts/**`, `.review/**` |
| completionCriteria | An `httptest` OpenAI-compatible server that sleeps briefly yields non-zero `EvalDuration`/`TotalDuration` and a non-zero benchmark rate labelled end-to-end; Ollama benchmark tests unchanged; run summary shows tokens/sec for OpenAI-compatible results; stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Client-side timing in openaicompat

- [ ] Both result paths set `TotalDuration` and `EvalDuration` from wall clock
- [ ] `spine_review_step` after the step

### Step 2: Benchmark end-to-end rate and label

- [ ] `BenchmarkResult` end-to-end flag; rate computed from the new durations
- [ ] Benchmark printer label `(end-to-end)` when flagged
- [ ] `spine_review_step` after the step

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] `openaicompat` test: `httptest` server sleeping ≥20 ms → `EvalDuration` and `TotalDuration` > 0 (assert a lower bound, no upper bound, to stay non-flaky)
- [ ] `benchmark` test: zero prompt duration plus non-zero eval duration → non-zero rate and end-to-end flag; Ollama-shaped input → flag false, rates unchanged
- [ ] `main` test: run summary for an OpenAI-shaped `RunStats` contains `tokens/sec`
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Fix:** `fix(SP-052): description`
- **Tests:** `test(SP-052): description`

## Do NOT

- Change the Ollama client or Ollama benchmark math
- Parse server-specific timing fields
- Touch `run` / `rerun` flag handling in `main.go` (SP-057 edits that area)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
