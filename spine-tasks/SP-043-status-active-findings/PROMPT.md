# Task: SP-043 — `stet status` reports active findings

**Created:** 2026-10-14
**Size:** S

## Review Level: 2

**Assessment:** One output line in the hot `cli/cmd/stet/main.go`; scripts parse this field.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 0, Security: 0, Reversibility: 1 (output contract). Level 2 because `main.go` is a hot shared module.

## Mission

Closes #7

`stet status` prints `findings: N` with `len(s.Findings)` (`cli/cmd/stet/main.go` ~line 1323), which includes dismissed findings. After every finding is dismissed, `stet list` is empty but `stet status` still prints `findings: 3` / `dismissed: 3`. Scripts that gate on `stet status | awk '/^findings:/ {print $2}'` (pi-spine contracts) can never pass after triage.

`docs/review-process-internals.md` (~line 337) already says all output commands, including status, use **active** findings only (findings whose ID is not in `DismissedIDs`).

Fix (operator decision: option 1 of #7):

- `findings:` reports the **active** count: findings whose ID is not in `s.DismissedIDs`. Compute it from the already-loaded session (no second `session.Load`); reuse an existing helper if one fits (`run.ActiveFindingCount` in `cli/internal/run/run.go` ~line 1952, or the filtering in `activeFindings` ~line 124 of `main.go`).
- `dismissed:` stays as is.
- No new fields, no other output changes.
- `docs/cli-extension-contract.md` "Other commands" → `stet status` bullet (~line 86): say "active finding count (findings not dismissed)" instead of "finding count".

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh lenient`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Dependencies

- **Task:** SP-045 (also edits `docs/cli-extension-contract.md`; serialize to avoid merge conflicts)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md` — "Stet in spine (dogfooding)" section

**Tier 3 (reference docs):**

- `cli/cmd/stet/main.go` — status command (~lines 1295–1340), `activeFindings` (~line 124)
- `cli/cmd/stet/main_test.go` — status-after-dismiss check (~line 1521)
- `cli/internal/run/run.go` — `ActiveFindingCount` (~line 1952), read only

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio at `http://127.0.0.1:1234` and OpenJev at `http://127.0.0.1:8080` for the stet contract step only (unit tests use fakes)

## File Scope

- `cli/cmd/stet/main.go`
- `cli/cmd/stet/main_test.go`
- `docs/cli-extension-contract.md`

## Documentation Requirements

**Must Update:**

- `docs/cli-extension-contract.md` — `stet status` bullet

**Check If Affected:**

- `docs/review-process-internals.md` (~line 337 already says active-only; do not edit)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/cmd/stet/... -count=1 && bash scripts/spine-stet-contract-run.sh lenient` |
| fileScopeMustChange | `cli/cmd/stet/main.go`, `cli/cmd/stet/main_test.go`, `docs/cli-extension-contract.md` |
| fileScopeMustNotChange | `cli/internal/**`, `extension/**`, `scripts/**`, `.review/**`, `go.mod`, `go.sum` |
| completionCriteria | After dismissing every finding, `stet status` prints `findings: 0` and `dismissed: N`; with no dismissals the count equals `len(s.Findings)`; contract doc matches; stet contract review exits 0 |

## Steps

### Step 1: Active count in status

- [ ] `findings:` uses the active count computed from the loaded session
- [ ] `docs/cli-extension-contract.md` `stet status` bullet updated

### Step 2: Testing & Verification

> ZERO test failures allowed.

- [ ] Extend the status-after-dismiss test (~`main_test.go` 1521): one finding, dismiss it → output contains `findings: 0` and `dismissed: 1`
- [ ] Test with two findings and one dismissed → `findings: 1`, `dismissed: 1`
- [ ] `rg -n 'findings: ' cli/cmd/stet/main_test.go` — update any other assertion that relied on the total count
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in Mission)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Implementation:** `fix(SP-043): description`
- **Tests:** `test(SP-043): description`

## Do NOT

- Rename `findings:` or add new status fields
- Change `stet list`, JSON output, or any other command
- Edit `cli/internal/**` (read `run.ActiveFindingCount` only)
- Edit `.review/config.toml` or `scripts/**`
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
