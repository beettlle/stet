**Current Step:** Step 1: In progress
**Status:** In Progress
**Last Updated:** 2026-10-08
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 1: Recursive glob matching

**Status:** In Progress

- [ ] `matchGlob` with `**` segment semantics, used by `FilterRules`
- [ ] Non-`**` behaviour preserved

## Step 2: Testing & Verification

**Status:** Not Started

- [ ] Table tests (Mission examples, edge cases, malformed pattern)
- [ ] Existing rules/prompt tests unchanged and passing
- [ ] Contract `testCommand` and full `go test ./... -cover`
- [ ] `rules.go` coverage ≥ 72%

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| | | | |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-10-08 | Found by SP-020 research (tech-debt backlog in CONTEXT.md) | Fix changes which rules reach review prompts |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-10-08 | Authored | From SP-020 follow-up table |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
