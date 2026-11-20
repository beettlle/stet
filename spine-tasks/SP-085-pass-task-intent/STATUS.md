# SP-085: pass-task-intent — Status

**Current Step:** Step 3
**Status:** 🔄 In Progress
**Last Updated:** 2026-11-20
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

### Step 0: Add `--intent-file` CLI flag
**Status:** ✅ Complete

- [x] Add `--intent-file` to main.go and pass content into run options

---

### Step 1: Thread intent through run and review
**Status:** ✅ Complete

- [x] Intent string accepted in run.go and passed to reviewer

---

### Step 2: Update Reviewer and Critic prompts
**Status:** ✅ Complete

- [x] Reviewer and System One critic prompts include intent when present

---

### Step 3: Testing & Verification
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
| `VerifyFindingsSystemOne` is also called by the critic-eval sweep; the new `taskIntent` parameter required a mechanical `""` argument there to keep the build compiling (out-of-file-scope, no behavior change). | Accepted (compile-required) | `cli/internal/criticeval/sweep.go` |
| `ReviewHunk` (review.go) is test-only; the pipeline path builds reviewer prompts from systemBase in run.go, so intent injection lives in `Run` + `review.AppendTaskIntent` instead of `ReviewHunk`. | Accepted (scope-minimal) | `cli/internal/review/review.go` |

---

## Execution Log

| Date | Event | Notes |
|------|-------|-------|
