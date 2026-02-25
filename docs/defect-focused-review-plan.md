# Research: Defect-Focused Review & Quality Plan for Stet

> **Status:** Research complete. Phase 1 (defect-focused prompt) and Phase 4 (dismissal feedback, prompt shadows) are implemented. Phase 2 (RAG-Lite) is implemented. Phase 3 (self-critique / critic) is implemented as an optional `--verify` flag. CRScore-style evaluation is future work.

For the full research map and other topics, see [code-review-research-topics.md](code-review-research-topics.md).

## Executive Summary

This document synthesizes research on "Defect-Focused Review", "CRScore", and "Actionability" and proposes a concrete plan to integrate these concepts into Stet within the constraints of a local-first, 32k context environment.

**Core Recommendation:** Shift Stet from a generic "code review" tool to a **"Defect & Maintainability Focus"** tool. Use a multi-pass or multi-role prompting strategy (Security, Bug, Performance) to increase "Key Bug Inclusion" (KBI) and suppress low-confidence "nitpicks" or "design" comments unless explicitly requested. Implement a lightweight "Self-Critique" step using the same local model to filter findings based on actionability criteria before presenting them to the user.

---

## 1. Key Concepts

### A. Defect-Focused Review (Context + Multi-role)

*Source: "Towards Practical Defect-Focused Automated Code Review" (arXiv:2505.17928)*

- **Concept:** Standard LLMs often act as "generalists," missing subtle bugs while generating fluent but non-critical comments. A "Defect-Focused" approach uses specific roles (e.g., "Security Auditor", "Concurrency Expert") and "Chain-of-Thought" (CoT) reasoning to hunt for bugs.
- **Context Slicing:** Instead of full files, use "Code Slicing" (data/control dependencies) to provide relevant context.
- **Relevance to Stet:**
  - **Multi-Role:** We can simulate this with a structured system prompt that iterates through specific inspections (e.g. `Review for: 1. Logic Errors, 2. Security, 3. Performance`).
  - **Slicing -> RAG-Lite:** Stet's "RAG-Lite" (fetching definitions of symbols) is a practical, lightweight implementation of "Slicing" for a local environment.

### B. CRScore (Quality Metrics)

*Source: "CRScore – grounding automated evaluation in code claims and smells" (arXiv:2409.19801)*

- **Concept:** A reference-free metric that evaluates review comments based on **Conciseness**, **Comprehensiveness**, and **Relevance**. It uses a model to "critique" the review comment against the code.
- **Relevance to Stet:**
  - We cannot run a heavy scoring pipeline for every user.
  - **Adaptation:** Implement a "Self-Critique" prompt. After generating a finding, ask the model (or a second pass): *"Is this finding actionable? Is the fix clear? Confidence 1-5."* and filter out low-scoring items.

### C. Actionability & RovoDev Insights

*Source: "RovoDev Code Reviewer" (arXiv:2601.01129)*

- **Concept:** Developers value **Bugs** and **Maintainability** issues. They often ignore "Design" (too subjective in a diff) and "Readability" (unless obvious).
- **Metrics:** Resolution Rate (did the user fix it?).
- **Relevance to Stet:**
  - **Dismissals as Signal:** Stet's "Dismissal" feature is the perfect feedback loop. If a user dismisses a "Design" comment, down-weight that category.
  - **Category Filtering:** By default, show `Bug`, `Security`, and `Performance`. Hide `Style` or `Design` unless actionable or high-confidence.

**BitsAI-CR and Go:** BitsAI-CR reports ~26.7% outdated rate for Go and 75% precision; use as a reference when tuning Stet for Go-heavy use.

---

## 2. CRScore-Style Actionability Checklist

Use these three dimensions as a minimal checklist for prompt tuning and for the Self-Critique filter:

- **Relevance:** The finding references a specific line or construct in the code. No generic "consider refactoring" without a concrete, observable issue.
- **Conciseness:** Message length is bounded (e.g. one short paragraph max); the model is instructed to be concise.
- **Comprehensiveness:** Every finding has either a `suggestion` or a clear "required change" (what the developer should do next).

---

## 3. Proposed Plan for Stet

### Phase 1: The "Defect-Hunter" Prompt (Immediate)

**Goal:** Improve *Key Bug Inclusion* (KBI) and reduce *False Alarm Rate* (FAR).

1. **Restructure System Prompt:** Move away from "You are a helpful code reviewer."
   - **New Persona:** "You are a Senior Defect Analyst. Your goal is to find BUGS, SECURITY VULNERABILITIES, and PERFORMANCE BOTTLENECKS. Do not comment on style unless it introduces a bug."
2. **Structured Steps (CoT):**
   - "Step 1: Analyze the code for logic errors (off-by-one, null pointer, etc.)."
   - "Step 2: Check for security risks (injection, sensitive data)."
   - "Step 3: Check for performance issues (loops, expensive calls)."
   - "Step 4: Output findings only for high-confidence issues."
3. **Actionability Check:** Enforce that every finding must have a `suggested_fix` or a specific question, not just a vague complaint.

**Evidence / required_change:** Consider adding an optional `evidence` field (e.g. "line 12: unvalidated input") or `required_change` (one-line description of the fix) to the findings schema so "actionable" is testable. When context is diff-only, document a policy to down-rank or collapse "Design" (and optionally broad "Style") by default.

