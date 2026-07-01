# stet — Context

**Last Updated:** 2026-06-30
**Status:** Active
**Next Task ID:** SP-009

---

## Current State

Backlog staged from gap analysis (extension, PRD, partial implementations). Optimizer work exists on `diffusion` branch — SP-002 ports it; do not reimplement.

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
| SP-008 | Multi-language hunk expansion (JS/TS, Python) | Ready | — |

### Phase 2 — Extension production UX

| Task | Summary | Status | Deps |
|------|---------|--------|------|
| SP-005 | Extension production review path (real CLI, streaming) | Ready | — |
| SP-006 | Extension dismiss findings | Ready | SP-005 |
| SP-007 | Extension confidence and category filters | Ready | SP-005 |

---

## Execution policy

**Operator runbook:** [`docs/adoption/operator-runbook.md`](../docs/adoption/operator-runbook.md) — install, preflight, start/monitor, land loop, gate races, resume/dismiss/complete, dashboard, troubleshooting.

1. **Preflight** before every batch: `spine preflight`.
2. **Land loop:** `spine batch start` → monitor `spine status --diagnose` → `spine gate approve` → `spine integrate` → `spine batch complete`.
3. **Never** hand-edit `.spine/batch-state.json`.

---

## Suggested batch waves

| Wave | Tasks | Notes |
|------|-------|-------|
| 0 | SP-001 | Exclude patterns (already planned) |
| 1 | SP-002, SP-003, SP-008 | Disjoint scopes; parallel OK |
| 2 | SP-005 | Extension real review — unblocks SP-004/006/007 |
| 3 | SP-006, SP-007 | Extension panel features (parallel) |
| 4 | SP-004 | Auto-finish (CLI + extension; depends SP-005) |

**Port note:** Before SP-002, diff `main..diffusion` for `scripts/optimize.py` and completed spine tasks SP-004/012/027 on that branch.

---
