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
**Status:** ⬜ Not Started

- [ ] `scripts/tests/spine-stet-contract-run-test.sh` cases (a)–(d)
- [ ] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
