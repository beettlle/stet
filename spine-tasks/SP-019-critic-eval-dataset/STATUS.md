**Current Step:** Step 1: Implement loaders
**Status:** In Progress
**Last Updated:** 2026-09-30
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 0: Confirm labeling against real data

**Status:** Done (2026-10-08)

- [x] Inspect real history/session schema and confirm label rules
- [x] Record rule changes in Discoveries

> ⚠️ Hydrate: adjust Step 1 checkboxes if label sources differ from PROMPT.

## Step 1: Implement loaders

**Status:** Not Started

- [ ] `LoadFromState`
- [ ] `LoadDataset` with validation
- [ ] Grouping, de-duplication, conflicts, counts

## Step 2: Testing and verification

**Status:** Not Started

- [ ] Synthetic fixtures
- [ ] Edge cases per PROMPT
- [ ] Contract `testCommand` and full `go test ./... -cover`
- [ ] `dataset.go` coverage ≥ 72%

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| | | | |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-09-30 | History stores `prompt_context` only for dismissed findings; positives must come from `session.json` `finding_prompt_context` or a curated dataset | Label sources in PROMPT reflect this (authoring-time finding) |
| 2026-09-30 | Decomposition pass: original M "threshold eval harness" split into SP-019 (dataset), SP-024 (scoring and sweep), SP-025 (`stet critic-eval` command) | Dataset loader has no dependencies and can run in the first wave |
| 2026-10-08 | Step 0 (real data, 365-record history.jsonl, no active session.json): 413 dismissals total — 134 `false_positive`, 29 `wrong_suggestion`, 110 `already_correct`, 140 `out_of_scope`; 104/413 have non-empty `prompt_context`, of which 75 are `false_positive`/`wrong_suggestion` | Drop-label rules confirmed: `false_positive`/`wrong_suggestion` with non-empty `prompt_context`; exclusions (`already_correct`, `out_of_scope`) confirmed as distinct reasons; no rule changes needed, Step 1 checkboxes unchanged |
| 2026-10-08 | `session.DismissedIDs`, `FindingPromptContext`, `Findings` are struct fields on `session.Session` (not methods); use `session.Load` + direct field access | PROMPT's API naming adjusted; no code impact |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-09-30 | Authored | Packet created as eval harness (M) |
| 2026-09-30 | Split | Narrowed to dataset loader; scoring → SP-024, command → SP-025 |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
