# Task: SP-004 — Auto-finish when zero findings

**Created:** 2026-06-30
**Size:** M

## Review Level: 2

**Assessment:** Session lifecycle change affecting CLI and extension; user-visible behavior shift with opt-out needed.
**Score:** 5/8 — Blast radius: 2, Pattern novelty: 1, Security: 0, Reversibility: 2

## Mission

Implement PRD §9 **auto-finish when review completes with zero active findings**: persist session state, remove worktree, write git note, and clear the extension panel without requiring an explicit Finish click.

Provide an **opt-out** (config/env/flag) so teams can keep explicit `stet finish` only. Retain explicit Finish when findings remain.

## Dependencies

- **SP-005** (extension must run real review path before auto-finish UX is meaningful in IDE)

## Context to Read First

- `docs/PRD.md` §9 (Session end alternative)
- `docs/implementation-plan.md` Phase 5.4 note on auto-finish
- `cli/internal/run/run.go` — `Start`, `Run`, `Finish`
- `cli/cmd/stet/main.go` — start/run/finish commands
- `extension/src/finishReview.ts`, `extension/src/extension.ts`

## Environment

- **Workspace:** stet monorepo (`cli/` + `extension/`)

## File Scope

- `cli/internal/run/run.go`
- `cli/internal/run/run_test.go`
- `cli/internal/config/config.go`
- `cli/internal/config/config_test.go`
- `cli/cmd/stet/main.go`
- `cli/cmd/stet/main_test.go`
- `extension/src/extension.ts`
- `extension/src/finishReview.ts`
- `extension/src/finishReview.test.ts`
- `docs/cli-extension-contract.md`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./internal/run/... ./internal/config/... ./cmd/stet/... -count=1 && cd ../extension && npm run compile && npm test` |
| fileScopeMustChange | `cli/internal/run/run.go`, `cli/cmd/stet/main.go`, `extension/src/extension.ts` |
| fileScopeMustNotChange | `scripts/**`, `spine-tasks/**` |
| completionCriteria | Zero-findings run triggers finish path when enabled; opt-out works; explicit finish unchanged when findings exist |

## Steps

### Step 0: Design opt-in default and triggers

- [ ] Read PRD §9; decide default (recommend **opt-in** via config `auto_finish_zero_findings = true` or env `STET_AUTO_FINISH_ZERO=1`)
- [ ] Define trigger points: end of `stet start` / `stet run` when active findings count is 0
- [ ] Document that dry-run may be excluded or included — record in STATUS

### Step 1: CLI auto-finish path

- [ ] Add config/env/flag for auto-finish-on-zero
- [ ] After successful start/run with zero findings, invoke existing `Finish` logic (or shared helper)
- [ ] Tests: enabled → worktree removed and note written; disabled → session remains

### Step 2: Extension behavior

- [ ] When streamed/JSON review completes with 0 findings and auto-finish enabled, call finish flow and clear panel
- [ ] When findings exist, require explicit Finish (unchanged)
- [ ] Update extension tests

### Step 3: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Run Contract `testCommand`
- [ ] Document behavior in `docs/cli-extension-contract.md`

## Completion Criteria

- [ ] All steps complete
- [ ] Auto-finish on zero findings with opt-out
- [ ] CLI and extension tests pass

## Git Commit Convention

- **Implementation:** `feat(SP-004): description`

## Do NOT

- Remove explicit `stet finish` command
- Auto-finish when dismissed-but-unresolved findings remain (active findings = findings minus dismissed)

---

## Amendments (Added During Execution)
