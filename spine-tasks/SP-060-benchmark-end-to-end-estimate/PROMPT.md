# Task: SP-060 — Benchmark: end-to-end hunk estimate grows with prompt size

**Created:** 2026-10-22
**Size:** S

## Review Level: 1

**Assessment:** Estimate formula change in `cli/internal/benchmark/benchmark.go` plus tests; one package, no CLI output format change.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 0

## Mission

Closes #31

For OpenAI-compatible providers, `resultToBenchmark` (`cli/internal/benchmark/benchmark.go` ~line 71) sets `EndToEndRate` because the client reports only wall clock: `EvalDuration` is the whole request and `PromptEvalDuration` is 0. The typical-hunk estimate (~line 100) is still computed the Ollama way, so with no prefill rate only `TypicalOutputTokens / EvalRateTPS` is added. The estimate ignores prompt size and understates the time for a 3k-token hunk prompt.

Fix:

1. Move the estimate into a small helper, for example `estimateHunkSeconds(r *BenchmarkResult, promptTokens, outputTokens int) float64`, so tests can vary the token counts (`TypicalPromptTokens` and `TypicalOutputTokens` are constants).
2. **Ollama-shaped results** (`EndToEndRate == false`): keep today's formula exactly: `promptTokens / PromptEvalRateTPS` (when > 0) plus `outputTokens / EvalRateTPS` (when > 0).
3. **End-to-end results** (`EndToEndRate == true`): scale the measured request time by the token ratios, so the estimate grows with `promptTokens`. Use the measured counts (`PromptEvalCount`, `EvalCount`) and the measured request seconds (`EvalDurationNs`). A simple, documented model is acceptable, for example: split the measured time between prompt and output in proportion to their token counts, then scale each part by `typical / measured` for its token kind. Guard every division (zero counts or zero duration give 0, as today). State the model in a short comment above the helper.
4. Keep the existing NaN/negative guard. `BenchmarkResult` fields and the `stet benchmark` printer in `cli/cmd/stet/main.go` stay unchanged (the printer already prints "~Ns estimated").

## Dependencies

- **None**

## Context to Read First

**Tier 3 (reference docs):**

- `cli/internal/benchmark/benchmark.go` — `BenchmarkResult`, `resultToBenchmark`
- `cli/internal/benchmark/benchmark_test.go` — `TestRun_openAICompatEndToEndRate` (~line 184), `TestResultToBenchmark_endToEndFlag` (~line 221)

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/benchmark/benchmark.go`
- `cli/internal/benchmark/benchmark_test.go`

## Documentation Requirements

**Must Update:** none (SP-065 updates `README.md`).

**Check If Affected:** none.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/benchmark/... -count=1 && bash scripts/spine-stet-contract-run.sh lenient` |
| fileScopeMustChange | `cli/internal/benchmark/benchmark.go`, `cli/internal/benchmark/benchmark_test.go` |
| fileScopeMustNotChange | `cli/internal/openaicompat/**`, `cli/internal/ollama/**`, `extension/**`, `.review/**` |
| completionCriteria | For an end-to-end result, the estimate with a larger prompt-token count is strictly larger (test); Ollama-shaped results give the same estimate as before (test with fixed numbers); zero counts or durations give 0 without NaN (test); existing benchmark tests pass; stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh lenient`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Estimate helper with an end-to-end branch

- [ ] Helper takes prompt and output token counts; `resultToBenchmark` calls it with the typical constants
- [ ] Ollama branch unchanged; end-to-end branch scales measured time by token ratios; divisions guarded
- [ ] `spine_review_step` after the step

### Step 2: Testing & Verification

> ZERO test failures allowed.

- [ ] End-to-end estimate grows with prompt tokens
- [ ] Ollama-shaped estimate unchanged (fixed-number test)
- [ ] Zero-count / zero-duration cases return 0
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Fix:** `fix(SP-060): description`
- **Tests:** `test(SP-060): description`

## Do NOT

- Change `cli/cmd/stet/main.go` (SP-061 edits `runBenchmark`)
- Change the OpenAI-compatible client timing or `BenchmarkResult` field names
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
