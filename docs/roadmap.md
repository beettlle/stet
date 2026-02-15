# Stet Future Roadmap: Beyond Phase 7

**Status:** Draft / Research

**Context:** This document outlines the feature roadmap for stet after the core "Defect-Focused" implementation (Phase 6) and "Polish" (Phase 7) are complete.

**Objective:** Evolve stet from a "local CLI tool" into a "universal review agent" that integrates with AI IDEs and learns from user behavior.

### Design principles

- **Precision-focused design:** Stet defaults to fewer, high-confidence, actionable findings (abstention, FP kill list, prompt shadowing, optimizer). Some false positives are expected; industry benchmarks for AI code review are roughly 5–15% FP rate, with precision-focused tools often at 5–8%. Stet aims for the lower end via context, filters, and feedback; monitor and tune (strictness presets, `stet optimize`, dismiss reasons) to keep noise acceptable.
- **Human-in-the-loop:** Treat stet output as first-pass review; the human makes the final call. Dismiss reasons and history improve future runs. This aligns with best practice: AI complements, not replaces, human review.
- **Context-aware review:** Git intent, hunk expansion, RAG-lite, and (roadmap) cross-file impact are core FP-reduction strategies—broader context reduces superficial or irrelevant flags.

---

## Phase 8: The "Ecosystem" Release (MCP & Integration)

**Goal:** Transform stet from a standalone tool into a service that AI Editors (Cursor, Windsurf, Claude) can consume directly.

### 8.1 Feature: Stet as an MCP Server

**Concept:** Implement the Model Context Protocol (MCP) interface. This allows stet to run as a background agent that provides review data to other AI tools without manual CLI execution.

**Architecture:**

- **Transport:** Stdio (standard input/output) JSON-RPC.
- **Capabilities:**
  - **Resource:** `stet://latest_report` (Read the JSON output of the last review).
  - **Tool:** `run_review(scope: string)` (Trigger a review on staged, commit, or branch).
  - **Tool:** `get_findings(min_confidence: float)` (Query specific findings).

**User Story:**

- User in Cursor Chat: "@stet Check my current work for security flaws."
- System: Cursor calls `stet.run_review()`, receives JSON, and summarizes it in the chat window.

### 8.2 Feature: Hybrid Linter Relay

**Concept:** LLMs are excellent at logic but poor at syntax. Static analysis tools (linters) are the opposite. This feature combines them.

**Workflow:**

1. **Pre-Process:** stet runs a fast local linter (e.g., golangci-lint, eslint) on the changed files.
2. **Ingest:** Capture the linter output (line numbers and error codes).
3. **Prompt Injection:** Feed these "facts" to the LLM.
4. **Synthesis:** The LLM explains why the linter error matters and suggests a fix (which linters often fail to do well).

**Prompt Example:**

"The static analyzer found a 'cognitive complexity' error on line 45. Explain this to the user and refactor the code to fix it."

### 8.3 Feature: Targeted Fix from Active Findings

**Concept:** After a review run, use active findings (session.Findings minus DismissedIDs) as input to a smaller local LLM that generates targeted code fix suggestions. The user dismisses false positives first; the remaining list is high-signal for fix generation. This composes with the review-only design: fix is a separate, optional step.

**Rationale:** Research (e.g. "How Small is Enough?" arXiv 2508.16499, InferFix, CORE) shows that small code models (7B parameters) achieve competitive fix accuracy when the task is narrow and well-scoped. The fix task—"given this finding (file, line, message, suggestion), produce the corrected code"—is simpler than full review and fits small models well.

