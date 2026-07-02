# Task: SP-011 — Fix spine task contract testCommand typos

**Created:** 2026-07-01
**Size:** S

## Review Level: 0

**Assessment:** Documentation-only fixes in task packets; no product code.
**Score:** 1/8 — Blast radius: 0, Pattern novelty: 0, Security: 0, Reversibility: 1

## Mission

Fix **contract `testCommand` values** in spine task packets so they match real Go test name filters and pass when run literally.

Known defect: **SP-002** uses `go test -run Optimize` (capital O) but tests are named `TestRunCLI_optimize*` — the filter matches nothing.

Audit all `spine-tasks/**/PROMPT.md` Contract tables for similar `-run` case mismatches and other non-runnable commands.

## Dependencies

- **None**

## Context to Read First

- `spine-tasks/SP-002-port-optimizer-from-diffusion/PROMPT.md` — broken `-run Optimize`
- `cli/cmd/stet/main_test.go` — `TestRunCLI_optimize*` test names
- SP-001 verification notes (2026-07-01)

## Environment

- **Workspace:** `spine-tasks/` task packets only

## File Scope

- `spine-tasks/SP-002-port-optimizer-from-diffusion/PROMPT.md`
- `spine-tasks/**/PROMPT.md` (audit only; edit files with defects)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `spine tasks validate && grep -q 'run optimize' spine-tasks/SP-002-port-optimizer-from-diffusion/PROMPT.md` |
| fileScopeMustChange | `spine-tasks/SP-002-port-optimizer-from-diffusion/PROMPT.md` |
| fileScopeMustNotChange | `cli/**`, `extension/**`, `scripts/**` |
| completionCriteria | SP-002 testCommand runs 3 optimize tests; audit documented in STATUS |

## Steps

### Step 0: Audit all PROMPT contracts

- [ ] Grep `testCommand` in every `spine-tasks/*/PROMPT.md`
- [ ] For each Go `-run` filter, verify matching tests exist (`go test -run <filter> -list` or dry run)
- [ ] List defects in STATUS Discoveries

### Step 1: Fix SP-002 and any other defects

- [ ] Change SP-002 to `-run optimize` (lowercase) or equivalent that lists `TestRunCLI_optimize*`
- [ ] Fix any other broken commands found in audit
- [ ] Run each corrected command once to confirm non-empty / passing

### Step 2: Testing and verification

- [ ] Run corrected SP-002 `testCommand` end-to-end (pytest + go test)
- [ ] Document audit results in STATUS

## Completion Criteria

- [ ] All steps complete
- [ ] Every edited `testCommand` verified runnable
- [ ] STATUS lists audited task IDs and outcomes

## Git Commit Convention

- **Implementation:** `chore(SP-011): description`

## Do NOT

- Change product code or test implementations (packet docs only)
- Weaken completion criteria in completed tasks' STATUS/.DONE
