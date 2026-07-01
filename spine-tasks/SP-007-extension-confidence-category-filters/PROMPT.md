# Task: SP-007 — Extension confidence and category filters

**Created:** 2026-06-30
**Size:** S

## Review Level: 1

**Assessment:** UI-only filtering on data the extension already parses; low blast radius.
**Score:** 2/8 — Blast radius: 0, Pattern novelty: 1, Security: 0, Reversibility: 1

## Mission

Complete Phase 4.5 / Phase 5 extension gaps: let users **filter and dim findings** in the panel by **confidence threshold** and **category** (e.g. show only security, hide low-confidence).

Filtering is display-only; session and CLI state unchanged.

## Dependencies

- **SP-006**

## Context to Read First

- `extension/src/findingsPanel.ts`
- `extension/src/contract.ts` — `Category`, `confidence`
- `docs/implementation-plan.md` Phase 4.5 / 5.2 notes
- `docs/cli-extension-contract.md` — finding JSON shape

## Environment

- **Workspace:** `extension/`

## File Scope

- `extension/src/findingsPanel.ts`
- `extension/src/findingsPanel.test.ts`
- `extension/src/extension.ts` (filter state / commands)
- `extension/package.json` (configuration: min confidence, category allowlist)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd extension && npm run compile && npm test` |
| fileScopeMustChange | `extension/src/findingsPanel.ts` |
| fileScopeMustNotChange | `cli/**` |
| completionCriteria | User can set min confidence and category filter; panel hides/dims filtered findings; default shows all |

## Steps

### Step 0: Design filter UX

- [ ] VS Code settings: `stet.minConfidence` (0–1, default 0), `stet.categories` (optional allowlist)
- [ ] Dim vs hide: low confidence dimmed (opacity/description) or hidden — pick one, document in STATUS

### Step 1: Implement filter logic

- [ ] Apply filters when rendering tree items
- [ ] Show active filter summary in panel title or status (e.g. "3 hidden by filter")

### Step 2: Tests and documentation

- [ ] Unit tests for filter predicates
- [ ] README extension section: filter settings

### Step 3: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Run `cd extension && npm run compile && npm test`

## Completion Criteria

- [ ] All steps complete
- [ ] Confidence and category filters work
- [ ] Tests pass

## Git Commit Convention

- **Implementation:** `feat(SP-007): description`

## Do NOT

- Change CLI finding schema
- Implement dismiss (SP-006)

---

## Amendments (Added During Execution)
