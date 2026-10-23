# SP-060: Benchmark: end-to-end hunk estimate grows with prompt size — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-22
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Estimate helper with an end-to-end branch
**Status:** ✅ Complete

- [x] Helper with token-count parameters (`estimateHunkSeconds(r *BenchmarkResult, promptTokens, outputTokens int) float64`; `resultToBenchmark` calls it with the typical constants)
- [x] Ollama branch unchanged; end-to-end branch scales measured time (documented model above the helper); divisions guarded

---

### Step 2: Testing & Verification
**Status:** ✅ Complete

- [x] End-to-end estimate grows with prompt tokens (`TestEstimateHunkSeconds_endToEndGrowsWithPromptTokens`)
- [x] Ollama-shaped estimate unchanged (`TestEstimateHunkSeconds_ollamaShapeUnchanged`, fixed numbers: 3000/2000 + 200/50 = 5.5s)
- [x] Zero cases return 0 (`TestEstimateHunkSeconds_zeroCountsAndDurations`)
- [x] Contract `testCommand` passing — check-gofmt, `go vet ./cli/...`, benchmark tests, `spine-stet-contract-run.sh lenient` exit 0 (2 findings triaged, 0 active)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|
| 1 | plan | 1 | skipped (engine-owned; batch engine runs reviews after .DONE) | .reviews/1-20261022T235807.md |

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| stet reviewer flagged `math` import as unused (45d73c5, ERROR) although `math.Abs` is used in two new tests; package compiles (go vet + go test pass). Diff-hunk-only context makes the reviewer miss usages outside changed hunks. | false_positive (dismissed) | `.review/history.jsonl` |
| stet reviewer warned the "zero counts" test "masks a bug" for estimating 0s when measured token counts are 0 (b95ee02); PROMPT explicitly requires zero counts → 0, so behavior is per spec. | already_correct (dismissed) | `.review/history.jsonl` |
| Pre-existing nuance: `TestRun_handlesZeroPromptEvalDuration` (prompt_eval_duration=0, eval_duration>0) is already classified `EndToEndRate=true` by the existing flag logic, so it now routes through the end-to-end estimate branch; test expectation updated from ~4s to ~21.3s with comment. | addressed | `cli/internal/benchmark/benchmark_test.go` |

---

## Amendments (Added During Execution)

- None.
