**Current Step:** Complete
**Status:** Complete
**Last Updated:** 2026-07-01
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

## Step 0: Design precedence and merge semantics

**Status:** Complete

- [x] Read PRD and current SystemPrompt flow
- [x] Record merge model in Discoveries
- [x] Decide optional config path key

## Step 1: Implement loader and precedence

**Status:** Complete

- [x] Repo prompt resolution
- [x] Precedence integration
- [x] Fail-open on missing file

## Step 2: Tests and documentation

**Status:** Complete

- [x] Unit tests for precedence cases
- [x] Contract doc update

## Step 3: Testing and verification

**Status:** Complete

- [x] Run test command
- [x] Coverage check (prompt 90.3%, run 70.1%, config 65.5%)

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| | | | |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-07-01 | **Merge model:** append — repo `.review/prompt.md` is appended to `DefaultSystemPrompt` under `## Repository review instructions`; optimized file replaces all. | Keeps JSON schema and review steps from default while allowing repo-specific rules. |
| 2026-07-01 | **Path:** always `repoRoot/.review/prompt.md`, not under custom `state_dir`. | Matches PRD; teams with custom state_dir still use repo-local prompt file. |
| 2026-07-01 | **No `prompt_file` config key** for this task — fixed path per PRD. | Simpler scope; can add later if needed. |
| 2026-07-01 | **`SystemPrompt(stateDir, repoRoot)`** signature extended with `repoRoot`. | Callers in `run.go` and `review.go` updated. |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-07-01 | Step 0–2 | Implemented loader, tests, contract docs |
| 2026-07-01 | Step 3 | `go test ./internal/prompt/... ./internal/config/... ./internal/run/...` pass; extension npm test pass |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |
