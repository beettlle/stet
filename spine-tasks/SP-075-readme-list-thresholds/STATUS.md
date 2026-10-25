# SP-075: Docs: README `stet list` thresholds and contract gating — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-24
**Review Level:** 0
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: README text
**Status:** ✅ Complete

- [x] `stet list` row mentions the flags
- [x] Gating paragraph with policy and env overrides

---

### Step 2: Testing & Verification
**Status:** ✅ Complete

- [x] Contract `rg` checks pass
- [x] Doc test references noted

---

## Verification Evidence

- `rg -n 'min-severity' README.md` → lines 56 (gating paragraph) and 140 (commands table row). ✅
- `rg -n 'min-confidence' README.md` → same lines. ✅
- Policy values match `scripts/spine-stet-contract-run.sh`: defaults `warning` / `0.8` (`SPINE_STET_MIN_SEVERITY:-warning`, `SPINE_STET_MIN_CONFIDENCE:-0.8`), gating via `stet list --min-severity ... --min-confidence ...`, env overrides supported. ✅
- Every statement verified against `cli/cmd/stet/main.go` (`newListCmd` ~L1410, `listThresholds`, `runList`, `writeFindingLines`): flag values `error|warning|info|nitpick` and `0..1`; empty output + exit 0 when nothing meets the threshold; invalid values exit 1 (`erruser.New` → `Run()` returns 1); hidden findings stay active and `stet status` still counts them (per `newListCmd` Long help). ✅
- `rg -l 'README.md' cli/ scripts/` → **no matches**; no test in `cli/` or `scripts/` references this doc. ✅
- Contract testCommand (`true`) passes. File scope respected: only `README.md` changed in the diff. ✅
- `gitnexus_detect_changes` (unstaged): 1 changed file, docs-only, risk level low, 0 affected processes. ✅

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|