**Commands:** Two entry points keep behavior explicit (cf. [roborev](https://www.roborev.io) fix/refine):

- **stet fix** — One-shot: generate fix(es) from active findings. Output patches for human review; optionally apply to working tree and commit once (e.g. `--apply` / `--commit`).
- **stet refine** — Looping: generate fix → apply and commit → run review → if findings remain, repeat until review passes or a max-iterations cap. Each commit uses the same attribution message (see below).

**Workflow:**

1. User runs `stet start` or `stet run` → gets findings.
2. User dismisses false positives via `stet dismiss <id>`.
3. Active findings = session.Findings - DismissedIDs (much smaller, higher precision).
4. **stet fix:** User runs `stet fix [--finding-id ID] [--dry-run] [--apply] [--commit]`:
   - Load session; filter to active findings (or the specified finding ID).
   - For each finding: extract code context, build fix prompt, call fix model (Ollama), parse response.
   - Output proposed patches (unified diff or code blocks). If `--apply`, apply to working tree; if `--commit`, also create a single commit with the attribution message below.
5. **stet refine:** User runs `stet refine [--finding-id ID] [--max-iterations N]`:
   - Same fix generation as fix, then apply and commit, then run stet review on the new commit. If there are still active findings, repeat (fix → apply & commit → review) until the review is clean or `--max-iterations` is reached. Each iteration produces one commit with the same attribution message.
6. User may otherwise review patches from `stet fix` and apply manually (no `--apply`/`--commit`).

**Context extraction: Largest-first, fallback-to-smaller**

To maximize fix quality, use the largest coherent chunk that fits the model's context window, then fall back to smaller chunks if needed.

| Chunk level | Description | Use when |
|-------------|-------------|----------|
| Enclosing function | Full function/block containing the finding (Go: `expand` package) | Go files; fits budget |
| N-line window | ±50 lines around finding line | Non-Go or when enclosing too large |
| Smaller window | ±30, ±20, ±10 lines | Budget still exceeded |
| Minimal | Finding range ±2–5 lines | Last resort |

**Algorithm:**

1. Compute token budget: `contextLimit - systemPrompt - findingPayload - responseReserve`.
2. Try largest chunk: for Go, use `expand` to get enclosing function at finding location; for others, try ±50 lines.
3. Estimate tokens; if over budget, fall back to smaller N-line window (30, 20, 10).
4. Last resort: minimal range around the finding.

**Config:**

- `fix_model` (default: `qwen2.5-coder:7b`): Model for fix generation. Smaller than review model; runs faster and uses less VRAM.
- `fix_temperature` (default: 0.1): Lower than review for more deterministic output.
- Env: `STET_FIX_MODEL`, `STET_FIX_TEMPERATURE`.

**Commit message and attribution:** When stet applies and commits (fix with `--commit`, or refine):

- **Commit message:** Include the line: `written by <MODEL> after review from stet`, where `<MODEL>` is the fix model name (e.g. `qwen2.5-coder:7b`). User may add a custom subject/body via a flag (e.g. `--message "Fix security finding"`).
- **git-ai:** When the git-ai CLI is available and the repo uses it, record fix-authored lines in `refs/notes/ai` per [Git AI Standard v3.0.0](https://github.com/git-ai-project/git-ai/blob/main/specs/git_ai_standard_v3.0.0.md): agent `tool=stet`, `model=<fix_model>`, session id derived from session + finding (e.g. `stet-{session_id}-{finding_short_id}`). Integrate with the published git-ai implementation (e.g. invoke its CLI after applying the fix) rather than reimplementing the note format.
- **Stet note:** Continue writing `refs/notes/stet` on `stet finish` with session/review scope; "reviewed by stet" is expressed there and via the commit message line above, not in the git-ai note.

**stet refine: loop semantics and caps:**

- **Max iterations:** Require a cap (e.g. `--max-iterations 5`; default 3) so the loop always terminates. Exit when the latest review has no active findings or the cap is reached.
- **One commit per iteration:** Each refine iteration creates one commit (fix → commit → review). Document that history may show multiple commits for one "refine" run; alternatively, a future option could accumulate fixes in the working tree and commit once when the review is clean (single-commit refine).
- **Quality bar:** "Passes" means stet's review reports no active findings. Tests, linters, and CI are not run by stet; the user or pipeline remains responsible for those.

**Prompt design (for implementers):**

- System: "You are a code fix assistant. Given a code snippet and a code review finding, output ONLY the corrected code. Do not explain."
- User: file path, line/range, message, suggestion (if any), category, and code context (the extracted chunk).
- Output format: code block only (or unified diff) for easy parsing.

**Dependencies to reuse:**

- `cli/internal/expand` — add `ExpandAtLocation(repoRoot, filePath, startLine, endLine, maxTokens)` for finding-based expansion (today `ExpandHunk` works from diff hunks).
- `cli/internal/ollama.Client` — same `Generate` API; pass fix_model.
- `cli/internal/session` — load session, get active findings (Findings minus DismissedIDs).
- `cli/internal/findings.Finding` — File, Line, Range, Message, Suggestion, Category.

**Output:** For `stet fix` without `--apply`/`--commit`: proposed patches for human review. With `--apply`/`--commit`, or when using `stet refine`, stet may apply and commit as described above. Extension may add "Fix this finding" or "Refine" actions later.

**Industry references:** InferFix (static analyzer + retrieval-augmented prompts), CORE (proposer + ranker LLMs), SonarQube AI CodeFix (finding → fix suggestion), RePair (process-based feedback for small models).

**Comparison:** One-shot fix vs looping refine mirrors [roborev](https://www.roborev.io) (`roborev fix` / `roborev refine`); stet uses its own fix model and session/review data instead of delegating to another agent.

### Impact Reporting

`stet stats` for volume (Stet-reviewed vs not), quality (actionability, FP rate, clean commits), and cost/energy (local kWh, avoided cloud spend). See implementation plan Phase 9.

---

## Phase 9: The "Adaptive" Release (Personalization)

**Goal:** Reduce "False Positive Fatigue" by learning what the user dislikes.

- Continuous learning from dismissals (and later from acceptance patterns) is a core differentiator; metrics (e.g. acceptance vs. dismissal rate by reason) should align with feedback-driven improvement.

### 9.1 Feature: Dynamic Suppression (The "Shut Up" Button)

**Concept:** If a user repeatedly dismisses specific types of feedback, stet should learn to stop offering it.

**Mechanism:**

- **Storage:** Maintain a lightweight local vector store (or simple JSON history) of "Dismissed Findings" in `.review/history`.
- **Retrieval:** Before a review, fetch the embeddings of the last 50 dismissed items.
- **Filter:**
  - **Option A (Prompt):** Inject "Do not report issues similar to: [Examples]" into the system prompt.
  - **Option B (Post-Process):** Calculate semantic similarity between new findings and dismissed findings. If similarity > 0.85, auto-suppress.

### 9.2 Feature: Team "Rulebook" Injection

**Concept:** Allow teams to enforce natural-language standards that aren't covered by `.cursor/rules`.

**Configuration:** Support a `.stet/rules.md` file.

**Usage:**

- A team lead writes: "We use snake_case for database columns but CamelCase for Go structs. Never suggest fmt.Printf in production code."
- stet injects these rules into the system prompt as "High Priority Constraints."

### 9.3 Feature: Feedback-based RAG and strictness tuning

**Concept:** Use dismissal (and optionally acceptance) history to suggest or apply configuration that improves actionability and reduces false-positive fatigue. Extend the feedback loop beyond prompt optimization to **RAG and strictness**.

**Mechanism:**

- **Input:** `.review/history.jsonl` (dismissals with reasons, findings per run). Optionally tag history records with the run's config (e.g. `rag_symbol_max_definitions`, `rag_symbol_max_tokens`, `strictness`) when available.
- **Output:** Suggested config changes (e.g. "suggested rag_symbol_max_definitions: 8", "suggested strictness: lenient") or an optional file (e.g. `.review/suggested_config.toml`) that the user can merge. Alternatively extend `stet optimize` to write both `system_prompt_optimized.txt` and suggested config snippets.
- **Logic:** Correlate dismissal rate (or acceptance rate) with RAG/strictness settings over time; suggest values that associate with higher acceptance or lower false_positive dismissal. No requirement to auto-apply — suggest-only keeps the user in control.

**Scope:** RAG symbol options (`rag_symbol_max_definitions`, `rag_symbol_max_tokens`) and `strictness`. This is a complement to per-hunk adaptive RAG (implementation plan Phase 6.11): 6.11 fits context automatically; feedback-based tuning refines defaults over time from user behavior.

---

## Phase 10: The "Deep Context" Release (Graph Awareness)

**Goal:** Solve "Spooky Action at a Distance" bugs.

Cross-file impact extends stet's context-aware design to reduce FPs from "spooky action at a distance" and aligns with industry emphasis on codebase-aware analysis.

### 10.1 Feature: Cross-File Impact Analysis

**Concept:** Detect when a change in File A breaks logic in File B (even if File B wasn't touched).

**Mechanism:**

- **Symbol Tracking:** Use Tree-sitter to index the public symbols in the changed hunks.
- **Reference Search:** Scan the codebase for usages of those symbols in other files.
- **Review Generation:**
  - If `auth.Login()` signature changed, and `auth_test.go` calls it, check if `auth_test.go` was updated.
  - If not, generate a finding: "You changed Login signature, but auth_test.go is stale. This will likely break the build."

---

## Research Topics (for "Spike" Tickets)

| Topic | Goal | Complexity |
|-------|------|------------|
| Local Vector Stores | Evaluate sqlite-vss vs chromadb for storing dismissal history locally without heavy dependencies. | Medium |
| LSP Integration | Can we tap into the user's running Language Server (LSP) instead of running our own Tree-sitter parsing? | High |
| Review Summarization | Generate a "PR Description" based on the findings (Auto-Draft PR). | Low |
| Documentation quality | Optional: use commit (and future PR) description to improve intent context; document that clear author-side documentation improves stet accuracy. | Low |
| Evaluation corpus | Periodic evaluation on a fixed set of hunks (known-good / known-bad) to track precision/recall as prompts and optimizer change. | Medium |

### Adoption and rollout

Recommend piloting on one team or repo, collecting dismiss reasons and running `stet optimize` periodically, and documenting when to use default vs. strict vs. nitpicky so rollout aligns with feedback-driven improvement.

---

## Feature: Finding Consolidation (Post-Processing)

**Goal:** Identify findings that represent the same underlying issue (fragmented by methodology) and group them for display, so users see conceptual issues rather than many near-duplicate entries.

### Concept

Findings fragment because stet reviews **per hunk** and assigns IDs from `(file, line, range, message)`. The same logical issue at different call sites (e.g. "Potential nil pointer dereference in StreamOut" at lines 301 and 498) or the same root cause reported in different hunks (e.g. "Duplicate function definition for newRunCmd") produce separate findings. One fix often addresses several of them. Consolidation groups these for display.

### Approach

- **Rule-based heuristics (primary):** No LLM calls. Fully compatible with small/local models (PRD: 32K context, laptop hardware).
- **Optional LLM validation (hybrid):** Narrow yes/no prompts for borderline groups only; fits small-model constraints for narrow, well-scoped tasks.

### Rule-based heuristics (implement first)

1. **Same file + message stem similarity** — Reuse `messageStem` / `collapseWhitespace` from [cli/internal/hunkid/hunkid.go](cli/internal/hunkid/hunkid.go). Group findings with high similarity (e.g. Jaccard on word stems or Levenshtein below threshold).
2. **Same file + shared category + overlapping keywords** — Group when category matches and messages share key phrases (e.g. "nil pointer", "StreamOut", "division by zero").
3. **Same file + nearby lines + similar message stem** — Group findings within N lines (e.g. 20) with similar message stem.

### Optional LLM validation (hybrid)

For candidate groups flagged by heuristics: "Are these N findings the same underlying issue? Answer Yes or No." Narrow task, small token footprint, simple output — suitable for 7B–13B models. Skip if heuristics are confident; use only when user enables (e.g. `stet list --grouped --verify`).

### Integration

- **Read-only in `stet list`** — e.g. `stet list --grouped`; session unchanged; display-only grouping.
- **Output format:** Canonical description per group, then member findings:

```text
[Group: Potential nil pointer dereference in StreamOut]
  ca1b234  cli/cmd/stet/main.go:301  warning  Potential nil pointer dereference in StreamOut assignment
  3a0ed5c  cli/cmd/stet/main.go:498  warning  Potential nil pointer dereference in StreamOut assignment

[Group: Duplicate function definition for newRunCmd]
  f0b4f0c  cli/cmd/stet/main.go:356  error  Duplicate function definition for newRunCmd
  54e82de  cli/cmd/stet/main.go:363  error  Duplicate function definition for newRunCmd
```

### Dependencies

- [cli/internal/findings](cli/internal/findings) — `Finding` struct, `ShortID`, active findings.
- [cli/internal/hunkid](cli/internal/hunkid) — `messageStem`, `collapseWhitespace` for similarity.
- No new Ollama call required for the rule-based path.

### Constraints

- **PRD:** Local/small models, 32K context. Rule-based path uses zero inference.
- **LLM path:** Only when explicitly enabled; narrow prompts; optional.
