# Code review research: reducing false positives and improving actionability

This document consolidates research from academic journals, arXiv, and early-release work on automated code review. The goal is to identify techniques, algorithms, and approaches that can improve the quality of reviews stet produces—especially reducing false positives and non-actionable feedback. Stet today is **diff/hunk-based** (one LLM call per hunk, with file path and hunk content only; no RAG or extra context). For stet-specific prompt guidelines and curated false positive patterns, see [review-quality.md](review-quality.md).

---

## 1. False positive reduction (static analysis + LLM)

- **LLM-driven false positive mitigation:** Using LLMs to filter or explain static analysis alarms, grounded in program facts (e.g. AdaTaint, LLM4FPM). *Relevance to stet:* Similar “ground in code facts” ideas could reduce the model flagging non-issues.
- **Sources:** [arXiv:2511.04023](https://arxiv.org/abs/2511.04023) (LLM-driven source–sink / false positive mitigation), LLM4FPM (eCPG, line-level context; high F1 on Juliet/D2A).
- **Postprocessing and ranking of alarms:** Survey-style work on clustering, ranking, and classifying static analysis warnings to cut manual inspection. *Relevance:* Treating each LLM finding as an “alarm” and adding a post-step (ranking, grouping, or filtering).
- **Sources:** IEEE “Mitigating False Positive Static Analysis Warnings: Progress, Challenges, and Opportunities”; SCAM 2016 alarm-handling categories.
- **Transformer/classifier filters for “likely false positive”:** Learning which code patterns correlate with false positives. *Relevance:* Down-rank or suppress findings that match “noise” patterns.
- **Sources:** “Learning to Reduce False Positives in Analytic Bug Detectors” (ML4Code); Tufts Foster et al. classifier-based filtering.

---

## 2. Limited context and context enrichment

- **Code review with limited vs enriched context:** Benchmarks and methods that vary context: diff-only vs +issue description vs +surrounding code vs full repo. *Relevance:* Stet is hunk-only; research shows adding issue text and/or surrounding function/class improves quality and reduces spurious comments.
- **Sources:** “Help Me to Understand this Commit!” (contextualized code review vision); [ContextCRBench](https://arxiv.org/abs/2511.07017) (67k+ entries, issue–PR pairs, full function/class); CodeFuse-CR-Bench (repo-level context).
- **What context helps most:** Textual context (issue/PR description) often yields greater gains than code context alone. *Relevance:* If adding one thing first, issue/PR description; then surrounding function/class.
- **Sources:** ContextCRBench; “Benchmarking LLMs for Fine-Grained Code Review with Enriched Context in Practice” ([arXiv:2511.07017](https://arxiv.org/abs/2511.07017)).
- **Code slicing for relevant context:** Using program slicing (data/control dependencies) to choose what to include instead of a fixed window. *Relevance:* For each hunk, include only dependency-relevant code to avoid noise and stay within token limits.
- **Sources:** “Towards Practical Defect-Focused Automated Code Review” ([arXiv:2505.17928](https://arxiv.org/abs/2505.17928))—slicing + multi-role LLM; Katana (dual slicing for bug fixes).

For a consolidated strategy and implementation plan for context enrichment and code slicing in stet, see [context-enrichment-research.md](context-enrichment-research.md).

---

## 3. Actionability and what developers actually fix

- **Which comment types get resolved:** Empirical studies of which LLM-generated comment categories developers actually address. *Relevance:* Emphasize high-resolution categories (readability, bugs, maintainability); deprioritize or soften low-resolution ones (e.g. vague design).
- **Sources:** Goldman et al.–style studies; industrial “Automated Code Review In Practice” (~73.8% resolution, but also longer PR closure).
- **Defining and measuring “actionable”:** Actionable = specific, tied to observable issue, with clear fix or next step; optional severity/type tags (BLOCKER / SUGGESTION / NIT). *Relevance:* System prompt already says “actionable”; make it operational via structured output and evaluation.
- **Sources:** CRScore (reference-free metric; code claims + smells; 0.54 Spearman with human judgment); CIDRe (relevance, informativeness, completeness, length); practitioner posts on actionable feedback.
- **RovoDev / large-scale adoption metrics:** Real-world resolution rate, PR cycle time, human comment volume. *Relevance:* Calibrate expectations (e.g. resolution rate, “outdated” rate).
- **Sources:** “RovoDev Code Reviewer: A Large-Scale Online Evaluation at Atlassian” ([arXiv:2601.01129](https://arxiv.org/abs/2601.01129))—~38.7% resolution, 30.8% cycle time reduction; BitsAI-CR (75% precision, 26.7% outdated for Go).

---

## 4. Evaluation and quality metrics

- **Reference-free evaluation of review comments:** Metrics that don’t rely on a single “gold” comment but on code-based criteria (claims, smells, consistency). *Relevance:* Evaluate and tune prompts using CRScore-like metrics.
- **Sources:** CRScore ([arXiv:2409.19801](https://arxiv.org/abs/2409.19801), NAACL 2025; 2.9k human-annotated corpus); DeepCRCEval (human + LLM evaluators; quality issues in OSS benchmarks).
- **Ground truth and data quality in benchmarks:** Many benchmarks have outdated code, shallow context, or file-level only; better benchmarks link issue–PR, full function/class, line- or hunk-level labels. *Relevance:* Prefer line-/hunk-level benchmarks with issue and code context.
- **Sources:** ContextCRBench; CodeFuse-CR-Bench; “Benchmarking and Studying the LLM-based Code Review” ([arXiv:2509.01494](https://arxiv.org/abs/2509.01494)).
- **Methodological pitfalls (data leakage, duplication):** False alarm detectors often overstate performance. *Relevance:* When running ablations or training filters, avoid leakage and document splits.
- **Sources:** Work on false alarm detection evaluation (e.g. [arXiv:2202.05982](https://arxiv.org/abs/2202.05982)).

---

## 5. RAG and retrieval for code review

- **Retrieval-augmented code review:** Retrieving similar reviewed code or review exemplars to condition the LLM. *Relevance:* With only a diff, retrieve “similar diffs + their accepted review comments” as few-shot or guidance.
- **Sources:** LAURA ([arXiv:2512.01356](https://arxiv.org/abs/2512.01356))—review exemplar retrieval + context; RAG for comment generation (+exact match / BLEU; low-frequency token improvement).
- **What to retrieve and what hurts:** In code generation, “similar code” can add noise; in-context API/code structure often helps. *Relevance:* Prefer retrieving structured context over raw similar-code unless validated.
- **Sources:** CodeRAG-Bench; “Can Retrieval Augment Code Generation?” style analyses.

---

## 6. Calibration and when to show or suppress

- **Calibration of LMs for code:** Aligning model confidence with actual correctness. *Relevance:* If you add confidence scores, use them to suppress low-confidence findings or to label “review carefully” vs “safe to apply.”
- **Sources:** “Calibration and Correctness of Language Models for Code” ([arXiv:2402.02047](https://arxiv.org/abs/2402.02047)); Google-scale experience (e.g. only 7.5% of reviewer comments addressed via ML-suggested edits—selective application).
- **Confounder-aware aggregation of multiple judges:** When using multiple models or criteria, naive voting can amplify shared biases; structured aggregation can separate “true quality” from confounders (e.g. verbosity). *Relevance:* If adding a second model or rule-based judge, aggregate in a principled way.
- **Sources:** CARE (Confounder-Aware Aggregation for Reliable Evaluation); “From many voices to one” (Snorkel).

For a full deep dive (research synthesis, ideas, and phased plan), see [calibration-fp-aggregation.md](calibration-fp-aggregation.md).

---

## 7. Defect-focused and multi-role review

- **Defect-focused automated code review:** Frameworks that optimize for real defects (key bug inclusion, false alarm rate) and human workflow, not just comment generation quality. *Relevance:* Align stet’s objective with “defects found” and “false alarms reduced”; consider multi-role prompts (e.g. one pass for bugs, one for style).
- **Sources:** “Towards Practical Defect-Focused Automated Code Review” ([arXiv:2505.17928](https://arxiv.org/abs/2505.17928))—four challenges: context, KBI, FAR, human integration; 2× vs standard LLM, 10× vs older baselines.
- **Multi-role or multi-pass LLM review:** Different “roles” or passes for different concern types (e.g. security vs style). *Relevance:* Reduces mixed-signal prompts and can improve precision per category.
- **Sources:** Same defect-focused paper; industry tools with separate flows for security vs quality.

For a consolidated implementation plan for defect-focused review in stet (defect-hunter prompt, RAG-Lite, self-critique filter, feedback loop, CRScore-style checklist, and risks), see [defect-focused-review-plan.md](defect-focused-review-plan.md).

---

## 8. Comment categorization and severity

- **Automated classification of review comments:** Taxonomy of comment types (bug, style, design, documentation) and severity. *Relevance:* Stet already has severity and category in the schema; align with empirically useful taxonomies and severity (e.g. BLOCKER vs NIT).
- **Sources:** CommentBERT; Deep Neural Network + CodeBERT (five categories); CRScore dimensions (conciseness, comprehensiveness, relevance).
- **Design vs bug vs style:** Design comments are often less actionable; bug/readability/maintainability more so. *Relevance:* Optionally down-rank or separate “design” when context is limited (diff-only).
- **Sources:** Goldman et al.–style resolution studies; “Automated Code Review Comment Classification to Improve Modern Code Reviews” (Springer).

---

## 9. Diff-only vs broader context (practical tradeoffs)

- **When diff-only is enough vs when it hurts:** Diff-only is cheap and works for small, localized changes; full repo or patch-level context helps for “should have been changed but wasn’t” and architectural issues. *Relevance:* Document that stet is diff/hunk-focused and where it will underperform; consider optional “expand context” (e.g. +issue, +surrounding function) for power users.
- **Sources:** Jane Street “Patch review vs diff review”; Graphite “How much context do AI code reviews need?”; matklad unified vs split diff.

---

## 10. Model limitations and “counterfeit” code

- **Models misunderstanding their own outputs:** Code LMs often misclassify incorrect-but-plausible code as correct and are weak at predicting execution or fixing it. *Relevance:* Avoid over-trusting the model on “is this really a bug?” for subtle logic; prefer grounding in static facts or tests where possible.
- **Sources:** “The Counterfeit Conundrum” ([arXiv:2402.19475](https://arxiv.org/abs/2402.19475)).

---

## Suggested order for deeper dives

1. **Immediate (reduce false positives, improve actionability):** Defect-focused code review, CRScore, actionability/resolution studies.
2. **Next (context under token limits):** Context enrichment (ContextCRBench, enriched-context papers), code slicing for context selection.
3. **Then (quality control and UX):** Calibration, false positive mitigation with LLMs (LLM4FPM, AdaTaint), multi-judge aggregation (CARE).

---

## Dive deeper: reading list and next steps

### 1. Defect-focused review, CRScore, and actionability (immediate priority)

**Key papers**

- **Towards Practical Defect-Focused Automated Code Review:** [arXiv:2505.17928](https://arxiv.org/abs/2505.17928). Addresses context capture, key bug inclusion, false alarm rate, and human workflow; uses code slicing and multi-role LLM; reports ~2× over standard LLM, ~10× over prior baselines on real merge requests.
- **CRScore – grounding automated evaluation in code claims and smells:** [arXiv:2409.19801](https://arxiv.org/abs/2409.19801) / NAACL 2025. Reference-free metric (conciseness, comprehensiveness, relevance); 0.54 Spearman with human judgment; 2.9k human-annotated corpus released.
- **Actionability and resolution:** RovoDev at Atlassian [arXiv:2601.01129](https://arxiv.org/abs/2601.01129) (resolution rate, cycle time, human comment volume); BitsAI-CR (precision, “outdated” rate for Go); Goldman et al.–style studies on which comment types (readability, bug, maintainability vs design) get resolved.

**Takeaway**

Focusing on defect-focused objectives (capturing context, reducing false alarms, integrating with workflow) and measuring comment quality with reference-free, code-grounded metrics (e.g. CRScore) aligns better with real developer behavior than raw generation metrics. Emphasizing comment types that developers actually fix (readability, bugs, maintainability) and softening or separating less actionable design feedback can reduce perceived false positives.

**Next steps / questions to explore**

- Map stet’s current prompt and output schema to CRScore-style dimensions (relevance, conciseness, comprehensiveness) and define a minimal actionability checklist for prompt tuning.
- Evaluate adding a single “evidence” or “required_change” field to findings (actionable vs suggestion) and whether to down-rank or tag “design” when operating with diff-only context.
- Set internal targets for resolution/outdated rates using RovoDev and BitsAI-CR as benchmarks; track resolution or “dismissed” signals if stet is integrated with a platform that supports it.

See [defect-focused-review-plan.md](defect-focused-review-plan.md) for a concrete implementation plan, benchmark targets, and evaluation approach.

---

### 2. Context enrichment and code slicing (next priority)

**Key papers**

- **Benchmarking LLMs for Fine-Grained Code Review with Enriched Context in Practice:** [arXiv:2511.07017](https://arxiv.org/abs/2511.07017). ContextCRBench: 67k+ entries, issue–PR pairs, full function/class context; hunk-level quality assessment, line-level defect localization, line-level comment generation; textual context yields greater gains than code context alone.
- **Towards Practical Defect-Focused Automated Code Review:** [arXiv:2505.17928](https://arxiv.org/abs/2505.17928). Uses code slicing for context extraction; multi-role LLM; language-agnostic principles (e.g. AST-based).
- **Katana – dual slicing for bug fixes:** [arXiv:2205.00180](https://arxiv.org/abs/2205.00180). Dual slicing (buggy and fixed versions) to extract repair-relevant context and reduce noise.

**Takeaway**

With only a diff/hunk, quality and relevance of comments are limited. Adding **textual context** (issue/PR description) and **code context** (surrounding function or class) improves results; textual context often helps more than code context alone. Code slicing (data/control dependencies) can select minimal, relevant context instead of fixed windows, keeping token use under control while reducing noise.

**Next steps / questions to explore**

- Evaluate adding issue/PR description to the stet prompt when available (e.g. from `git log` or extension-provided metadata) and measure impact on false positives and actionability.
- Prototype “surrounding function/class” extraction for Go (and other languages) for each hunk and add it to the user prompt when under a token budget; A/B compare vs hunk-only.
- Investigate lightweight slicing or dependency scopes (e.g. callers/callees, def-use) for stet’s supported languages to include only dependency-relevant lines in the context window.

See [context-enrichment-research.md](context-enrichment-research.md) for a detailed strategy, tiered approach, config/token guidance, and implementation phases.

---

### 3. Calibration, false positive mitigation, and multi-judge aggregation (then)

**Key papers**

- **Calibration and Correctness of Language Models for Code:** [arXiv:2402.02047](https://arxiv.org/abs/2402.02047). Code LMs are often poorly calibrated; high-confidence outputs should be more reliable; Platt scaling and similar methods can improve calibration with correctness data.
- **LLM-driven false positive mitigation:** [arXiv:2511.04023](https://arxiv.org/abs/2511.04023) (source–sink / taint); LLM4FPM (eCPG, line-level context; high F1 on Juliet/D2A). Grounding model suggestions in program facts and constraint validation reduces false positives.
- **CARE – Confounder-Aware Aggregation for Reliable Evaluation:** Snorkel-style “From many voices to one”; multi-judge aggregation with latent true quality and confounders (e.g. verbosity); reduces aggregation error vs majority vote.

**Takeaway**

Using confidence or quality scores to decide when to show vs suppress findings (or to label “review carefully”) can reduce noise and improve trust. Combining LLM output with program facts (e.g. simple static checks or slicing) can filter obvious false positives. If multiple models or rule-based judges are used, aggregating them in a principled way (e.g. CARE-style) avoids amplifying shared biases.

**Next steps / questions to explore**

- Add an optional confidence or “actionability” score to stet’s output (e.g. from a second pass or from structured prompt instructions) and document thresholds for “show always” vs “show if high confidence” vs “suppress.”
- Explore lightweight “grounding” checks (e.g. line exists in diff, category matches a small rule set) to filter clearly invalid findings before presentation.
- If a second reviewer (model or rule-based) is introduced, design aggregation (e.g. when both agree vs disagree) using confounder-aware or similar methods so that verbosity/length does not dominate the combined signal.

See [calibration-fp-aggregation.md](calibration-fp-aggregation.md) for a detailed research synthesis, implementation ideas, and phased plan.
