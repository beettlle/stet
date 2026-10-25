# SP-076: Bring review-process-internals up to date for v1.4.0 — Status

**Current Step:** 3 (complete)
**Status:** ✅ Complete
**Last Updated:** 2026-10-25
**Review Level:** 0
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Expand, RAG, and post-filter sections
**Status:** ✅ Complete

- [x] Areas 1–3 from Mission

---

### Step 2: Critic and dismissal sections
**Status:** ✅ Complete

- [x] Areas 4–5 from Mission

---

### Step 3: Testing & Verification
**Status:** ✅ Complete

- [x] Contract `rg` checks pass
- [x] Doc test references noted

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Verification evidence

- Contract `testCommand` (`true`) trivially passes.
- `rg -n existence docs/review-process-internals.md` → hits (§1 summary, §7.10 item 5, §9 pipeline, §13 index).
- `rg -n line_remap docs/review-process-internals.md` → hits (§8.1 Line re-mapping, §9 Dismissed-hunk skip, §13 index).
- `rg -n defaultCriticContextCap docs/review-process-internals.md` → hits (§7.10 item 6, §13 Critic row).
- `rg -n -i 'unchanged since its record' docs/review-process-internals.md` → no hits (exit 1).
- No remaining text claims the critic sees only the raw hunk: the §7.10 systemone bullet now says "The prepared code context is verified…", and the only "raw hunk" mentions state the fallback-when-empty rule.
- `rg -l 'review-process-internals.md' cli/ scripts/` → no matches: **no test references this doc**.
- Statements verified against code: `expandJSTSHunk` (expand_jsts.go:190–256, imports for any hunk below the import block, quarter-share budget clamped to `minJSTSTruncateTokens`, truncate-never-drop), `lookupDefinitions`/`resolveImportedDefinition`/`resolveModulePath` (rag/js/resolver.go:466–633), `FilterExistenceClaims` + wiring in both `runReviewPipeline` branches (run.go:513–522, 611–621, outside strictness conditionals, trace `Existence claims: N -> M`), `applyCritic`/`applyCriticSystemOne`/`criticInputFor` (run.go:653–739) and `defaultCriticContextCap = 16384` (critic.go:20–24), `remapRestorableDismissals` (restore_dismissals.go:121–158), `loadLineRemaps`/`remapSpan` (line_remap.go:55–140), `filterHunksWithDismissedFindings`/`remapDismissedLocations` (run.go:919–1004, `SkippedDismissed=N` trace).

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| SP-072's STATUS flagged `docs/cli-extension-contract.md` line 88 ("restores dismissals for unchanged files") as stale after the re-mapping change. That file is outside this task's File Scope (`docs/review-process-internals.md` only), so it was **not** edited here; it still needs the suggested reword in a future docs task. | out of scope, flagged | docs/cli-extension-contract.md |
| `stet rerun` sets `ForceFullReview: true`, so the §9 dismissed-hunk skip applies to `stet run` only; the doc says so explicitly. | documented | cli/cmd/stet/main.go:1146 |
| The old JS/TS expand bullet's "top-level changes are left unchanged" was false since SP-067 (top-level hunks get the import section); replaced with the imports-for-any-hunk behavior. | fixed | §7.5 |
| §9 had no description of the dismissed-hunk skip at all; added a full bullet covering `filterHunksWithDismissedFindings` and the `LastReviewedAt` re-mapping. | added | §9 |
