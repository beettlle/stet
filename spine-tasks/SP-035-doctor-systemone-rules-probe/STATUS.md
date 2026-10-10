**Current Step:** Complete
**Status:** Done — all completion criteria met
**Last Updated:** 2026-10-10
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 1: Implementation

**Status:** Complete

- [x] Widen the `checkSystemOne` guard to critic=systemone OR `rules_selector` OR `rules_annotate`
- [x] Update the doc comment

Commit: `be737e4 fix(SP-035): probe System One in doctor when rules selector or annotator is on`

## Step 2: Testing & Verification

**Status:** Complete

- [x] Table test over trigger combinations (`TestRunCLI_doctorSystemOneTriggerCombinations`: none / critic=systemone / selector / annotate / selector+annotate / all three / critic=llm → no probe)
- [x] Failure path with selector on (`TestRunCLI_doctorSystemOneSelectorUnreachableExits2`: exit 2, "System One server unreachable at <url>")
- [x] Contract `testCommand` — `go test ./cmd/stet/... -count=1` ok, `go vet ./...` ok, `scripts/spine-stet-contract-run.sh default` exits 0 (stet review: 1 finding dismissed as false positive, re-run 0 findings, finished)
- [x] Coverage gate — `check-coverage.sh` PASS: project 83.1%, all files ≥ 72%; `checkSystemOne` 86.1%, cmd/stet package 76.1%

Commit: `a38595f test(SP-035): cover doctor System One probe trigger combinations`

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| 2026-10-10 | 1 | plan | skipped — nested reviewer spawn blocked in worker (SP-195); engine runs final review after .DONE |
| 2026-10-10 | contract | stet review (default) | 1 finding `3d4a5b9` dismissed (false positive); re-run 0 findings; `stet finish` ok |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-10-09 | Found while setting up stet-in-spine: doctor reported OK with only `rules_selector`/`rules_annotate` on | Doctor misses System One misconfig for SP-033/SP-034 features |
| 2026-10-10 | Stet reviewer hallucinated a nonexistent identifier: flagged `cfg.RulesAnnotate` as a possible typo for `RulesAnnotator`, which appears nowhere in the repo (grep exit 1); the field is `RulesAnnotate` with toml key `rules_annotate` (config.go:117) and compiles. Dismissed as `false_positive`. | Stet reviewer bug (plausible-identifier nitpick); candidates for prompt hardening — a "field does not exist / would not compile" check before flagging typos |
| 2026-10-10 | Environment repair (not a stet bug): LM Studio could not serve `qwen/qwen3-coder-next` at all — any chat completion failed with `SpeculativeDecodingNotSupportedError` / `Speculative decoding requires a trimmable prompt cache (got {'ArraysCache'})`. Root cause: saved per-model default config `~/.lmstudio/.internal/user-concrete-model-default-config/qwen/qwen3-coder-next.json` pinned `llm.prediction.speculativeDecoding.draftModel = Qwen2.5-0.5B-Instruct-MLX-4bit`, which is incompatible with the qwen3_next MLX runtime. Worker removed that single field (backup at `/tmp/qwen3-coder-next.json.bak`), reloaded the model at ctx 65536, and the review then completed normally. | Operator: do not re-enable a draft model for `qwen3-coder-next` until the LM Studio MLX runtime supports it; SP-039 presets are unaffected |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-10-09 | Authored | First stet-gated packet (contract ends with `spine-stet-contract-run.sh default`) |
| 2026-10-10 | Step 1 | Guard widened in `checkSystemOne` (main.go ~1921); doc comment lists all three triggers; committed `be737e4` |
| 2026-10-10 | Step 2 | Table test + selector-unreachable test added; committed `a38595f` |
| 2026-10-10 | Contract | First stet run failed: LM Studio draft-model misconfig for coder-next (see Discoveries); repaired env, second run triaged 1 false positive; third run 0 findings, finished |
| 2026-10-10 | Coverage | `check-coverage.sh` PASS; untracked `coverage.out` removed (no git restore needed) |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes

- Doctor has no `--dry-run`, so selector/annotate count whenever set — mirrors `systemOneRunOptions` `want*` booleans without the `!dryRun` condition (per PROMPT).
- Exit codes and messages unchanged; API key never printed (asserted in both new happy-path tests and existing unauthorized tests).
