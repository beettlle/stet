**Current Step:** Step 1 (Run qwen/qwen3-coder-next)
**Status:** In progress
**Last Updated:** 2026-10-11
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

## Step 0: Preflight

**Status:** Complete (2026-10-11)

- [x] Tool versions, `lms` estimates, real-ref SHAs — both estimates fit (coder-next 58.49 GiB, 27b 20.97 GiB); 3 SHAs verified; stet rebuilt from HEAD 9d16987 (≥ b0d43f8) and installed to ~/.local/bin before preflight
- [x] Preset parity check (saved LM Studio presets for both models) — "preset parity OK", exit 0
- [x] Smoke request per model (loaded with `--no-speculative-draft-mtp`) — both HTTP 200, finish_reason stop
- [x] Reasoning probe for `qwen/qwen3.8-27b` — PASS: reasoning_content empty, reasoning_tokens 0, no `<think>`, content `[]`

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
| 2026-10-11 | Batch `20261011T174423-6acb` (aborted): coder-next `go-map-race` run 2 lost its finding to an `evidence_lines` counting loop (806 s, 0 findings); 27b with thinking on spent whole 8191-token responses on reasoning (1713 s for one case) | stet fixes `ea2124f` and `14fce09`, harness pins 65536/4096 (`b0d43f8`), 27b thinking off; pre-fix data in `docs/research/model-eval/baseline-pre-fix/` (see PROMPT Amendments) |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-10-10 | Authored | Phase 7 model benchmark; real ref `e9b2e61` replaces the test-only `3248a08` from the draft plan |
| 2026-10-10 | Amended pre-run | MTP off on all loads, preset parity check, per-model smoke request, reasoning-model note (see PROMPT Amendments) |
| 2026-10-11 | Aborted and amended | Batch `20261011T174423-6acb` aborted during the 27b pass; stet fixes for #2/#3, 64k context and 4096 cap, 27b thinking off; re-run from scratch (see PROMPT Amendments) |
| 2026-10-11 | Step 0 complete | Preflight all green (preset parity OK, both smoke requests 200/stop, reasoning probe PASS); stet binary rebuilt from HEAD and reinstalled; preflight in `docs/research/model-eval/preflight.txt` |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
