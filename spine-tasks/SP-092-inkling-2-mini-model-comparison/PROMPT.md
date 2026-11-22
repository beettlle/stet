# Task: SP-092 — Compare thinkingmachines/inkling-2-mini vs qwen/qwen3.8-27b and recommend a stet model

**Created:** 2026-11-22
**Size:** M
**Type:** llm

## Review Level: 1

**Risk:** Measurement and a research document only; no production code. Long-running (≈2–3 h of LM Studio time) and needs exclusive use of LM Studio, so it runs alone.
**Score:** 2/8 — Blast radius: 0, Pattern novelty: 1, Security: 0, Reversibility: 1

## Mission

`qwen/qwen3.8-27b` is stet's current default review model. `thinkingmachines/inkling-2-mini` (26.4B, ~18 GB) is a candidate replacement already available in LM Studio. Measure what stet gains or loses by reviewing with Inkling, and write a recommendation using a decision rule fixed **before** any results exist (below).

Reuse the SP-037 harness (`scripts/model_eval.py`) and SP-038 fixtures (`scripts/model-eval-cases/`, 12 seeded bugs + 2 clean cases), plus the same three real stet commits as SP-039. Write all artifacts under a **new** results root so SP-039's `docs/research/model-eval/` is not overwritten.

### Operator checklist (done by the human before `spine batch start SP-092`)

The worker cannot do these; if Step 0 shows any of them is not done, record a Blocker and stop.

1. OpenJev / other GPU consumers stopped (exclusive LM Studio).
2. No other spine batch running; nothing else using LM Studio.
3. LM Studio per-model presets set to these exact values on **both** models (stet only sends temperature and `max_tokens`; everything else comes from these presets). Set each one explicitly in the UI and **save** the per-model config so files exist under `~/.lmstudio/.internal/user-concrete-model-default-config/`:
   - Sampling: Top K 20 (on), Top P 0.9 (on), Min P off, Repeat Penalty off
   - Settings: Context Overflow "Stop at Limit", Limit Response Length off, Reasoning Parsing on
   - No draft model, System Prompt empty, Structured Output off, default Jinja prompt template
   - Load tab: Max Concurrent Predictions 1
4. Speculative decoding off at load time: every `lms load` passes `--no-speculative-draft-mtp`. Do not load either model by hand (UI, JIT, or `lms`) while the batch runs.
5. **Inkling reasoning controls (register before results):**
   - Reasoning Effort = **none** (`model.yaml` custom field `reasoningEffort`)
   - Reasoning budget **off** (do **not** set budget to 0 — that wiped `content` in Step 0 smoke and returned empty completions with `finish_reason: length`)
   - Expect **residual** `reasoning_content` / `reasoning_tokens` even at effort=none (observed ~50 on "OK", ~250 on a tiny review). That is not a Blocker if `content` is non-empty and `finish_reason` is `stop`. Record the counts in `preflight.txt`.
6. **Baseline** `qwen/qwen3.8-27b` keeps Enable Thinking **off** (same as SP-039).
7. LM Studio server running at `http://127.0.0.1:1234`.

### Fixed run conditions

- Harness-pinned stet env (SP-037 / `.review/config.toml`): temperature 0.1, `max_completion_tokens` 4096, context 65536, strictness `default`, critic / rules selector / rules annotate off.
- stet build: current `main` install on PATH. Record `stet --version` and `git rev-parse HEAD` in `preflight.txt`.
- LM Studio load: `lms load <model> -c 65536 --parallel 1 --no-speculative-draft-mtp -y`, one model at a time.
- Fixture cases: 3 runs per model. Real refs: 1 run per model each:
  - `3ee3055f13cf00b5cb4e182f169829b78961743e`
  - `2326c0487a36e9b306edbed7da91dda1b536d00b`
  - `e9b2e61f637cf165c630061c6d8f89c5e9f39404`
- Model order: candidate `thinkingmachines/inkling-2-mini` first, then baseline `qwen/qwen3.8-27b`.
- Results root: `docs/research/model-eval-inkling-2-mini/` (tags `thinkingmachines__inkling-2-mini` and `qwen__qwen3.8-27b`).

### Decision rule (pre-registered; apply mechanically from `summary.json`)

Let C = `qwen__qwen3.8-27b`, Q = `thinkingmachines__inkling-2-mini`, B = number of seeded bugs (12).

