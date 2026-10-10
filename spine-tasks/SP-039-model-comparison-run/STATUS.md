**Current Step:** Not started
**Status:** Ready
**Last Updated:** 2026-10-10
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

## Step 0: Preflight

**Status:** Not started

- [ ] Tool versions, `lms` estimates, real-ref SHAs
- [ ] Reasoning probe for `qwen/qwen3.8-27b`

## Step 1: Run qwen/qwen3-coder-next

**Status:** Not started

- [ ] Load, `stet benchmark`, harness run (14 cases × 3, 3 real refs × 1), retry failed
- [ ] Commit results

## Step 2: Run qwen/qwen3.8-27b

**Status:** Not started

- [ ] Load, `stet benchmark`, harness run, retry failed
- [ ] Commit results

## Step 3: Blind-judge real-diff findings

**Status:** Not started

- [ ] Export blind TSV, label every row, commit `real-labels.tsv`

## Step 4: Score

**Status:** Not started

- [ ] `summary.json`

## Step 5: Write the document

**Status:** Not started

- [ ] Research doc with pre-registered `## Recommendation`

## Step 6: Restore LM Studio

**Status:** Not started

- [ ] Reload `qwen/qwen3-coder-next` at 65536

## Step 7: Testing & Verification

**Status:** Not started

- [ ] Contract `testCommand`
- [ ] `git status` clean outside File Scope

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-10-10 | Authored | Phase 7 model benchmark; real ref `e9b2e61` replaces the test-only `3248a08` from the draft plan |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
