# SP-051: One JS/TS extension list for expand and RAG — Status

**Current Step:** Not Started
**Status:** ⬜ Not Started
**Last Updated:** 2026-10-17
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Shared list
**Status:** ⬜ Not Started

- [ ] `cli/internal/jsts/ext.go` with `Extensions` and `IsExt`; unit test

---

### Step 2: Wire both consumers
**Status:** ⬜ Not Started

- [ ] `expand` uses `jsts.IsExt`
- [ ] `rag/js` registers all extensions (adds `.jsx`)

---

### Step 3: Testing & Verification
**Status:** ⬜ Not Started

- [ ] `.jsx` RAG test
- [ ] `.jsx` expand test
- [ ] `.mjs`/`.cjs` still resolve
- [ ] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
