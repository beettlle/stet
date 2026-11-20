# SP-084: unused-import-false-positive — Status

**Current Step:** Step 2
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
**Status:** ✅ Complete

- [x] Critic avoids ERROR for imported callee return claims without callee body evidence

---

### Step 2: Testing & Verification
**Status:** 🔄 In Progress

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
| The systemone critic only keeps/drops findings — it never emits severity itself; finding 2's ERROR came from the reviewer and was vouched for by the critic. Fix: deterministic guard in `VerifyFindingsSystemOne` demotes ERROR → warning when the message asserts a callee return value (`\`callee\` returns X`, "return value of `callee`") and no claimed callee is declared in the critic context. Exported `findings.IdentDeclared` (wraps unexported `identDeclared`) so declaration detection is not duplicated; imports do not count as declarations. SP-080-pinned criteria strings untouched | Applied | `cli/internal/review/critic_systemone.go`, `cli/internal/findings/existence.go` |

---

## Execution Log

| Date | Event | Notes |
|------|-------|-------|
| 2026-11-20 | Step 0 complete | `fix(SP-084)` existence filter: negated-show claim phrasing + unused-import single-use rule; 4 new tests |
| 2026-11-20 | Step 1 complete | `fix(SP-084)` critic demotes ERROR callee-return claims without callee body in context to warning; 3 new tests + 6-case unit table |
