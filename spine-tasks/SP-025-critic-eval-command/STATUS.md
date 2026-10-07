**Current Step:** Step 1: Not started
**Status:** Ready
**Last Updated:** 2026-09-30
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 1: Implement command

**Status:** Not Started

- [ ] Command and flags
- [ ] Config → client → dataset → score → evaluate
- [ ] Output formats, errors, exit codes

## Step 2: Testing and verification

**Status:** Not Started

- [ ] `TestRunCLI_criticEval*` cases per PROMPT
- [ ] Contract `testCommand` and full `go test ./... -cover`
- [ ] `main.go` coverage ≥ 72%

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| | | | |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-09-30 | Created by decomposition of original SP-019 (threshold eval harness) | Serialized after SP-018 because both edit `main.go` |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-09-30 | Authored | Split from SP-019 |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
