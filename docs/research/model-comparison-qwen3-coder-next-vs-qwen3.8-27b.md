# Research: stet model comparison — `qwen/qwen3-coder-next` vs `qwen/qwen3.8-27b`

**Task:** SP-039 · **Date:** 2026-10-11 · **Status:** Complete
**Question:** Does stet lose anything by reviewing with `qwen/qwen3.8-27b` (27B dense, 16.08 GB — fits in memory while OpenJev runs) instead of `qwen/qwen3-coder-next` (80B MoE, 44.86 GB — does not)?

---

## Summary

**Recommendation: adopt `qwen/qwen3.8-27b` as stet's default review model.** All four pre-registered decision-rule conditions hold: the 27B model matches or beats the 80B MoE on seeded-bug recall (1.00 vs 0.92 — it catches the map-race bug that coder-next missed in all 3 runs), its unexpected error/warning rate is within the noise threshold, real-diff actionable rates are identical (both 0.00 on 70 blind-labeled findings), and it completed all 45 runs without a failure. Its only cost is speed (~1.5× slower per fixture case, ~2.3× slower per real commit), which the rule explicitly excludes.

---

## Setup

**Hardware/stack:** Apple M1 Ultra (64 GB), LM Studio (MLX), local server at `http://127.0.0.1:1234`. One model loaded at a time; OpenJev stopped for the duration.

**Models:**

| Model | Arch | Params | Disk | Tag |
|---|---|---|---|---|
| `qwen/qwen3-coder-next` | qwen3_next | 80B MoE | 44.86 GB | `qwen__qwen3-coder-next` |
| `qwen/qwen3.8-27b` | qwen3_5 | 27B dense | 16.08 GB | `qwen__qwen3.8-27b` |

**LM Studio load (both models, every load):** `lms load <model> -c 65536 --parallel 1 --no-speculative-draft-mtp -y`. MTP speculative drafting stayed off: under MLX it fails every request on `qwen3_next` ("not supported for batched MLX models" at parallel > 1; "requires a trimmable prompt cache (got ArraysCache)" after reload — Qwen3-Next's hybrid-attention cache cannot be trimmed). `lms ps` confirmed context 65536 / parallel 1 for both.

**Operator presets (controlled confound — sampling parity).** stet sends only temperature and `max_tokens`; everything else comes from the saved per-model presets, which were set to identical values on both models before the run and verified by comparing the saved LM Studio config files (Step 0 "preset parity OK"): Top K 20 (on), Top P 0.9 (on), Min P off, Repeat Penalty off, Context Overflow "Stop at Limit", Limit Response Length off, Reasoning Parsing on, no draft model, empty System Prompt, Structured Output off, Max Concurrent Predictions 1. `qwen/qwen3.8-27b` additionally has Enable Thinking **off** (`ext.virtualModel.customField.qwen.qwen3.827b.enableThinking` = `false`). Caveat: parity was verified from the saved presets, not observed per request.

**Reasoning probe (27b, thinking off):** PASS — `message.reasoning_content` empty, `usage.completion_tokens_details.reasoning_tokens` 0, no `<think>` in `content` (content was exactly `[]`).

**Smoke requests:** both models returned HTTP 200, `finish_reason: "stop"`, no "Channel Error"/"Speculative decoding" text (coder-next max_tokens 64, 27b max_tokens 512).

**Pinned stet env (harness `PINNED_ENV`, matches `.review/config.toml`):** `STET_PROVIDER=openai`, `STET_OPENAI_BASE_URL=http://127.0.0.1:1234/v1`, `STET_TEMPERATURE=0.1`, `STET_MAX_COMPLETION_TOKENS=4096`, `STET_NUM_CTX=65536`, `STET_CONTEXT_LIMIT=65536`, `STET_STRICTNESS=default`, critic / rules selector / rules annotate off, `STET_TIMEOUT=30m`.

**stet build:** `stet version dev`, built from `main` at `9d16987` (includes `ea2124f` truncated-response handling, `14fce09` compact single-line JSON schema, `b0d43f8` harness context/cap pins). Harness: `scripts/model_eval.py` at `9d16987` plus one bug fix during scoring (`blind-key` path→dict, see Amendment A2). Repo HEAD at preflight: `9d16987`.

**Harness commands:**

```bash
# per model (order: coder-next first, then 27b)
lms unload --all && lms load <model> -c 65536 --parallel 1 --no-speculative-draft-mtp -y
stet benchmark --model <model> --warmup        # token counts only; see below
python3 scripts/model_eval.py run --model <model> --cases scripts/model-eval-cases \
  --out docs/research/model-eval/results --runs 3 \
  --real-ref 3ee3055f13cf00b5cb4e182f169829b78961743e \
  --real-ref 2326c0487a36e9b306edbed7da91dda1b536d00b \
  --real-ref e9b2e61f637cf165c630061c6d8f89c5e9f39404 --real-runs 1 --max-minutes 25
python3 scripts/model_eval.py export-blind --results docs/research/model-eval/results \
  --out docs/research/model-eval/blind.tsv --key docs/research/model-eval/blind-key.json
python3 scripts/model_eval.py score --cases scripts/model-eval-cases \
  --results docs/research/model-eval/results --labels docs/research/model-eval/real-labels.tsv \
  --blind-key docs/research/model-eval/blind-key.json --out docs/research/model-eval/summary.json
```

**`stet benchmark` tokens/s:** not measurable through LM Studio's OpenAI-compatible endpoint — it returns token counts but no Ollama-style eval-timing stats, so the eval rate prints 0.0 t/s (coder-next: 1028 in / 86 out; 27b: 1060 in / 129 out). Per-run completion-token usage is likewise absent from the harness logs (stet's stderr logs are empty; usage is not captured). Speed below is wall-clock only.

