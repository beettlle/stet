**Current Step:** Complete
**Status:** Done
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

**Status:** Complete (2026-10-11)

- [x] Load, `stet benchmark`, harness run (14 cases × 3, 3 real refs × 1), retry failed — first benchmark attempt hit a 15-min warmup timeout right after load (server responded 1.3 s on retry; no recurrence); harness finished 45/45 `ok` in one invocation, no `--retry-failed` needed
- [x] Commit results (`731c4f0`)

## Step 2: Run qwen/qwen3.8-27b

**Status:** Complete (2026-10-11)

- [x] Load, `stet benchmark`, harness run, retry failed — 44/45 `ok` in invocation 1 (exit 3, budget), last real ref in invocation 2; 45/45 `ok`, no retries
- [x] Commit results (`a0a9a9a`)

## Step 3: Blind-judge real-diff findings

**Status:** Complete (2026-10-11)

- [x] Export blind TSV (70 findings), label every row without opening `blind-key.json` (each row checked against `git show <ref>:<file>` and the commit diff), commit `real-labels.tsv` (`bda1ecb`)
- Result: 0 actionable, 60 false_positive, 8 out_of_scope, 2 wrong_suggestion

## Step 4: Score

**Status:** Complete (2026-10-11)

- [x] `summary.json` — harness bug fix first (score passed blind-key path instead of loaded dict; `b05ecf5` + regression test + PROMPT Amendment); score `6a2cc1c`

## Step 5: Write the document

**Status:** Complete (2026-10-11)

- [x] Research doc with pre-registered `## Recommendation` — all four conditions PASS → **adopt `qwen/qwen3.8-27b` as stet's default model**; `6a2cc1c`

## Step 6: Restore LM Studio

**Status:** Complete (2026-10-11)

- [x] Reloaded `qwen/qwen3-coder-next` at 65536 / parallel 1 with `--no-speculative-draft-mtp`; `lms ps` confirms. Per the recommendation (unconditional adoption of 27b), OpenJev can be restarted; the operator should still switch `.review/config.toml` to 27b (follow-up, deliberately not done here)

## Step 7: Testing & Verification

**Status:** Complete (2026-10-11)

