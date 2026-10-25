# SP-072: Dismissals follow line shifts in `stet start` and `stet run` — Status

**Current Step:** Step 3
**Status:** 🔄 In Progress
**Last Updated:** 2026-10-25
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Start-time restore re-maps instead of dropping
**Status:** ✅ Complete (commit 8a6d6fa)

- [x] Re-map per `diffRef`; edited spans dropped; `Start` call site updated

---

### Step 2: Run-time skip re-maps from `LastReviewedAt`
**Status:** ✅ Complete

- [x] Dismissed spans re-mapped before the overlap check; fallback when unavailable

---

### Step 3: Testing & Verification
**Status:** ⬜ Not Started

- [ ] Tests (a)–(d)
- [ ] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Plan

- GitNexus impact (required by AGENTS.md): `filterUnchangedFiles` and `filterHunksWithDismissedFindings` both flag HIGH risk. Blast radius is contained: one direct caller each (`Start` → `stet start`; `Run` → `stet run`/`rerun`/`commit-msg`), and both call sites are in the PROMPT scope. Fallback paths keep today's behavior when re-mapping is unavailable.
- Step 1: replace `filterUnchangedFiles` with `remapRestorableDismissals` (group by diffRef, one `loadLineRemaps` per ref, `remapSpan` per dismissal, drop ok=false/unresolvable). Remove `changedFilesSince` (no remaining callers) and prune imports.
- Step 2: `filterHunksWithDismissedFindings(ctx, repoRoot, toReview, s, targetRef, forceFullReview)`; re-map dismissed spans from `s.LastReviewedAt` when non-empty, ≠ targetRef, and git succeeds; ok=false spans are dropped so edited dismissed code is re-reviewed; update the `Run` caller.
- Step 3 tests: unit tests pin shift/keep/drop + fallbacks; end-to-end (a) `Start` restore after a 2-line insertion above a ranged dismissal (finding lands inside the shifted span); (b) start restore dropped when the dismissed line itself is edited; (c) run skips hunk at shifted lines (discriminating 2-hunk geometry) + edited-dismissed-line hunk is re-reviewed.

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| `docs/cli-extension-contract.md` line 88 (`stet dismiss`) says the next `stet start` restores dismissals "for unchanged files" — no longer accurate after SP-072 (changed files are restored when their line span survives the shift). SP-076 should reword, e.g. "restores matching dismissals from recent history, re-mapping recorded line spans to the current HEAD; dismissals whose own lines changed are not restored". | For SP-076 | docs/cli-extension-contract.md |
