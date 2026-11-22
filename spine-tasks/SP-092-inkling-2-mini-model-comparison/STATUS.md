# SP-092: Compare thinkingmachines/inkling-2-mini vs qwen/qwen3.8-27b — Status

**Current Step:** Step 2
**Status:** 🟡 In Progress (manual Cursor execution)
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
**Status:** 🔄 In Progress

- [ ] Baseline loaded and run under same harness flags
- [ ] Results committed

---

### Step 3: Blind-judge real-diff findings
**Status:** ⬜ Not Started

- [ ] `blind.tsv` / `blind-key.json` exported
- [ ] `real-labels.tsv` labeled without opening the key; committed

---

### Step 4: Score
**Status:** ⬜ Not Started

- [ ] `summary.json` written

---

### Step 5: Write the document
**Status:** ⬜ Not Started

- [ ] Comparison doc with `## Recommendation` applying the pre-registered rule

---

### Step 6: Restore LM Studio
**Status:** ⬜ Not Started

- [ ] `qwen/qwen3.8-27b` reloaded at 65536 / parallel 1 / MTP off

---

### Step 7: Testing & Verification
**Status:** ⬜ Not Started

- [ ] Contract `testCommand` passing
- [ ] Nothing outside File Scope changed

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

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-11-22 | Task staged | PROMPT.md and STATUS.md created after operator Step 0 smoke |
| 2026-11-22 | Spine batch aborted | `20261122T021336-7f28` archived; manual Cursor execution |
| 2026-11-22 | Step 0 preflight | Smoke/probe OK; benchmark failed expected; started Step 1 |
