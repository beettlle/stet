# Task: SP-006 — Extension dismiss findings

**Created:** 2026-06-30
**Size:** M

## Review Level: 2

**Assessment:** Wires CLI dismiss into extension UI; must preserve history/optimizer feedback loop.
**Score:** 4/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 2

## Mission

Add **dismiss** capability to the Cursor extension: user can mark a finding as won't-fix from the findings panel, invoking `stet dismiss <id> [reason]` and removing it from the active list. Optional reason picker aligned with CLI reasons (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`).

## Dependencies

- **SP-005**

## Context to Read First

- `cli/cmd/stet/main.go` — `stet dismiss` command and reasons
- `cli/internal/history/schema.go` — dismissal reasons for optimizer
- `extension/src/findingsPanel.ts`
- `extension/src/extension.ts`
- `docs/cli-extension-contract.md`

## Environment

- **Workspace:** `extension/`

## File Scope

- `extension/src/findingsPanel.ts`
- `extension/src/findingsPanel.test.ts`
- `extension/src/extension.ts`
- `extension/src/cli.ts` (if dismiss spawn helper needed)
- `extension/src/cli.test.ts`
- `extension/package.json` (commands/menus)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd extension && npm run compile && npm test` |
| fileScopeMustChange | `extension/src/findingsPanel.ts`, `extension/src/findingsPanel.test.ts` |
| fileScopeMustNotChange | `cli/**` |
| completionCriteria | Dismiss from panel calls CLI; finding removed from active list; reason passed when selected |

## Steps

### Step 0: Design UX

- [ ] Context menu or inline action on finding row
- [ ] Reason: quick-pick optional; default dismiss without reason if user cancels picker
- [ ] Confirm CLI id format (prefix vs full id)

### Step 1: CLI integration

- [ ] Add `spawnStet(['dismiss', id, reason?])` helper or reuse `spawnStet`
- [ ] On success, refresh findings from session or remove locally

### Step 2: Panel and commands

- [ ] Register `stet.dismissFinding` command
- [ ] Wire tree item context menu
- [ ] Tests for dismiss flow (mock CLI)

### Step 3: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Run `cd extension && npm run compile && npm test`

## Completion Criteria

- [ ] All steps complete
- [ ] Dismiss works from extension with CLI parity
- [ ] Tests pass

## Git Commit Convention

- **Implementation:** `feat(SP-006): description`

## Do NOT

- Implement confidence/category filters (SP-007)
- Change Go dismiss semantics

---

## Amendments (Added During Execution)

**2026-07-01:** `fileScopeMustChange` narrowed to `findingsPanel*` — dismiss command may register in `extension.ts`; contract proof is panel deliverables. Removed `spine-tasks/**` from must-not-change.
