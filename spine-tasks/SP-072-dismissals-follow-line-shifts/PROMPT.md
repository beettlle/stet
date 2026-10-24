# Task: SP-072 — Dismissals follow line shifts in `stet start` and `stet run`

**Created:** 2026-10-24
**Size:** S

## Review Level: 2

**Assessment:** Wires SP-071's line remapping into the start-time restore and the run-time dismissed-hunk skip; touches `cli/internal/run/run.go` (hot shared module) and session dismissal behavior.
**Score:** 4/8 — Blast radius: 2, Pattern novelty: 0, Security: 0, Reversibility: 2 — Level 2 for the hot module and session state

## Mission

Partial #38

Dismissed findings come back under new IDs after edits above them shift their lines (pi-spine SP-835, SP-822). Two paths lose them:

1. **Start-time restore.** `filterUnchangedFiles` (`cli/internal/run/restore_dismissals.go` ~line 125, called from `Start` in `run.go` ~line 1291) drops every dismissal whose file changed between the dismissal's `diffRef` and `headSHA`.
2. **Run-time skip.** `filterHunksWithDismissedFindings` (`run.go` ~line 877, called ~line 1741) compares stored finding lines, recorded at an earlier commit, with hunk ranges at the current commit.

Change, using `loadLineRemaps` / `remapSpan` from `cli/internal/run/line_remap.go` (SP-071):

1. Replace `filterUnchangedFiles` with a function that re-maps each restorable dismissal's span from its `diffRef` to `headSHA`: one `loadLineRemaps` call per distinct `diffRef` (as today, one git call per ref, never per dismissal); shifted spans replace `line` / `rangeStart` / `rangeEnd`; dismissals with `ok = false` (their own lines changed) or an unresolvable `diffRef` are dropped. Update the `Start` call site. Remove `changedFilesSince` if it has no remaining caller.
2. In `filterHunksWithDismissedFindings`, re-map dismissed finding spans from `s.LastReviewedAt` to the commit being reviewed before the overlap check (one `loadLineRemaps` call for all dismissed files). Add `ctx`, `repoRoot`, and the target ref as parameters and update the caller. When `s.LastReviewedAt` is empty, equals the target ref, or the git call fails, keep today's behavior (no re-mapping).
3. Matching in `matchesRestorableDismissal` and the skip overlap check are otherwise unchanged.

## Dependencies

- **Task:** SP-071 (provides `line_remap.go`)
- **Task:** SP-070 (both edit `cli/internal/run/run.go`)

## Context to Read First

**Tier 3 (reference docs):**

- `cli/internal/run/line_remap.go` — `loadLineRemaps`, `remapSpan` (SP-071)
- `cli/internal/run/restore_dismissals.go` and `restore_dismissals_test.go`
- `cli/internal/run/run.go` — `Start` (~line 1280), `filterHunksWithDismissedFindings` (~line 877) and its caller (~line 1741)
- `cli/internal/run/run_test.go` — `filterHunksWithDismissedFindings` table test (~line 1300)

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/run/run.go`
- `cli/internal/run/run_test.go`
- `cli/internal/run/restore_dismissals.go`
- `cli/internal/run/restore_dismissals_test.go`

## Documentation Requirements

**Must Update:** none (SP-076 updates `docs/review-process-internals.md`).

**Check If Affected:** `docs/cli-extension-contract.md` — the dismissal-restore description; record needed wording under Discoveries for SP-076 rather than editing.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/run/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/run/run.go`, `cli/internal/run/restore_dismissals.go` |
| fileScopeMustNotChange | `cli/internal/run/line_remap.go`, `cli/internal/review/**`, `cli/internal/findings/**`, `extension/**`, `.review/**` |
| completionCriteria | Temp-repo tests: (a) a dismissed finding whose lines shift by N after an edit above it, then `stet start` on the new HEAD with a reworded finding at the shifted lines, is restored as dismissed; (b) a dismissal whose own lines were edited is not restored; (c) within a session, after an edit above a dismissed finding, `stet run` skips the hunk at the shifted lines; (d) a genuinely new finding in another span stays active; existing restore and skip tests still pass; stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Start-time restore re-maps instead of dropping

- [ ] Restorable dismissals re-mapped per `diffRef`; edited spans dropped; `Start` call site updated
- [ ] `spine_review_step` after the step

### Step 2: Run-time skip re-maps from `LastReviewedAt`

- [ ] `filterHunksWithDismissedFindings` re-maps dismissed spans before the overlap check; fallback to current behavior when re-mapping is unavailable
- [ ] `spine_review_step` after the step

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Tests (a)–(d) from Contract `completionCriteria`
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Fix:** `fix(SP-072): description`
- **Tests:** `test(SP-072): description`

## Do NOT

- Change `line_remap.go` behavior (record any defect under Discoveries and fix only if a test proves it)
- Add message-similarity matching or change history retention
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
