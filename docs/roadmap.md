# Stet Future Roadmap: Beyond Phase 7

**Status:** Draft / Research

**Context:** This document outlines the feature roadmap for stet after the core "Defect-Focused" implementation (Phase 6) and "Polish" (Phase 7) are complete.

**Objective:** Evolve stet from a "local CLI tool" into a "universal review agent" that integrates with AI IDEs and learns from user behavior.

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

---

## Phase 9: The "Adaptive" Release (Personalization)

**Goal:** Reduce "False Positive Fatigue" by learning what the user dislikes.

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

---

## Phase 10: The "Deep Context" Release (Graph Awareness)

**Goal:** Solve "Spooky Action at a Distance" bugs.

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
