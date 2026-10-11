# Task: SP-039 — Benchmark qwen3-coder-next vs qwen3.8-27b and recommend a stet model

**Created:** 2026-10-10
**Size:** M

## Review Level: 1

**Assessment:** Measurement and a research document; no production code. Long-running (≈2–3 h of LM Studio time) and needs exclusive use of LM Studio, so it runs alone.
**Score:** 2/8 — Blast radius: 0, Pattern novelty: 1, Security: 0, Reversibility: 1

## Mission

`qwen/qwen3-coder-next` (80B MoE, 44.9 GB) does not fit in memory while OpenJev runs; `qwen/qwen3.8-27b` (27B, 16.1 GB) does. Measure what stet loses, if anything, by reviewing with the 27B model, and write a recommendation using a decision rule fixed **before** any results exist (below).

Use the SP-037 harness (`scripts/model_eval.py`) and the SP-038 fixtures (`scripts/model-eval-cases/`, 12 seeded bugs + 2 clean cases), plus three real stet commits.

### Operator checklist (done by the human before `spine batch start SP-039`)

The worker cannot do these; if Step 0 shows any of them is not done, record a Blocker and stop.

1. OpenJev stopped (frees memory for `qwen3-coder-next`).
2. No other spine batch running; nothing else using LM Studio.
3. LM Studio per-model presets set to these exact values on **both** models (stet only sends temperature and `max_tokens`; everything else comes from these presets). Set each one explicitly in the UI so the saved config files carry the same keys; Step 0 compares them:
   - Sampling: Top K 20 (on), Top P 0.9 (on), Min P off, Repeat Penalty off
   - Settings: Context Overflow "Stop at Limit", Limit Response Length off, Reasoning Parsing on
   - No draft model, System Prompt empty, Structured Output off, default Jinja prompt template
   - Load tab: Max Concurrent Predictions 1
   These match `qwen/qwen3-coder-next`'s current preset; on 2026-10-10 `qwen/qwen3.8-27b` had Top P 0.8, Min P 0.5 (on), and no Top K / Repeat Penalty keys, so it needs changing.
4. Speculative decoding off at load time: SP-039 passes `--no-speculative-draft-mtp` on every `lms load`. Do not load either model by hand (UI, JIT, or `lms`) while the batch runs.
5. Thinking off for `qwen/qwen3.8-27b`: its preset sets Enable Thinking off (`ext.virtualModel.customField.qwen.qwen3.827b.enableThinking` = `false`). Step 0 verifies it.
6. LM Studio server running at `http://127.0.0.1:1234`.

### Fixed run conditions

