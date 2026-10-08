**Current Step:** Complete
**Status:** Done
**Last Updated:** 2026-10-08
**Last Updated:** 2026-09-30
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 1: Flag and client wiring

**Status:** Complete

- [x] `--critic-backend` flag and override
- [x] Client and fields at every `CriticEnabled` call site

## Step 2: Docs

**Status:** Complete

- [x] Contract doc
- [x] Internals §7.10
- [x] README pointer

## Step 3: Testing and verification

**Status:** Complete

- [x] Flag and client tests
- [x] End-to-end start with System One mock
- [x] Contract `testCommand` and full `go test ./... -cover`
- [x] `main.go` coverage ≥ 72%

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| | | | |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-09-30 | Created by decomposition of original SP-017 (pipeline wiring); docs kept here rather than a separate packet | Docs only describe the flag once it exists; avoids a tiny serial docs packet |
| 2026-10-08 | `cli/internal/run` has a pre-existing flaky TempDir cleanup failure (macOS `unlinkat .../.git: directory not empty`), reproduced on a clean checkout with a different test each run | Unrelated to this packet (mustNotChange `cli/internal/**`); full suite green on re-run. Not fixed here to respect file scope |
| 2026-10-08 | `cmd/stet/main.go` and `main_test.go` were already not gofmt-clean on HEAD (4 pre-existing alignment hunks, untouched) | Kept the diff scoped; only realigned the rerun options block this packet edited |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-09-30 | Authored | Split from SP-017 |
| 2026-10-08 | Step 1 done | `--critic-backend` on start/run/rerun; `systemOneRunOptions` helper wires client + 5 System One fields at all three `CriticEnabled` call sites; commit a620c2f |
| 2026-10-08 | Step 2 done | Contract doc (5 keys + env-only API key + flag), internals §7.10 both backends, README `--verify` pointer; commit d9ac73b |
| 2026-10-08 | Step 3 done | Override/helper/e2e tests added; contract testCommand green (74.8% pkg, main.go 74.8% ≥ 72%); full `go test ./... -cover` green (35 pkgs); extension `npm test` 93/93 + coverage guard PASS |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