1. **Recall:** `(recall_mean[C] − recall_mean[Q]) × B ≤ 1` (Q misses at most one more seeded bug on average).
2. **Noise:** Q's mean unexpected `error`+`warning` findings per run ≤ 1.25 × C's (if C's is 0, Q's must be ≤ 1.0).
3. **Real-diff precision:** `actionable_rate[C] − actionable_rate[Q] ≤ 0.10`.
4. **Reliability:** Q has ≤ 2 non-`ok` runs in total.

- All four hold → **Adopt `thinkingmachines/inkling-2-mini` as stet's default model** (operator edits `.review/config.toml` after this task; do not edit it here).
- Otherwise → **Keep `qwen/qwen3.8-27b`**. List which seeded bugs/categories Q missed and which conditions failed.
- If Q beats C on conditions 1–3, say so explicitly.
- Speed (`seconds_per_case_mean`, optional `stet benchmark`) is reported but is **not** part of the rule.
- State the small-sample caveat: each seeded bug is ≈ 8 recall points; real-diff precision is LLM-judged; Inkling residual reasoning at effort=none is a controlled confound vs Qwen thinking-off (zero reasoning tokens).

## Dependencies

- **Task:** SP-037 (harness)
- **Task:** SP-038 (fixtures)
- **External:** LM Studio with both models downloaded (`lms ls`), `lms` CLI on PATH, `stet` on PATH, operator checklist above

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md` — Phase 7 and Phase 7b
- `spine-tasks/SP-039-model-comparison-run/PROMPT.md` — prior procedure and Amendments

**Tier 3 (reference docs):**

- `scripts/model_eval.py` (`--help` for each subcommand)
- `docs/research/model-comparison-qwen3-coder-next-vs-qwen3.8-27b.md` — decision-rule writeup house style
- `docs/efficacy-tests.md` — A1, A4, H1, H2

## Environment

- **Workspace:** repo root
- **Services required:** LM Studio at `http://127.0.0.1:1234` (exclusive use), OpenJev **stopped**

## File Scope

- `docs/research/model-comparison-qwen3.8-27b-vs-inkling-2-mini.md` (new)
- `docs/research/model-eval-inkling-2-mini/**` (new: `results/`, `summary.json`, `blind.tsv`, `blind-key.json`, `real-labels.tsv`, `preflight.txt`, per-model `benchmark.txt` if obtained)
- `scripts/model_eval.py`, `scripts/tests/test_model_eval.py` — **bug fixes only**, each with a regression test and an Amendment entry

## Contract

| Field | Value |
|-------|-------|
| testCommand | `python3 -m pytest scripts/tests/test_model_eval.py -q && python3 scripts/model_eval.py score --cases scripts/model-eval-cases --results docs/research/model-eval-inkling-2-mini/results --labels docs/research/model-eval-inkling-2-mini/real-labels.tsv --blind-key docs/research/model-eval-inkling-2-mini/blind-key.json --check docs/research/model-eval-inkling-2-mini/summary.json && grep -q '^## Recommendation' docs/research/model-comparison-qwen3.8-27b-vs-inkling-2-mini.md` |
| fileScopeMustChange | `docs/research/model-comparison-qwen3.8-27b-vs-inkling-2-mini.md`, `docs/research/model-eval-inkling-2-mini/summary.json`, `docs/research/model-eval-inkling-2-mini/real-labels.tsv` |
| fileScopeMustNotChange | `cli/**`, `extension/**`, `.review/**`, `scripts/model-eval-cases/**`, `docs/research/model-eval/**` |
| completionCriteria | Both models have results for all 14 cases × 3 runs and all 3 real refs (non-`ok` runs retried once and reported); `summary.json` regenerates exactly; the doc applies the decision rule verbatim and ends with an explicit recommendation |

No stet contract gate: this task swaps the loaded LM Studio model, and its diff is Markdown/JSON/TSV/logs only.

## Steps

### Step 0: Preflight

Write all output to `docs/research/model-eval-inkling-2-mini/preflight.txt`.

