# SP-083: Rule-budget A/B eval and data-driven default — Status

**Current Step:** Complete
**Status:** ✅ Complete — all steps done, contract green
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
**Status:** ✅ Complete (2026-11-02)

- [x] Contract `testCommand` passes (exit 0: gofmt, vet, rules tests, summary + Decision present, stet contract review)
  - Two review rounds triaged: fbf6835 fixed (test row pinned to literal 1000); 9a0f82a / 1cdbef8 dismissed `already_correct` (sufficiency answered by the pre-registered A/B); c85f49d + 694b485 dismissed `already_correct` (critic read removed minus lines — both rows are literal 1000 at HEAD; suite passes)
- [x] LM Studio restored (`lms unload --all && lms load qwen/qwen3.8-27b -c 65536 --parallel 1 --no-speculative-draft-mtp -y`)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| `cli/internal/config/config.go` line 29 comment still says "clamped to [1000, 8192]" — file is outside this task's File Scope (cli/internal/config/** in mustNotChange) | Logged as tech debt | spine-tasks/CONTEXT.md |
| stet critic reads removed minus lines as current code: after ad9f7e1 made both test rows literal `1000`, the reviewer re-emitted findings citing `want: maxReviewRuleTokens` (the removed line) and claimed "the test will fail" (it passes) | stet reviewer bug; dismissed `already_correct` with evidence | #11 comment 6591663565; findings 694b485/c85f49d |
| Dismissal restore did not carry c85f49d across a finished session (re-emitted identical finding id came back active, `dismissed: 0` in the new session) | stet behavior observation for follow-up | this session (.review/history.jsonl) |

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-11-02 | Preflight: 7 cases copied to /tmp/sp083-cases; 41 rule files; model reloaded per fixed conditions; smoke HTTP 200 | Recorded in preflight.txt |
| 2026-11-02 | Variant 1000: 21/21 ok, exit 0, 46.7s max case | results-1000/ committed |
| 2026-11-02 | Variant 2000: 21/21 ok, exit 0 | results-2000/ committed |
| 2026-11-02 | Variant default (8192): 21/21 ok, exit 0 | results-default/ committed |
| 2026-11-02 | Scored all variants; decision rule → v*=1000; cap 8192→1000; docs updated; posted to #11 | comment 6591663565 |
| 2026-11-02 | Contract round 1: 2 findings → 1 fixed (fbf6835), 1 dismissed (9a0f82a) | commits 68863c2 |
| 2026-11-02 | Contract round 2: 2 findings → 1 dismissed (1cdbef8), 1 fixed (c85f49d → ad9f7e1) | commits ad9f7e1 |
| 2026-11-02 | Contract round 3: stale minus-line findings dismissed; contract re-run exit 0 | CONTRACT_EXIT=0 |

---

## Blockers

*None*
