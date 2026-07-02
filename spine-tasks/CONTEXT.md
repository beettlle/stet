# stet — Context

**Last Updated:** 2026-07-02
**Status:** Active
**Next Task ID:** SP-014

---

## Current State

**Wave 0 landed on `main` (manual merge, batch `20260701T232227`):** SP-001, SP-002, SP-003, SP-004, SP-005, SP-008 marked `.DONE`. Remaining feature backlog: SP-006, SP-007, SP-009.

**Hygiene backlog (SP-010–SP-013):** From 2026-07-01 `.DONE` verification — coverage gates, contract testCommand typos, wave-plan revision, coverage-artifact guard. Run **SP-011, SP-012, SP-013** before the next feature batch; **SP-010** (L) can run in parallel or after wave 1.

**pi-spine v1.2.0:** `contract.mode: required`; preflight `prelanded-file-scope` warns when `fileScopeMustChange` paths already on `main`. Upstream: serial-lane cumulative contract verify ([#62](https://github.com/beettlle/pi-spine/issues/62)), manual-merge detection ([#73](https://github.com/beettlle/pi-spine/issues/73)), DirtyWorktree guard ([#74](https://github.com/beettlle/pi-spine/issues/74)).

### Wave-0 failure modes (batch `20260701T232227`)

1. **Serial-lane contract cross-contamination** — SP-001, SP-002, and SP-003 ran sequentially in lane-1. Final contract review diffs the **cumulative lane worktree** against `main`, so SP-002/SP-003 saw SP-001 deliverables outside their `fileScopeMustChange` and exhausted review (`review_exhausted`). **Fix:** at most one `cli/**` core task per serial lane; use disjoint parallel waves (the documented SP-002 ∥ SP-003 ∥ SP-005 ∥ SP-008 layout), not `spine batch start pending --wave 0` when scopes overlap.

2. **DirtyWorktree from tracked `extension/coverage/`** — Vitest coverage output was tracked on `main`, blocking batch complete with DirtyWorktree. Fixed in commit `175c1a4`; guard in SP-013.

3. **Manual land after batch failure** — Work landed on `main` via operator merge while spine batch status remained failed. Use `spine batch complete --detect-manual-merge` (see [Operator recovery](#operator-recovery)).

### GitHub issues ([beettlle/stet](https://github.com/beettlle/stet/issues))

| Issue | Type | Task | Status |
|-------|------|------|--------|
| [#1](https://github.com/beettlle/stet/issues/1) Configurable file exclude/include patterns | enhancement | SP-001 | Done |

### Phase 1 — CLI core (remaining)

| Task | Summary | Status | Deps |
|------|---------|--------|------|
| SP-009 | Python hunk expansion | Ready | SP-008 (done) |

### Phase 2 — Extension production UX (remaining)

| Task | Summary | Status | Deps |
|------|---------|--------|------|
| SP-006 | Extension dismiss findings | Ready | SP-005 (done) |
| SP-007 | Extension confidence and category filters | Ready | SP-006 |

### Phase 3 — Hygiene (post wave-0 verification)

| Task | Summary | Status | Deps |
|------|---------|--------|------|
| SP-010 | Raise CLI coverage to AGENTS.md gates (77%/72%) | Ready | — |
| SP-011 | Fix spine task contract testCommand typos | Ready | — |
| SP-012 | Revise spine wave plan after wave-0 lessons | Ready | — |
| SP-013 | Guard against extension/coverage re-tracking | Ready | — |

### Completed (wave 0 + follow-on)

SP-001, SP-002, SP-003, SP-004, SP-005, SP-008 — `.DONE` on `main`.

---

## Execution policy

1. **Preflight** before every batch: `spine preflight`.
2. **One CLI core task per serial lane** — never chain SP-001-style `cli/**` tasks in one lane; use disjoint parallel batches. `spine plan pending` may still serialize SP-009 + SP-010 in lane-2 — **do not use** `pending --wave N` when that happens; start named task batches instead.
3. **Parallel lanes** only when `fileScopeMustChange` paths are disjoint (see waves below).
4. **Land loop:** `spine gate approve` → `spine integrate` → `spine batch complete` (or `complete --detect-manual-merge` after manual lane merge).
5. **Never** ban `spine-tasks/**` in `fileScopeMustNotChange`.
6. **Pre-landed paths:** Narrow `fileScopeMustChange` to new deliverables when preflight warns.
7. **Before extension batches:** run `scripts/check-untracked-coverage.sh` once SP-013 lands.
8. **Prefer named batches** over `spine batch start pending --wave N` when tasks share `cli/**`, `extension/**`, or dependency chains that the auto-planner serializes unsafely.

---

## Suggested batch waves (remaining work)

| Wave | Tasks | Lanes | Notes |
|------|-------|-------|-------|
| H0 | SP-011, SP-012, SP-013 | up to 3 | Hygiene; disjoint scopes; run before feature wave |
| 1 | SP-009 | 1 | Python expand; `cli/internal/expand/` only |
| 2a | SP-006 | 1 | Extension dismiss; `findingsPanel*` |
| 2b | SP-007 | 1 | After SP-006 — same `findingsPanel.ts` |
| H1 | SP-010 | 1 | Coverage gates (L); `cli/**/*_test.go` only |

**Start commands:**

```bash
spine batch start SP-011 SP-012 SP-013
spine batch start SP-009
spine batch start SP-006
spine batch start SP-007
spine batch start SP-010
```

**Avoid:** `spine batch start pending --wave 0` — auto-plan serializes SP-009 + SP-010 (both `cli/**`) in one lane, repeating wave-0 contract contamination ([#62](https://github.com/beettlle/pi-spine/issues/62)).

---

## Operator recovery

When implementation is **Done** in a lane worktree (`.DONE` present, tests pass) but the batch failed (contract `review_exhausted`, integrate error, or DirtyWorktree):

1. **Diagnose** — `spine batch status`; read lane worker output and `.reviews/` for the failing task.
2. **Retry** — If the failure is transient (stall, gate timeout): `spine batch retry <task-id>` or restart the single task batch.
3. **Skip** — If the task is obsolete or superseded: mark skipped in STATUS and remove from the active batch (operator decision only).
4. **Manual merge** — If code is correct and already merged to `main` outside spine integrate:
   - Confirm `main` contains the lane commits.
   - From the repo root: `spine batch complete --detect-manual-merge`
   - Spine matches lane SHAs to `main` and closes the batch without re-integrating ([#73](https://github.com/beettlle/pi-spine/issues/73)).
5. **Force-merge** — Last resort when SHAs diverged but operator accepts risk: land manually, then `complete --detect-manual-merge`; document in task STATUS Execution Log.

**DirtyWorktree** (untracked `extension/coverage/`): run `scripts/check-untracked-coverage.sh`; ensure coverage dirs are gitignored ([#74](https://github.com/beettlle/pi-spine/issues/74)). SP-013 adds the guard.

---
