**Current Step:** Step 1: Not started
**Status:** Ready
**Last Updated:** 2026-09-30
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 1: Add fields, defaults, and TOML keys

**Status:** Not Started

- [ ] Config fields with `toml` tags (API key `toml:"-"`)
- [ ] Defaults in `DefaultConfig`; TOML loading in `mergeFile`
- [ ] `Overrides.CriticBackend`

## Step 2: Env vars, overrides, and validation

**Status:** Not Started

- [ ] `applyEnv` for five keys plus `STET_SYSTEMONE_API_KEY`
- [ ] `applyOverrides` for `CriticBackend`
- [ ] Validation for backend, URL, min_keep, samples

## Step 3: Testing and verification

**Status:** Not Started

- [ ] Table tests (defaults, TOML, env, precedence, invalid values)
- [ ] API key not loadable from TOML; never in error text
- [ ] Contract `testCommand` and full `go test ./... -cover`
- [ ] `config.go` coverage ≥ 72%

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| | | | |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-09-30 | Decomposition pass: kept as one packet, re-sized M → S | `critic_backend` alone would be half-wired config; five keys follow the established `applyEnv`/`validateStrictness` pattern |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-09-30 | Authored | Packet created |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
