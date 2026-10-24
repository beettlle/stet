# SP-073: Spine contract fails only on actionable stet findings — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-24
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Threshold gate in the contract script
**Status:** ✅ Complete

- [x] Gating list, full list on failure, below-threshold note, env overrides
  - `stet list --min-severity "$min_severity" --min-confidence "$min_confidence"` gates the lane (defaults `warning`/`0.8`; `SPINE_STET_MIN_SEVERITY` validated against `error|warning|info|nitpick` → exit 2 on bad value; confidence left to `stet list` non-zero exit).
  - Non-empty gating list → full `stet list` + triage instructions + threshold count on stderr, exit 1.
  - Empty gating list with active findings → one-line stderr note (N below threshold), then existing finish logic unchanged.
  - Header comment documents the policy and both env vars; session-restore block and `stet run` invocation untouched.
  - `bash -n` passes; blast radius LOW (GitNexus impact, no symbol-level callers).

---

### Step 2: Testing & Verification
**Status:** ✅ Complete

- [x] `scripts/tests/spine-stet-contract-run-test.sh` cases (a)–(d) — 23/23 assertions pass, exit 0. Cases: (a) below-threshold info → exit 0 with note + finish still runs; (b) warning@0.9 → exit 1, full list + triage text, no finish; (c) `stet list` failure → exit 1; (d) `SPINE_STET_MIN_SEVERITY=bogus` → exit 2 with no stet invocation; plus (e) valid `info`/`0.5` overrides honored. Harness is hermetic: unsets `SPINE_STET_NO_AUTO_FINISH`/`SPINE_STET_MIN_SEVERITY`/`SPINE_STET_MIN_CONFIDENCE` (the spine worker exports `SPINE_STET_NO_AUTO_FINISH=1`).
- [x] Contract `testCommand` — `bash -n` ok; tests 23/23 pass; real stet review (lenient) **exit 0**. First contract run found exactly 1 gating finding (`812c426` WARNING, confidence ≥ 0.8): full `stet list` + triage text printed and the lane was blocked (exit 1) — the new failure path working as specified. Root cause was environmental, not the diff: the installed stet on PATH (`~/.local/bin/stet` v1.3.0) predates `e35636b` and rejects `--min-severity` (`unknown flag`), so the gating call itself failed. Rebuilt the repo CLI (`go build -o /tmp/stet-sp073/stet ./cli/cmd/stet`), triaged `812c426` (dismissed `out_of_scope`, see Discoveries), re-ran the contract with the fresh binary first on PATH → 0 findings, `stet finish` ran, exit 0. Superseded by the amendment below: the shipped script now handles v1.3.0 directly.

---

### Amendment (2026-10-24): fallback for stet without threshold flags
**Status:** ✅ Complete

- [x] Flag-support detection + fallback in `scripts/spine-stet-contract-run.sh`
  - Before computing the gating list, the script runs `stet list --help` and greps for `--min-severity`. Supported → Step 1 behavior unchanged.
  - Not supported → one stderr line (`stet: installed stet does not support 'stet list --min-severity'; gating on all active findings`) and the unfiltered `stet list` becomes the gating list (pre-v1.4.0 policy: any active finding blocks; never silently passes). In that mode the failure-block count line says threshold filtering is unavailable instead of claiming a threshold match.
  - Header comment documents the fallback; session-restore block and `stet run` invocation untouched. Commit `75738e8`.
- [x] Test coverage extended — the fake `stet` answers `list --help` with the `--min-severity` flag line only when `FAKE_STET_SUPPORTS_THRESHOLDS=1` (`run_contract` defaults it to 1 so existing cases still advertise the flag). New case (e): no flag support + one below-threshold `info` finding → exit 1, fallback warning printed, full list shown, `--min-severity` never passed to `list`, no `finish`. Previous env-override case renumbered (f). 31/31 assertions pass. Commit `be3b9fa`.
- [x] Contract re-verified against the v1.3.0 stet on PATH (no rebuilt binary needed): fallback warning printed, review produced 3 findings → triaged (all reviewer misreads, see Discoveries) → dismissed → re-run: 0 findings, contract exit 0.

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|
| 1 | code | 1–2 | pending (engine runs after .DONE) | — |

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| 2026-10-24: The stet binary on `PATH` (`~/.local/bin/stet` v1.3.0) predates `e35636b` and fails with `unknown flag: --min-severity`, so the contract script's own gating call fails (exit 1) even when the review succeeded. Stet finding `812c426` (WARNING, conf ≥ 0.8) flagged exactly this version-skew; dismissed as `out_of_scope` — the packet mandates the flags and specifies that a non-zero `stet list` exit fails the script, and the release flow ships/replaces the binary with the published one. **Resolved by the PROMPT amendment:** the script now detects missing flag support and falls back to gating on all active findings, so v1.3.0 works without a rebuilt binary. | Environment gotcha, resolved | `scripts/spine-stet-contract-run.sh` |
| 2026-10-24: Fallback contract run (v1.3.0, gates on all findings) produced 3 findings; all reviewer misreads, none were real defects, dismissed: `3f4ce6e` `false_positive` — claims `if !` exempts `stet list` from failing the script, but the gating call has an explicit `exit 1` in its failure branch; only the `stet list --help` detection call proceeds on failure, which is the designed fallback. `f041c08` `out_of_scope` — wants script-side `SPINE_STET_MIN_CONFIDENCE` validation; the packet explicitly mandates leaving confidence validation to `stet list`, whose non-zero exit produces a visible exit-1 failure. `ca99dbd` `false_positive` — confuses `active_count` (full `stet list`) with `gating_count` (threshold subset); the below-threshold note only prints after gating passed (gating_count = 0, so remaining active findings are necessarily below threshold; in fallback mode gating = active so it never prints), and the failure path exits 1 before that line. Pattern for the optimizer: the reviewer misreads `if ! cmd; then ... exit 1; fi` as a swallowed error and swaps similarly-named counters. | Stet reviewer bugs (dismissed with reasons) | `scripts/spine-stet-contract-run.sh` |

---

## Completion Criteria

- [x] All steps complete (incl. 2026-10-24 amendment: fallback for stet without threshold flags)
- [x] All tests passing (31/31 test-script assertions; contract review exits 0 against the v1.3.0 stet on PATH)