- Harness-pinned stet env (see SP-037; amended 2026-10-11 to match `.review/config.toml`): temperature 0.1, `max_completion_tokens` 4096, context 65536, strictness `default`, critic / rules selector / rules annotate off.
- stet build: `main` at or after `b0d43f8` (issue #3 truncation handling, issue #2 compact schema). Record `stet --version` and the commit in `preflight.txt`.
- LM Studio load: `lms load <model> -c 65536 --parallel 1 --no-speculative-draft-mtp -y` (matches `.review/config.toml` `num_ctx`), one model loaded at a time. MTP drafting must stay off: on `qwen3_next` under MLX it fails every request ("Speculative decoding is not supported for batched MLX models" with parallel > 1, and "requires a trimmable prompt cache (got ArraysCache)" after a reload, because Qwen3-Next's hybrid-attention cache cannot be trimmed).
- `qwen/qwen3.8-27b` is a reasoning model, run with thinking **off** (operator checklist item 5). With thinking on, its reasoning used the whole output cap (see Amendments). Report output-token usage per model where the stderr logs or LM Studio usage allow.
- Fixture cases: 3 runs per model. Real refs: 1 run per model each:
  - `3ee3055f13cf00b5cb4e182f169829b78961743e` — SP-034 step 1 (config key/env, `main.go`, `run.go`)
  - `2326c0487a36e9b306edbed7da91dda1b536d00b` — SP-033 step 3 (shared System One client, `main.go`)
  - `e9b2e61f637cf165c630061c6d8f89c5e9f39404` — SP-033 step 2 (selector wiring, `review.go`, `run.go`)
- Model order: `qwen/qwen3-coder-next` first, then `qwen/qwen3.8-27b`.

### Decision rule (pre-registered; apply mechanically from `summary.json`)

Let C = `qwen__qwen3-coder-next`, Q = `qwen__qwen3.8-27b`, B = number of seeded bugs (12).

1. **Recall:** `(recall_mean[C] − recall_mean[Q]) × B ≤ 1` (Q misses at most one more seeded bug on average).
2. **Noise:** Q's mean unexpected `error`+`warning` findings per run ≤ 1.25 × C's (if C's is 0, Q's must be ≤ 1.0).
3. **Real-diff precision:** `actionable_rate[C] − actionable_rate[Q] ≤ 0.10`.
4. **Reliability:** Q has ≤ 2 non-`ok` runs in total.

- All four hold → **Adopt `qwen/qwen3.8-27b` as stet's default model.**
- Otherwise → **Split:** `qwen/qwen3.8-27b` day-to-day (OpenJev running); `qwen/qwen3-coder-next` for operator wave review and pre-merge review (OpenJev stopped). List which seeded bugs/categories Q missed and which conditions failed.
- If Q beats C on conditions 1–3, say so explicitly.
- Speed (`seconds_per_case_mean`, `stet benchmark` tokens/s) is reported but is not part of the rule.
- State the small-sample caveat: each seeded bug is ≈ 8 recall points; real-diff precision is LLM-judged.

## Dependencies

- **Task:** SP-037 (harness)
- **Task:** SP-038 (fixtures)
- **External:** LM Studio with both models downloaded (`lms ls`), `lms` CLI on PATH, `stet` on PATH, operator checklist above

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md` — "Stet in spine (dogfooding)", "Phase 7 — Model benchmark"

**Tier 3 (reference docs):**

- `scripts/model_eval.py` (`--help` for each subcommand) and `spine-tasks/SP-037-model-eval-harness/PROMPT.md` (results layout, metrics)
- `spine-tasks/SP-038-model-eval-fixtures/PROMPT.md` (case list)
- `docs/efficacy-tests.md` — A1, A4, H1, H2
- `docs/research/systemone-rules-compliance.md` — house style for research docs

## Environment

- **Workspace:** repo root
- **Services required:** LM Studio at `http://127.0.0.1:1234` (exclusive use), OpenJev **stopped**

## File Scope

- `docs/research/model-comparison-qwen3-coder-next-vs-qwen3.8-27b.md` (new)
- `docs/research/model-eval/**` (new: `results/`, `summary.json`, `blind.tsv`, `blind-key.json`, `real-labels.tsv`, `preflight.txt`)
- `scripts/model_eval.py`, `scripts/tests/test_model_eval.py` — **bug fixes only**, each with a regression test and an Amendment entry

## Contract

| Field | Value |
|-------|-------|
| testCommand | `python3 -m pytest scripts/tests/test_model_eval.py -q && python3 scripts/model_eval.py score --cases scripts/model-eval-cases --results docs/research/model-eval/results --labels docs/research/model-eval/real-labels.tsv --blind-key docs/research/model-eval/blind-key.json --check docs/research/model-eval/summary.json && grep -q '^## Recommendation' docs/research/model-comparison-qwen3-coder-next-vs-qwen3.8-27b.md` |
| fileScopeMustChange | `docs/research/model-comparison-qwen3-coder-next-vs-qwen3.8-27b.md`, `docs/research/model-eval/summary.json`, `docs/research/model-eval/real-labels.tsv` |
| fileScopeMustNotChange | `cli/**`, `extension/**`, `.review/**`, `scripts/model-eval-cases/**` |
| completionCriteria | Both models have results for all 14 cases × 3 runs and all 3 real refs (non-`ok` runs retried once and reported); `summary.json` regenerates exactly; the doc applies the decision rule verbatim and ends with an explicit recommendation |

No stet contract gate: this task swaps the loaded LM Studio model, and its diff is Markdown/JSON/TSV/logs only.

## Steps

### Step 0: Preflight

Write all output to `docs/research/model-eval/preflight.txt`.

- [ ] `lms ls`, `lms ps`, `stet --version`, `python3 scripts/model_eval.py --help`, `git rev-parse HEAD`
- [ ] `lms load qwen/qwen3-coder-next --estimate-only -c 65536` and the same for `qwen/qwen3.8-27b`; if coder-next's estimate does not fit, Blocker "OpenJev still running?" and stop
- [ ] Verify the three real-ref SHAs exist (`git cat-file -e <sha>^{commit}`)
- [ ] **Preset parity check** (operator checklist item 3). Run this and append its output to `preflight.txt`; on exit 1, Blocker "LM Studio presets differ: <lines>" and stop. Min P and Repeat Penalty count as off when the key is absent or unchecked.

  ```bash
  python3 - <<'EOF'
  import json, os
  base = os.path.expanduser("~/.lmstudio/.internal/user-concrete-model-default-config/qwen")
  want = {
      "llm.prediction.topKSampling": 20,
      "llm.prediction.topPSampling": {"checked": True, "value": 0.9},
      "llm.prediction.contextOverflowPolicy": "stopAtLimit",
      "llm.load.numParallelSessions": 1,
  }
  off_or_absent = ["llm.prediction.minPSampling", "llm.prediction.repeatPenalty"]
  problems = []
  for name in ("qwen3-coder-next", "qwen3.8-27b"):
      with open(os.path.join(base, name + ".json")) as fh:
          cfg = json.load(fh)
      fields = {f["key"]: f["value"] for sec in ("operation", "load") for f in cfg[sec]["fields"]}
      for key, value in want.items():
          if fields.get(key) != value:
              problems.append(f"{name}: {key} = {fields.get(key)!r}, want {value!r}")
      for key in off_or_absent:
          if key in fields and fields[key].get("checked"):
              problems.append(f"{name}: {key} is on ({fields[key]!r})")
      for key in fields:
          if "speculative" in key.lower() or "draft" in key.lower():
              problems.append(f"{name}: unexpected {key} = {fields[key]!r}")
  thinking_key = "ext.virtualModel.customField.qwen.qwen3.827b.enableThinking"
  with open(os.path.join(base, "qwen3.8-27b.json")) as fh:
      cfg = json.load(fh)
  fields = {f["key"]: f["value"] for sec in ("operation", "load") for f in cfg[sec]["fields"]}
  if fields.get(thinking_key) is not False:
      problems.append(f"qwen3.8-27b: {thinking_key} = {fields.get(thinking_key)!r}, want False")
  print("\n".join(problems) or "preset parity OK")
  raise SystemExit(1 if problems else 0)
  EOF
  ```

- [ ] **Smoke request per model.** For each model: `lms unload --all && lms load <model> -c 65536 --parallel 1 --no-speculative-draft-mtp -y`, then `curl -s -w '\nHTTP %{http_code}\n' http://127.0.0.1:1234/v1/chat/completions -H 'Content-Type: application/json' -d '{"model":"<model>","messages":[{"role":"user","content":"Reply with OK."}],"temperature":0.1,"max_tokens":<N>}'` with `N` = 64 for `qwen/qwen3-coder-next` and 512 for `qwen/qwen3.8-27b`. Pass = HTTP 200 and `finish_reason` `"stop"`. HTTP 5xx, "Channel Error", or "Speculative decoding" in the body → Blocker "MTP or speculative decoding still active for <model>" and stop. Append both responses to `preflight.txt`.
- [ ] **Reasoning probe** (27b still loaded from the smoke request): send `{"model":"qwen/qwen3.8-27b","messages":[{"role":"user","content":"Reply with the JSON array [] only."}],"temperature":0.1,"max_tokens":512}` to the same endpoint. Expected pass (thinking off): `message.reasoning_content` absent or empty, `usage.completion_tokens_details.reasoning_tokens` absent or 0, and `message.content` free of `<think>`. If reasoning content or reasoning tokens are present, Blocker "thinking still on for qwen3.8-27b" and stop. If `<think>` text appears in `content`, Blocker "enable Reasoning Parsing for qwen3.8-27b" and stop (stet would fail to parse findings). Record the result and the reasoning-token count.

### Step 1: Run qwen/qwen3-coder-next

- [ ] `lms unload --all && lms load qwen/qwen3-coder-next -c 65536 --parallel 1 --no-speculative-draft-mtp -y`; confirm with `lms ps` (expect `parallel` 1, context 65536)
- [ ] `mkdir -p docs/research/model-eval/results/qwen__qwen3-coder-next && STET_PROVIDER=openai STET_OPENAI_BASE_URL=http://127.0.0.1:1234/v1 stet benchmark --model qwen/qwen3-coder-next --warmup > docs/research/model-eval/results/qwen__qwen3-coder-next/benchmark.txt 2>&1` (`score` only reads `cases/` and `real/` under each model tag)
- [ ] Repeat until it exits 0 (exit 3 = budget reached, re-invoke; each invocation ≤ 25 min so the lane never looks stalled):
  `python3 scripts/model_eval.py run --model qwen/qwen3-coder-next --cases scripts/model-eval-cases --out docs/research/model-eval/results --runs 3 --real-ref 3ee3055f13cf00b5cb4e182f169829b78961743e --real-ref 2326c0487a36e9b306edbed7da91dda1b536d00b --real-ref e9b2e61f637cf165c630061c6d8f89c5e9f39404 --real-runs 1 --max-minutes 25`
- [ ] Re-run once with `--retry-failed`; record remaining non-`ok` runs in STATUS Discoveries
- [ ] Commit results (`docs(SP-039): coder-next results`)

### Step 2: Run qwen/qwen3.8-27b

- [ ] Same as Step 1 with `qwen/qwen3.8-27b` (tag `qwen__qwen3.8-27b`), loading with `lms unload --all && lms load qwen/qwen3.8-27b -c 65536 --parallel 1 --no-speculative-draft-mtp -y`
- [ ] Commit results

### Step 3: Blind-judge real-diff findings

- [ ] `python3 scripts/model_eval.py export-blind --results docs/research/model-eval/results --out docs/research/model-eval/blind.tsv --key docs/research/model-eval/blind-key.json`
- [ ] Copy `blind.tsv` to `real-labels.tsv` and label every row **without opening `blind-key.json`**. For each row read the code at that ref (`git show <ref>:<file>`, ±20 lines around `line`) and the commit diff (`git show <ref> -- <file>`). Labels:
  - `actionable` — a real defect or a clearly worthwhile change in the diff
  - `false_positive` — the claimed problem does not exist
  - `wrong_suggestion` — real concern, but the suggested fix is wrong or harmful
  - `out_of_scope` — about code not changed by the commit, or pure style preference
  Put a one-line reason in `notes`. Commit `real-labels.tsv` before scoring (`docs(SP-039): blind labels`).

### Step 4: Score

- [ ] `python3 scripts/model_eval.py score --cases scripts/model-eval-cases --results docs/research/model-eval/results --labels docs/research/model-eval/real-labels.tsv --blind-key docs/research/model-eval/blind-key.json --out docs/research/model-eval/summary.json`

### Step 5: Write the document

- [ ] `docs/research/model-comparison-qwen3-coder-next-vs-qwen3.8-27b.md` with sections:
  - **Summary** — the recommendation in 2–3 sentences
  - **Setup** — models and sizes, LM Studio load settings and the operator presets (as a controlled confound), pinned stet env, reasoning probe result, harness commands, stet version/commit
  - **Results** — tables per model: recall (mean, all-runs, some-runs) and per category; seeded bugs each model missed (case ids); unexpected findings by severity and on clean cases; repeatability; reliability; seconds per case and per real ref; `stet benchmark` tokens/s; real-diff label counts and actionable rate
  - **Pre-fix baseline** — short comparison of `docs/research/model-eval/baseline-pre-fix/results/qwen__qwen3-coder-next` (stet before `b0d43f8`, 32k context, 8192 cap) with the new coder-next run: recall per seeded case, `go-map-race` (baseline run 2 took 806 s with 0 findings after an `evidence_lines` counting loop), seconds per case, and completion tokens where the logs show them. Score the baseline by passing a temporary results dir to `score --out /tmp/...`; do not commit a second `summary.json`. This is the before/after measurement issue #2 asks for
  - **Threats to validity** — 12 synthetic bugs, single author, LLM-as-judge for real diffs, temperature 0.1 sampling variance; sampling parity was verified from the saved LM Studio presets in Step 0, not observed per request; MTP speculative decoding disabled for both models; 27b ran with thinking off, so this does not measure 27b with reasoning; the pre-fix baseline differs from the final run in both the stet build and context size (32k vs 64k), so baseline-vs-final real-diff differences cannot be attributed to either change alone
  - **`## Recommendation`** — apply the decision rule above condition by condition, with the numbers, then the outcome
  - **Follow-ups** — e.g. the `.review/config.toml` change the operator should make (do not make it here)

### Step 6: Restore LM Studio

- [ ] Restore the repo default: `lms unload --all && lms load qwen/qwen3-coder-next -c 65536 --parallel 1 --no-speculative-draft-mtp -y` (matches `.review/config.toml`); note in Execution Log that OpenJev can be restarted only if the recommendation allows it

### Step 7: Testing & Verification

> ZERO test failures allowed.

- [ ] Run Contract `testCommand` (harness tests, `summary.json` regenerates exactly, `## Recommendation` present)
- [ ] Confirm nothing outside File Scope changed (`git status`)

## Completion Criteria

- [ ] All steps complete
- [ ] Contract `testCommand` passes
- [ ] Recommendation is explicit and follows the pre-registered rule

## Git Commit Convention

- **Results / docs:** `docs(SP-039): description`
- **Harness bug fix:** `fix(SP-039): description`

## Do NOT

- Change the decision rule, run counts, models, real refs, or pinned settings after seeing results (if something forces a change, add an Amendment explaining why **before** re-running)
- Open `blind-key.json` before `real-labels.tsv` is committed
- Edit `.review/config.toml` or any LM Studio preset
- Load a model without `--no-speculative-draft-mtp`
- Run stet against the lane worktree itself (the harness uses temp repos/clones)
- Edit fixtures under `scripts/model-eval-cases/`
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)

- 2026-10-10 (operator, pre-run — no results exist yet): LM Studio setup fixes from the W0 batch (SP-035/037/038). (1) `qwen/qwen3-coder-next` failed every request with load-time MTP speculative decoding, which LM Studio enables by default and the Inference tab does not show ("not supported for batched MLX models" at parallel 3; "requires a trimmable prompt cache (got ArraysCache)" after a reload; the first successful request came only after a reload with MTP off); every `lms load` now passes `--no-speculative-draft-mtp`. (2) Max Concurrent Predictions set to 1 in both saved presets. (3) The two presets' sampling differed (27b: Top P 0.8, Min P 0.5 on, no Top K); the operator checklist now names exact values and Step 0 verifies them from the saved config files, plus a smoke request per model. (4) `qwen/qwen3.8-27b` confirmed as a reasoning model (`reasoning_content`). Decision rule, models, run counts, real refs, and pinned stet settings are unchanged.
- 2026-10-11 (operator, before the re-run; written after batch `20261011T174423-6acb` was aborted). **What happened:** with stet at `4e59e6e`, coder-next finished 45/45 `ok` (one real-ref timeout passed on `--retry-failed`). Its `go-map-race` run 2 took 806 s and returned 0 findings: the model emitted one valid finding, then counted `evidence_lines` from 44 to 1865 until the output cap, and stet's continuation rounds could not recover it (issue #2, issue #3). The 27b pass, with thinking on, needed 1713 s for its first case. In LM Studio, 2 of its first 3 requests used all 8191 output tokens on reasoning with empty `content`, and stet then sent continuation rounds from that empty content (issue #3 comment). At that rate the 27b pass would have taken 13–30 h and failed the reliability condition from per-review timeouts. **Changes before re-running:** (1) stet `ea2124f`: no continuation from empty or repetition-loop content, salvage of complete findings from a truncated array, and a per-hunk output budget of 2× the completion cap; (2) stet `14fce09`: compact single-line JSON, `cursor_uri` and `evidence_lines` dropped from the model-facing schema; (3) harness `b0d43f8`: pinned `max_completion_tokens` 4096 and context 65536 to match `.review/config.toml`, the configuration this recommendation is for, and every `lms load` uses `-c 65536`; (4) `qwen/qwen3.8-27b` runs with thinking off through its preset, and Step 0 now checks this and requires zero reasoning tokens. Both models are re-run from scratch on the new build. The pre-fix results (coder-next 45 runs, recall_mean 0.9167; one 27b thinking-on case) and the logs are kept in `docs/research/model-eval/baseline-pre-fix/` and are not scored into `summary.json`. Decision rule, models, run counts, and real refs are unchanged.
