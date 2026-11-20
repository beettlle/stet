# SP-085: pass-task-intent — Status

**Current Step:** Step 3
**Status:** ✅ Complete
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
**Status:** ✅ Complete

- [x] Contract `testCommand` passes

**Evidence:** `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/cmd/stet/... ./cli/internal/run/... ./cli/internal/review/... -count=1` — all ok (stet 78.2s, run 14.3s, review 7.7s). Also `go build ./cli/...` and `go test ./cli/internal/criticeval/` ok after the sweep.go caller update.

New tests:
- `TestVerifyFindingsSystemOne_taskIntentPrefixesState` (review): objective prefixes request state, trimmed, blank intent byte-identical, objective never in instructions.
- `TestRun_taskIntent_reachesReviewerPromptAndSystemOneCritic` (run): end-to-end Run with mock LLM + fake decider — intent lands in reviewer system prompt and critic state; no-intent control unchanged.
- `TestRunCLI_runIntentFile` (cmd/stet): missing intent file exits non-zero naming the path; existing file accepted.
- Existing pinned System One tests updated for the new `taskIntent` parameter (21 call sites).

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

## Execution Log

| Date | Event | Notes |
|------|-------|-------|
| 2026-11-20 | Step 0 committed | `--intent-file` flag on `stet run`; `RunOptions.Intent` added |
| 2026-11-20 | Step 1 committed | `review.AppendTaskIntent`; Run injects intent into reviewer systemBase + token-estimation prompt; `reviewPipelineOpts.TaskIntent` |
| 2026-11-20 | Step 2 committed | `VerifyFindingsSystemOne(taskIntent)` state prefix; sweep.go compile fix; pinned tests updated |
| 2026-11-20 | Step 3 committed | New tests (review/run/cmd/stet); contract testCommand passes |

---

## Execution Log

| Date | Event | Notes |
|------|-------|-------|
