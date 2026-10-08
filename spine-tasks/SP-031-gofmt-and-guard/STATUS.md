**Current Step:** Step 1: Not started
**Status:** Ready
**Last Updated:** 2026-10-08
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 1: Format and guard

**Status:** Not Started

- [ ] `gofmt -w` under `cli/` (formatting-only diff)
- [ ] Guard in `scripts/build-all.sh`
- [ ] Guard proven with a scratch file (not committed)

## Step 2: Testing & Verification

**Status:** Not Started

- [ ] Contract `testCommand`
- [ ] Full `go test ./... -count=1 -cover`
- [ ] Coverage gate (77% / 72%)

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| | | | |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-10-08 | `gofmt -l cli/` lists 35 files, not only `cli/cmd/stet` | Repo-wide formatting; must run alone in its wave |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-10-08 | Authored | From CONTEXT.md tech-debt backlog |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
