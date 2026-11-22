# Research: stet model comparison — `qwen/qwen3.8-27b` vs `thinkingmachines/inkling-2-mini`

**Task:** SP-092 · **Date:** 2026-11-22 · **Status:** Complete
**Question:** Does stet gain enough (or lose little enough) by reviewing with `thinkingmachines/inkling-2-mini` (26.4B, ~18 GB) to replace the current default `qwen/qwen3.8-27b` (27B, 16.08 GB)?

---

## Summary

**Recommendation: keep `qwen/qwen3.8-27b` as stet's default review model.** The pre-registered recall condition fails: Inkling's seeded-bug recall is 0.7917 vs Qwen's 0.9167, so `(0.9167 − 0.7917) × 12 = 1.50 > 1`. Noise, real-diff actionable rate, and reliability all pass, and Inkling is quieter on unexpected findings, but the decision rule requires all four conditions.

---

## Setup

**Hardware/stack:** Apple Silicon (64 GB), LM Studio local server at `http://127.0.0.1:1234`. One model loaded at a time; OpenJev stopped. Manual Cursor execution after spine batch `20261122T021336-7f28` aborted (worker SIGTERM).

**Models:**

| Model | Arch | Params | Disk | Tag |
|---|---|---|---|---|
| `qwen/qwen3.8-27b` (baseline / C) | qwen3_5 | 27B | 16.08 GB | `qwen__qwen3.8-27b` |
| `thinkingmachines/inkling-2-mini` (candidate / Q) | inkling2_loop | 26.4B | 18.12 GB | `thinkingmachines__inkling-2-mini` |

**LM Studio load (both):** `lms load <model> -c 65536 --parallel 1 --no-speculative-draft-mtp -y`.

**Reasoning controls (registered before results):**

- Inkling: Reasoning Effort = **none**, reasoning budget **off** (budget=0 emptied content in preflight). Residual `reasoning_tokens` still observed (~50–56 on smoke; ~44 on `[]` probe).
- Qwen: Enable Thinking **off** (`ext.virtualModel.customField.qwen.qwen3.827b.enableThinking = false`).

**Sampling presets (Qwen saved; Inkling UI-confirmed, no saved preset file under `user-concrete-model-default-config/`):** Top K 20, Top P 0.9, Min P off, Repeat Penalty off, Context Overflow "Stop at Limit", Limit Response Length off, Reasoning Parsing on, empty System Prompt, Structured Output off, Max Concurrent Predictions 1.

**Pinned stet env:** temperature 0.1, `max_completion_tokens` 4096, context 65536, strictness `default`, critic / rules selector / rules annotate off, `STET_TIMEOUT=30m`, provider openai → `http://127.0.0.1:1234/v1`.

**stet build:** `stet version v1.7.0` · repo HEAD at preflight `36864e4`.

**Harness:** `scripts/model_eval.py` · fixtures `scripts/model-eval-cases/` · results root `docs/research/model-eval-inkling-2-mini/` · 14 cases × 3 runs + 3 real refs × 1 = 63 runs per model.

**`stet benchmark`:** both models failed the optional benchmark (openai client default 10s timeout vs residual/TTFT). Primary speed = harness wall times.

**Run notes:** Inkling first real ref `3ee3055f…` failed once (`RAG resolve… signal: killed`) then succeeded on `--retry-failed`. Final reliability: both models 63/63 `ok`.

---

## Results

Source: `docs/research/model-eval-inkling-2-mini/summary.json`.

### Recall (12 seeded bugs × 3 runs)

| Metric | Qwen (C) | Inkling (Q) |
|---|---|---|
| recall_mean | **0.9167** | 0.7917 |
| caught all runs | 14 cases listed (incl. rule cases with all-run catches) | 12 |
| missed all runs | — | `go-rule-global-prefix`, `py-rule-naming` |
| caught some runs | `go-rule-context`, `go-rule-global-prefix` | `go-resource-leak`, `py-rule-membership` |

Per category (caught_mean):

