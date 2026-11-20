# SP-084: unused-import-false-positive — Status

**Current Step:** Step 1
**Status:** 🔄 In Progress
**Last Updated:** 2026-11-20
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 0: Fix existence claim filter for unused imports
**Status:** ✅ Complete

- [x] Update existence filter for unused-import claims across the whole file
- [x] Add tests matching issue #39

---

### Step 1: Adjust critic criteria for imported callee returns
**Status:** 🔄 In Progress

- [ ] Critic avoids ERROR for imported callee return claims without callee body evidence

---

### Step 2: Testing & Verification
**Status:** ⬜ Not Started

- [ ] Contract `testCommand` passes

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| Issue #39 finding 1 message ("the diff does not show it being used") matched neither `unusedClaimRegex` nor `missingClaimRegex`; extended `unusedClaimRegex` with negated-show phrasing | Applied | `cli/internal/findings/existence.go` |
| Unused-import claims need only ONE use outside decl/import lines (the import binding is the declaration); generic unused claims keep the two-use rule. `run.go` already passes full post-change file content to the filter, so cross-hunk uses were already visible — the gap was claim detection + two-use threshold | Applied | `cli/internal/findings/existence.go` |

---

## Execution Log

| Date | Event | Notes |
|------|-------|-------|
| 2026-11-20 | Step 0 complete | `fix(SP-084)` existence filter: negated-show claim phrasing + unused-import single-use rule; 4 new tests |
