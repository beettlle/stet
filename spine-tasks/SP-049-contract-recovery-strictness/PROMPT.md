# Task: SP-049 — Contract script recovery `stet start` uses the requested strictness

**Created:** 2026-10-17
**Size:** S

## Review Level: 0

**Assessment:** One-flag change in a shell script.
**Score:** 1/8 — Blast radius: 0, Pattern novelty: 0, Security: 0, Reversibility: 0 (+1 shared script)

## Mission

Closes #22

In `scripts/spine-stet-contract-run.sh`, when `.review/session.json` is missing, the script recreates the session with:

```bash
stet start "$baseline" --allow-dirty --quiet
```

That `stet start` reviews the whole diff at the config's default strictness, and the following `stet run --strictness "$strictness" --quiet` then has no new hunks, so the requested strictness never applies.

Change the recovery line to:

```bash
stet start "$baseline" --allow-dirty --strictness "$strictness" --quiet
```

`$strictness` is already parsed and validated earlier in the script (the `case` block before this line). Do not change anything else in the script.

## Dependencies

- **None**

## Context to Read First

**Tier 3 (reference docs):**

- `scripts/spine-stet-contract-run.sh`

## Environment

- **Workspace:** repo root
- **Services required:** None

## File Scope

- `scripts/spine-stet-contract-run.sh`

## Documentation Requirements

**Must Update:** none

**Check If Affected:**

- `spine-tasks/CONTEXT.md` — "Stet in spine (dogfooding)" section, if it describes the recovery path

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash -n scripts/spine-stet-contract-run.sh && rg -q -- '--allow-dirty --strictness' scripts/spine-stet-contract-run.sh` |
| fileScopeMustChange | `scripts/spine-stet-contract-run.sh` |
| fileScopeMustNotChange | `cli/**`, `extension/**`, `.review/**` |
| completionCriteria | The recovery `stet start` passes `--strictness` with the script's strictness argument; script syntax-checks clean |

## Steps

### Step 1: Pass strictness to the recovery start

- [ ] Recovery `stet start` line includes `--strictness "$strictness"`

### Step 2: Testing & Verification

- [ ] `bash -n scripts/spine-stet-contract-run.sh` exits 0
- [ ] Run Contract `testCommand`

## Completion Criteria

- [ ] All steps complete

## Git Commit Convention

- **Fix:** `fix(SP-049): description`

## Do NOT

- Change the `stet run` line, exit handling, or strictness parsing
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
