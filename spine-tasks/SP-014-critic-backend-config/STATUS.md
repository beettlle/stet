**Current Step:** Step 2: Env vars, overrides, and validation
**Status:** In Progress
**Last Updated:** 2026-10-08
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 1: Add fields, defaults, and TOML keys

**Status:** Complete

- [x] Config fields with `toml` tags (API key `toml:"-"`)
- [x] Defaults in `DefaultConfig`; TOML loading in `mergeFile`
- [x] `Overrides.CriticBackend`

## Step 2: Env vars, overrides, and validation

**Status:** In Progress

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
| 2026-10-08 | `internal/stats` has an environment-dependent TempDir cleanup flake | Not a config regression; stash-verified on baseline; candidate tech-debt packet (outside SP-014 scope) |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-09-30 | Authored | Packet created |
| 2026-10-08 | Step 1 complete | Fields/defaults/TOML/overrides added; validators shared with env/override paths; package coverage 80.4% |
| 2026-10-08 | Pre-existing flake | `internal/stats` TempDir RemoveAll cleanup flake (`TestVolume_noNotesInRange`, `TestGitAIMetrics_validNote`) reproduces on pristine baseline (stash-verified); macOS `.git` unlink race, out of SP-014 file scope |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
