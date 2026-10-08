# Task: SP-034 — Wire rule-conflict annotation behind `rules_annotate`

**Created:** 2026-10-08
**Size:** S

## Review Level: 1

**Assessment:** Opt-in wiring of an annotation-only feature across config, pipeline, CLI, and docs; sends hunk text and finding text to the configured System One server.
**Score:** 3/8 — Blast radius: 2, Pattern novelty: 0, Security: 1, Reversibility: 0

## Mission

Let users opt into SP-027's rule-conflict annotation so findings can carry `rule_conflict` ("possible conflict with project guideline …"). **Off by default; annotation never drops, reorders, or fails a review.**

**Behaviour:**

- **Config:** bool `rules_annotate` (TOML) / `STET_RULES_ANNOTATE` (env; same parsing as `STET_RULES_SELECTOR`), default `false`. Reuses `systemone_*` keys and env-only `STET_SYSTEMONE_API_KEY`.
- **CLI (`main.go`):** the single shared System One client (from SP-033) is also built when `rules_annotate` is true (not in `--dry-run`); pass it to start/run/rerun options as the annotation client.
- **Pipeline (`run.go`):** after the critic step for a hunk (so only surviving findings are annotated), when the annotation client is non-nil and not dry-run: sections = `rules.SplitSections` over `rules.FilterRules(rulesForFile, hunk.FilePath)`; call `review.AnnotateRuleConflicts`. On error: keep the findings unannotated and print **one** warning per run to stderr naming the server URL (no API key). Annotation must never change finding count, order, IDs, or other fields.
- **Output:** `rule_conflict` appears in findings JSON (stdout, session state) only when set; history/git-note formats unchanged unless they already serialize `Finding` verbatim (then document it).
- **Docs (`docs/cli-extension-contract.md`):** document `rules_annotate` / `STET_RULES_ANNOTATE`, the `rule_conflict` object (`source`, `section`, `probability`), that it is advisory only, one request per ≤ 32 findings, skipped when a hunk has > 64 findings, and that hunk + finding text is sent to the server. Note that the extension does not display it yet.

**Why gated after SP-028:** the SP-020 research requires a ≥ 100-pair validation set before productizing rules compliance; SP-028 provides it.

## Dependencies

- **Task:** SP-027 (`AnnotateRuleConflicts`, `Finding.RuleConflict`)
- **Task:** SP-033 (shared client construction and the same config/run/main files)
- **Task:** SP-028 (validation set — productization gate from SP-020)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `cli/internal/review/rules_annotation.go` (SP-027)
- `cli/internal/run/run.go` — `applyCritic` and the pipeline post-processing order
- `cli/cmd/stet/main.go` — shared System One client helper (SP-033)
- `cli/internal/config/config.go` — `rules_selector` (SP-033) as the pattern
- `docs/research/systemone-rules-compliance.md` — Q4, Q5, Recommendation item 4

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none (tests use fakes)

## File Scope

- `cli/internal/config/config.go`
- `cli/internal/config/config_test.go`
- `cli/internal/run/run.go`
- `cli/internal/run/run_test.go`
- `cli/cmd/stet/main.go`
- `cli/cmd/stet/main_test.go`
- `docs/cli-extension-contract.md`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go vet ./... && go test ./internal/config/... ./internal/run/... ./cmd/stet/... -count=1 -cover` |
| fileScopeMustChange | `cli/internal/config/config.go`, `cli/internal/run/run.go`, `cli/cmd/stet/main.go`, `docs/cli-extension-contract.md` |
| fileScopeMustNotChange | `cli/internal/review/**`, `cli/internal/rules/**`, `cli/internal/findings/**`, `cli/internal/systemone/**`, `extension/**` |
| completionCriteria | Default-off output identical to before; enabled path annotates via fake decider without changing finding count/order; error path warns once without API key; dry-run never contacts the server; per-file coverage ≥ 72% for every changed Go file |

## Steps

### Step 1: Config and CLI

- [ ] `rules_annotate` key + env + tests
- [ ] Shared client built when annotation is on; passed to start/run/rerun

### Step 2: Pipeline and docs

- [ ] Annotation after critic; fail-open single warning
- [ ] `docs/cli-extension-contract.md` updated

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Default-off: findings JSON identical to pre-change for a fixture run
- [ ] Enabled with fake decider: `rule_conflict` set on the expected finding; count/order unchanged; error → one warning, findings unannotated; dry-run → zero requests
- [ ] CLI: client built once when any of critic=systemone / selector / annotate is on
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Coverage gate: `cd cli && go test ./... -coverprofile=../coverage.out -count=1 && cd .. && bash scripts/check-coverage.sh coverage.out` (then `git checkout -- coverage.out`)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Implementation:** `feat(SP-034): description`
- **Tests:** `test(SP-034): description`
- **Docs:** `docs(SP-034): description`

## Do NOT

- Drop or reorder findings based on annotations, or enable annotation by default
- Change the annotator, selector, or systemone libraries
- Add extension UI (log as backlog in `spine-tasks/CONTEXT.md`)
- Print or log the API key
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