- [ ] `lms ls`, `lms ps`, `stet --version`, `python3 scripts/model_eval.py --help`, `git rev-parse HEAD`
- [ ] `lms load thinkingmachines/inkling-2-mini --estimate-only -c 65536` and the same for `qwen/qwen3.8-27b`
- [ ] Verify the three real-ref SHAs exist (`git cat-file -e <sha>^{commit}`)
- [ ] Confirm a saved Inkling preset exists under `~/.lmstudio/.internal/user-concrete-model-default-config/` (create/save from the UI if missing). Record the path and the `reasoningEffort` / thinking-related field values. If effort is not `none`, Blocker "Inkling Reasoning Effort not none" and stop.
- [ ] Confirm Qwen 27b Enable Thinking is still false in its saved preset (SP-039 key `ext.virtualModel.customField.qwen.qwen3.827b.enableThinking`).
- [ ] **Smoke request (Inkling).** `lms unload --all && lms load thinkingmachines/inkling-2-mini -c 65536 --parallel 1 --no-speculative-draft-mtp -y`, then:

  ```bash
  curl -s -w '\nHTTP %{http_code}\n' http://127.0.0.1:1234/v1/chat/completions \
    -H 'Content-Type: application/json' \
    -d '{"model":"thinkingmachines/inkling-2-mini","messages":[{"role":"user","content":"Reply with OK."}],"temperature":0.1,"max_tokens":256}'
  ```

  Pass = HTTP 200, `finish_reason` `"stop"`, and non-empty `message.content`. Append the response. Record `reasoning_tokens` (residual expected). Empty `content` → Blocker "Inkling returning empty content (check reasoning budget is off, not 0)" and stop.
- [ ] **Reasoning probe (Inkling).** Same endpoint with body `{"model":"thinkingmachines/inkling-2-mini","messages":[{"role":"user","content":"Reply with the JSON array [] only."}],"temperature":0.1,"max_tokens":512}`. Pass = non-empty `content` that is parseable as `[]` (or contains `[]` without `<think>` leaking into the final answer). Record residual reasoning token count. If `<think>` appears inside `content`, Blocker "enable Reasoning Parsing for inkling-2-mini" and stop.
- [ ] **Optional `stet benchmark` (Inkling).** Best-effort only:

  ```bash
  mkdir -p docs/research/model-eval-inkling-2-mini/results/thinkingmachines__inkling-2-mini
  STET_PROVIDER=openai STET_OPENAI_BASE_URL=http://127.0.0.1:1234/v1 \
    stet benchmark --model thinkingmachines/inkling-2-mini --warmup \
    > docs/research/model-eval-inkling-2-mini/results/thinkingmachines__inkling-2-mini/benchmark.txt 2>&1 \
    || echo "benchmark failed (known: openai client default 10s timeout vs residual reasoning TTFT)" \
    >> docs/research/model-eval-inkling-2-mini/results/thinkingmachines__inkling-2-mini/benchmark.txt
  ```

  Do **not** Blocker on benchmark failure. Primary speed metrics come from `model_eval.py` wall times.

### Step 1: Run thinkingmachines/inkling-2-mini

- [ ] Confirm Inkling still loaded (`lms ps`: parallel 1, context 65536)
- [ ] Repeat until exit 0 (exit 3 = budget reached, re-invoke; each invocation ≤ 25 min):

  ```bash
  python3 scripts/model_eval.py run \
    --model thinkingmachines/inkling-2-mini \
    --cases scripts/model-eval-cases \
    --out docs/research/model-eval-inkling-2-mini/results \
    --runs 3 \
    --real-ref 3ee3055f13cf00b5cb4e182f169829b78961743e \
    --real-ref 2326c0487a36e9b306edbed7da91dda1b536d00b \
    --real-ref e9b2e61f637cf165c630061c6d8f89c5e9f39404 \
    --real-runs 1 --max-minutes 25
  ```

- [ ] Re-run once with `--retry-failed`; record remaining non-`ok` runs in STATUS Discoveries
- [ ] Commit results (`docs(SP-092): inkling-2-mini results`)

### Step 2: Run qwen/qwen3.8-27b

- [ ] `lms unload --all && lms load qwen/qwen3.8-27b -c 65536 --parallel 1 --no-speculative-draft-mtp -y`
- [ ] Optional `stet benchmark --model qwen/qwen3.8-27b --warmup` into `.../results/qwen__qwen3.8-27b/benchmark.txt` (same best-effort rule as Step 0)
- [ ] Same `model_eval.py run` as Step 1 with `--model qwen/qwen3.8-27b`
- [ ] `--retry-failed` once; commit (`docs(SP-092): qwen3.8-27b baseline results`)

### Step 3: Blind-judge real-diff findings

- [ ] `python3 scripts/model_eval.py export-blind --results docs/research/model-eval-inkling-2-mini/results --out docs/research/model-eval-inkling-2-mini/blind.tsv --key docs/research/model-eval-inkling-2-mini/blind-key.json`
- [ ] Copy `blind.tsv` to `real-labels.tsv` and label every row **without opening `blind-key.json`**. For each row read the code at that ref (`git show <ref>:<file>`, ±20 lines around `line`) and the commit diff (`git show <ref> -- <file>`). Labels:
  - `actionable` — a real defect or a clearly worthwhile change in the diff
  - `false_positive` — the claimed problem does not exist
  - `wrong_suggestion` — real concern, but the suggested fix is wrong or harmful
  - `out_of_scope` — about code not changed by the commit, or pure style preference
  Put a one-line reason in `notes`. Commit `real-labels.tsv` before scoring (`docs(SP-092): blind labels`).

