# SP-069: Critic sees the reviewer's expanded context and RAG definitions — Status

**Current Step:** Not Started
**Status:** ⬜ Not Started
**Last Updated:** 2026-10-24
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

### Step 1: Critic context cap in the review package
**Status:** ⬜ Not Started

- [ ] Named 16384-character cap for both critics; annotation cap unchanged

---

### Step 2: Pass the prepared user prompt to both critics
**Status:** ⬜ Not Started

- [ ] `applyCritic` / `applyCriticSystemOne` use `p.User`; trace prints critic input length

---

### Step 3: Testing & Verification
**Status:** ⬜ Not Started

- [ ] Tests for every Contract `completionCriteria` case
- [ ] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
