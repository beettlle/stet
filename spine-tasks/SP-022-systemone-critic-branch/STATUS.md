**Current Step:** Step 1: Not started
**Status:** Ready
**Last Updated:** 2026-09-30
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 1: Options and pass-through

**Status:** Not Started

- [ ] Fields on pipeline, Start, and Run options
- [ ] Pass-through at every build site

## Step 2: Backend branch

**Status:** Not Started

- [ ] `applyCritic` System One branch
- [ ] Error message and trace
- [ ] Dry-run never calls System One

## Step 3: Testing and verification

**Status:** Not Started

- [ ] Fake-decider cases per PROMPT
- [ ] Contract `testCommand` and full `go test ./... -cover`
- [ ] `run.go` coverage ≥ 72%

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| | | | |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-09-30 | Created by decomposition of original SP-017 (pipeline wiring) | Internal run options only; user-visible flag lands in SP-023 |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-09-30 | Authored | Split from SP-017 |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
