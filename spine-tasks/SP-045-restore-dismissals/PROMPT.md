# Task: SP-045 — Dismissals survive `stet finish`

**Created:** 2026-10-14
**Size:** M

## Review Level: 2

**Assessment:** Session-state behavior change in `cli/internal/run` (hot module); reads history, changes what `stet start` reports.
**Score:** 5/8 — Blast radius: 2, Pattern novelty: 1, Security: 0, Reversibility: 2 (persisted dismissal semantics). Reversibility 2 → Level 2.

## Mission

Closes #8

Dismissals live only in the active session. `stet finish` writes them to `.review/history.jsonl` (`run.Finish`, `cli/internal/run/run.go` ~line 1546), but `run.Start` (~line 1107) always creates a session with `DismissedIDs: nil` and never reads history. The next `stet start` on the same baseline reviews every hunk again, and a re-generated false positive comes back as active. PRD FR-3 (`docs/PRD.md` ~line 249) requires dismissals to survive re-runs and "Finish review".

Finding IDs are `hunkid.StableFindingID(file, line, rangeStart, rangeEnd, message)` (`cli/internal/review/parse.go` ~line 219). The message is model text, so a fresh review of the same code can produce a different ID for the same false positive. Matching must therefore fall back to location.

**Behaviour (new file `cli/internal/run/restore_dismissals.go`, wired into `Start`):**

1. **Collect restorable dismissals.** Read history with `history.ReadRecords(opts.StateDir)` and scan only the newest `maxRestoreHistoryRecords = 50` records. A dismissal is restorable when its finding ID appears in a record's `UserAction.DismissedIDs` and the finding object is in that record's `ReviewOutput`. Exclude any ID whose most recent recorded reason (`UserAction.Dismissals`) is `already_correct`: that reason marks auto-dismissals (the code was fixed) and must not hide future findings. Keep the record's `DiffRef` with each dismissal.
2. **Keep only dismissals whose file is unchanged.** For each distinct `DiffRef`, run one `git diff --name-only <DiffRef> <HEAD>` in the repo (at most 50 calls; skip records whose `DiffRef` no longer resolves). Keep a dismissal only if its file is not in that list, so its line numbers still apply.
3. **Re-apply after the review.** In `Start`, after the review loop sets `s.Findings = collected` (~line 1463) and before the session is saved, mark each new finding dismissed when it matches a restorable dismissal:
   - same ID, or
   - same `File` and `Category` with overlapping line spans (`Line`, or `Range.Start`..`Range.End`).
   Matched IDs are appended to `s.DismissedIDs` (no duplicates). Findings stay in `s.Findings`, so `stet status` shows them as dismissed and `stet list` hides them.
4. **No effect elsewhere.** The baseline-equals-HEAD early return, `Run`, `Rerun`, `Finish`, and history-based prompt suppression are unchanged. A history read error is a warning on stderr (`warning: could not restore dismissals: <err>`), never a `Start` failure. With `--verbose`, print `restored N dismissals from history` when N > 0, and record the count in the trace (`--trace`) Partition section.

A fresh `stet start` still sends every hunk to the model; only matching findings are dismissed. Document that in the docs below (issue acceptance 2 allows documenting instead of skipping hunks).

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh strict`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Dependencies

- **None**

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md` — "Stet in spine (dogfooding)" section, "Known hazard (SP-035)"

**Tier 3 (reference docs):**

- `cli/internal/run/run.go` — `Start` (~1107–1470), `applyAutoDismiss` (~813), `filterHunksWithDismissedFindings` (~865), `Finish` (~1481), dry-run finding IDs (~747)
- `cli/internal/history/schema.go` — `Record`, `UserAction`, `Dismissal`, reason constants
- `cli/internal/history/append.go` — `ReadRecords` (record order, archives)
- `cli/cmd/stet/main.go` — dismiss command history record (~1464–1484), read only
- `docs/PRD.md` — FR-3 (~line 249), read only

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only (unit tests use dry-run and fakes)

## File Scope

- `cli/internal/run/restore_dismissals.go`
- `cli/internal/run/restore_dismissals_test.go`
- `cli/internal/run/run.go`
- `docs/review-process-internals.md`
- `docs/cli-extension-contract.md`

## Documentation Requirements

**Must Update:**

- `docs/review-process-internals.md` — session/dismissal section: dismissals are restored on `stet start` from recent history for unchanged files, matched by ID or file+category+line overlap; all hunks are still reviewed
- `docs/cli-extension-contract.md` — `stet dismiss` and `stet finish` bullets (~lines 88–89): dismissals persist across `stet finish`

**Check If Affected:**

- `docs/PRD.md` — FR-3 (do not edit)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/run/... ./cli/internal/history/... -count=1 && bash scripts/spine-stet-contract-run.sh strict` |
| fileScopeMustChange | `cli/internal/run/restore_dismissals.go`, `cli/internal/run/restore_dismissals_test.go`, `cli/internal/run/run.go`, `docs/review-process-internals.md`, `docs/cli-extension-contract.md` |
| fileScopeMustNotChange | `cli/cmd/**`, `cli/internal/history/**`, `cli/internal/session/**`, `extension/**`, `scripts/**`, `.review/**`, `go.mod`, `go.sum` |
| completionCriteria | review → dismiss → finish → start on the same baseline leaves the re-reported finding dismissed (not active); `already_correct` dismissals are not restored; dismissals for files changed since their record are not restored; history read errors only warn; at most 50 records and one git call per distinct DiffRef; docs updated; stet contract review exits 0 |

## Steps

### Step 1: Restorable dismissals

- [ ] `restore_dismissals.go`: read newest 50 records, collect dismissed findings with their DiffRef, drop latest-reason `already_correct`
- [ ] Changed-file filter with one `git diff --name-only` per distinct DiffRef

### Step 2: Re-apply in `Start`

- [ ] Match new findings by ID, then by file + category + overlapping lines; append matched IDs to `s.DismissedIDs`
- [ ] Wire into `Start` before the final session save; warning on history error; verbose and trace output

### Step 3: Docs

- [ ] `docs/review-process-internals.md` and `docs/cli-extension-contract.md` updated as listed above

### Step 4: Testing & Verification

> ZERO test failures allowed.

- [ ] Unit tests for collection: newest-50 window, `already_correct` excluded (including an ID dismissed earlier with `false_positive` and later auto-dismissed), finding missing from `ReviewOutput` skipped, unresolvable DiffRef skipped
- [ ] Unit tests for matching: same ID; different ID with same file+category+overlapping range; same location but different category → not matched; different file → not matched
- [ ] Integration test in a temp repo with `DryRun: true` (issue acceptance): `Start` → dismiss a finding the way `stet dismiss` does (history record with reason `false_positive`) → `Finish` → `Start` on the same baseline → the re-reported finding's ID is in `DismissedIDs` and active count drops accordingly
- [ ] Integration test: after the dismissal, change the finding's file in a new commit → the dismissal is not restored
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in Mission)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing; new file `restore_dismissals.go` ≥ 72% line coverage

## Git Commit Convention

- **Implementation:** `fix(SP-045): description`
- **Tests:** `test(SP-045): description`

## Do NOT

- Skip hunks in `Start` or change `Run` / `Rerun` / `Finish` behavior
- Change the history or session schema, or edit `cli/internal/history/**` / `cli/internal/session/**`
- Change finding ID generation
- Fail `stet start` because of history problems
- Edit `.review/config.toml` or `scripts/**`
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
