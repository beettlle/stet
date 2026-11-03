# Research: Cursor-rule budget A/B — 1000 vs 2000 vs scaled default (8192)

**Task:** SP-083 · **Date:** 2026-11-02 · **Status:** Complete
**Question:** What does the Cursor-rule digest budget (`rules_max_tokens` / `STET_RULES_MAX_TOKENS`) buy on rule-targeted cases, and what should the default cap (`maxReviewRuleTokens`) be?

This re-runs the #11 A/B (comment of 2026-11-01) on the SP-082 rule-targeted cases, where each seeded defect violates one specific section of a rule file. The first A/B found no recall gain because no case in the old suite needed a rule, and the scaled default cost 2–6× latency.

---

## Summary

**Decision (pre-registered rule): cap the default budget at 1000 tokens** — `maxReviewRuleTokens` 8192 → 1000 in `cli/internal/rules/criteria.go`, so the scaled default is a flat 1000. Seeded rule-bug recall is equal at budgets 1000 and 2000 (R = 3.33 of 4 bugs per run) and lower at the scaled default (R = 3.00); the rule picks the smallest budget within one case of the best, which is 1000. Budget 1000 is also the cheapest (23.6 s/case vs 25.9) and had the fewest unexpected findings (0.95/run vs 1.43). Unlike the first A/B, the big budget's latency penalty did not reproduce on these small diffs (~9% slower, not 2–6×) — the decision rests on the recall rule, not latency.

---

## Setup

Full record: `preflight.txt` (versions, model load, smoke request, case list, rules count).

- **Repo:** HEAD `61301aa` (worktree lane-1); harness `scripts/model_eval.py` at HEAD.
- **stet:** `v1.4.0` (tag `14d6969`); cli/ changes after the tag (SP-077 status filtering, SP-080 doc wording, SP-081 critic default) do not touch the review-prompt rules path.
- **Model:** `qwen/qwen3.8-27b`, loaded `lms load qwen/qwen3.8-27b -c 65536 --parallel 1 --no-speculative-draft-mtp -y` after `lms unload --all`; smoke request HTTP 200.
- **Cases (7):** the 4 SP-082 seeded rule cases (`go-rule-context` §1.3, `go-rule-global-prefix` §2.1, `py-rule-membership` §2.3, `py-rule-naming` §3.4), the 2 rule-targeted clean controls (`go-rule-clean`, `py-rule-clean`), and `go-clean-refactor`.
- **Rules:** `--rules-dir .cursor/rules` — 41 `*.mdc` files, copied into each case repo's **before** commit (present in the tree, absent from the reviewed diff).
- **Variants:** `STET_RULES_MAX_TOKENS=1000`, `2000`, unset (scaled default = 15% of 65536 = 8192). Dropped-section warnings confirm what each run used: budget 1000 dropped 207 sections, 2000 dropped 191, 8192 dropped 88.
- **Runs:** 3 per case per variant (`--runs 3`), order 1000 → 2000 → default, each invocation `--max-minutes 25`. All three variants finished 21/21 `ok` in a single invocation (exit 0); `--retry-failed` was never needed.
- **Pinned env otherwise unchanged** (harness `PINNED_ENV`: temperature 0.1, max completion tokens 4096, context 65536, critic/selector/annotate off, strictness default, timeout 30m).

Commands:

```bash
python3 scripts/model_eval.py run --model qwen/qwen3.8-27b --cases /tmp/sp083-cases \
  --out docs/research/rules-budget-ab/results-1000 --runs 3 \
  --rules-dir .cursor/rules --rules-max-tokens 1000 --max-minutes 25
# ... results-2000 with --rules-max-tokens 2000; results-default without the flag
python3 scripts/model_eval.py score --cases /tmp/sp083-cases \
  --results docs/research/rules-budget-ab/results-<v> --out docs/research/rules-budget-ab/summary-<v>.json
```

---

## Results

Full data: `summary-{1000,2000,default}.json`, `results-{1000,2000,default}/`. 63 runs, all `ok`.

**R(v)** = mean seeded rule bugs caught per run across the 4 seeded rule cases (0–4), harness matching (±3-line window).

**Per-case recall (runs caught / 3 runs):**

| Case (seeded rule) | 1000 | 2000 | default (8192) |
|---|---|---|---|
| go-rule-context (go §1.3 context in request path) | **3/3** | **3/3** | 1/3 |
| go-rule-global-prefix (go §2.1 unexported global) | 1/3 | 1/3 | **3/3** |
| py-rule-membership (py §2.3 list membership in loop) | **3/3** | **3/3** | 2/3 |
| py-rule-naming (py §3.4 camelCase functions) | **3/3** | **3/3** | **3/3** |
| **R(v)** | **3.33** | **3.33** | 3.00 |

