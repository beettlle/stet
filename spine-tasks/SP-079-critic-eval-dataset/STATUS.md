# SP-079: Critic-eval dataset from pi-spine release dismissals — Status

**Current Step:** Complete
**Status:** ✅ Complete (2026-11-02)
**Last Updated:** 2026-11-02
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Harvest `drop` rows from pi-spine
**Status:** ✅ Complete (2026-11-02)

- [x] Dismissals extracted with hunks, skips counted

---

### Step 2: Harvest `keep` rows
**Status:** ✅ Complete (2026-11-02)

- [x] Fixed findings with pre-fix hunks (≥ 10 or WARN recorded)

---

### Step 3: Provenance and README
**Status:** ✅ Complete (2026-11-02)

- [x] Provenance TSV and README

---

### Step 4: Testing & Verification
**Status:** ✅ Complete (2026-11-02)

- [x] `LoadDataset` check recorded
- [x] Contract `testCommand` passes
- [x] Nothing outside File Scope changed

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| pi-spine task folders carry committed `.reviews/contract-fail-*.log` files embedding exact stet finding records (`<id>  <file>:<line>  <SEVERITY>  <message>`); 46 logs across SP-820..SP-845 gave per-finding file/line/severity/message for rows whose ids STATUS names or whose claims match STATUS triage bullets | Used | `pi-spine/spine-tasks/SP-8*/.reviews/` |
| Log-mined vs manual rows: 60 drop rows carry id/file/line/severity plus verbatim-or-clipped log messages; 28 rows reconstructed from STATUS text + git (tasks whose dismissal rounds have no log: SP-820/821/824/826/828/837, SP-827 round 1, and 3 keeps) | Done | `docs/research/critic-eval/` |
| Only STATUS-reconciled findings become rows; log ids whose per-id disposition STATUS does not record are counted as skips (skip ledger computed in README) | Done | README |
| STATUS Location column can disagree with the log (SP-835 4396a10: STATUS Location says `src/config/env-overrides.mjs`, log records `task-agent-profile.mjs:130`) — the reviewer-time log is treated as authoritative; noted in provenance | Documented | provenance TSV |

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-11-02 | Step 4: `LoadDataset` via throwaway program at /tmp/sp079-build/loaddataset-check (module `stet/cli/sp079check`, outside the repo): `OK LoadDataset: keep=11 drop=78 groups=82 conflict=0`. Contract testCommand: jq OK; grep keep=11, drop=78; `go test ./cli/internal/criticeval/... -count=1 -run TestLoadDataset` → `ok stet/cli/internal/criticeval 0.173s`. `git status` clean — only docs/research/critic-eval/* and this STATUS.md changed | All green |
| 2026-11-02 | Step 3: provenance TSV (89 rows: id/label/repo/task/commit/status_file/status_line/reason) and README (sources, row rules, counts 78 drop / 11 keep, 139-finding skip ledger by reason, threats to validity) | Written |
| 2026-11-02 | Step 2: keeps harvested from fix commits (b8deb92a, 1ba31c36, 5a1024a0, f14fb499, 2ed04a88, e7abf035, d8b531d3, 80ca9e3d, b61b9fbb); ids for e7abf035/80ca9e3d/b61b9fbb keeps recovered from contract-fail logs (7e9057f, 360352d, 8653ab9, 85d0dd4) | 11 keep rows, each with pre-fix hunk from cumulative diff up to fix-commit^; no WARN needed |
| 2026-11-02 | Step 1: mined 46 `.reviews/contract-fail-*.log` files + SP-820..SP-845 STATUS triage records; reconstructed hunks via cumulative task diffs | 77 drop rows written (60 log-mined, 17 manual); skip ledger computed (120 findings skipped with reasons); dataset committed |

---

## Blockers

*None*
