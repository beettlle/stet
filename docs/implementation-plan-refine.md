# implementation-plan-refine.md

This document outlines the implementation roadmap for the **Stet Refine (Auto-Fix)** feature. It extends the core Stet architecture to support automated code fixing.

**Source of truth:** [docs/PRD-Refine.md](PRD-Refine.md)

---

## 1. Decisions & Constraints

| Decision | Choice |
| :--- | :--- |
| **Fix Strategy** | **Hybrid:** On-demand (`stet fix <id>`) is the default. Pre-computed (`--suggest-fixes`) is opt-in for batch processing. |
| **Context** | **Hunk-Centric:** Context window is strictly limited to the finding's hunk + immediate surroundings to fit 32k limits and ensure speed. |
| **Output** | **Unified Diff:** The fix engine outputs a standard git patch (unified diff) that can be applied by the extension or CLI. |
| **Verification** | **Self-Correction:** All generated fixes MUST undergo a verification pass (Review Engine check) before being returned to the user, unless explicitly skipped via flag. |

---

## 2. Phase Overview

| Phase | Focus |
| :--- | :--- |
| **Phase R1** | **Core Fix Engine & CLI:** Implement `stet fix <id>` logic, context extraction, and the LLM fix prompt. |
| **Phase R2** | **Verification Loop:** Implement the "Apply -> Review -> Check" safety loop to reduce hallucinations/regressions. |
| **Phase R3** | **Pre-Computation:** Add `--suggest-fixes` to `stet start` for batch generation. |
| **Phase R4** | **Extension Integration:** Add "Fix" button, diff preview, and apply logic in VS Code. |

```mermaid
flowchart LR
  R1[Phase_R1_Core_Engine] --> R2[Phase_R2_Verification]
  R2 --> R3[Phase_R3_PreCompute]
  R3 --> R4[Phase_R4_Extension]
```

---

## 3. Per-phase detail

### Phase R1: Core Fix Engine & CLI

| Sub-phase | Deliverable | Tests / Coverage |
| :--- | :--- | :--- |
| **R1.1** | **Context Extractor:** Given a finding ID, retrieve the original hunk and extract N lines of context from the read-only worktree. | Unit tests: finding -> correct file context extracted. |
| **R1.2** | **Fix Prompt:** Design the system/user prompt for fixing. Inputs: context, hunk, finding message. Output: Corrected code block. | Unit tests: mock inputs -> expected prompt structure. |
| **R1.3** | **CLI `stet fix`:** Implement `stet fix <id>`. Calls LLM, parses code block, computes diff against original file. Outputs JSON: `{ "patch": "...", "id": "..." }`. | Integration: `stet fix` with mock LLM returns valid patch. 77% project code coverage maintained. |

**Phase R1 Exit:** User can run `stet fix <id>` (with a mock or real LLM) and get a patch.

---

### Phase R2: Verification Loop (Safety)

| Sub-phase | Deliverable | Tests / Coverage |
| :--- | :--- | :--- |
| **R2.1** | **In-Memory Patcher:** Utility to apply a patch to the file content in memory (or temp file) without touching working tree. | Unit: apply patch -> verify content. |
| **R2.2** | **Review Loop:** pipeline: (1) Generate Fix -> (2) Apply to Temp -> (3) Run Review Engine on new hunk -> (4) If severe findings, discard/retry. | Integration: Mock LLM generates "bad" fix -> Verification rejects it. |
| **R2.3** | **Result Metadata:** specific failure reasons in output JSON if fix fails verification. | Unit: error propagation. |

**Phase R2 Exit:** `stet fix` is "safe" — it won't suggest code that introduces obvious syntax errors or high-severity regressions (detectable by strict/semantic analysis).

---

### Phase R3: Pre-Compution (Batch)

| Sub-phase | Deliverable | Tests / Coverage |
| :--- | :--- | :--- |
| **R3.1** | **`--suggest-fixes` Flag:** Add flag to `stet start`. If present, trigger Fix Engine for generated findings. | Integration: `stet start --suggest-fixes --dry-run` -> findings include `suggestion` field. |
| **R3.2** | **Concurrency/Batching:** Ensure fix generation doesn't block the *entire* review output if streaming, or parallelize requests if possible (subject to local LLM limits). | Performance check: minimal blocking. |

**Phase R3 Exit:** Users can choose to pay the latency cost upfront to get fixes ready-to-go.

---

### Phase R4: Extension Integration

| Sub-phase | Deliverable | Tests / Coverage |
| :--- | :--- | :--- |
| **R4.1** | **UI Action:** Add "Fix" icon/button to findings in the custom panel. | Manual/UI Test. |
| **R4.2** | **Fix Protocol:** Extension invokes `stet fix <id>`. Handles "loading" state. | Extension unit tests. |
| **R4.3** | **Diff Preview & Apply:** Display the returned patch using VS Code's diff view ( `vscode.diff` command on virtual documents). "Apply" button writes to disk. | Manual/UI verification. |

**Phase R4 Exit:** Complete feature available in IDE.

---

## 4. Coverage

Same rules as core project:
-   **77%** line coverage project-wide.
-   **72%** minimum per file.
