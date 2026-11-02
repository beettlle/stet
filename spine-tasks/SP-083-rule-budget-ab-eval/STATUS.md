# SP-083: Rule-budget A/B eval and data-driven default — Status

**Current Step:** Step 2
**Status:** 🔄 Step 2 scoring
**Last Updated:** 2026-11-02
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

### Step 0: Preflight
**Status:** ✅ Complete (2026-11-02)

- [x] Versions, cases, rules count recorded (`docs/research/rules-budget-ab/preflight.txt`)
- [x] LM Studio smoke request OK (HTTP 200, "OK", finish_reason stop)

---

### Step 1: Run the three variants
**Status:** ✅ Complete (2026-11-02)

- [x] 1000, 2000, default runs complete; retries recorded; results committed
  - 21/21 `ok` per variant, exit 0, single invocation each (no budget reached, `--retry-failed` not needed)
  - Rule digests confirmed in prompt: dropped-section warnings show budget 1000/2000/8192 respectively

---

### Step 2: Score, decide, and set the default
**Status:** ⬜ Not Started

- [ ] Three summaries
- [ ] Note with `## Decision`; cap and docs updated if the rule says so
- [ ] Results posted to #11 (URL below)

---

### Step 3: Testing & Verification
**Status:** ⬜ Not Started

- [ ] Contract `testCommand` passes
- [ ] LM Studio restored

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-11-02 | Preflight: 7 cases copied to /tmp/sp083-cases; 41 rule files; model reloaded per fixed conditions; smoke HTTP 200 | Recorded in preflight.txt |
| 2026-11-02 | Variant 1000: 21/21 ok, exit 0, 46.7s max case | results-1000/ committed |
| 2026-11-02 | Variant 2000: 21/21 ok, exit 0 | results-2000/ committed |
| 2026-11-02 | Variant default (8192): 21/21 ok, exit 0 | results-default/ committed |

---

## Blockers

*None*
