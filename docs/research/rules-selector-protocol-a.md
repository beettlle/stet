# Research: Protocol A vs current selector vs digest round-robin (SP-082)

**Date:** 2026-11-21  
**Status:** Complete  
**Question:** Does a fixed System One choice selector put the planted rule section into the review prompt more often than digest round-robin, and does Protocol A (`digest_fair`) beat the current selector (`full`)?

## Pre-registered decision rule (written before any run)

1. **Primary H(v)** = mean section-hit over the 4 seeded cases (`go-rule-context`, `go-rule-global-prefix`, `py-rule-membership`, `py-rule-naming`). Hit = planted section appears in that run’s `meta.rules_sections` labels (continued-file stem match; see harness `section_hit`).
2. **H\* = max H(v)** among the three packing arms at `critic=off`. Prefer the **cheapest** arm with H(v) ≥ H\* − 0.25. Cost order: `digest-rr` < `selector-full` ≈ `selector-protocol-a`. Between selectors, prefer Protocol A only if H(PA) ≥ H(full).
3. **Secondary:** R(v) (seeded rule-bug recall, SP-083 definition) must stay within **1.0** of max R; else escalate.
4. **C(v)** (clean-case findings/run) reported; flag if winner’s C exceeds min C by >1.0.
5. Critic-on latency is **descriptive only** — not part of v\*.

## Arms

| Arm ID | Packing | Config |
|---|---|---|
| `digest-rr` | Default digests | `STET_RULES_SELECTOR=false`, budget 1000 |
| `selector-full` | Current selector | `rules_selector=true`, candidates=`full` |
| `selector-protocol-a` | Protocol A | `rules_selector=true`, candidates=`digest_fair` |

Critic axis: off (primary quality) vs on (latency only). Model: `qwen/qwen3.8-27b`, ctx 65536, 3 runs/case, `--rules-dir .cursor/rules`. OpenJev at `http://127.0.0.1:8080`. stet built from this branch (`stet version dev`).

Section-hit matching: `file.mdc#N.N` hits when a label’s heading contains section id as a leading token and source is `file.mdc` or `file-continued.mdc` (SP-082 drift for `#2.1` / `#3.4`).

## Results (critic=off — primary)

| Arm | H(v) | R(v) | C clean findings/run | Mean s/case | Reliability |
|---|---|---|---|---|---|
| digest-rr | **0.25** | 0.833 | 1.22 (11/9) | 26.5 | 21/21 ok |
| selector-full | **0.00** | 0.833 | 1.78 (16/9) | 95.5 | 21/21 ok |
| selector-protocol-a | **0.75** | **1.00** | 1.89 (17/9) | 83.5 | 21/21 ok |

Per-case section-hit (critic=off):

| Case | digest-rr | full | protocol-a |
|---|---|---|---|
| go-rule-context (`…standards.mdc#1.3`) | 0.0 | 0.0 | **1.0** |
| go-rule-global-prefix (`…#2.1` → continued) | **1.0** | 0.0 | **1.0** |
| py-rule-membership (`…#2.3`) | 0.0 | 0.0 | 0.0 |
| py-rule-naming (`…#3.4` → continued) | 0.0 | 0.0 | **1.0** |

`py-rule-membership` never hit under Protocol A: candidates included other python sections (1.2, 1.4, 3.4, brutal-audit) but OpenJev did not rank `### 2.3 Sets vs Lists…` into the filled budget — a ranking miss, not a candidate-fairness miss.

## Mechanical decision (critic=off)

1. H\* = 0.75 at `selector-protocol-a`. Slack band H\* − 0.25 = 0.50.
2. Only Protocol A clears the band (digest-rr 0.25, full 0.00). **v\* = selector-protocol-a**.
3. R(v\*) = 1.00 = max R (no escalation).
4. C(v\*) − min C = 1.89 − 1.22 = 0.67 ≤ 1.0 (threat flag does not fire).
5. Latency (descriptive): Protocol A ≈ +57 s/case vs digest-rr at critic=off; slightly cheaper than `selector-full` (83.5 vs 95.5).

**Product implication:** Prefer shipping / enabling `rules_selector_candidates = digest_fair` where OpenJev is available. Do **not** enable bare `full` selector on CursorRules-heavy repos — it scored **worse** than digest round-robin on H (0.00 vs 0.25) while costing ~3.6× wall time.

## Results (critic=on — latency only)

| Arm | H(v) | R(v) | Mean s/case | Reliability |
|---|---|---|---|---|
| digest-rr | 0.25 | 0.750 | **46.8** | 20 ok / 1 failed |
| selector-full | 0.00 | 0.667 | 81.8 | 19 ok / 2 failed |
| selector-protocol-a | **0.67** | 0.750 | 162.4 | 21/21 ok |

Critic-on stacks a second System One path (noul critic) with the selector choice. Protocol A remains best on H but ~3.5× slower than digest-rr under critic-on. Failures on digest-rr/full are environmental (LLM/OpenJev load), not used for v\*.

## Implementation landed with this experiment

- `rules_selector_candidates` / `STET_RULES_SELECTOR_CANDIDATES`: `full` \| `digest_fair`
- Selector honors `rules_max_tokens` / `ReviewRuleBudget`
- `STET_RULES_SECTION_LOG=1` stderr markers; harness `--rules-selector` / `--rules-selector-candidates` + `section_hit_mean` in score
- Contract: `docs/cli-extension-contract.md`

## Artifacts

- `preflight.txt`, `summary-*-{off,on}.json`, `results-*-{off,on}/`, `run-*.log`

## Threats

- Synthetic SP-082 cases; 3 runs/case; OpenJev / LM Studio variance under load (critic-on failures).
- `py-rule-membership` ranking miss under Protocol A.
- expected.json continued-file drift handled in scorer; consider fixing fixture `rule` paths separately.
