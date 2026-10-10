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
- [x] Contract `testCommand` — round 1 (worker): 1 finding dismissed (false positive), re-run 0 findings, `stet finish` ok. Round 2 (engine re-runs at 18:45/18:46 UTC): failed with 4 findings — `stet finish` had cleared the session, so the script started a fresh review that re-flagged the hallucinated `RulesAnnotator` typo and added 3 new false positives; all 4 re-triaged and dismissed (see Reviews/Discoveries); re-run `0 finding(s)`, exit 0, `stet finish` ok, session cleared
- [x] Coverage gate — `check-coverage.sh` PASS: project 83.1%, all files ≥ 72%; `checkSystemOne` 86.1%, cmd/stet package 76.1% (re-verified after round-2 triage)

Commit: `a38595f test(SP-035): cover doctor System One probe trigger combinations`

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| 2026-10-10 | 1 | plan | skipped — nested reviewer spawn blocked in worker (SP-195); engine runs final review after .DONE |
| 2026-10-10 | contract (round 1, worker) | stet review (default) | 1 finding `3d4a5b9` dismissed (false positive); re-run 0 findings; `stet finish` ok |
| 2026-10-10 | contract (round 2, engine re-runs) | stet review (default) | fresh session after round-1 finish → 4 findings (`3d4a5b9`, `716c2e1`, `a547c2e`, `15547de`); each independently re-verified, all false positive / already correct, dismissed; re-run `0 finding(s)`, exit 0; `stet finish` ok |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-10-09 | Found while setting up stet-in-spine: doctor reported OK with only `rules_selector`/`rules_annotate` on | Doctor misses System One misconfig for SP-033/SP-034 features |
| 2026-10-10 | Stet reviewer hallucinated a nonexistent identifier: flagged `cfg.RulesAnnotate` as a possible typo for `RulesAnnotator`, which appears nowhere in the repo (grep exit 1); the field is `RulesAnnotate` with toml key `rules_annotate` (config.go:117) and compiles. Dismissed as `false_positive` — and the reviewer regenerated the identical finding after the session was reset, so the hallucination is repeatable, not a one-off. | Stet reviewer bug (plausible-identifier nitpick); candidates for prompt hardening — a "field does not exist / would not compile" check before flagging typos |
| 2026-10-10 | `stet finish` clears the session, so any later re-run of the contract script starts a **fresh review** from the baseline; the LLM reviewer then resurfaces old (dismissed) findings and can invent new ones. Round 2 engine re-runs failed this way with 3 new false positives: (a) `716c2e1` claimed the "critic llm → no probe" test is wrong, but the guard requires backend `systemone` and the PROMPT mandates that exact case — `already_correct`; (b) `a547c2e` worried the API key could leak via "a logger", but doctor writes only via `fmt.Fprintln(os.Stdout/os.Stderr)` and both tests assert the key is absent from stdout+stderr — `false_positive`; (c) `15547de` said the unreachable-test must keep its listener open, but that would make the port reachable and break the test — the listen/close/free-port pattern is used 4× elsewhere in main_test.go — `false_positive`. | Dogfooding/process: `scripts/spine-stet-contract-run.sh` is idempotent only while the session stays open. If an orchestrator re-runs a contract after a passing (finished) run, expect re-triage. Consider persisting dismissals across sessions (baseline-scoped) or `SPINE_STET_NO_AUTO_FINISH=1` for engine-side verification |
| 2026-10-10 | Environment repair (not a stet bug): LM Studio could not serve `qwen/qwen3-coder-next` at all — any chat completion failed with `SpeculativeDecodingNotSupportedError` / `Speculative decoding requires a trimmable prompt cache (got {'ArraysCache'})`. Root cause: saved per-model default config `~/.lmstudio/.internal/user-concrete-model-default-config/qwen/qwen3-coder-next.json` pinned `llm.prediction.speculativeDecoding.draftModel = Qwen2.5-0.5B-Instruct-MLX-4bit`, which is incompatible with the qwen3_next MLX runtime. Worker removed that single field (backup at `/tmp/qwen3-coder-next.json.bak`), reloaded the model at ctx 65536, and the review then completed normally. | Operator: do not re-enable a draft model for `qwen3-coder-next` until the LM Studio MLX runtime supports it; SP-039 presets are unaffected |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-10-09 | Authored | First stet-gated packet (contract ends with `spine-stet-contract-run.sh default`) |
| 2026-10-10 | Step 1 | Guard widened in `checkSystemOne` (main.go ~1921); doc comment lists all three triggers; committed `be737e4` |
| 2026-10-10 | Step 2 | Table test + selector-unreachable test added; committed `a38595f` |
| 2026-10-10 | Contract | First stet run failed: LM Studio draft-model misconfig for coder-next (see Discoveries); repaired env, second run triaged 1 false positive; third run 0 findings, finished |
| 2026-10-10 | Contract (engine re-runs) | Two engine verification runs (18:45/18:46 UTC) failed with 4 findings in a fresh session (finish had cleared round-1 dismissals); all 4 re-triaged and dismissed with verified reasons; re-run 0 findings, exit 0, finish ok |
| 2026-10-10 | Coverage | `check-coverage.sh` PASS; untracked `coverage.out` removed (no git restore needed); re-verified PASS after round-2 triage |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes

- Doctor has no `--dry-run`, so selector/annotate count whenever set — mirrors `systemOneRunOptions` `want*` booleans without the `!dryRun` condition (per PROMPT).
- Exit codes and messages unchanged; API key never printed (asserted in both new happy-path tests and existing unauthorized tests).
