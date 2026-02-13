# Research: Calibration, False Positive Mitigation, and Multi-Judge Aggregation for Stet

For the full research map and other topics, see [code-review-research-topics.md](code-review-research-topics.md).

## Executive Summary

This document synthesizes research on **calibration** (when to show or suppress findings), **false positive mitigation** (grounding in program facts, critic pass), and **multi-judge aggregation** (CARE, Snorkel) for stet. The main recommendation is to start with optional confidence scoring and lightweight grounding plus a rule-based FP filter, then add dismissal-history–driven calibration and an optional critic loop or multi-persona aggregation for teams that prioritize precision over speed.

---

## 1. Research Findings

### A. Calibration (code LMs)

- **Source:** [Calibration and Correctness of Language Models for Code](https://arxiv.org/abs/2402.02047) (ICSE’25).
- **Problem:** Code LMs are often poorly calibrated out of the box: confidence (or “how sure” the model seems) does not reliably match actual correctness.
- **Approach:** Evaluate calibration across tasks and correctness criteria; improve it with **Platt scaling** (post-hoc logistic regression on model scores vs correctness). Platt scaling requires **correctness labels** (e.g. from holdout data or user feedback).
- **Abstention:** If the model’s confidence is below a threshold, the model can “abstain” (not output a finding) rather than outputting a low-quality one. This is a powerful calibration technique when confidence is available or can be elicited.

### B. False positive mitigation

- **Grounding in program facts:** AdaTaint ([arXiv:2511.04023](https://arxiv.org/abs/2511.04023)) and LLM4FPM combine LLM inference with symbolic validation (program facts, constraint checks). AdaTaint reduces false positives by ~43.7% vs LLM-only/CodeQL/Joern. Principle: do not trust the LLM alone—validate against code/structure.
- **Specification-grounding:** SGCR ([arXiv:2512.17540](https://arxiv.org/abs/2512.17540)) uses a dual path: (1) explicit path = deterministic rules from specs; (2) implicit path = heuristic discovery. Achieves 42% developer adoption vs 22% baseline.
- **Generator–Discriminator / “Critic”:** A second pass asks “Is this finding definitely correct and actionable?” and filters findings that fail verification. This “Double-Check” or critic loop can drastically increase precision at the cost of roughly doubling inference.

### C. Multi-judge aggregation

- **Source:** CARE (Confounder-Aware Aggregation for Reliable Evaluation); Snorkel “From many voices to one.”
- **Problem:** Multiple judges (e.g. several LLMs or LLM + rule-based) are correlated and share biases (e.g. verbosity, position). Naive majority or averaging can **amplify** those biases.
- **CARE:** Models aggregation as inference in a latent-factor MRF: latent “true quality,” judge correlations, and **confounders** (e.g. length, verbosity). A two-stage estimator separates true quality from confounders. Can mix **programmatic judges** (cheap rules) with LLM judges. Reduces aggregation error by up to ~25% vs majority vote.
- **Relevance:** If stet adds a second reviewer (second model or rule-based judge), aggregate in a principled way so that verbosity/length does not dominate the combined signal.

---

## 2. Ideas for Stet

- **Confidence or actionability score:** Add an optional `confidence` (0–1) or actionability field per finding (from structured prompt or a second pass). Enables “review carefully” vs “likely safe to apply” and, once correctness data exists, calibration (e.g. Platt) for show/suppress thresholds.
- **Lightweight grounding + rule-based FP filter:** Before or after presenting findings: (1) grounding checks (line in diff, file in scope, category in allowed set); (2) match against the curated false positive table in [review-quality.md](review-quality.md); suppress or down-rank matches (e.g. `false_positive`, `already_correct`).
- **Second judge with principled aggregation:** Introduce a programmatic judge (e.g. line-in-diff, category allowlist, FP table) and/or an optional critic pass. Combine via a clear policy (e.g. suppress only if programmatic says FP and confidence low); avoid naive averaging. If multiple judges are added later, consider CARE-style aggregation.
- **Dismissal history as correctness signal:** Use `.review/history.jsonl` dismissal reasons (`false_positive`, `already_correct`, etc.) as (noisy) correctness labels for calibration and for extending the curated FP table. The optimizer (e.g. `stet optimize`) can use these to refine prompts and filters.

---

## 3. Grounding Techniques (Zero Inference Cost)

- **Syntactic grounding:** If the finding is “Unused variable X,” use a regex or Tree-sitter query to confirm X appears only in the declaration before showing the finding.
- **Scope grounding:** If the finding says “Function Y is undefined,” check imports or project structure to see if Y is available in scope.
- **Curated FP table:** Match (category, message pattern) against the table in [review-quality.md](review-quality.md); if the reason is `false_positive` or `already_correct`, suppress or tag the finding.

---

## 4. Phased Implementation Plan

- **Phase 1:** Grounding checks (line in hunk, file in diff) and rule-based FP filter using the curated table; optional `confidence` field in the finding schema and prompt. No suppression by confidence yet.
- **Phase 2:** Use dismissal history as a correctness proxy; optional offline calibration (e.g. Platt) when enough data exists; configurable show/suppress thresholds in CLI or extension.
- **Phase 3:** Programmatic judge (grounding + FP table) with a clear aggregation policy; optional critic loop (e.g. `--verify` or `--deep`) or multi-persona (Security / Style / Logic) with merge and deduplication. Document tradeoff: critic loop roughly doubles inference cost for higher precision.

---

## 5. References

| Topic | Source |
| ----- | ------ |
| Calibration, Platt scaling | [arXiv:2402.02047](https://arxiv.org/abs/2402.02047) (Calibration and Correctness of Language Models for Code) |
| FP mitigation (neuro-symbolic) | [arXiv:2511.04023](https://arxiv.org/abs/2511.04023) (AdaTaint) |
| Specification-grounding | [arXiv:2512.17540](https://arxiv.org/abs/2512.17540) (SGCR) |
| Multi-judge, confounders | CARE (OpenReview); Snorkel “From many voices to one” |
| Curated FP patterns and schema | [review-quality.md](review-quality.md) |
