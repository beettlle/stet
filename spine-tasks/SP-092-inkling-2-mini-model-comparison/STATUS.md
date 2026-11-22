# SP-092: Compare thinkingmachines/inkling-2-mini vs qwen/qwen3.8-27b — Status

**Current Step:** Not Started
**Status:** 🔵 Ready for Execution
**Last Updated:** 2026-11-22
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

### Step 0: Preflight
**Status:** ⬜ Not Started

- [ ] Tool versions, estimates, real-ref SHAs recorded in `preflight.txt`
- [ ] Inkling preset saved with Reasoning Effort = none and budget off
- [ ] Qwen 27b Enable Thinking still false
- [ ] Inkling smoke: HTTP 200, non-empty content, finish stop (residual reasoning recorded)
- [ ] Inkling reasoning probe: content usable as `[]` (no `<think>` leak)
- [ ] Optional `stet benchmark` attempted; failure allowed and logged

---

### Step 1: Run thinkingmachines/inkling-2-mini
**Status:** ⬜ Not Started

- [ ] All 14 cases × 3 runs + 3 real refs recorded under `thinkingmachines__inkling-2-mini`
- [ ] `--retry-failed` once; remaining non-`ok` noted in Discoveries
- [ ] Results committed

---

### Step 2: Run qwen/qwen3.8-27b
**Status:** ⬜ Not Started

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

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-11-22 | Task staged | PROMPT.md and STATUS.md created after operator Step 0 smoke |