**Single prompt vs multi-pass:** The current design simulates multi-role in one structured prompt (single round-trip). True multi-pass (e.g. separate Security then Bug passes) is a future option if metrics justify the extra cost.

### Phase 2: Context Enrichment via "RAG-Lite" (Next)

**Goal:** Mimic "Code Slicing" to provide relevant context within 32k limits.

1. **Symbol Resolution:** When analyzing a hunk, identify function calls and types.
2. **Definition Injection:** Fetch the signature and docstring (or first N lines) of these symbols from the codebase (using `grep` or `ctags`) and inject them into the prompt.
   - *Example:* If `user.CalculateRewards()` is called, inject `func CalculateRewards(...)` definition.
3. **Dependency Context:** If a variable `x` is modified, try to include its definition or previous usages in the hunk window.

**Token and cost:** RAG-Lite increases prompt size; cap symbols per hunk to stay within 32k. Self-Critique (Phase 3) implies a second LLM call per hunk (~2× cost) and should be optional or severity-gated (e.g. Security/Bug only).

### Phase 3: The "Self-Critique" Filter (Quality Control)

**Goal:** Filter out "False Positives" and "Nitpicks" before the user sees them.

1. **Generation:** Model generates potential findings.
2. **Critique (Internal Monologue or Second Pass):**
   - For each finding, ask: *"Is this merely a style preference? Is this incorrect? Is it actionable?"*
   - Assign a **Confidence Score (1-5)**.
3. **Filtering:** Drop findings with Confidence < 4 or Category == "Style" (unless configured otherwise).
4. **User UI:** Show "High Confidence" findings prominently. Group "Low Confidence" or "Nitpicks" in a collapsible section "Potentially noteworthy".

### Phase 4: Feedback Loop (Long Term)

**Goal:** Personalized Actionability.

1. **Track Dismissals:** When a user dismisses a finding of type `Naming`, record this.
2. **Prompt Shadowing:** In the next run, inject: *"User previously dismissed `Naming` suggestions. Ignore variable naming unless it is misleading."*
3. **Optimization:** Use the collected "Dismissal History" to fine-tune the few-shot examples in the prompt (as described in PRD §3h).

---

## 4. Targets and Metrics

Internal targets (not user-facing SLAs). Exact measurement may require platform integration (e.g. "fixed in next commit") in a later phase.

- **Resolution rate:** Aim in the ballpark of RovoDev (~38.7%) or better as the tool improves.
- **Outdated rate:** For Go, use BitsAI-CR's ~26.7% as a benchmark to try to stay under.
- **Dismissal rate:** Track dismissals by category (e.g. in `.review/history`) for prompt shadowing and to down-weight Style/Design.

---

## 5. Evaluation

How to measure success:

- **Before/after:** Compare dismissal rate or findings-per-hunk on a fixed test set (fixtures) before and after the Defect-Hunter prompt.
- **Ongoing:** Use dismissal-by-category from `.review/history`; add resolution/outdated tracking once Phase 4 or platform integration exists.
- **Optional:** Run on a small set of diffs with known defects (e.g. from benchmarks or past bugs) and measure recall/precision if such a set exists.

---

## 6. Risks and Limitations

- **Self-Critique:** The same model critiquing its own output may reinforce its own biases or miss the same mistakes; it is not an independent judge.
- **Diff-only:** A fundamental limit; architectural or "should-have-changed-elsewhere" issues remain out of scope unless context is enriched (Phase 2 and beyond).
- **Model capability:** Quality of Defect-Hunter and Self-Critique depends on the local model; document recommended model(s) (e.g. qwen2.5-coder:32b) if applicable.
- **Calibration caveat:** Code LMs are often poorly calibrated (confidence ≠ correctness). Use confidence for ranking/filtering, not as a guarantee; optionally log confidence vs dismissal rate over time to check calibration.

---

## 7. Implementation Roadmap

1. **Update Prompt Strategy:** Modify the system prompt in **cli/internal/prompt/prompt.go** to be "Defect-Focused".
2. **Implement RAG-Lite:** Accurate symbol extraction and injection (see also [context-enrichment-research.md](context-enrichment-research.md)).
3. **Add "Confidence" Field:** Update the findings schema to include `confidence` and optionally `actionability_score`.
4. **UI Updates:** Update Extension/CLI to sort by Severity + Confidence and to support collapsible "Potentially noteworthy" section.

---

## References

- **Towards Practical Defect-Focused Automated Code Review:** [arXiv:2505.17928](https://arxiv.org/abs/2505.17928)
- **CRScore: Grounding Automated Evaluation of Code Review Comments in Code Claims and Smells:** [arXiv:2409.19801](https://arxiv.org/abs/2409.19801)
- **RovoDev Code Reviewer: A Large-Scale Online Evaluation at Atlassian:** [arXiv:2601.01129](https://arxiv.org/abs/2601.01129)
- **BitsAI-CR: Automated Code Review via LLM in Practice:** [arXiv:2501.15134](https://arxiv.org/abs/2501.15134)
- **Calibration and Correctness of Language Models for Code:** [arXiv:2402.02047](https://arxiv.org/abs/2402.02047)
