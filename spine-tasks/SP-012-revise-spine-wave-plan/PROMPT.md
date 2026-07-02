# Task: SP-012 — Revise spine wave plan after wave-0 lessons

**Created:** 2026-07-01
**Size:** S

## Review Level: 0

**Assessment:** Planning documentation only; prevents repeat of batch failures, no product code.
**Score:** 1/8 — Blast radius: 0, Pattern novelty: 0, Security: 0, Reversibility: 1

## Mission

Update `spine-tasks/CONTEXT.md` execution policy and suggested waves so future batches **avoid serial-lane contract cross-contamination** that caused SP-002/SP-003 false `review_exhausted` failures when SP-001/002/003 shared lane-1.

Incorporate lessons from batch `20260701T232227`:

- Do **not** run multiple overlapping `cli/**` tasks in one serial lane
- Prefer the documented disjoint wave (SP-002, SP-003, SP-005, SP-008 parallel) over `spine batch start pending --wave 0` when tasks have overlapping scopes
- Document operator recovery: manual lane merge + `spine batch complete --detect-manual-merge` when work is Done in worktree but spine status failed
- Link upstream: [pi-spine #62](https://github.com/beettlle/pi-spine/issues/62), [#73](https://github.com/beettlle/pi-spine/issues/73), [#74](https://github.com/beettlle/pi-spine/issues/74)

## Dependencies

- **None**

## Context to Read First

- `spine-tasks/CONTEXT.md` — current waves and execution policy
- Wave-0 verification summary (2026-07-01): serial lane-1 SP-001→002→003 contract failures
- `.spine/spine-config.json` — `lanes.maxParallel`

## Environment

- **Workspace:** `spine-tasks/CONTEXT.md` only

## File Scope

- `spine-tasks/CONTEXT.md`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `spine tasks validate && spine plan pending` |
| fileScopeMustChange | `spine-tasks/CONTEXT.md` |
| fileScopeMustNotChange | `cli/**`, `extension/**`, `scripts/**` |
| completionCriteria | CONTEXT reflects post-wave-0 policy; `spine plan pending` shows safe waves for remaining tasks |

## Steps

### Step 0: Document wave-0 failure modes

- [ ] Summarize serial-lane contract contamination (final review sees cumulative lane diff)
- [ ] Summarize DirtyWorktree from tracked `extension/coverage/` (fixed in `175c1a4`)
- [ ] Record in STATUS Discoveries

### Step 1: Rewrite suggested waves for remaining backlog

- [ ] Mark SP-001/002/003/005/008 as Done on `main`
- [ ] Update waves for SP-004, SP-006, SP-007, SP-009 per disjoint file scopes
- [ ] Add explicit rule: max one `cli/**` core task per serial lane; prefer named task batches over `pending --wave N` when scopes overlap

### Step 2: Add operator recovery appendix

- [ ] Short section: when tasks are Done in lane worktree but batch failed — skip/retry/force-merge/manual merge steps
- [ ] Reference `spine batch complete --detect-manual-merge`

### Step 3: Testing and verification

- [ ] Run `spine tasks validate`
- [ ] Run `spine plan pending` and confirm wave assignments match policy

## Completion Criteria

- [ ] All steps complete
- [ ] CONTEXT.md updated with post-wave-0 policy and recovery notes
- [ ] `spine plan pending` output reviewed and noted in STATUS

## Git Commit Convention

- **Implementation:** `chore(SP-012): description`

## Do NOT

- Change `dependencies.json` unless wave deps need correction for remaining tasks
- Edit completed task PROMPT.md files (use SP-011 for contract typos)
