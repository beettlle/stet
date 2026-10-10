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
- [ ] Preset parity check (saved LM Studio presets for both models)
- [ ] Smoke request per model (loaded with `--no-speculative-draft-mtp`)
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
| 2026-10-10 | During the W0 batch, `qwen/qwen3-coder-next` failed every chat request: load-time MTP speculative decoding (LM Studio default, not shown in the Inference tab) errors with "not supported for batched MLX models" at parallel 3 and "requires a trimmable prompt cache (got ArraysCache)" after a reload. Saved presets also differed: 27b had Top P 0.8, Min P 0.5 on, no Top K | PROMPT amended pre-run: `--no-speculative-draft-mtp` on every load, exact preset values in the operator checklist, Step 0 preset parity check and smoke request per model |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-10-10 | Authored | Phase 7 model benchmark; real ref `e9b2e61` replaces the test-only `3248a08` from the draft plan |
| 2026-10-10 | Amended pre-run | MTP off on all loads, preset parity check, per-model smoke request, reasoning-model note (see PROMPT Amendments) |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
