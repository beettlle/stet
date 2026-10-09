**Current Step:** Not started
**Status:** Ready
**Last Updated:** 2026-10-09
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 1: Implementation

**Status:** Not started

- [ ] Widen the `checkSystemOne` guard to critic=systemone OR `rules_selector` OR `rules_annotate`
- [ ] Update the doc comment

## Step 2: Testing & Verification

**Status:** Not started

- [ ] Table test over trigger combinations
- [ ] Failure path with selector on
- [ ] Contract `testCommand` (stet review exits 0)
- [ ] Coverage gate

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-10-09 | Found while setting up stet-in-spine: doctor reported OK with only `rules_selector`/`rules_annotate` on | Doctor misses System One misconfig for SP-033/SP-034 features |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-10-09 | Authored | First stet-gated packet (contract ends with `spine-stet-contract-run.sh default`) |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