**Run schedule:** coder-next finished 45/45 `ok` in a single harness invocation; 27b needed two invocations (wall budget 25 min; last real ref spilled into the second), 45/45 `ok`. No `--retry-failed` needed for either model.

---

## Results

Full data: `docs/research/model-eval/summary.json`, `results/`, `blind.tsv`, `real-labels.tsv`. 12 seeded-bug cases × 3 runs + 3 real refs × 1 run = 45 runs per model, all `ok`.

### Recall (seeded bugs, 12 cases × 3 runs)

| Metric | coder-next | 27b |
|---|---|---|
| recall_mean | 0.9167 | **1.0000** |
| caught in all runs | 11/12 | **12/12** |
| caught in some runs | 0 | 0 |
| missed in all runs | go-map-race | — |

Per category (caught_mean):

| Category | coder-next | 27b |
|---|---|---|
| correctness (8) | 0.875 | **1.000** |
| performance (1) | 1.000 | 1.000 |
| security (3) | 1.000 | 1.000 |

**The one recall gap is `go-map-race`** (concurrent unsynchronized map write, ground truth `internal/metrics/collector.go:44`). coder-next returned 2 findings in every run but placed them at lines 28 ("counts map is unused") and 51 ("errs loop returns first nil error") — neither is the race, and the "unused map" claim is not a real defect. 27b reported the race at line 44 with severity `error` in all 3 runs. Notably, coder-next produced *more* findings on this case while missing the actual bug.

### Unexpected findings (on bug-free code or not matching ground truth)

| Metric | coder-next | 27b |
|---|---|---|
| total unexpected | 25 | 19 |
| mean per run | 0.595 | 0.452 |
| error | 8 | 3 |
| warning | 5 | 13 |
| info | 12 | 3 |
| nitpick | 0 | 0 |
| on clean cases | 3 | 9 |

