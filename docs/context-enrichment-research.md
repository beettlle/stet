# Research: Context Enrichment and Code Slicing for Stet

For the full research map and other topics (false positives, actionability, calibration), see [code-review-research-topics.md](code-review-research-topics.md).

## Executive Summary

This document synthesizes research on **context enrichment** and **code slicing** to improve stet's review quality. The key findings are that **textual intent** (commit messages / PR descriptions) and **surrounding function context** provide the highest ROI for reducing false positives and improving actionability within our 32k token and local-compute constraints. This aligns with ContextCRBench and defect-focused automated code review work.

---

## 1. Key Research Findings

### A. ContextCRBench and Textual Context

- **Source:** [ContextCRBench (arXiv:2511.07017)](https://arxiv.org/abs/2511.07017)
- **Insight:** Adding textual context (issue descriptions, PR summaries) often yields greater quality improvements than adding raw code context alone. The benchmark has 67k+ entries with issue–PR pairs and full function/class context; it supports three evaluation scenarios: hunk-level quality assessment, line-level defect localization, and line-level comment generation. Deployed at ByteDance, it drove a self-evolving code review system with ~61.98% improvement.
- **Relevance to stet:** Since we are local-only and don't (yet) have GitHub integration, we infer "PR description" from **git commit messages** or branch descriptions. This is cheap context (few tokens) with high semantic value.

### B. Defect-Focused Review and Slicing

- **Source:** [Towards Practical Defect-Focused Automated Code Review (arXiv:2505.17928)](https://arxiv.org/abs/2505.17928)
- **Insight:** "Parent function" slicing (identifying the smallest function enclosing a change) is a highly effective, lightweight alternative to full program slicing. The approach uses code slicing algorithms for context extraction and language-agnostic, AST-based analysis. Validated on real-world merge requests, it achieves ~2× improvement over standard LLMs and ~10× over prior baselines.
- **Relevance to stet:** Identifying the parent function (or enclosing type) solves the "missing context" problem where the model hallucinates bugs because it can't see variable definitions in the function signature. For Go, this can be done with the standard library (`go/ast`) or Tree-sitter.

### C. Dual Slicing (Katana)

- **Source:** [Katana (arXiv:2205.00180)](https://arxiv.org/abs/2205.00180)
- **Insight:** "Dual slicing" (analyzing dependencies in both the buggy and fixed versions) precisely identifies "repair ingredients" and reduces noise. The principle of **def-use tracking** (simple data flow) can show where a variable comes from.
- **Relevance to stet:** Full dual slicing is complex for v1. For Go, def-use or lightweight intraprocedural slicing can be implemented using GoDoctor or similar analysis. Dual slicing (before+after) is left as future work for defect-focused review.

### D. "Help Me to Understand this Commit!"

- **Source:** [Help Me to Understand this Commit! (arXiv:2402.09528)](https://arxiv.org/abs/2402.09528)
- **Insight:** Review tools provide a "unit of analysis" (e.g. the diff), but reviewers need a larger "unit of attention"—intent, impact, and related artifacts. The paper argues for contextualized code review environments that aggregate information to reduce the gap between what is shown and what is needed to make decisions.
- **Relevance to stet:** Rationale for enriching context beyond the raw hunk; supports prioritizing textual intent and structural (function/class) context.

---

## 2. Proposed Strategy for Stet

We propose a tiered approach to context enrichment, prioritizing high-value, low-cost techniques. Each tier should be **configurable** (e.g. `include_commit_context`, `include_scope`) so the default remains hunk-only. **Token caps** (e.g. `commit_context_max_tokens`, `scope_max_lines`) and a single check against `context_limit` ensure we stay within the 32k budget; when over budget, truncate or drop parts in a defined order (e.g. commit context first, then scope).

### Tier 1: Intent Context (Textual)

**Goal:** Give the model the "why" behind the change.

**Implementation:**

- **Source:** `git log --no-merges --format=%B <baseline>..HEAD`
- **Method:** Concatenate unique commit messages from the branch.
- **Prompt injection:** e.g. "Usage context: The user is working on the following changes: <commit_summaries>"
- **Cost:** Very low (text is dense).

### Tier 2: Structural Context (The "Hub")

**Goal:** Give the model the "where" (local scope).

**Implementation:**

- **Technique:** **Parent function extraction** (smallest function or type enclosing the hunk).
- **Logic:**
  1. Parse the file modified by the hunk.
  2. Locate the smallest `function_declaration` or `method_declaration` (or type) node that fully encloses the hunk's line range.
  3. Include the signature and body of this parent function in the prompt, augmenting the hunk.
- **Go option:** Use `go/ast` (standard library) to find enclosing `*ast.FuncDecl` or `*ast.GenDecl` (type)—no CGO, minimal dependencies. Tree-sitter can be used later for multi-language (e.g. TypeScript, Python).
- **Fallbacks:**
  - If no parent function (e.g. global var change), use a window of ±N lines.
  - If the parent function or type is **oversized**, cap by lines or tokens, or fall back to hunk-only so one large function does not consume the context window.
- **Cost:** Medium (requires parsing the file; Go's `go/ast` is fast). 32k context allows several full functions.

### Tier 3: Relational Context (The "Spokes")

**Goal:** Give the model the "what else" (dependencies).

**Implementation:**

- **Technique:** **RAG-lite (symbol lookup)** or lightweight def-use.
- **Logic:**
  1. Identify identifiers *used* in the hunk but *not defined* in the hunk or parent function.
  2. Use a fast index (ctags, grep/ripgrep) or static analysis to find the *definition* of these symbols.
  3. Inject the definition (signature and optionally docstring) into the prompt.
- **Go option:** [GoDoctor](https://pkg.go.dev/github.com/godoctor/godoctor/analysis/dataflow) provides `DefUse()`, `DefsReaching()`, and CFG—enabling def-use or a lightweight intraprocedural backward slice for dependency-relevant lines.
- **Cost:** Low token cost (signatures are short); higher engineering complexity to build the index or integrate analysis robustly.

---

## 3. Implementation Plan

### Phase 1: Parent Function and Commit Intent (Recommended first)

1. **Commit intent:** Extract `baseline..HEAD` commit messages and inject into the prompt (e.g. system or user block). Respect `commit_context_max_tokens` and config flag.
2. **Parent function:**
   - For Go: implement `GetParentFunction(file, lineRange)` using `go/ast` (or optionally go-tree-sitter).
   - Return enclosing `FuncDecl` or `GenDecl` (type) source span; cap size with `scope_max_lines` or `scope_max_tokens`.
   - Update prompt to present "File: X (showing function Y)" or equivalent, augmenting the hunk with the scope.
   - Fallbacks: no parent → ±N lines; oversized → truncate or hunk-only.

### Phase 2: RAG-Lite (Follow-up)

1. Implement symbol resolution (definitions of external calls / used identifiers).
2. Inject definitions as "Reference material" in the prompt, with token budget checks.

### Evaluation (optional)

Once enriched prompts exist, consider running stet (or a thin harness) on a subset of ContextCRBench or similar for Go to compare hunk-only vs +intent vs +scope. Track false positive and actionability metrics internally (e.g. dismissals, resolution).

---

## 4. Future Work

- **Dual slicing (before+after):** Slice both pre- and post-change versions of the code to extract defect-focused context (Katana-style). Defer until after Tier 3 is in place; higher complexity and requires robust analysis integration.

---

## 5. Conclusion

By implementing **commit intent** extraction and **parent function** context (Go via `go/ast` or Tree-sitter), we can approximate the high-performance "enriched" benchmarks (ContextCRBench) without the latency of full RAG or heavy static analysis. This aligns with stet's local, lightweight philosophy. Config flags and token budgeting keep the default behavior unchanged and ensure we stay within context limits.
