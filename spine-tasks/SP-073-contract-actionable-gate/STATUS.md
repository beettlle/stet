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
- [x] Contract `testCommand` — `bash -n` ok; tests 23/23 pass; real stet review (lenient) **exit 0**. First contract run found exactly 1 gating finding (`812c426` WARNING, confidence ≥ 0.8): full `stet list` + triage text printed and the lane was blocked (exit 1) — the new failure path working as specified. Root cause was environmental, not the diff: the installed stet on PATH (`~/.local/bin/stet` v1.3.0) predates `e35636b` and rejects `--min-severity` (`unknown flag`), so the gating call itself failed. Rebuilt the repo CLI (`go build -o /tmp/stet-sp073/stet ./cli/cmd/stet`), triaged `812c426` (dismissed `out_of_scope`, see Discoveries), re-ran the contract with the fresh binary first on PATH → 0 findings, `stet finish` ran, exit 0.

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|
| 1 | code | 1–2 | pending (engine runs after .DONE) | — |

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| 2026-10-24: The stet binary on `PATH` (`~/.local/bin/stet` v1.3.0) predates `e35636b` and fails with `unknown flag: --min-severity`, so the contract script's own gating call fails (exit 1) even when the review succeeded. Stet finding `812c426` (WARNING, conf ≥ 0.8) flagged exactly this version-skew; dismissed as `out_of_scope` — the packet mandates the flags and specifies that a non-zero `stet list` exit fails the script, and the release flow ships/replaces the binary with the published one. Operators must run the contract with a binary built from this repo (`go build -o <dir>/stet ./cli/cmd/stet`, prepend `<dir>` to PATH) until `~/.local/bin/stet` is refreshed. | Environment gotcha + triage | `scripts/spine-stet-contract-run.sh` |

---

## Completion Criteria

- [x] All steps complete
- [x] All tests passing (23/23 test-script assertions; contract review exits 0)