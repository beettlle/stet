# SP-045: Dismissals survive `stet finish` — Status

**Current Step:** 1
**Status:** 🟡 In Progress
**Last Updated:** 2026-10-15
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

### Step 1: Restorable dismissals
**Status:** ✅ Complete

- [x] Newest-50 history scan; `already_correct` excluded
- [x] Changed-file filter (one git call per distinct DiffRef)

---

### Step 2: Re-apply in `Start`
**Status:** ✅ Complete

- [x] ID match, then file + category + line-overlap match
- [x] Wired before final session save; warning on error; verbose/trace output

---

### Step 3: Docs
**Status:** ✅ Complete

- [x] `docs/review-process-internals.md` and `docs/cli-extension-contract.md` updated as listed above

---

### Step 4: Testing & Verification
**Status:** ⬜ Not Started

- [ ] Collection and matching unit tests
- [ ] start → dismiss → finish → start integration test; changed-file test
- [ ] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| Reviewer re-reported c8a8d8d verbatim after the fix landed (claim contradicted by the code at the reported line) | stet reviewer bug — SP-040 stale-hunk hazard again; dismissed `already_correct` with fix-commit evidence | spine-tasks/CONTEXT.md "Known hazard (SP-040)" |
| Reviewer claimed parallel test mutates a shared git repo; repos come from `testutil.TempGitDir` (fresh `os.MkdirTemp` per test) | stet reviewer false positive; dismissed `false_positive` with testutil source as evidence | cli/internal/testutil/gitenv.go:124 |

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-10-14 | Task staged | PROMPT.md and STATUS.md created (release v1.1.0) |
| 2026-10-15 | Plan drafted | See plan below; in-worker plan review skipped (nested spawn blocked, SP-195) |
| 2026-10-15 | Step 1 implemented | `restore_dismissals.go`: collection (newest-50, latest-reason/DiffRef, latest mention must carry finding object) + changed-file filter |
| 2026-10-15 | Step 2 implemented | Wired into `Start`: collect before Partition trace section, matched after review loop before final save; warning/verbose/trace verified by tests; 13 new tests pass, file coverage 91.7–100% per func |
| 2026-10-15 | Step 3 done | Docs committed (6b0cf07) |
| 2026-10-15 | Contract triage r1 | 3 findings: 9af269d + a30f8e4 fixed (span consistency, trace guard, cb3756f); 50a40f3 dismissed `false_positive` (TempGitDir isolates each test repo) |
| 2026-10-15 | Contract triage r2 | c8a8d8d fixed: inverted ranges fall back to Line (d148306) |
| 2026-10-15 | Contract triage r3 | c8a8d8d re-reported verbatim against the already-fixed line → dismissed `already_correct` (SP-040 stale-hunk hazard) |
| 2026-10-15 | Verification | Full contract testCommand PASS (gofmt, vet, run+history tests, stet strict 0 findings); extension `npm test` PASS (90.71% lines); check-untracked-coverage.sh PASS; `go test ./...` 36/36 packages ok; changed files exactly the 5 in scope; restore_dismissals.go coverage 94.4–100% per func |

## Plan

**`restore_dismissals.go` (new):**
- `maxRestoreHistoryRecords = 50`.
- `restorableDismissal` struct: id, diffRef, file, category, line, rangeStart, rangeEnd.
- `collectRestorableDismissals(stateDir)`: `history.ReadRecords` (oldest first) → newest 50 window; per record map ReviewOutput by ID; for each `UserAction.DismissedIDs` entry with a finding object present, keep the most recent location + DiffRef (later records overwrite); track most recent recorded reason per ID (only updated when a `Dismissals` entry actually carries a reason, so a Finish record without reasons doesn't wipe an earlier reason); drop IDs whose latest reason is `already_correct`. Output sorted by ID for determinism.
- `filterUnchangedFiles(ctx, repoRoot, dismissals, headRef)`: one `git diff --no-color --name-only <DiffRef> <headRef>` per distinct DiffRef (`git.MinimalEnv()`, `cmd.Dir = repoRoot`); unresolvable DiffRef → drop its dismissals; keep only dismissals whose file is not in the changed set.
- `matchesRestorableDismissal(d, f)`: same ID, or same File + Category with overlapping line spans (span = `Range.Start..Range.End` when Range set, else `Line..Line`).
- `applyRestoredDismissals(s, collected, restorable) int`: append matched finding IDs to `s.DismissedIDs` (no duplicates); returns restored count.

**`run.go` `Start` wiring:**
- After partition and the empty-ToReview guard condition (collected only when `len(part.ToReview) > 0`), before the Partition trace section: collect + filter; history read error → `warning: could not restore dismissals: <err>` on stderr, `Start` continues.
- Partition trace section gains `Restorable dismissals from history: N`.
- After the review loop sets `s.Findings = collected` (post `applyAutoDismiss`, pre final `session.Save`): `applyRestoredDismissals`; when N > 0 print `restored N dismissals from history` to stderr with `--verbose` and to the trace.
- Note: the matched count is only known after the review, so the trace Partition section records the restorable-candidate count; the matched count is traced/verbose-printed at the restore point.

**Tests (`restore_dismissals_test.go`):** collection unit tests (newest-50 window, already_correct precedence incl. false_positive→auto-dismiss, missing ReviewOutput entry, unresolvable DiffRef); matching table test (same ID; different ID + file/category/overlap; different category; different file; boundary overlap); integration tests with `DryRun: true` (start → dismiss like `stet dismiss` → finish → start restores; changed-file commits → not restored); warning-path test; trace/verbose output test.

**Docs:** `docs/review-process-internals.md` (session/dismissal sections) + `docs/cli-extension-contract.md` (dismiss/finish bullets).

---

## Amendments (Added During Execution)

- Trace nuance: the matched "restored N" count is only known after the review, while the Partition trace section prints before it. The Partition section therefore reports the restorable-candidate count (`Restorable dismissals from history: N`, post changed-file filter); the matched count is printed at the restore point to stderr (`--verbose`) and to the trace.
- Collection semantics clarified: a dismissal is restorable from the **most recent record that mentions the ID** (the finding object must be in that record's ReviewOutput); a later mention without the finding object makes it non-restorable. A recorded reason is only overwritten by a later record that actually records one (Finish records carry no reasons, so they don't wipe earlier reasons).
- Docs fix in scope: `docs/review-process-internals.md` §10 note wrongly claimed the session file survives `stet finish` (run.Finish deletes it); corrected while documenting restoration.

---

## Blockers

*None*
