# Task: SP-083 — Rule-budget A/B eval and data-driven default

**Created:** 2026-11-01
**Size:** M

## Review Level: 1

**Assessment:** Long LM Studio measurement (≈1–2 h), a research note, and at most one constant change with its tests and doc rows.
**Score:** 3/8 — Blast radius: 1, Pattern novelty: 0, Security: 0, Reversibility: 2 (prompt-size default for every review) — Level 1 (single validated constant)

## Mission

Closes #11

#11's digest work is on `main` (`1a1782e`); its last acceptance item is an A/B eval showing what the rule budget buys. The first A/B (comment on #11) found no recall gain because no case needed a rule, and the scaled default cost 2–6x latency. SP-082 added rule-targeted cases and harness flags (`--rules-dir`, `--rules-max-tokens`). Re-run the A/B on those cases and set the default budget from the result.

Current default (`cli/internal/rules/criteria.go`): `ReviewRuleBudget` = 15% of `context_limit` (`reviewRuleBudgetPercent`), clamped to [`MaxRuleTokens` = 1000, `maxReviewRuleTokens` = 8192]. At the harness's pinned `context_limit` 65536 that is 8192.

### Operator checklist (done by the human before the wave starts)

1. LM Studio server at `http://127.0.0.1:1234` with `qwen/qwen3.8-27b` loadable; swap headroom available (`lms unload --all` first).
2. Nothing else using LM Studio during the run (no other eval or spine batch).

### Fixed run conditions (pre-registered)

- Cases: a temporary directory holding copies of the SP-082 cases (`go-rule-*`, `py-rule-*`, including both clean controls) plus `go-clean-refactor`. Record the exact list in `preflight.txt`.
- Rules: `--rules-dir .cursor/rules` (the repo's rules at the run's commit; record `ls .cursor/rules | wc -l`).
- Variants (`--rules-max-tokens`): `1000`, `2000`, and unset (scaled default, 8192 at context 65536). Each variant writes to its own `--out` directory.
- Runs: `--runs 3` per case per variant; model `qwen/qwen3.8-27b` loaded with `lms load qwen/qwen3.8-27b -c 65536 --parallel 1 --no-speculative-draft-mtp -y`; harness pinned env otherwise unchanged.
- Run order: 1000, 2000, default. Each invocation `--max-minutes 25`, re-invoked until exit 0 (exit 3 = budget reached).

### Decision rule (pre-registered; apply mechanically from `score` output)

Let `R(v)` = mean seeded rule bugs caught per run across the 4 seeded rule cases (0–4) for variant `v`, and `C(v)` = mean findings per run on the clean cases.

1. `R* = max R(v)`. Pick `v*` = the **smallest** budget with `R(v*) ≥ R* − 1` (within one case of the best).
2. If `v*` is `1000` or `2000`, set `maxReviewRuleTokens` to `v*` (the scaled default then never exceeds it). If `v*` is the scaled default, leave the constants unchanged.
3. Report `C(v)` and mean seconds per case for every variant. If `C(v*)` exceeds the smallest `C` by more than 1.0, say so in Threats to validity; it does not change `v*`.

If `R(v)` is equal across variants (the rules still add no recall), the rule picks `1000`. Record that outcome plainly: the digests stay, but the budget is capped to bound latency.

## Dependencies

- **Task:** SP-082 (cases and harness flags)
- **Task:** SP-081 (serializes LM Studio and OpenJev use and the shared contract-doc file)
- **External:** LM Studio per the operator checklist

## Context to Read First

**Tier 3 (reference docs):**

- `gh issue view 11 --repo beettlle/stet` (proposal and the first A/B comment)
- `scripts/model_eval.py` (`run` and `score` `--help`), `spine-tasks/SP-082-rule-targeted-eval-cases/PROMPT.md`
- `cli/internal/rules/criteria.go` (`ReviewRuleBudget`, constants), `cli/internal/rules/criteria_test.go`
- `docs/research/model-comparison-qwen3-coder-next-vs-qwen3.8-27b.md` — house style for research docs

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio at `http://127.0.0.1:1234` (exclusive use)

## File Scope

