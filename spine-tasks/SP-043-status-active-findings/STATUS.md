# SP-043: `stet status` reports active findings — Status

**Current Step:** Step 2 (Testing & Verification)
**Status:** 🟡 Step 1 Implemented
**Last Updated:** 2026-10-15
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Active count in status
**Status:** ✅ Implemented (build verified; plan review engine-owned)

- [x] `findings:` uses the active count from the loaded session
- [x] `docs/cli-extension-contract.md` `stet status` bullet updated

---

### Step 2: Testing & Verification
**Status:** ⬜ Not Started

- [ ] Status-after-dismiss tests (all dismissed; partial)
- [ ] Other `findings: ` assertions checked
- [ ] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-10-14 | Task staged | PROMPT.md and STATUS.md created (release v1.1.0) |
| 2026-10-15 | Step 1 plan review requested | Skipped in-worker (SP-195); engine runs after worker success |
| 2026-10-15 | Step 1 implemented | `runStatus` computes active count inline from loaded session; contract doc bullet updated; `go build ./cli/...` OK |

---

## Blockers

*None*
