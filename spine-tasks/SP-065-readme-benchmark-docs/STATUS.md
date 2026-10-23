# SP-065: README: `stet benchmark` config source and end-to-end estimate — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-22 (complete)
**Review Level:** 0
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Benchmark section
**Status:** ✅ Complete

- [x] Config-source sentence
- [x] Estimate sentence

---

### Step 2: Testing & Verification
**Status:** ✅ Complete

- [x] `rg` shows both additions (README.md lines 154–155)
- [x] Test references to README noted (none: `rg -l 'README.md' cli/ scripts/` returns no matches)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| No test in `cli/` or `scripts/` references README.md | Noted (expected per PROMPT) | Step 2 verification |
| Behaviors verified in code: `runBenchmark` loads repo `.review/config.toml` via `config.Load(RepoRoot)`; `estimateHunkSeconds` treats OpenAI-compat rates as end-to-end and scales measured request time to typical hunk (~3k prompt, ~200 out) | Statements match code | cli/cmd/stet/main.go:1854, cli/internal/benchmark/benchmark.go:121 |
