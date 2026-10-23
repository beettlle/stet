# SP-064: `jsts.Extensions()` returns a copy; unexport the slice — Status

**Current Step:** 2
**Status:** 🟨 Step 2 In Progress
**Last Updated:** 2026-10-23
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Accessor and callers
**Status:** ✅ Complete

- [x] Unexported slice; `Extensions()` returns a clone
- [x] Callers use the accessor

---

### Step 2: Testing & Verification
**Status:** ⬜ Not Started

- [ ] Mutation-isolation test
- [ ] No non-call uses of `jsts.Extensions`
- [ ] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