**Other metrics:**

| Metric | 1000 | 2000 | default (8192) |
|---|---|---|---|
| C(v): findings per run on clean cases | **1.44** | 1.67 | 1.67 |
| — go-clean-refactor findings/run | 2.00 | 2.00 | 1.67 |
| — go-rule-clean findings/run | 1.33 | 2.00 | 2.33 |
| — py-rule-clean findings/run | 1.00 | 1.00 | 1.00 |
| Mean seconds per case (all 7 cases) | **23.6** | 24.3 | 25.9 |
| Unexpected findings per run (all cases) | **0.95** | 1.29 | 1.43 |
| Unexpected findings on clean cases (of 9 clean runs) | **13** | 15 | 15 |
| Repeatability (mean pairwise Jaccard) | 0.86 | **0.97** | 0.76 |
| Runs failed / timed out | 0 | 0 | 0 |

Observations:

- No budget recovers all four rule bugs in all runs. `go-rule-global-prefix` (rename `minSpan` → `_minSpan`) is caught reliably only at 8192 (3/3 vs 1/3 at 1000/2000); `go-rule-context` is caught reliably only at 1000/2000 (3/3 vs 1/3 at 8192). These swings go in opposite directions and are each within one case.
- Recall at 1000 equals recall at 2000, run for run, on all four cases.
- The first A/B's 2–6× latency penalty for the scaled budget did **not** reproduce here: 25.9 vs 23.6 s/case (~9%). Those cases were larger real-ref diffs (more hunks, each prompt paying the digest cost); these synthetic cases are 30–120-line files with 1–2 hunks.
- Even at 8192 the digests are partial: 88 sections still dropped per run (207 at 1000, 191 at 2000) — the 41-rule corpus does not fit any of these budgets.

---

## Decision

Applied mechanically from the `score` output, per the pre-registered rule:

1. **R\* = max R(v) = 3.33** (achieved at 1000 and 2000; default is 3.00). Pick `v*` = the smallest budget with R(v*) ≥ R\* − 1 = 2.33 → **v\* = 1000** (3.33 ≥ 2.33).
2. **v\* = 1000 → set `maxReviewRuleTokens` to 1000.** The scaled default (15% of `context_limit`, clamped to [MaxRuleTokens, maxReviewRuleTokens] = [1000, 1000]) is therefore a flat 1000 tokens; `rules_max_tokens` overrides still work at any value.
3. **C(v) reported above for every variant.** C(v\*) = 1.44 is itself the smallest C, so the "exceeds the smallest C by more than 1.0" Threats-to-validity flag does not trigger.

(The tie clause — "if R(v) is equal across variants, pick 1000" — was not needed: R is not equal across variants, but both paths pick 1000.)

**Change:** `maxReviewRuleTokens` 8192 → 1000 in `cli/internal/rules/criteria.go` (commit `feat(SP-083): …`), with `TestReviewRuleBudget` updated and the clamp wording in `docs/cli-extension-contract.md`, `docs/review-process-internals.md`, and `README.md` set to the new [1000, 1000] clamp.

Interpretation: with these cases, more rule text does not buy rule-bug recall — the budget that matches the old fixed floor is the right default. The digests stay (the first A/B measured digest@1000 at ~2.2× the full-text baseline's 16.5 s/case on the old suite — the cost of digestion is accepted, the question here was only whether a larger budget buys recall), and operators who want more rule text can still raise `rules_max_tokens`.

---

## Threats to validity

- **Synthetic, single-bug cases, single author.** The four seeded defects were designed (SP-082) to be invisible to generic review and attributable to one rule section; real rule-driven defects may differ.
- **3 runs, temperature 0.1.** Per-case swings are large (3/3 → 1/3 for `go-rule-context` between 1000 and default; the reverse for `go-rule-global-prefix`). R differences across variants are within the decision rule's 1-case band, which is why the rule picks the cheapest budget rather than claiming a recall ranking.
- **Latency non-replication.** The 2–6× cost measured on the first A/B's larger diffs did not appear on these small cases; the latency column here must not be read as "big budgets are cheap in general".
- **One rule corpus, one model.** 41 `.cursor/rules` files at `61301aa` and `qwen/qwen3.8-27b` only; other rule sets (larger applicable slices per file) or models could shift the crossover.
- **stet binary v1.4.0** predates HEAD by commits that do not touch the review-prompt path (see Setup); results are recorded against the harness at HEAD.
- Digests were rebuilt every run (fresh temp repos defeat the digest cache); cache-hit behavior is not measured here.
