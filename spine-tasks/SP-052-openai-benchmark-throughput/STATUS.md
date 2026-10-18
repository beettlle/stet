# SP-052: Benchmark and run summary show throughput for OpenAI-compatible providers — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-17
**Review Level:** 2
**Review Counter:** 2
**Iteration:** 1
**Size:** S

---

### Step 1: Client-side timing in openaicompat
**Status:** ✅ Complete

- [x] Both result paths set `TotalDuration` / `EvalDuration` (wall clock from before `httpClient.Do` to after body decode; `PromptEvalDuration` stays 0)

---

### Step 2: Benchmark end-to-end rate and label
**Status:** ✅ Complete

- [x] End-to-end flag and rate (`BenchmarkResult.EndToEndRate` when `PromptEvalDuration == 0 && EvalDuration > 0`)
- [x] `(end-to-end)` label in `stet benchmark` printer

---

### Step 3: Testing & Verification
**Status:** ✅ Complete

- [x] `openaicompat` timing test (`TestClient_generateSetsWallClockDurations`, both paths, ≥25 ms lower bound)
- [x] `benchmark` end-to-end and Ollama tests (`TestRun_openAICompatEndToEndRate`, `TestResultToBenchmark_endToEndFlag`, Ollama mock asserts flag false)
- [x] Run summary shows tokens/sec (`TestWriteFindingsHuman_openAIStatsShowTokensPerSec`); CLI label test `TestRunCLI_benchmarkOpenAILabelsEndToEnd`
- [x] Contract `testCommand` passing (gofmt, vet, tests ok; stet contract review exit 0 after triage)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|
| 1 | plan | 1 | skipped (engine-owned) | .reviews/1-20261017T224924.md |
| 2 | plan | 2 | skipped (engine-owned) | .reviews/2-20261017T224939.md |

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| `.review/spine-stet-baseline.ref` was missing in lane-2 (lane reused after SP-049); contract script exited 1. Recreated it with pre-worker HEAD `91efbcf` (gitignored, same as worktreeSetupHook). | Worked around; worth checking hook behavior for reused lanes | `scripts/spine-worktree-setup.sh` |
| stet findings fixed: e833b33 (brittle zero-rate string match → parse rate). | Fixed | `cli/cmd/stet/main_test.go` |
| stet reviewer false positives: claimed unused import/variables in code that compiles (a96abf6, 0165283, 9c4f406); claimed token counts were zero when they come from the response (4acb2e0); flagged label width breaking alignment of unrelated lines (db6d5c5); flagged ≥delay assertion as flaky though timer starts before request (7c475d2); misread "do not run in parallel" convention (53b4096). | Dismissed false_positive — reviewer hunk-context bugs | stet reviewer |
| Wall clock in `EvalDuration` / strict `PromptEvalDuration == 0` / struct invariant (3f49517, 964d236, d3fe01d, 93f502e) contradict the PROMPT design (rate is labelled end-to-end). | Dismissed wrong_suggestion | — |
| README / cli-extension-contract do not document benchmark output format. | No doc change needed | README.md |
| `EstimatedSecPerTypicalHunk` for end-to-end rates uses only output tokens / e2e rate (no prefill term) — rough estimate. | Tech debt, out of scope | `cli/internal/benchmark/benchmark.go` |
