# Task: SP-057 — `stet run` warns when review flags have no effect

**Created:** 2026-10-17
**Size:** S

## Review Level: 2

**Assessment:** New `RunStats` field in `cli/internal/run/run.go` and a warning in `cli/cmd/stet/main.go` (both hot shared modules).
**Score:** 4/8 — Blast radius: 2, Pattern novelty: 1, Security: 0, Reversibility: 1 — Level 2 for the hot modules

## Mission

Closes #23

`stet start` reviews the full diff. A later `stet run` reviews only hunks not yet reviewed; when there are none, flags such as `--strictness`, `--verify`, `--critic-backend`, or `--nitpicky` passed to `stet run` do nothing, and nothing says so (especially with `--quiet`).

1. **Count reviewed hunks.** Add `HunksReviewed int` to `RunStats` (`cli/internal/run/run.go` ~line 1099). `Run` (~line 1669) sets it to the number of hunks sent for review (`len(toReview)`); the zero-hunk early return (~line 1739) leaves it 0. Set it in `Start` and `Rerun` paths too if they return `RunStats`, so the field is never misleading.
2. **Warn.** In `runRun` (`cli/cmd/stet/main.go` ~line 769), after `run.Run` succeeds (~line 916): if `stats.HunksReviewed == 0` and any review-affecting flag was set on the command line (`Flags().Changed` for `strictness`, `nitpicky`, `verify`, `critic-backend`, `rules-max-tokens`, `context`, `num-ctx` — check which of these `stet run` actually registers and use only those), print **one** line to stderr regardless of `--quiet`:

   `warning: no new hunks to review; --strictness, --verify had no effect (pass them to "stet start" or use "stet rerun")`

   listing only the flags that were set, in a stable order. Exit status and stdout stay unchanged.

Extract the "which flags were set" check into a small helper in `main.go` so it is unit-testable.

## Dependencies

- **Task:** SP-052 (also edits `cli/cmd/stet/main.go`; serialize to avoid merge conflicts)
- **Task:** SP-054 (also edits `cli/internal/run/run.go`; serialize to avoid merge conflicts)
- **Task:** SP-048 (also edits `docs/cli-extension-contract.md`; serialize to avoid merge conflicts)

## Context to Read First

**Tier 3 (reference docs):**

- `cli/internal/run/run.go` — `RunStats`, `Run`, zero-hunk return
- `cli/cmd/stet/main.go` — `newRunCmd` (~line 539), flag `Changed` checks (~lines 576–590), `runRun`
- `cli/cmd/stet/main_test.go` — existing `run` command tests and stderr capture helpers

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/run/run.go`
- `cli/internal/run/run_test.go`
- `cli/cmd/stet/main.go`
- `cli/cmd/stet/main_test.go`
- `docs/cli-extension-contract.md`

## Documentation Requirements

**Must Update:**

- `docs/cli-extension-contract.md` — `stet run` description: one sentence on the no-new-hunks warning

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/run/... ./cli/cmd/stet/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/run/run.go`, `cli/cmd/stet/main.go`, `cli/cmd/stet/main_test.go`, `docs/cli-extension-contract.md` |
| fileScopeMustNotChange | `cli/internal/findings/**`, `cli/internal/benchmark/**`, `cli/internal/openaicompat/**`, `extension/**`, `scripts/**`, `.review/**` |
| completionCriteria | `stet run --strictness lenient` with no new hunks prints the warning to stderr (also with `--quiet`) and exits as before (test); no warning when hunks were reviewed or no review-affecting flag was set (tests); `RunStats.HunksReviewed` set correctly (test); contract doc updated; stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: HunksReviewed in RunStats

- [ ] Field added and set on every return path that reviews hunks
- [ ] `spine_review_step` after the step

### Step 2: Warning in runRun

- [ ] Helper lists changed review-affecting flags; warning printed once to stderr even with `--quiet`
- [ ] Contract doc sentence
- [ ] `spine_review_step` after the step

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] `run_test.go`: zero-hunk `Run` → `HunksReviewed == 0`; dry-run with hunks → `HunksReviewed > 0`
- [ ] `main_test.go`: helper returns only the set flags in stable order; warning printed with `--quiet` when zero hunks and a flag set; not printed otherwise
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Feature:** `feat(SP-057): description`
- **Tests:** `test(SP-057): description`

## Do NOT

- Change review behavior, exit codes, or stdout output
- Apply the flags retroactively or trigger a re-review
- Edit the benchmark printer (SP-052 owns that area)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