- `docs/research/rules-budget-ab.md` (new)
- `docs/research/rules-budget-ab/preflight.txt` (new)
- `docs/research/rules-budget-ab/summary-1000.json`, `summary-2000.json`, `summary-default.json` (new)
- `docs/research/rules-budget-ab/results-1000/**`, `results-2000/**`, `results-default/**` (new, harness output)
- `cli/internal/rules/criteria.go` (only `maxReviewRuleTokens`, only if the rule changes it)
- `cli/internal/rules/criteria_test.go`
- `docs/cli-extension-contract.md` (only the `rules_max_tokens` row and the "Cursor rules in the review prompt" budget line)
- `docs/review-process-internals.md` (only the budget sentence in the rules "Append" bullet)
- `README.md` (only the `--rules-max-tokens` line)

## Documentation Requirements

**Must Update:**

- `docs/research/rules-budget-ab.md` — Summary, Setup (cases, rules count, model, commit), Results per variant (`R`, per-case recall, `C`, seconds per case), `## Decision` (rule applied step by step), Threats to validity (synthetic cases, single author, 3 runs, temperature 0.1)
- `docs/cli-extension-contract.md`, `docs/review-process-internals.md`, `README.md` — the stated clamp `[1000, 8192]`, only if the cap changes

**Check If Affected:** none.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/rules/... -count=1 && test -s docs/research/rules-budget-ab/summary-default.json && grep -q '^## Decision' docs/research/rules-budget-ab.md && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `docs/research/rules-budget-ab.md`, `docs/research/rules-budget-ab/preflight.txt`, `docs/research/rules-budget-ab/summary-1000.json`, `docs/research/rules-budget-ab/summary-2000.json`, `docs/research/rules-budget-ab/summary-default.json` |
| fileScopeMustNotChange | `cli/internal/prompt/**`, `cli/internal/review/**`, `cli/internal/run/**`, `cli/internal/config/**`, `cli/cmd/**`, `scripts/**`, `.cursor/**`, `extension/**`, `.review/**` |
| completionCriteria | All three variants have results for every case × 3 runs (non-`ok` runs retried once and reported); each summary regenerates from its results with `score`; the note applies the decision rule step by step; if the cap changes, `maxReviewRuleTokens`, `criteria_test.go`, and the three doc mentions show the new clamp; results posted to #11 (`gh issue comment 11 --repo beettlle/stet --body-file …`, URL in STATUS); stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 0: Preflight

Write all output to `docs/research/rules-budget-ab/preflight.txt`.

- [ ] `stet --version`, `git rev-parse HEAD`, `lms ps`, case list, `ls .cursor/rules | wc -l`
- [ ] Load the model per Fixed run conditions; smoke request ("Reply with OK.", `max_tokens` 64) returns HTTP 200

### Step 1: Run the three variants

- [ ] `1000`, `2000`, then default, per Fixed run conditions; `--retry-failed` once per variant; remaining non-`ok` runs in Discoveries
- [ ] Commit results (`docs(SP-083): rules budget A/B results`)

### Step 2: Score, decide, and set the default

- [ ] `python3 scripts/model_eval.py score --cases <temp cases dir> --results docs/research/rules-budget-ab/results-<v> --out docs/research/rules-budget-ab/summary-<v>.json` per variant
- [ ] `rules-budget-ab.md` with `## Decision`; if the rule changes the cap: `maxReviewRuleTokens`, its test, and the three doc mentions
- [ ] Post the results table and decision to #11
- [ ] `spine_review_step` after the step

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)
- [ ] Restore LM Studio: `lms unload --all && lms load qwen/qwen3.8-27b -c 65536 --parallel 1 --no-speculative-draft-mtp -y`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing
- [ ] Decision follows the pre-registered rule

## Git Commit Convention

- **Results / docs:** `docs(SP-083): description`
- **Default change:** `feat(SP-083): description`

## Do NOT

- Change the decision rule, variants, cases, run counts, model, or pinned settings after seeing results (add an Amendment **before** re-running if something forces a change)
- Change `reviewRuleBudgetPercent`, `MaxRuleTokens`, digest content, rule ordering, or the selector
- Edit `.cursor/rules/`, fixture cases, the harness, `.review/config.toml`, or LM Studio presets
- Close or reopen #11 yourself (the release operator closes it after the wave lands)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
