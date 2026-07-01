# stet — Context

**Last Updated:** 2026-06-30
**Status:** Active
**Next Task ID:** SP-010

---

## Current State

Backlog staged from gap analysis. Optimizer on `diffusion` — SP-002 ports it. **SP-008 split:** JS/TS (SP-008) + Python (SP-009). Contracts fixed: no `spine-tasks/**` in `fileScopeMustNotChange`.

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
| SP-007 | Extension confidence and category filters | Ready | SP-005 |

---

## Execution policy

1. **Preflight** before every batch: `spine preflight`.
2. **One CLI core task per batch** when scopes overlap `cli/**` or `docs/cli-extension-contract.md`.
3. **Parallel lanes** only when `fileScopeMustChange` paths are disjoint (see waves below).
4. **Land loop:** `spine gate approve` → `spine integrate` → `spine batch complete`.
5. **Never** ban `spine-tasks/**` in `fileScopeMustNotChange` — workers update STATUS/.DONE.

---

## Suggested batch waves (parallel where disjoint)

| Wave | Tasks | Lanes | Notes |
|------|-------|-------|-------|
| 0 | SP-001 | 1 | Core pipeline; alone |
| 1 | SP-002, SP-003, SP-005, SP-008 | up to 4 | `scripts/**`, `cli/internal/prompt/**`, `extension/**`, `cli/internal/expand/expand_jsts.go` — disjoint |
| 2 | SP-009 | 1 | After SP-008; `expand_py.go` |
| 3 | SP-006, SP-007 | 2 | Extension panel; after SP-005 |
| 4 | SP-004 | 1 | After SP-005; CLI + extension auto-finish |

**Start commands:**

```bash
spine batch start SP-001
spine batch start SP-002 SP-003 SP-005 SP-008
spine batch start SP-009
spine batch start SP-006 SP-007
spine batch start SP-004
```

**Port note:** Before SP-002, diff `main..diffusion` for `scripts/optimize.py`.

---