27b emits fewer hard-severity (`error`) findings (3 vs 8) but more warnings (13 vs 5) and flags more things on the 2 clean cases (9 vs 3 findings). error+warning per run — the noise condition's measure — is 13/42 = 0.310 (coder-next) vs 16/42 = 0.381 (27b).

### Repeatability, reliability, speed

| Metric | coder-next | 27b |
|---|---|---|
| repeatability_jaccard_mean | 0.9841 | 0.9841 |
| reliability (ok / failed / timeout / zero_hunks) | 45 / 0 / 0 / 0 | 45 / 0 / 0 / 0 |
| seconds per case (mean) | **9.89** | 14.62 |
| seconds per real ref (mean) | **249.3** | 579.9 |
| `stet benchmark` tokens/s | n/a (see Setup) | n/a |

Per real ref (findings, wall): coder-next — `3ee3055` (15, 316 s), `2326c04` (8, 189 s), `e9b2e61` (15, 243 s); 27b — `3ee3055` (11, 819 s), `2326c04` (8, 297 s), `e9b2e61` (13, 624 s).

### Real-diff findings (blind-labeled)

70 findings across the 3 refs were anonymized (`export-blind`, fixed seed), labeled against the diffs without model identity, then de-anonymized by the harness. Labels: `actionable` / `false_positive` / `wrong_suggestion` / `out_of_scope`.

| Label | coder-next | 27b | total |
|---|---|---|---|
| actionable | 0 | 0 | 0 |
| false_positive | 34 | 26 | 60 |
| out_of_scope | 3 | 5 | 8 |
| wrong_suggestion | 1 | 1 | 2 |
| **labeled** | **38** | **32** | **70** |
| **actionable_rate** | **0.00** | **0.00** | — |

Both models found zero actionable issues on these three reviewed commits (which had already passed spine review). The dominant failure mode for both is confident hallucination — miscounted function parameters, invented "compile error" claims, speculative "ensure X is wired" findings, and multi-paragraph reasonings that conclude "no defect found" yet are still emitted as findings. Full labels with reasons are in `real-labels.tsv`.

---

## Pre-fix baseline

