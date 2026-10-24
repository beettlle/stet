# SP-072: Dismissals follow line shifts in `stet start` and `stet run` — Status

**Current Step:** Not Started
**Status:** ⬜ Not Started
**Last Updated:** 2026-10-24
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Start-time restore re-maps instead of dropping
**Status:** ⬜ Not Started

- [ ] Re-map per `diffRef`; edited spans dropped; `Start` call site updated

---

### Step 2: Run-time skip re-maps from `LastReviewedAt`
**Status:** ⬜ Not Started

- [ ] Dismissed spans re-mapped before the overlap check; fallback when unavailable

---

### Step 3: Testing & Verification
**Status:** ⬜ Not Started

- [ ] Tests (a)–(d)
- [ ] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
