# SP-092: Compare thinkingmachines/inkling-2-mini vs qwen/qwen3.8-27b — Status

**Current Step:** Complete
**Status:** ✅ Complete (manual Cursor execution)
**Last Updated:** 2026-11-22
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

### Step 0: Preflight
**Status:** ✅ Complete (2026-11-22, manual)

- [x] Tool versions, estimates, real-ref SHAs recorded in `preflight.txt`
- [x] Inkling preset: **no saved file** under `user-concrete-model-default-config/`; operator confirmed Reasoning Effort = none and budget off in UI; smoke residual (~50–56 tokens) matches effort=none pattern
- [x] Qwen 27b Enable Thinking still false (`ext.virtualModel.customField.qwen.qwen3.827b.enableThinking = False`)
- [x] Inkling smoke: HTTP 200, content=`OK`, finish stop, reasoning_tokens=56
- [x] Inkling reasoning probe: content=`[]`, finish stop, reasoning_tokens=44 (no `<think>` in content)
- [x] Optional `stet benchmark` attempted; failed as expected (10s openai client timeout)

---

### Step 1: Run thinkingmachines/inkling-2-mini
**Status:** ✅ Complete (2026-11-22)

- [x] All 14 cases × 3 runs + 3 real refs recorded under `thinkingmachines__inkling-2-mini` (63/63 ok after retry)
- [x] `--retry-failed` once; first attempt of real `3ee3055f…` failed (`RAG resolve… signal: killed`), retry ok 1 finding in 1280s
- [x] Results committed (`6b67e5d`)

---

### Step 2: Run qwen/qwen3.8-27b
**Status:** ✅ Complete (2026-11-22)

- [x] Baseline loaded and run under same harness flags (63/63 ok)
- [x] Results committed (`4b04866`)

---

### Step 3: Blind-judge real-diff findings
**Status:** ✅ Complete (2026-11-22)

- [x] `blind.tsv` / `blind-key.json` exported
- [x] `real-labels.tsv` labeled without opening the key; committed (`e5034f0`) — 43 findings (42 Qwen + 1 Inkling); 0 actionable

---

### Step 4: Score
**Status:** ✅ Complete (2026-11-22)

- [x] `summary.json` written; `--check` regenerates exactly

---

### Step 5: Write the document
**Status:** ✅ Complete (2026-11-22)

- [x] Comparison doc with `## Recommendation`: keep `qwen/qwen3.8-27b` (recall FAIL 1.50 > 1)

---

### Step 6: Restore LM Studio
**Status:** ✅ Complete (2026-11-22)

- [x] `qwen/qwen3.8-27b` reloaded at 65536 / parallel 1 / MTP off; OpenJev may restart

---

### Step 7: Testing & Verification
**Status:** ✅ Complete (2026-11-22)

- [x] Contract `testCommand` passing (92 harness tests; summary check ok; Recommendation present)
- [x] Harness bug fix: validate `--systemone-base-url` only when critic or rules-selector uses it (Amendment)
- [x] Nothing outside File Scope changed (git status clean after commits)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| Inkling residual reasoning at effort=none (~50–250 tokens) | Controlled confound; documented in PROMPT Amendments | Step 0 |
| Reasoning budget=0 empties content | Forbidden in checklist | Step 0 |
| `stet benchmark` openai 10s timeout fails on Inkling | Optional; wall times from model_eval primary | Step 0 |
| No saved Inkling preset JSON under user-concrete-model-default-config | Proceeding on operator UI confirmation + smoke; please Save As default in LM Studio so reloads keep effort=none | Step 0 |
| Spine batch `20261122T021336-7f28` aborted for manual Cursor run (worker SIGTERM ×3) | Archived | Execution Log |
| SP-091 unconditional systemone URL validate broke `test_systemone_url_ignored_without_critic` | Fixed in `scripts/model_eval.py`; Amendment | Step 7 |

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-11-22 | Task staged | PROMPT.md and STATUS.md created after operator Step 0 smoke |
| 2026-11-22 | Spine batch aborted | `20261122T021336-7f28` archived; manual Cursor execution |
| 2026-11-22 | Step 0 preflight | Smoke/probe OK; benchmark failed expected; started Step 1 |
| 2026-11-22 | Steps 1–4 | Both models 63/63; labels + summary; recall rule FAIL |
| 2026-11-22 | Steps 5–7 | Keep-qwen doc; LM Studio restored; contract green; harness URL validate fix |
