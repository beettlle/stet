**Current Step:** Not started
**Status:** Ready
**Last Updated:** 2026-10-09
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 1: Config

**Status:** Not started

- [ ] `SystemOneTimeout` field, TOML key, env var, default, validation
- [ ] Config table tests

## Step 2: CLI wiring and docs

**Status:** Not started

- [ ] `newSystemOneClient` helper at all three call sites
- [ ] Contract doc row

## Step 3: Testing & Verification

**Status:** Not started

- [ ] Slow-server helper test
- [ ] Contract `testCommand` (stet review exits 0)
- [ ] Coverage gate

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-10-09 | Stet-in-spine smoke: System One critic failed 3/3 with `Client.Timeout exceeded` (10s hardcoded vs 7.5–51s local OpenJev) | Repo `.review/config.toml` runs with `critic_enabled = false` until this lands |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-10-09 | Authored | Follow-up to the stet-in-spine setup |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
