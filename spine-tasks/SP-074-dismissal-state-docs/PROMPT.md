# Task: SP-074 — Docs: dismissals follow line shifts; keep history across `.review` cleanup

**Created:** 2026-10-24
**Size:** S

## Review Level: 0

**Assessment:** Docs-only; describes behavior on `main` after SP-072 and the state-loss hazard from #38.
**Score:** 0/8 — Blast radius: 0, Pattern novelty: 0, Security: 0, Reversibility: 0

## Mission

Closes #38

SP-071 and SP-072 make dismissals follow line shifts. The last part of #38 is documentation: dismissals can only be restored while `history.jsonl` exists, and a cleanup such as `git clean -fdX -- .review` (which pi-spine runs between attempts) deletes it when `.review/*` is gitignored.

Update `docs/cli-extension-contract.md` to match the code on `main` (read the code; it wins over this text):

1. **`stet dismiss` entry (~line 88):** replace "restores matching dismissals from recent history for unchanged files" with the new behavior: a dismissal's lines are re-mapped through the edits made since it was recorded, and it is dropped only when its own lines changed. Also state that `stet run` skips hunks overlapping a dismissed finding at its re-mapped location. Check `cli/internal/run/restore_dismissals.go` and `filterHunksWithDismissedFindings` in `cli/internal/run/run.go`.
2. **State storage (the `.gitignore` paragraph ~line 226 and "State storage and history" ~line 234):** add a short "Keeping dismissals across cleanups" note: tools that clean ignored files under `.review` (for example `git clean -fdX -- .review`) delete `session.json` and `history.jsonl`, so nothing remains to restore. Give the two options: set `state_dir` / `STET_STATE_DIR` to a directory outside the cleaned path, or keep history out of the cleanup with a gitignore exception and the matching `git clean` exclusion. Show the exact `.gitignore` lines and verify them with `git check-ignore -v` in a scratch repo before writing them.

## Dependencies

- **Task:** SP-072 (documents its behavior)

## Context to Read First

**Tier 3 (reference docs):**

- `docs/cli-extension-contract.md`
- `cli/internal/run/restore_dismissals.go`, `cli/internal/run/line_remap.go`, `cli/internal/run/run.go` (`filterHunksWithDismissedFindings`)
- `cli/internal/config/config.go` — `StateDir` / `STET_STATE_DIR`

## Environment

- **Workspace:** repo root
- **Services required:** None

## File Scope

- `docs/cli-extension-contract.md`

## Documentation Requirements

**Must Update:**

- `docs/cli-extension-contract.md` — the two areas in Mission

## Contract

| Field | Value |
|-------|-------|
| testCommand | `true` |
| fileScopeMustChange | `docs/cli-extension-contract.md` |
| fileScopeMustNotChange | `cli/**`, `extension/**`, `scripts/**`, `.review/**`, `README.md` |
| completionCriteria | `docs/cli-extension-contract.md` no longer says dismissals are restored only "for unchanged files" (`rg -n 'for unchanged files' docs/cli-extension-contract.md` finds nothing); it describes re-mapping and the `stet run` skip; it documents the cleanup hazard with `state_dir` and a verified gitignore exception (`rg -n 'git clean' docs/cli-extension-contract.md` finds it); every statement matches the code on `main` |

## Steps

### Step 1: Dismissal and state-storage text

- [ ] `stet dismiss` entry describes re-mapping and the run-time skip
- [ ] Cleanup note with `state_dir` and a verified gitignore exception

### Step 2: Testing & Verification

- [ ] Contract `completionCriteria` `rg` checks pass
- [ ] `rg -l 'cli-extension-contract.md' cli/ scripts/` — note in STATUS whether any test references this doc

## Completion Criteria

- [ ] All steps complete
- [ ] Every statement matches the code on `main`

## Git Commit Convention

- **Docs:** `docs(SP-074): description`

## Do NOT

- Change code, `.gitignore`, or unrelated sections
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`

---

## Amendments (Added During Execution)
