# Task: SP-073 — Spine contract fails only on actionable stet findings

**Created:** 2026-10-24
**Size:** S

## Review Level: 1

**Assessment:** One bash script plus a self-contained bash test with a fake `stet` on `PATH`; changes what blocks a spine lane.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 0, Security: 0, Reversibility: 1

## Mission

Closes #37
Closes #25

`scripts/spine-stet-contract-run.sh` fails the contract when `stet list` prints any active finding, at any severity or confidence. In the pi-spine release that meant ~13 contract failures for 3 real defects; praise-only and low-severity findings (#25) block lanes like errors do. `stet list` now has `--min-severity` and `--min-confidence` (commit `e35636b`).

Gate policy for this release (operator-approved with the release scope): fail on `stet list --min-severity warning --min-confidence 0.8`.

Change the script:

1. After `stet run`, compute the gating list with `stet list --min-severity "$min_severity" --min-confidence "$min_confidence"`. Defaults `warning` and `0.8`; overridable with `SPINE_STET_MIN_SEVERITY` and `SPINE_STET_MIN_CONFIDENCE` (validate the severity against `error|warning|info|nitpick`; leave confidence validation to `stet list`, whose non-zero exit already fails the script).
2. When the gating list is non-empty: print the full `stet list` (all active findings, for triage) and the existing triage instructions, state how many findings are at or above the threshold, and exit 1.
3. When the gating list is empty but `stet list` still has active findings: print one line to stderr saying N findings are below the threshold and did not block. Then continue with the existing finish logic unchanged (exit 0 when `SPINE_STET_NO_AUTO_FINISH=1`, otherwise `stet finish`).
4. Update the header comment to describe the policy and the two env vars.

Add `scripts/tests/spine-stet-contract-run-test.sh`: creates a temp directory with `.review/session.json`, puts a fake `stet` script first on `PATH` (records its arguments; `list` output depends on whether `--min-severity` was passed), and runs the contract script for: (a) only a below-threshold `info` finding → exit 0; (b) a `warning` at 0.9 → exit 1 with the full list printed; (c) `stet list` failure → exit 1; (d) `SPINE_STET_MIN_SEVERITY=bogus` → exit 2. The test exits non-zero on any failed case and needs no network or model.

## Dependencies

- **None**

## Context to Read First

**Tier 3 (reference docs):**

- `scripts/spine-stet-contract-run.sh`
- `cli/cmd/stet/main.go` — `stet list` flags (~line 1410)
- `docs/cli-extension-contract.md` — `stet list` entry (~line 87)

## Environment

- **Workspace:** repo root
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `scripts/spine-stet-contract-run.sh`
- `scripts/tests/spine-stet-contract-run-test.sh`

## Documentation Requirements

**Must Update:** none (SP-075 updates `README.md`).

**Check If Affected:** `spine-tasks/CONTEXT.md` "Contract convention" bullet — leave it; the release operator updates it after this lands.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash -n scripts/spine-stet-contract-run.sh && bash scripts/tests/spine-stet-contract-run-test.sh && bash scripts/spine-stet-contract-run.sh lenient` |
| fileScopeMustChange | `scripts/spine-stet-contract-run.sh`, `scripts/tests/spine-stet-contract-run-test.sh` |
| fileScopeMustNotChange | `cli/cmd/**`, `extension/**`, `scripts/spine-worktree-setup.sh`, `scripts/stet-wave-review.sh`, `.review/**` |
| completionCriteria | Test cases (a)–(d) pass; with real stet, the contract fails only when a finding at `warning`+ with confidence ≥ 0.8 is active; full `stet list` is printed on failure; stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh lenient` (the updated script reviews its own diff). If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Threshold gate in the contract script

- [ ] Gating list uses the thresholds; full list printed on failure; below-threshold note on pass; env overrides validated
- [ ] `spine_review_step` after the step

### Step 2: Testing & Verification

> ZERO test failures allowed.

- [ ] `scripts/tests/spine-stet-contract-run-test.sh` covers cases (a)–(d)
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Feature:** `feat(SP-073): description`
- **Tests:** `test(SP-073): description`

## Do NOT

- Change `stet list` or any Go code
- Change the session-restore block or the `stet run` invocation
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)

### 2026-10-24 — Fallback for a released stet without the threshold flags

The first attempt failed its Contract: the `stet` on the operator's `PATH` is v1.3.0, and its `stet list` rejects `--min-severity` (`unknown flag: --min-severity`). The flags ship in v1.4.0, so every lane in this release runs the script against v1.3.0.

Keep the Step 1 work and add:

1. Before computing the gating list, detect flag support with `stet list --help` (look for `--min-severity`). When supported, behave exactly as in Step 1.
2. When not supported: print one line to stderr, `stet: installed stet does not support 'stet list --min-severity'; gating on all active findings`, and use the unfiltered `stet list` as the gating list. This is the pre-v1.4.0 policy: any active finding fails the contract. Never silently pass.
3. Extend the test: the fake `stet` answers `list --help` with or without the `--min-severity` line based on an env var, and add case (e) — no flag support plus one `info` finding → exit 1, the warning line is printed, and `--min-severity` is never passed to `list`. Keep the existing cases passing (their fake must advertise the flag).
4. Update the header comment to mention the fallback.

File Scope and Contract are unchanged.
