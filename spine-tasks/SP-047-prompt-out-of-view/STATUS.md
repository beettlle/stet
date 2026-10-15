# SP-047: Prompt: out-of-view imports and definitions are not evidence — Status

**Current Step:** 2
**Status:** 🟡 In Progress
**Last Updated:** 2026-10-14
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Prompt sentence
**Status:** ✅ Complete

- [x] Review step 1 extended with the out-of-view sentence

---

### Step 2: Testing & Verification
**Status:** ⬜ Not Started

- [ ] Prompt test asserts the new wording; exact-text assertions updated
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
| 2026-10-15 | Worktree setup | Setup hook had not run; opened stet session at pre-worker HEAD 63bdf07 via `scripts/spine-worktree-setup.sh` |
| 2026-10-15 | Step 1 | Added out-of-view sentence to `DefaultSystemPrompt` review step 1 (`cli/internal/prompt/prompt.go`); `"await"` quoted with double quotes because the const is a backtick-delimited raw string literal |

---

## Blockers

*None*
