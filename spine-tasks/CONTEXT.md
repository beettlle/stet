# stet — Context

**Last Updated:** 2026-07-01
**Status:** Active
**Next Task ID:** SP-014

---

## Current State

**Wave 0 landed on `main` (manual merge, batch `20260701T232227`):** SP-001, SP-002, SP-003, SP-005, SP-008 marked `.DONE`. Remaining feature backlog: SP-004, SP-006, SP-007, SP-009.

**Hygiene backlog (SP-010–SP-013):** From 2026-07-01 `.DONE` verification — coverage gates, contract testCommand typos, wave-plan revision, coverage-artifact guard. Run **SP-011, SP-012, SP-013** before the next spine batch; **SP-010** (L) can run in parallel or after wave 1.

**pi-spine v1.2.0:** `contract.mode: required`; preflight `prelanded-file-scope` warns when `fileScopeMustChange` paths already on `main`. Serial-lane cumulative contract verify ([#62](https://github.com/beettlle/pi-spine/issues/62)) — **do not** put multiple `cli/**` tasks in one serial lane; use disjoint parallel waves. DirtyWorktree from tracked `extension/coverage/` fixed in `175c1a4` — see SP-013.

### GitHub issues ([beettlle/stet](https://github.com/beettlle/stet/issues))

| Issue | Type | Task | Status |
|-------|------|------|--------|
| [#1](https://github.com/beettlle/stet/issues/1) Configurable file exclude/include patterns | enhancement | SP-001 | Done |

### Phase 1 — CLI core (remaining)

| Task | Summary | Status | Deps |
|------|---------|--------|------|
| SP-004 | Auto-finish when zero findings | Ready | SP-005 |
| SP-009 | Python hunk expansion | Ready | SP-008 |

### Phase 2 — Extension production UX (remaining)

| Task | Summary | Status | Deps |
|------|---------|--------|------|
| SP-006 | Extension dismiss findings | Ready | SP-005 |
| SP-007 | Extension confidence and category filters | Ready | SP-006 |

### Phase 3 — Hygiene (post wave-0 verification)

| Task | Summary | Status | Deps |
|------|---------|--------|------|
| SP-010 | Raise CLI coverage to AGENTS.md gates (77%/72%) | Ready | — |
| SP-011 | Fix spine task contract testCommand typos | Ready | — |
| SP-012 | Revise spine wave plan after wave-0 lessons | Ready | — |
| SP-013 | Guard against extension/coverage re-tracking | Ready | — |

### Completed (wave 0)

SP-001, SP-002, SP-003, SP-005, SP-008 — `.DONE` on `main`.

---

## Execution policy

1. **Preflight** before every batch: `spine preflight`.
2. **One CLI core task per serial lane** — never chain SP-001-style tasks in lane-1; use disjoint parallel batches.
3. **Parallel lanes** only when `fileScopeMustChange` paths are disjoint (see waves below).
4. **Land loop:** `spine gate approve` → `spine integrate` → `spine batch complete` (or `complete --detect-manual-merge` after manual lane merge).
5. **Never** ban `spine-tasks/**` in `fileScopeMustNotChange`.
6. **Pre-landed paths:** Narrow `fileScopeMustChange` to new deliverables when preflight warns.
7. **Before extension batches:** run `scripts/check-untracked-coverage.sh` once SP-013 lands.

---

## Suggested batch waves (remaining work)

| Wave | Tasks | Lanes | Notes |
|------|-------|-------|-------|
| H0 | SP-011, SP-012, SP-013 | up to 3 | Hygiene; disjoint scopes; run before feature wave |
| 1 | SP-009 | 1 | Python expand; after SP-008 (done) |
| 2a | SP-006 | 1 | Extension dismiss |
| 2b | SP-007 | 1 | After SP-006 — same `findingsPanel.ts` |
| 3 | SP-004 | 1 | Auto-finish; CLI + extension |
| H1 | SP-010 | 1 | Coverage gates (L); test-only |

**Start commands:**

```bash
spine batch start SP-011 SP-012 SP-013
spine batch start SP-009
spine batch start SP-006
spine batch start SP-007
spine batch start SP-004
spine batch start SP-010
```

**Avoid:** `spine batch start pending --wave 0` when the plan serializes overlapping `cli/**` tasks in one lane.

---
