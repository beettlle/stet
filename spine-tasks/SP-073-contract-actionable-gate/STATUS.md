# SP-073: Spine contract fails only on actionable stet findings — Status

**Current Step:** 2
**Status:** 🔄 In Progress
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
- [x] Contract `testCommand` — see Reviews/evidence below (stet review outcome recorded after run)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
