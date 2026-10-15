# SP-045: Dismissals survive `stet finish` — Status

**Current Step:** Not Started
**Status:** 🔵 Ready for Execution
**Last Updated:** 2026-10-14
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

### Step 1: Restorable dismissals
**Status:** ⬜ Not Started

- [ ] Newest-50 history scan; `already_correct` excluded
- [ ] Changed-file filter (one git call per distinct DiffRef)

---

### Step 2: Re-apply in `Start`
**Status:** ⬜ Not Started

- [ ] ID match, then file + category + line-overlap match
- [ ] Wired before final session save; warning on error; verbose/trace output

---

### Step 3: Docs
**Status:** ⬜ Not Started

- [ ] `docs/review-process-internals.md` and `docs/cli-extension-contract.md` updated

---

### Step 4: Testing & Verification
**Status:** ⬜ Not Started

- [ ] Collection and matching unit tests
- [ ] start → dismiss → finish → start integration test; changed-file test
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

---

## Blockers

*None*
