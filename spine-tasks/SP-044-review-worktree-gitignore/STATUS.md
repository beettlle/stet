# SP-044: Keep the review worktree out of `git add` — Status

**Current Step:** Not Started
**Status:** 🔵 Ready for Execution
**Last Updated:** 2026-10-14
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Ignore helper and wiring
**Status:** ⬜ Not Started

- [ ] `ignore.go` helper (inside-repo check, exclusive create, `*`)
- [ ] `Create` calls it; failure warns only

---

### Step 2: Docs
**Status:** ⬜ Not Started

- [ ] `docs/cli-extension-contract.md` `.review/` paragraph accurate

---

### Step 3: Testing & Verification
**Status:** ⬜ Not Started

- [ ] `git add -A` regression test (no gitlink staged)
- [ ] Existing file untouched; outside-repo root; read-only helper error
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
