# SP-059: JS RAG skips built-in method names only as member calls — Status

**Current Step:** 2
**Status:** 🔄 In Progress
**Last Updated:** 2026-10-22
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Split globals from member-method names
**Status:** ✅ Complete

- [x] Globals set and member-method set; member names skipped only as `.name(`
- [x] Import-bound names, bare arguments, and declarations kept

---

### Step 2: Testing & Verification
**Status:** ⬜ Not Started

- [ ] Import-bound `get` resolved from temp repo
- [ ] Member calls add no built-in candidates
- [ ] Bare `log(x)` is a candidate
- [ ] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|
| 1 | plan | 1 | skipped (engine-owned; SP-195) | .reviews/1-20261022T234625.md |

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