### Step 4: Score

- [ ] `python3 scripts/model_eval.py score --cases scripts/model-eval-cases --results docs/research/model-eval-inkling-2-mini/results --labels docs/research/model-eval-inkling-2-mini/real-labels.tsv --blind-key docs/research/model-eval-inkling-2-mini/blind-key.json --out docs/research/model-eval-inkling-2-mini/summary.json`

### Step 5: Write the document

- [ ] `docs/research/model-comparison-qwen3.8-27b-vs-inkling-2-mini.md` with sections:
  - **Summary** — the recommendation in 2–3 sentences
  - **Setup** — models and sizes, LM Studio load settings, Inkling effort=none + budget off (and residual reasoning), Qwen thinking off, pinned stet env, harness commands, stet version/commit, optional benchmark outcome
  - **Results** — tables per model: recall; unexpected findings; reliability; seconds per case / real ref; benchmark if present; real-diff label counts and actionable rate
  - **Threats to validity** — 12 synthetic bugs; LLM-as-judge; Inkling residual reasoning vs Qwen zero-reasoning; temperature 0.1 variance; `stet benchmark` 10s OpenAI client timeout may omit throughput numbers
  - **`## Recommendation`** — apply the decision rule condition by condition with numbers, then the outcome
  - **Follow-ups** — e.g. `.review/config.toml` change if adopting; optional fix for `stet benchmark` openai client timeout when `NewClient(..., nil)`

### Step 6: Restore LM Studio

- [ ] Restore the repo default: `lms unload --all && lms load qwen/qwen3.8-27b -c 65536 --parallel 1 --no-speculative-draft-mtp -y`; note in Execution Log that OpenJev can be restarted

### Step 7: Testing & Verification

> ZERO test failures allowed.

- [ ] Run Contract `testCommand` (harness tests, `summary.json` regenerates exactly, `## Recommendation` present)
- [ ] Confirm nothing outside File Scope changed (`git status`)

## Documentation Requirements

**Must Update:**

- `docs/research/model-comparison-qwen3.8-27b-vs-inkling-2-mini.md` — full comparison writeup *(also in File Scope)*

**Check If Affected:**

- `docs/research/model-comparison-qwen3-coder-next-vs-qwen3.8-27b.md` — reference only; do not rewrite
- `.review/config.toml` — operator may change after recommendation; not in this task

## Completion Criteria

- [ ] All steps complete
- [ ] Contract `testCommand` passes
- [ ] Recommendation is explicit and follows the pre-registered rule

## Git Commit Convention

- **Results / docs:** `docs(SP-092): description`
- **Harness bug fix:** `fix(SP-092): description`

## Do NOT

- Change the decision rule, run counts, models, real refs, or pinned settings after seeing results (if something forces a change, add an Amendment explaining why **before** re-running)
- Open `blind-key.json` before `real-labels.tsv` is committed
- Edit `.review/config.toml` or any LM Studio preset
- Set Inkling reasoning budget to 0
- Load a model without `--no-speculative-draft-mtp`
- Overwrite or edit `docs/research/model-eval/**` (SP-039 archive)
- Run stet against the lane worktree itself (the harness uses temp repos/clones)
- Edit fixtures under `scripts/model-eval-cases/`
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT — log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)

- 2026-11-22 (operator, pre-authoring Step 0): With default/high reasoning, a review-like probe spent the entire `max_tokens` budget on `reasoning_content` and returned empty `content` (`finish_reason: length`, ~183 s). Setting reasoning budget to **0** cleared `reasoning_tokens` but also returned empty `content` with `finish_reason: length` — unusable. Setting Reasoning Effort to **none** and turning the budget **off** restored usable output (`content=OK` / `[]`) with residual reasoning (~50–250 tokens). `stet benchmark` still fails under the openai client's default 10 s timeout (`llm.NewClient(..., nil)`); full reviews use `timeout = "30m"` from `.review/config.toml` and succeed. Decision rule, models, run counts, and real refs are fixed before any scored results.
- 2026-11-22 (manual Cursor): Contract `testCommand` failed on pre-existing harness regression from SP-091: `_run_options_from_args` always validated `--systemone-base-url`, so `test_systemone_url_ignored_without_critic` failed. Restored validate-only-when-critic-or-rules-selector (regression covered by existing test).
