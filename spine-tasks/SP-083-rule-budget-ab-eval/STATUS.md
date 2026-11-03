# SP-083: Rule-budget A/B eval and data-driven default — Status

**Current Step:** Step 3
**Status:** 🔄 Step 3 testing & verification
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
**Status:** ✅ Complete (2026-11-02)

- [x] Three summaries (`summary-{1000,2000,default}.json`, each regenerates from its results with `score`)
- [x] Note with `## Decision` (`docs/research/rules-budget-ab.md`); cap changed: `maxReviewRuleTokens` 8192 → 1000, `TestReviewRuleBudget` row updated, three doc mentions show the new clamp
- [x] Results posted to #11: https://github.com/beettlle/stet/issues/11#issuecomment-6591663565

Decision trace: R(1000)=3.33, R(2000)=3.33, R(default)=3.00 → R*=3.33, smallest budget within 1 case of best = 1000 → cap := 1000. C(1000)=1.44 (smallest; >1.0 flag not triggered).

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
| `cli/internal/config/config.go` line 29 comment still says "clamped to [1000, 8192]" — file is outside this task's File Scope (cli/internal/config/** in mustNotChange) | Logged as tech debt | spine-tasks/CONTEXT.md |

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-11-02 | Preflight: 7 cases copied to /tmp/sp083-cases; 41 rule files; model reloaded per fixed conditions; smoke HTTP 200 | Recorded in preflight.txt |
| 2026-11-02 | Variant 1000: 21/21 ok, exit 0, 46.7s max case | results-1000/ committed |
| 2026-11-02 | Variant 2000: 21/21 ok, exit 0 | results-2000/ committed |
| 2026-11-02 | Variant default (8192): 21/21 ok, exit 0 | results-default/ committed |
| 2026-11-02 | Scored all variants; decision rule → v*=1000; cap 8192→1000; docs updated; posted to #11 | comment 6591663565 |

---

## Blockers

*None*
