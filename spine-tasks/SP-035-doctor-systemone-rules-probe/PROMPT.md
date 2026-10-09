# Task: SP-035 — `stet doctor` probes System One for the rules selector and annotator

**Created:** 2026-10-09
**Size:** S

## Review Level: 1

**Assessment:** Small condition change in one doctor helper plus table tests; no new config, no wire changes.
**Score:** 1/8 — Blast radius: 1, Pattern novelty: 0, Security: 0, Reversibility: 0

## Mission

`checkSystemOne` in `cli/cmd/stet/main.go` only probes the System One server when `critic_enabled && critic_backend == "systemone"`. Since SP-033/SP-034, the rules selector (`rules_selector`) and rules annotator (`rules_annotate`) also call System One through the shared client built in `systemOneRunOptions`, so `stet doctor` reports success while those features would fail at review time.

**Behaviour:**

- `checkSystemOne` probes (health + model presence, unchanged logic and messages) when **any** of: critic enabled with backend `systemone`, `rules_selector` true, `rules_annotate` true. Mirror the `want*` booleans in `systemOneRunOptions` (doctor has no `--dry-run`, so selector/annotate count whenever set).
- When none are on, doctor output is unchanged (no System One lines).
- Update the doc comment on `checkSystemOne` to list all three triggers.
- Exit codes and stderr messages unchanged (2 for unauthorized/unreachable, 1 for other errors and missing model). Never print the API key.

**First stet-gated packet:** the contract ends with `scripts/spine-stet-contract-run.sh default`, which reviews this task's diff with stet itself (repo `.review/config.toml`). If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record any stet reviewer bug (bad prompt, parser, critic) under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Dependencies

- **None** (SP-018, SP-033, SP-034 are on `main`)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md` — "Stet in spine (dogfooding)" section

**Tier 3 (reference docs):**

- `cli/cmd/stet/main.go` — `checkSystemOne` (~line 1921) and `systemOneRunOptions` (~line 711)
- `cli/cmd/stet/main_test.go` — existing `TestRunCLI_doctorSystemOne*` tests (~line 3383; fake System One server via `httptest`) and `TestRunCLI_doctorSystemOneDisabledMakesNoRequest`

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** LM Studio at `http://127.0.0.1:1234` for the stet contract step only (unit tests use `httptest` fakes)

## File Scope

- `cli/cmd/stet/main.go`
- `cli/cmd/stet/main_test.go`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./cmd/stet/... -count=1 && go vet ./... && cd .. && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/cmd/stet/main.go`, `cli/cmd/stet/main_test.go` |
| fileScopeMustNotChange | `extension/**`, `cli/go.mod`, `cli/go.sum`, `.review/**` |
| completionCriteria | Doctor probes System One when critic=systemone, selector, or annotate is on, and is silent otherwise; table tests cover every trigger combination; stet contract review exits 0; per-file coverage ≥ 72% for changed Go files |

## Steps

### Step 1: Implementation

- [ ] Widen the `checkSystemOne` guard to critic=systemone OR `rules_selector` OR `rules_annotate`
- [ ] Update the doc comment

### Step 2: Testing & Verification

> ZERO test failures allowed.

- [ ] Table test: none on → no request to the fake server, no output; each trigger alone → health + models probed, "System One OK"; critic on with `llm` backend and selector/annotate off → no probe
- [ ] Failure path with selector on: unreachable server → exit 2 and the existing message
- [ ] Run Contract `testCommand` (includes the stet review; triage findings as described in Mission)
- [ ] Coverage gate: `cd cli && go test ./... -coverprofile=../coverage.out -count=1 && cd .. && bash scripts/check-coverage.sh coverage.out` (then `git checkout -- coverage.out`)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Implementation:** `fix(SP-035): description`
- **Tests:** `test(SP-035): description`

## Do NOT

- Change System One client code, timeouts, or config keys (timeout is SP-036)
- Change doctor messages or exit codes
- Edit `.review/config.toml` or commit anything under `.review/`
- Print or log the API key
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
