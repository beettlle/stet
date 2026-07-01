# stet — Context

**Last Updated:** 2026-07-01
**Status:** Active
**Next Task ID:** SP-010

---

## Current State

Backlog staged from gap analysis. Optimizer on `diffusion` — SP-002 ports it. **SP-008 split:** JS/TS (SP-008) + Python (SP-009). Contracts fixed: no `spine-tasks/**` in `fileScopeMustNotChange`.

**pi-spine v1.2.0:** `contract.mode: required`; preflight `prelanded-file-scope` warns when `fileScopeMustChange` paths already on `main` — amend contract to delivery artifacts before retry. M-sized tasks get **180m** stall floor (SP-088) regardless of `lanes.stallTimeoutMinutes: 120`. Serial-lane cumulative contract verify ([#62](https://github.com/beettlle/pi-spine/issues/62)) still applies — one overlapping CLI task per batch.

### GitHub issues ([beettlle/stet](https://github.com/beettlle/stet/issues))

| Issue | Type | Task | Status |
|-------|------|------|--------|
| [#1](https://github.com/beettlle/stet/issues/1) Configurable file exclude/include patterns | enhancement | SP-001 | Ready |

No open **bug** issues as of 2026-06-30.

### Phase 1 — CLI core gaps

| Task | Summary | Status | Deps |
|------|---------|--------|------|
| SP-001 | Configurable file exclude patterns for review scope | Ready | — |
| SP-002 | Port optimizer sidecar from diffusion branch | Ready | — |
| SP-003 | Per-repo prompt override (`.review/prompt.md`) | Ready | — |
| SP-004 | Auto-finish when zero findings | Ready | SP-005 |
| SP-008 | JS/TS hunk expansion | Ready | — |
| SP-009 | Python hunk expansion | Ready | SP-008 |

### Phase 2 — Extension production UX

| Task | Summary | Status | Deps |
|------|---------|--------|------|
| SP-005 | Extension production review path (real CLI, streaming) | Ready | — |
| SP-006 | Extension dismiss findings | Ready | SP-005 |
| SP-007 | Extension confidence and category filters | Ready | SP-006 |

---

## Execution policy

1. **Preflight** before every batch: `spine preflight`.
2. **One CLI core task per batch** when scopes overlap `cli/**` or `docs/cli-extension-contract.md`.
3. **Parallel lanes** only when `fileScopeMustChange` paths are disjoint (see waves below).
4. **Land loop:** `spine gate approve` → `spine integrate` → `spine batch complete`.
5. **Never** ban `spine-tasks/**` in `fileScopeMustNotChange` — workers update STATUS/.DONE.
6. **Pre-landed paths:** If preflight warns `prelanded-file-scope`, narrow `fileScopeMustChange` to this task's new deliverables (see SP-004/SP-006 pattern).

---

## Suggested batch waves (parallel where disjoint)

| Wave | Tasks | Lanes | Notes |
|------|-------|-------|-------|
| 0 | SP-001 | 1 | Core pipeline; alone |
| 1 | SP-002, SP-003, SP-005, SP-008 | up to 4 | Disjoint: `scripts/**`, `cli/internal/prompt/**`, `extension/**`, `expand_jsts.go` |
| 2 | SP-009 | 1 | After SP-008; `expand_py.go` |
| 3a | SP-006 | 1 | Extension dismiss; after SP-005 |
| 3b | SP-007 | 1 | After SP-006 — both touch `findingsPanel.ts`; **not parallel** |
| 4 | SP-004 | 1 | After SP-005; CLI + extension auto-finish |

**Start commands:**

```bash
spine batch start SP-001
spine batch start SP-002 SP-003 SP-005 SP-008
spine batch start SP-009
spine batch start SP-006
spine batch start SP-007
spine batch start SP-004
```

**Port note:** Before SP-002, diff `main..diffusion` for `scripts/optimize.py`.

---
