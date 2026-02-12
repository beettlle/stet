# Product Requirements Document: Stet Refine (Auto-Fix)

**Feature:** Stet Refine (Auto-Fix)
**Status:** Draft
**Parent PRD:** [docs/PRD.md](PRD.md)

**One-line summary:** An optional refinement layer for Stet that uses local LLMs to automatically suggest and apply fixes for review findings, with user control over batch vs. on-demand generation.

---

## 1. Motivation

The core Stet tool provides high-quality code review using local LLMs. However, users often want to immediately *act* on findings without context switching or manual coding. "Refining" (auto-fixing) bridges the gap between identification and resolution, leveraging the same local models.

**Key Constraints:**
-   **Local Models:** Target 32k context window (e.g., qwen2.5-coder:32b).
-   **Performance:** Inference is expensive; users must control when costs are incurred.
-   **Safety:** Fixes must be verified to minimalize hallucinations or breakages.

---

## 2. Goals and Non-Goals

### Goals
-   **Automated Fixes:** Generate code patches to resolve specific findings.
-   **User Control:** Support both "On-Demand" (fix this finding now) and "Pre-Computed" (fix all findings during review) flows, controlled by flags/config to manage latency.
-   **Hunk-Centric Context:** Use surgical context windows (hunk + surrounding lines) to fit within 32k limits.
-   **Self-Correction:** Use the existing Review Engine to verify generated fixes before showing them to the user.
-   **IDE Integration:** Seamless "Fix" button in the VS Code extension.

### Non-Goals
-   **Whole-File Refactoring:** We target specific findings, not general "refactor this file" requests.
-   **Multi-File Fixes:** v1 focuses on single-file, single-hunk fixes.
-   **Autonomous Agents:** The user is always in the loop to approve/apply fixes.

---

## 3. Design Decisions

| Decision | Choice |
| :--- | :--- |
| **Fix Strategy** | **Hybrid: On-Demand (Default) + Pre-Computed (Opt-in).**<br>By default, `stet start` is review-only (fast). Users can click "Fix" in the IDE to generate a fix on-demand.<br>Alternatively, users can run `stet start --suggest-fixes` (or configured flag) to pre-compute fixes for all findings during the initial review, accepting higher latency. |
| **Context Strategy** | **Hunk-Centric.**<br>Input: Finding + Original Hunk + Surrounding Lines (e.g., +/- 20 lines) + Signature Injections (RAG-Lite).<br>Target: < 2k tokens per fix prompt. |
| **Verification** | **"The Stet Loop" (Self-Correction).**<br>When a fix is generated, the tool temporarily applies it to an in-memory/worktree buffer and runs the Review Engine on the *new* hunk.<br>If the Review Engine flags errors, the fix is discarded or retried. |
| **Output format** | **Git Patch / Diff.**<br>Fixes are returned as unified diffs that can be applied via `git apply` or extension API. |

---

## 4. User Flows

### 4.1 Interactive Fix (Extension - Default)
1.  User runs review (`stet start` / extension). Findings appear.
2.  User sees a finding (e.g., "Style: Unused variable").
3.  User clicks the **"Magic Wand" / "Fix"** icon on the finding.
4.  Extension calls `stet fix <finding-id>`.
5.  CLI generates the fix (on-demand), verifies it, and returns a diff.
6.  Extension shows a **Diff View** (Preview).
7.  User accepts; extension applies the patch to the working tree.

### 4.2 Pre-Computed Fixes (CLI / Opt-in)
1.  User runs `stet start --suggest-fixes`.
2.  CLI runs review. For every finding (or high-confidence category), it *also* prompts the LLM for a fix.
3.  Output includes a `suggestion` field with the patch.
4.  Extension displays findings *with* the "Apply Fix" button already available (no waiting).

### 4.3 Batch Refine (CLI)
1.  User runs `stet refine` (or `stet fix --all`).
2.  CLI identifies all open findings.
3.  CLI generates fixes for them sequentially or in parallel.
4.  CLI applies them to the working tree (or a new branch `stet-fixes`).

---

## 5. Architecture Updates

### 5.1 CLI
-   **New Command:** `stet fix <finding-id>`
    -   Inputs: Finding ID (look up hunk/context from session state).
    -   Outputs: JSON with `patch`, `verification_result`.
-   **Updated Command:** `stet start --suggest-fixes`
    -   Pipeline: Review -> For each finding -> Generate Fix -> Verify -> Output.
-   **Fix Engine:**
    -   Prompt construction (Hunk + Finding -> New Code).
    -   Verification loop (Apply -> Review -> Check).

### 5.2 Extension
-   **UI:** Add actions to Findings tree/list.
-   **Preview:** Leverage VS Code's `diff` document provider to show side-by-side diffs of the fix.

## 6. Functional Requirements

| ID | Requirement | Notes |
| :--- | :--- | :--- |
| FR-R1 | Generate a code fix for a specific finding ID | Input: finding ID. Output: patch. |
| FR-R2 | Support `--suggest-fixes` flag in `stet start` | Pre-computes fixes during review. |
| FR-R3 | Verify fixes using the Review Engine | "Stet Loop". Ensure fix doesn't introduce new high-severity issues. |
| FR-R4 | Apply fixes to working tree | Via CLI (`stet apply`) or Extension. |

## 7. Open Questions
-   **Fuzzy Patching:** If the user has edited the file since the review, the hunk might not match exactly. `git apply` is strict. We may need a "smart patcher" that uses line numbers or fuzzy matching if the file has drifted slightly.
-   **Formatting:** Generated code might not match project style perfectly. We rely on the user's IDE format-on-save or a post-fix `gofmt`/`prettier` step if configured.
