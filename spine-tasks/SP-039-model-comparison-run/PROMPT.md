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
3. LM Studio per-model inference presets set **identically** for both models (stet only sends temperature and `max_tokens`; everything else comes from these presets): Repeat Penalty off (or 1.0), Top P 0.9, Top K 40, Min P off, Context Overflow "Stop at Limit", Limit Response Length off, Reasoning Parsing on, System Prompt empty, Structured Output off, default Jinja prompt template.
4. LM Studio server running at `http://127.0.0.1:1234`.

### Fixed run conditions

- Harness-pinned stet env (see SP-037): temperature 0.1, `max_completion_tokens` 8192, context 32768, strictness `default`, critic / rules selector / rules annotate off.
- LM Studio load: `lms load <model> -c 49152 --parallel 1 -y` (32k prompt + 8k output + margin), one model loaded at a time.
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
- [ ] `lms load qwen/qwen3-coder-next --estimate-only -c 49152` and the same for `qwen/qwen3.8-27b`; if coder-next's estimate does not fit, Blocker "OpenJev still running?" and stop
- [ ] Verify the three real-ref SHAs exist (`git cat-file -e <sha>^{commit}`)
- [ ] Load `qwen/qwen3.8-27b` (`lms unload --all && lms load qwen/qwen3.8-27b -c 49152 --parallel 1 -y`) and send one tiny chat request (`curl -s http://127.0.0.1:1234/v1/chat/completions -H 'Content-Type: application/json' -d '{"model":"qwen/qwen3.8-27b","messages":[{"role":"user","content":"Reply with the JSON array [] only."}],"temperature":0.1,"max_tokens":512}'`). Record whether `message.content` contains `<think>` and whether `message.reasoning_content` is present. If `<think>` text appears in `content`, Blocker "enable Reasoning Parsing for qwen3.8-27b" and stop (stet would fail to parse findings).

### Step 1: Run qwen/qwen3-coder-next

- [ ] `lms unload --all && lms load qwen/qwen3-coder-next -c 49152 --parallel 1 -y`; confirm with `lms ps`
- [ ] `mkdir -p docs/research/model-eval/results/qwen__qwen3-coder-next && STET_PROVIDER=openai STET_OPENAI_BASE_URL=http://127.0.0.1:1234/v1 stet benchmark --model qwen/qwen3-coder-next --warmup > docs/research/model-eval/results/qwen__qwen3-coder-next/benchmark.txt 2>&1` (`score` only reads `cases/` and `real/` under each model tag)
- [ ] Repeat until it exits 0 (exit 3 = budget reached, re-invoke; each invocation ≤ 25 min so the lane never looks stalled):
  `python3 scripts/model_eval.py run --model qwen/qwen3-coder-next --cases scripts/model-eval-cases --out docs/research/model-eval/results --runs 3 --real-ref 3ee3055f13cf00b5cb4e182f169829b78961743e --real-ref 2326c0487a36e9b306edbed7da91dda1b536d00b --real-ref e9b2e61f637cf165c630061c6d8f89c5e9f39404 --real-runs 1 --max-minutes 25`
- [ ] Re-run once with `--retry-failed`; record remaining non-`ok` runs in STATUS Discoveries
- [ ] Commit results (`docs(SP-039): coder-next results`)

### Step 2: Run qwen/qwen3.8-27b

- [ ] Same as Step 1 with `qwen/qwen3.8-27b` (tag `qwen__qwen3.8-27b`)
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
  - **Threats to validity** — 12 synthetic bugs, single author, LLM-as-judge for real diffs, temperature 0.1 sampling variance, preset parity depends on the operator
  - **`## Recommendation`** — apply the decision rule above condition by condition, with the numbers, then the outcome
  - **Follow-ups** — e.g. the `.review/config.toml` change the operator should make (do not make it here)

### Step 6: Restore LM Studio

- [ ] Restore the repo default: `lms unload --all && lms load qwen/qwen3-coder-next -c 65536 -y` (matches `.review/config.toml`); note in Execution Log that OpenJev can be restarted only if the recommendation allows it

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
- Run stet against the lane worktree itself (the harness uses temp repos/clones)
- Edit fixtures under `scripts/model-eval-cases/`
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
