**Current Step:** Step 1: Not started
**Status:** Ready
**Last Updated:** 2026-09-30
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 1: Types, shared request helper, Decide

**Status:** Not Started

- [ ] Request/response types (noul only)
- [ ] Shared request helper (auth, errors, retry, size cap, redaction)
- [ ] `Decide` with response validation

## Step 2: Testing and verification

**Status:** Not Started

- [ ] `httptest.Server` cases per PROMPT
- [ ] Auth header only with key; key never in errors
- [ ] Contract `testCommand` and full `go test ./... -cover`
- [ ] `client.go` coverage ≥ 72%

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| | | | |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-09-30 | Decomposition pass: split M → S. `Health`/`Models` moved to SP-021 | SP-016 (verifier) only needs `Decide`, so it can start as soon as this lands; SP-021 runs in parallel with SP-016 |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-09-30 | Authored | Packet created |
| 2026-09-30 | Split | Health/Models → SP-021 |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