- [x] Contract `testCommand`: 74/74 harness tests pass, `summary.json` regenerates exactly (`check ok`), `## Recommendation` present
- [x] `git status` clean; nothing outside File Scope changed (only `docs/research/model-eval/**`, the research doc, harness script + its test, this STATUS/PROMPT)
- [x] Extension `npm test` (batch hygiene; extension untouched): 93/93 pass; `check-untracked-coverage.sh` PASS

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| 2026-10-11 | 0 | plan | skipped in-worker (nested reviewer spawn blocked; engine runs reviews after worker success) |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-10-10 | During the W0 batch, `qwen/qwen3-coder-next` failed every chat request: load-time MTP speculative decoding (LM Studio default, not shown in the Inference tab) errors with "not supported for batched MLX models" at parallel 3 and "requires a trimmable prompt cache (got ArraysCache)" after a reload. Saved presets also differed: 27b had Top P 0.8, Min P 0.5 on, no Top K | PROMPT amended pre-run: `--no-speculative-draft-mtp` on every load, exact preset values in the operator checklist, Step 0 preset parity check and smoke request per model |
| 2026-10-11 | Batch `20261011T174423-6acb` (aborted): coder-next `go-map-race` run 2 lost its finding to an `evidence_lines` counting loop (806 s, 0 findings); 27b with thinking on spent whole 8191-token responses on reasoning (1713 s for one case) | stet fixes `ea2124f` and `14fce09`, harness pins 65536/4096 (`b0d43f8`), 27b thinking off; pre-fix data in `docs/research/model-eval/baseline-pre-fix/` (see PROMPT Amendments) |
| 2026-10-11 | First coder-next `stet benchmark` after load failed with "LLM server unreachable … context deadline exceeded"; identical retry 30 s later succeeded in ~2 s and all subsequent requests were normal | Transient load-finalization race, not an MTP/preset problem; no Blocker — smoke requests and 45/45 runs were healthy |
| 2026-10-11 | Harness `score --labels/--blind-key` crashed (`AttributeError: 'PosixPath' object has no attribute 'get'`): `cmd_score` passed the blind-key path to `build_summary` instead of the loaded dict | `fix(SP-039)` `b05ecf5`: `load_blind_key()` helper + regression test `test_cmd_score_with_labels_takes_key_paths`; PROMPT Amendment added; affects scoring only, both model passes predate it |
| 2026-10-11 | `stet benchmark` eval rate is 0.0 t/s through LM Studio's OpenAI endpoint (no Ollama-style timing stats); per-run completion tokens are also absent from harness logs (stderr empty) | Speed compared via harness wall-clock (`seconds_per_case_mean`, `seconds_per_real_ref_mean`); benchmark token counts recorded (coder-next 1028 in/86 out; 27b 1060 in/129 out) |
| 2026-10-11 | coder-next misses `go-map-race` post-fix for a new reason: it reports 2 confident findings at lines 28/51 (one is not a real defect) in every run, never the race at line 44; 27b hits line 44 in 3/3 runs | The recall gap (0.9167 vs 1.0) is a hallucinated-finding failure, not a truncation failure; the 806 s loop from the pre-fix build is gone (14.1 s mean) |
| 2026-10-11 | 0/70 real-diff findings are actionable for either model (60 false positives); both models hallucinate compile errors, miscount parameters, and emit self-refuting "no defect" texts | Real-diff precision ties at 0.00; the dominant cost of these models on real refs is false-positive volume, same as the pre-benchmark concern |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-10-10 | Authored | Phase 7 model benchmark; real ref `e9b2e61` replaces the test-only `3248a08` from the draft plan |
| 2026-10-10 | Amended pre-run | MTP off on all loads, preset parity check, per-model smoke request, reasoning-model note (see PROMPT Amendments) |
| 2026-10-11 | Aborted and amended | Batch `20261011T174423-6acb` aborted during the 27b pass; stet fixes for #2/#3, 64k context and 4096 cap, 27b thinking off; re-run from scratch (see PROMPT Amendments) |
| 2026-10-11 | Step 0 complete | Preflight all green (preset parity OK, both smoke requests 200/stop, reasoning probe PASS); stet binary rebuilt from HEAD and reinstalled; preflight in `docs/research/model-eval/preflight.txt` (`fab416f`) |
| 2026-10-11 | Step 1 complete | coder-next benchmark + 45/45 ok; results `731c4f0`; recall_mean 0.9167, 9.9 s/case, 249 s/ref |
| 2026-10-11 | Step 2 complete | 27b benchmark + 45/45 ok across two invocations; results `a0a9a9a`; recall_mean 1.0, 14.6 s/case, 580 s/ref |
| 2026-10-11 | Steps 3–5 complete | 70 blind labels (`bda1ecb`), harness fix + Amendment (`b05ecf5`), summary + research doc; decision rule: all four conditions PASS → adopt `qwen/qwen3.8-27b` (`6a2cc1c`) |
| 2026-10-11 | Steps 6–7 complete | LM Studio restored to coder-next @ 65536; contract testCommand green (74/74, check ok, Recommendation present); extension npm test 93/93 |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes

- Recommendation (from the pre-registered rule, applied mechanically): **adopt `qwen/qwen3.8-27b` as stet's default model.** Q beats C outright on recall (1.0 vs 0.9167); noise condition passes with a thin margin (0.381 vs 0.387 threshold); real-diff actionable rate ties at 0.00; reliability 45/45 ok. Speed favors coder-next but is outside the rule. Small-sample caveats in the doc.
- Operator follow-up (not done here, per File Scope): switch `.review/config.toml` to `model = "qwen/qwen3.8-27b"`.
