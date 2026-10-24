# Task: SP-075 — Docs: README `stet list` thresholds and contract gating

**Created:** 2026-10-24
**Size:** S

## Review Level: 0

**Assessment:** Docs-only; describes `stet list --min-severity / --min-confidence` (commit `e35636b`) and the contract policy from SP-073.
**Score:** 0/8 — Blast radius: 0, Pattern novelty: 0, Security: 0, Reversibility: 0

## Mission

`stet list` gained `--min-severity <error|warning|info|nitpick>` and `--min-confidence <0..1>` (commit `e35636b`). `docs/cli-extension-contract.md` already documents them; `README.md` does not. SP-073 makes the spine contract script gate on `--min-severity warning --min-confidence 0.8`.

Update `README.md` to match the code on `main` (read the code; it wins over this text):

1. **Commands table (`stet list` row, ~line 138):** mention the two threshold flags.
2. **Workflow / scripting text (find the best spot near "Inspect findings", ~line 49, or the "Useful flags" section):** two or three sentences on gating CI or agent scripts on actionable findings: use `stet list --min-severity warning --min-confidence 0.8`; hidden findings stay active and `stet status` still counts them; invalid values exit 1. Mention that the repo's `scripts/spine-stet-contract-run.sh` uses this policy and that `SPINE_STET_MIN_SEVERITY` / `SPINE_STET_MIN_CONFIDENCE` override it (check the script).

## Dependencies

- **Task:** SP-073 (documents the contract policy)

## Context to Read First

**Tier 3 (reference docs):**

- `README.md`
- `cli/cmd/stet/main.go` — `stet list` flags (~line 1410)
- `scripts/spine-stet-contract-run.sh`

## Environment

- **Workspace:** repo root
- **Services required:** None

## File Scope

- `README.md`

## Documentation Requirements

**Must Update:**

- `README.md` — the two areas in Mission

## Contract

| Field | Value |
|-------|-------|
| testCommand | `true` |
| fileScopeMustChange | `README.md` |
| fileScopeMustNotChange | `cli/cmd/**`, `extension/**`, `scripts/**`, `.review/**`, `docs/**` |
| completionCriteria | `rg -n 'min-severity' README.md` and `rg -n 'min-confidence' README.md` each find the new text; the policy values match `scripts/spine-stet-contract-run.sh`; every statement matches the code on `main` |

## Steps

### Step 1: README text

- [ ] `stet list` row mentions the flags
- [ ] Short gating paragraph with the contract policy and env overrides

### Step 2: Testing & Verification

- [ ] Contract `completionCriteria` `rg` checks pass
- [ ] `rg -l 'README.md' cli/ scripts/` — note in STATUS whether any test references this doc

## Completion Criteria

- [ ] All steps complete
- [ ] Every statement matches the code on `main`

## Git Commit Convention

- **Docs:** `docs(SP-075): description`

## Do NOT

- Change code or rewrite unrelated README sections
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`

---

## Amendments (Added During Execution)
