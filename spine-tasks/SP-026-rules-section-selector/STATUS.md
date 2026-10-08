**Current Step:** Step 1: Not started
**Status:** Ready
**Last Updated:** 2026-10-08
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 1: Sections and selection

**Status:** Not Started

- [ ] `Section`, `SplitSections`
- [ ] `ChoiceDecider`, `SelectSections`

## Step 2: Prompt assembly

**Status:** Not Started

- [ ] `AppendRuleSections`

## Step 3: Testing & Verification

**Status:** Not Started

- [ ] Split, select, and prompt tests per PROMPT
- [ ] Contract `testCommand` and full `go test ./... -cover`
- [ ] Coverage ≥ 72% for `selector.go` and `prompt.go`

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| | | | |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-10-08 | Whole rule files are larger than the 1,000-token budget, so selection works on sections, not files | Section split is part of the selector |
| 2026-10-08 | Split library (SP-026) from pipeline wiring (SP-033) | Keeps each packet S and file scopes disjoint |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-10-08 | Authored | From SP-020 follow-up table |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