`docs/research/model-eval/baseline-pre-fix/` holds the aborted 2026-10-11 coder-next run (stet `4e59e6e`, before the issue #2/#3 fixes; context 32768, output cap 8192) plus one thinking-on 27b case. Scored to `/tmp` only (not committed): recall_mean 0.9167, 45/45 ok — the same recall as the post-fix run, but a very different shape:

| go-map-race run | pre-fix (32k ctx, 8192 cap) | post-fix (64k ctx, 4096 cap) |
|---|---|---|
| run 1 | 10.4 s, 1 finding | 14.1 s, 2 findings |
| run 2 | **806.4 s, 0 findings** (evidence_lines counting loop) | 12.2 s, 2 findings |
| run 3 | 9.6 s, 1 finding | 16.1 s, 2 findings |
| mean | 275.5 s | **14.1 s** |

The pre-fix build still missed the race (its 1-finding runs did not match line 44), so recall is unchanged; what the fixes bought is wall-clock (28.96 → 9.89 s/case overall) and the elimination of the degenerate 806 s run. The one thinking-on 27b baseline case took 1713 s with reasoning consuming the whole output cap — thinking-off (this run) is the only tractable configuration for 27b today. Caveat: baseline and final runs differ in *both* stet build and context size (32k → 64k), so baseline-vs-final differences cannot be attributed to either change alone; this is the before/after measurement issue #2 asked for, not a controlled comparison.

---

## Threats to validity

- **12 synthetic bugs, single author.** The fixture set is small and authored by one person (the same ecosystem that wrote stet's prompts); recall on it overestimates real-world recall. Each seeded bug is worth ≈ 8.3 recall points (1/12), so one case flips the comparison: the entire coder-next deficit is the single `go-map-race` case.
- **LLM-as-judge for real diffs.** Real-diff labels were produced by one judge (this task's agent) blind to model identity; `actionable_rate` 0.00 for both models is as much a statement about judge strictness and commit quality as about the models.
- **Temperature 0.1 sampling variance.** 3 runs per case is a small sample; run-to-run Jaccard was high (0.984) but findings on a single run can vary.
- **Sampling parity is verified from saved presets, not observed per request.** Step 0 compared the saved LM Studio config files; individual requests were not intercepted.
- **MTP speculative decoding disabled for both models**, matching `.review/config.toml`'s constraint, not measuring either model's speculative-decoding performance.
- **27b ran with thinking off.** This measures 27b as a non-reasoning model only; with thinking on it was 13–30 h/review in the aborted pass and is not a usable configuration at this output cap.
- **Pre-fix baseline confound:** differs in both stet build and context size (32k vs 64k); see above.

---

## Recommendation

Applying the pre-registered decision rule mechanically from `docs/research/model-eval/summary.json` (C = `qwen__qwen3-coder-next`, Q = `qwen__qwen3.8-27b`, B = 12):

1. **Recall:** `(recall_mean[C] − recall_mean[Q]) × B = (0.9167 − 1.0000) × 12 = −1.00 ≤ 1` → **PASS**. Q misses at most one more seeded bug on average — in fact it misses none, and catches `go-map-race` (line 44) in 3/3 runs where C missed it in 3/3.
2. **Noise:** C's mean unexpected error+warning per run = 13/42 = 0.3095 (≠ 0, so the fallback clause does not apply). Q's = 16/42 = 0.3810. Is `0.3810 ≤ 1.25 × 0.3095 = 0.3869`? Yes → **PASS** (margin ≈ 0.006, i.e. within half a finding per 40 runs).
3. **Real-diff precision:** `actionable_rate[C] − actionable_rate[Q] = 0.00 − 0.00 = 0.00 ≤ 0.10` → **PASS** (tie at 0 actionable out of 38 vs 32 labeled findings).
4. **Reliability:** Q has 0 non-`ok` runs (45/45 ok) ≤ 2 → **PASS**.

**All four conditions hold → adopt `qwen/qwen3.8-27b` as stet's default model.**

Where Q beats C outright: recall (condition 1: 1.00 vs 0.92 — the only strict win, but on the hardest seeded bug). Condition 3 is a tie. On condition 2 Q is slightly *noisier* (0.381 vs 0.310 error+warning per run), though within threshold, and it flags more findings on clean cases (9 vs 3). Speed favors C (9.9 vs 14.6 s/case; 249 vs 580 s per real ref) but is excluded from the rule; the 27B model's ~16 GB footprint also fits alongside OpenJev, which the 80B MoE's ~45 GB cannot — the operational conflict that motivated this benchmark disappears.

**Small-sample caveat:** with 12 synthetic bugs, each bug ≈ 8 recall points, so the recall verdict rests on one case × 3 runs; real-diff precision is a single blind LLM-judged labeling of 70 findings; treat the noise-condition margin (0.381 vs 0.387) as a coin-flip-level difference, not a measured property.

---

## Follow-ups

- **Operator (not this task):** switch `.review/config.toml` to `model = "qwen/qwen3.8-27b"` (with the preset/thinking-off setup verified in Step 0). Per the rule this is unconditional adoption; OpenJev can restart immediately.
- Optional: keep `qwen/qwen3-coder-next` as a manual "deep review" model for operator wave review / pre-merge when OpenJev is stopped — it is faster and matched 27b everywhere except the map race, but nothing in this data shows it finding more real issues.
- stet#2/#3 hardening is validated by the go-map-race wall-clock fix (806 s → ~14 s), but coder-next still hallucinates findings on that case; a stricter "verify the claim exists at the cited lines" pass (System One or self-check) would attack the 60/70 false-positive rate both models share.
- Re-run this benchmark when either model updates, when thinking-capable 27b inference becomes affordable at stet's output cap, or with a larger/judge-independent real-diff set.
