**Current Step:** Step 1: Not started
**Status:** Ready
**Last Updated:** 2026-09-30
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 1: Extract helper

**Status:** Not Started

- [ ] `applyCritic` with current logic
- [ ] Both call sites replaced
- [ ] Diff confined to critic blocks

## Step 2: Testing and verification

**Status:** Not Started

- [ ] Existing critic tests unchanged and passing
- [ ] Direct helper test if coverage needs it
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
| 2026-09-30 | Decomposition pass: original M "pipeline wiring" split into SP-017 (refactor), SP-022 (System One branch in `run.go`), SP-023 (CLI flag, client construction, docs) | Refactor has no deps and can run in the first wave; docs kept with the flag (SP-023) so they never describe an unavailable option |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-09-30 | Authored | Packet created as pipeline wiring (M) |
| 2026-09-30 | Split | Narrowed to refactor; backend → SP-022, CLI/docs → SP-023 |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
