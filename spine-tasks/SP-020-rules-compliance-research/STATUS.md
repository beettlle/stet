**Current Step:** Step 3: In progress (writing document)
**Status:** In progress (operator scheduled)
**Last Updated:** 2026-10-08
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

## Step 0: Design the experiment

**Status:** Complete

- [x] Premise-extraction approach and labeled-set plan recorded (see Discoveries 2026-10-08)

## Step 1: Premise extraction and cost measurement

**Status:** Complete

- [x] Premise yield from `.cursor/rules` (Stage A heuristic 8,231→1,597 = 19.4%; Stage B 100-premise sample: ~32% crisp, ~20% rewordable, ~48% non-checkable)
- [x] Question counts and latency per hunk (260 premises/hunk as-implemented; ~10.9s findings=1; latency model ~300ms base + ~30ms/question)

## Step 2: Accuracy and injection experiments

**Status:** Complete

- [x] `noul` vs `choice` accuracy on 24 hand-labeled pairs (A: 24/24, AUROC 1.000; B: 22/24 argmax 91.7%, AUROC 0.965)
- [x] Injection flip rate (6 variants × 3 base pairs = 18 runs + 1 protocol-B spot check: 2/18 flipped, both exemption/authority-style)

## Step 3: Write the document

**Status:** Not Started

- [ ] Research doc with `## Recommendation`

## Step 4: Testing and verification

**Status:** Not Started

- [ ] Contract `testCommand`
- [ ] Only the doc committed

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| | | | |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-09-30 | Decomposition pass: kept as M | Experiments feed one recommendation; splitting would produce partial docs with no standalone decision value. Dependency updated SP-019 → SP-025 after SP-019 split |
| 2026-10-08 | Experiment design (Step 0) | See notes below |
| 2026-10-08 | `filepath.Match` does not support `**` — frontmatter globs like `**/*.go` silently match nothing; only `alwaysApply` rules ever apply in this corpus | Product bug: language rules never injected, never checked; logged to CONTEXT.md as tech debt; makes as-implemented premise load 260/hunk (always-apply only) vs 398–416 as-intended |
| 2026-10-08 | Matched always-apply rule content = 88,004 chars ≈ 22K est. tokens vs `MaxRuleTokens` 1,000 → ~95.5% of applicable rules never reach the review prompt | Motivates the rule-selector placement (Q5) over per-finding verification |
| 2026-10-08 | Injection flips only from exemption/authority-style comments (`[SYSTEM INSTRUCTION …] exempt`, `lint-exempt`); generic compliance claims no effect; choice protocol argmax stable under the same attack | A rules-compliance signal must never silently drop findings — annotation/selector only |
| 2026-10-08 | OpenJev server rejects > 256 questions per request (`OPENJEV_MAX_QUESTIONS`); stet client chunks at 32 | Full-premise checks require ceil(premises×findings/32) sequential requests |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-09-30 | Authored | Packet created; deferred until operator schedules |
| 2026-10-08 | Step 0 | Experiment design recorded; OpenJev 0.6.0 + MLX DiffusionGemma 26B running locally on :8080; smoke test P=0.9995 |
| 2026-10-08 | Step 1 | Premise yield + cost/latency measured (throwaway Python + Go prototypes in /tmp, not committed) |
| 2026-10-08 | Step 2 | Accuracy (noul vs choice) + injection (6 variants) experiments complete on 24 labeled pairs |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes

### Step 0 design (2026-10-08)

- **Corpus:** `.cursor/rules` at the stet main checkout (39 top-level `.mdc`; gitignored, so not present in the worktree — this is the same corpus `rules.DiscoverRulesDirs` would load). One nested dir (`taskplane/`) exists but `LoadRules` reads only top-level `.mdc`, matching stet behaviour for the root dir.
- **Q1 premise extraction:** two-stage. Stage A: deterministic heading/bullet/sentence heuristic (throwaway Python in /tmp) that parses frontmatter, splits body on headings, bullets (`-`, `*`, `|` table rows), and sentence boundaries; filters to declarative single statements ≤ 220 chars with rule-ish verbs (must/never/always/avoid/prefer/do not/no X in Y). Stage B: manual curation of the surviving set for the labeled experiment (drop meta-guidance, process rules, and statements not checkable against a code hunk). Report yield at each stage.
- **Q2 cost:** throwaway Go module in /tmp with `replace stet => <worktree>` so the real `rules.LoadRules` + `rules.FilterRules` are measured, not re-implemented. Sample = file paths touched in the last 40 commits (git log, dedup). Per file: applicable-rule count, summed curated-premise count → questions/hunk = applicable premises × assumed findings (1 and 3 reported; findings/hunk assumption stated in doc). Latency: warm `/v1/systemone` calls at 1/12/32 noul questions × two state sizes (typical ~2 KB hunk, critic cap 4,096 chars), p50/p95 over ≥ 15 reps.
- **Q3 accuracy:** ≥ 24 hand-curated snippet/premise pairs in Go (this repo's language), 12 violating / 12 compliant, snippets synthetic (no real user code). Two protocols: (a) `noul` per premise — one question per premise, instructions ask "does this code violate: <premise>", decision = P(yes) ≥ 0.5; (b) one `choice` question listing the premises plus `none`. Metrics: accuracy @ 0.5, AUROC (rank-based Mann-Whitney).
- **Q4 injection:** ≥ 6 adversarial comment variants prepended to known-violating snippets (compliance claim, fake CI badge, rule-deprecation claim, authority claim, instruction to reviewer, exemption claim). Metric: P(violation) delta and flip rate at 0.5, vs the same snippet without the comment.
- **Q5 placement:** decided analytically after Q2–Q4 (filter vs annotate vs pre-prompt selector) using measured flip rate and latency.
- **Server:** OpenJev 0.6.0 source run locally (venv, MLX backend, DiffusionGemma 26B 4-bit on M1 Ultra) at the stet default `http://127.0.0.1:8080`; model `openjev-latest`. Smoke test answered a ghost-layer question at P=0.9995.
