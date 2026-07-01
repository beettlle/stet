# Task: SP-003 — Per-repo prompt override (`.review/prompt.md`)

**Created:** 2026-06-30
**Size:** M

## Review Level: 2

**Assessment:** Config and prompt-build path change; clear precedence rules but touches review entry points.
**Score:** 4/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 2

## Mission

Implement PRD-configured **per-repo prompt override**: when `.review/prompt.md` exists in the repo, merge or override the default system prompt for review runs. Precedence must be documented and tested:

**optimized prompt (`system_prompt_optimized.txt`) > repo prompt (`.review/prompt.md`) > default system prompt** — or document an alternate merge if PRD/contract specifies append semantics; record choice in STATUS Discoveries.

## Dependencies

- **None**

## Context to Read First

- `docs/PRD.md` — config keys section (`.review/prompt.md`)
- `cli/internal/prompt/prompt.go` — `SystemPrompt`, `optimizedPromptFilename`, `DefaultSystemPrompt`
- `cli/internal/review/review.go` — system prompt assembly
- `cli/internal/run/run.go` — where `SystemPrompt(stateDir)` is loaded
- `docs/cli-extension-contract.md`

## Environment

- **Workspace:** stet monorepo (`cli/` Go module)

## File Scope

- `cli/internal/prompt/prompt.go`
- `cli/internal/prompt/prompt_test.go`
- `cli/internal/config/config.go` (optional `prompt_file` key if relative path override needed)
- `cli/internal/config/config_test.go`
- `cli/internal/run/run_test.go` (if wiring tests needed)
- `docs/cli-extension-contract.md`
- `docs/PRD.md` (precedence note only if missing)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./internal/prompt/... ./internal/config/... ./internal/run/... -count=1` |
| fileScopeMustChange | `cli/internal/prompt/prompt.go`, `cli/internal/prompt/prompt_test.go` |
| fileScopeMustNotChange | `extension/**`, `scripts/**` |
| completionCriteria | `.review/prompt.md` loaded when present; precedence vs optimized/default tested and documented |

## Steps

### Step 0: Design precedence and merge semantics

- [ ] Read PRD and existing `SystemPrompt` / optimized file behavior
- [ ] Choose merge model (replace vs append section) and document in STATUS Discoveries
- [ ] Decide optional config key for alternate path (default: `.review/prompt.md` under repo root)

### Step 1: Implement loader and precedence

- [ ] Add repo-root prompt file resolution (via state dir / git repo root)
- [ ] Integrate into `SystemPrompt` or dedicated helper with explicit precedence
- [ ] Fail open: missing or unreadable repo prompt → fall back without error

### Step 2: Tests and documentation

- [ ] Unit tests: default only, repo prompt only, optimized wins, repo + default merge
- [ ] Document keys, path, and precedence in `docs/cli-extension-contract.md`

### Step 3: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Run Contract `testCommand`
- [ ] Verify coverage thresholds per `AGENTS.md` for touched files

## Completion Criteria

- [ ] All steps complete
- [ ] `.review/prompt.md` override works with documented precedence
- [ ] Tests pass

## Git Commit Convention

- **Implementation:** `feat(SP-003): description`

## Do NOT

- Change optimizer sidecar behavior (SP-002)
- Modify extension code

---

## Amendments (Added During Execution)