| Category | Qwen | Inkling |
|---|---|---|
| correctness (8) | 1.000 | 1.000 |
| performance (1) | 1.000 | 0.333 |
| security (3) | 1.000 | 1.000 |
| rule (4) | 0.667 | 0.333 |

### Unexpected findings (fixture runs)

| Metric | Qwen | Inkling |
|---|---|---|
| unexpected total | 48 | 13 |
| error + warning counts | 15 + 28 = 43 | 8 + 3 = 11 |
| error+warning / 42 case-runs | 1.024 | **0.262** |
| on clean cases | 21 | 2 |

### Real-diff blind labels

| Metric | Qwen | Inkling |
|---|---|---|
| labeled findings | 42 | 1 |
| actionable | 0 | 0 |
| actionable_rate | 0.00 | 0.00 |
| false_positive / out_of_scope / wrong_suggestion | 32 / 10 / 0 | 1 / 0 / 0 |

### Reliability

| Metric | Qwen | Inkling |
|---|---|---|
| ok / failed / timeout | 63 / 0 / 0 | 63 / 0 / 0 |

### Speed (wall clock; not in decision rule)

| Metric | Qwen | Inkling |
|---|---|---|
| seconds_per_case_mean | **16.3** | 59.6 |
| seconds_per_real_ref_mean | **622** | 989 |

---

## Threats to validity

- 12 synthetic bugs → each miss ≈ 8 recall points; the failing recall margin is one-and-a-half bug-equivalents.
- Real-diff labels are human/LLM-assisted judgments without opening `blind-key.json` until after `real-labels.tsv` was committed; Inkling produced only 1 real finding vs Qwen's 42, so precision comparison is thin.
- Inkling residual reasoning at effort=none vs Qwen zero-reasoning is a controlled confound.
- Temperature 0.1 still admits run-to-run variance.
- `stet benchmark` openai 10s client timeout omitted throughput numbers.
- No saved Inkling preset JSON on disk; settings relied on operator UI confirmation + smoke.

---

## Recommendation

Applying the pre-registered decision rule mechanically from `docs/research/model-eval-inkling-2-mini/summary.json` (C = `qwen__qwen3.8-27b`, Q = `thinkingmachines__inkling-2-mini`, B = 12):

1. **Recall:** `(recall_mean[C] − recall_mean[Q]) × B = (0.9167 − 0.7917) × 12 = 1.50 ≰ 1` → **FAIL**. Inkling misses `go-rule-global-prefix` and `py-rule-naming` in all 3 runs; also weak on `go-resource-leak` (some runs only).
2. **Noise:** C error+warning/run = 43/42 = 1.024; Q = 11/42 = 0.262. Is `0.262 ≤ 1.25 × 1.024 = 1.280`? Yes → **PASS** (Inkling is quieter).
3. **Real-diff precision:** `actionable_rate[C] − actionable_rate[Q] = 0.00 − 0.00 = 0.00 ≤ 0.10` → **PASS**.
4. **Reliability:** Q non-`ok` final runs = 0 ≤ 2 → **PASS**.

**Not all four conditions hold → keep `qwen/qwen3.8-27b` as stet's default model.** Do not change `.review/config.toml` in this task.

Where Inkling beats Qwen: lower unexpected noise; still perfect correctness and security category means on the fixtures; fewer real-diff findings emitted (1 vs 42). Speed strongly favors Qwen (~3.7× faster per fixture case). Speed is excluded from the rule.

**Small-sample caveat:** recall failure rests on two all-miss rule cases (~16 recall points); treat noise/precision as supportive but not decisive once recall fails.

---

## Follow-ups

- Leave `.review/config.toml` on `qwen/qwen3.8-27b`.
- Optional: fix `stet benchmark` openai client timeout when `NewClient(..., nil)` so residual-reasoning models can be timed.
- Optional: save an Inkling LM Studio preset JSON with Reasoning Effort=none for reproducible reloads.
- Optional: re-run with thinking effort > none as a separate study (not this decision rule).
