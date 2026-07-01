# Task: SP-001 — Configurable file exclude patterns for review scope

**Created:** 2026-06-30
**Size:** M

## Review Level: 2

**Assessment:** Touches the core review pipeline (config, diff filtering, run/partition wiring) with established patterns but non-trivial merge and incremental-review semantics.
**Score:** 5/8 — Blast radius: 2, Pattern novelty: 1, Security: 1, Reversibility: 1

## Mission

Implement [GitHub issue #1](https://github.com/beettlle/stet/issues/1): let users skip review of certain file types (e.g. `*.md`, `*.txt`) during active coding, then review documentation separately when exclusions are lifted.

`cli/internal/diff/diff.go` already has `Options.ExcludePatterns` and `filterByPatterns()`; `scope.Partition` accepts `*diff.Options`, but `run.go` currently passes `nil`. Wire user-configurable exclude patterns through config, env, and CLI into the partition/diff path with correct precedence and merge semantics.

## Dependencies

- **None**

## Context to Read First

- `spine-tasks/CONTEXT.md`
- `docs/constitution.md`
- `AGENTS.md`
- `docs/cli-extension-contract.md`
- `docs/review-process-internals.md` (§6.3 File-level filters)
- `cli/internal/diff/diff.go` — `Options`, `defaultExcludePatterns`, `filterByPatterns`
- `cli/internal/config/config.go` — load order and `Overrides` pattern
- `cli/internal/run/run.go` — `scope.Partition` call sites (currently pass `nil`)
- https://github.com/beettlle/stet/issues/1 — acceptance criteria and out-of-scope list

## Environment

- **Workspace:** stet monorepo (`cli/` Go module)
- **Services required:** none (unit/integration tests only)

## File Scope

- `cli/internal/config/config.go`
- `cli/internal/config/config_test.go`
- `cli/internal/diff/diff.go`
- `cli/internal/diff/diff_test.go`
- `cli/internal/run/run.go`
- `cli/cmd/stet/main.go`
- `cli/cmd/stet/main_test.go`
- `docs/cli-extension-contract.md`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./... -cover` |
| fileScopeMustChange | `cli/internal/config/config.go`, `cli/internal/diff/diff.go`, `cli/internal/run/run.go`, `cli/cmd/stet/main.go` |
| fileScopeMustNotChange | `extension/**`, `.spine/**`, `scripts/**` |
| completionCriteria | Issue #1 acceptance criteria met; Go tests pass with project coverage thresholds |

## Steps

### Step 0: Design and discovery

- [ ] Read issue #1 and the files listed in Context to Read First
- [ ] Confirm current behavior: custom `ExcludePatterns` **replaces** defaults (not merges); document the intended merge/replace change in STATUS Discoveries
- [ ] Choose `STET_EXCLUDE_PATTERNS` convention (comma-separated list, consistent with existing env style)
- [ ] Identify all `scope.Partition` call sites that must receive resolved `*diff.Options`

### Step 1: Config, env, and diff merge semantics

- [ ] Add `exclude_patterns` and `exclude_patterns_replace` to `config.Config` and `config.Overrides`
- [ ] Load from repo `.review/config.toml` and global config; apply env `STET_EXCLUDE_PATTERNS` and `STET_EXCLUDE_PATTERNS_REPLACE` (or equivalent bool env) per existing config conventions
- [ ] Precedence: CLI > env > repo config > global config > built-in defaults
- [ ] Update `diff` layer so user patterns **merge** with `defaultExcludePatterns` unless replace mode is set; empty user list with replace=false keeps defaults
- [ ] Add unit tests in `config_test.go` and `diff_test.go` for precedence, merge, and replace

### Step 2: CLI flags and run wiring

- [ ] Add repeatable `--exclude PATTERN` and `--no-exclude` on `stet start`, `stet run`, and `stet rerun` (match existing flag patterns in `main.go`)
- [ ] Build resolved `*diff.Options` from config + overrides and pass to every `scope.Partition` call in `run.go` (and any other review entry points in scope)
- [ ] Incremental review: excluded hunks are omitted from review (not approved); when exclusions are lifted, matching hunks in `baseline..HEAD` become to-review on the next `stet run`
- [ ] Add integration coverage via `stet start --dry-run` (or equivalent) showing excluded paths are skipped

### Step 3: Observability and documentation

- [ ] Log skipped file paths to stderr when exclusions apply (at least in verbose/trace mode; align with existing progress/trace patterns)
- [ ] Document config keys, env vars, flags, precedence, and merge semantics in `docs/cli-extension-contract.md` and `docs/review-process-internals.md`
- [ ] Do **not** implement out-of-scope follow-ups from issue #1 (`include_patterns`, named review modes, extension UI, full `**` glob)

### Step 4: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Run full CLI test suite: `cd cli && go test ./... -cover`
- [ ] Verify per-file coverage meets project minimum (72% per file, 77% overall per `AGENTS.md`)
- [ ] Fix all failures introduced by this task

## Documentation Requirements

**Must Update:**

- `docs/cli-extension-contract.md` — new config/env/CLI surface for exclude patterns
- `docs/review-process-internals.md` — file-level filter behavior, merge semantics, incremental review

**Check If Affected:**

- `README.md` — only if user-facing config examples are expected there

## Completion Criteria

- [ ] All steps complete
- [ ] Issue #1 acceptance criteria satisfied:
  - [ ] `exclude_patterns` in config skips matching hunks from LLM review
  - [ ] CLI `--exclude` and `--no-exclude` override config per run
  - [ ] Built-in exclusions still apply when using user excludes (merge)
  - [ ] `stet run` after lifting exclusions reviews previously skipped doc hunks
  - [ ] Skipped files reported on stderr (at least in verbose mode)
  - [ ] Documented in contract and review-process-internals docs
  - [ ] Unit tests in `cli/internal/diff` and integration test via `stet start --dry-run`
- [ ] All tests passing with coverage thresholds met

## Git Commit Convention

All commits for this task MUST include the task ID:

- **Implementation:** `feat(SP-001): description`
- **Bug fixes:** `fix(SP-001): description`
- **Tests:** `test(SP-001): description`
- **Checkpoints:** `checkpoint: SP-001 description`

## Do NOT

- Expand scope beyond issue #1 (no `include_patterns`, review modes, extension UI, or `**` glob)
- Skip the Testing step
- Modify `extension/**` or spine orchestration files
- Load docs not listed in Context to Read First
- Commit without the task ID prefix

---

## Amendments (Added During Execution)
