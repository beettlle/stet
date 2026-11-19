# Task: SP-085 — pass-task-intent

## Mission
Closes #35

Pass task intent into the review and critic via a new flag (e.g., `stet run --intent-file`). This provides the mission or PR description so the reviewer and critic know the objective of the changes. 

Add the flag to `cli/cmd/stet/main.go`, read its contents, and thread it down through `run.go`, `review.go`, and into the prompts for both the LLM reviewer and the System One critic.

## Dependencies

- **None**

## Context to Read First

- None

## File Scope

- `cli/cmd/stet/main.go`
- `cli/cmd/stet/main_test.go`
- `cli/internal/run/run.go`
- `cli/internal/run/run_test.go`
- `cli/internal/review/review.go`
- `cli/internal/review/critic_systemone.go`
- `cli/internal/review/critic_systemone_test.go`

## Contract

| Field | Value |
|-------|-------|
| `testCommand` | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/cmd/stet/... ./cli/internal/run/... ./cli/internal/review/... -count=1` |
| `fileScopeMustChange` | `cli/cmd/stet/main.go`, `cli/internal/run/run.go`, `cli/internal/review/review.go` |
| `fileScopeMustNotChange` | `.review/**`, `spine-tasks/**` |

## Steps

### Step 0: Add `--intent-file` CLI flag
Update `cli/cmd/stet/main.go` to add `--intent-file`. If provided, read the file's content and pass it into the run options.

### Step 1: Thread intent through run and review
Update `cli/internal/run/run.go` to accept the intent string. Pass it to the reviewer (`cli/internal/review/review.go`).

### Step 2: Update Reviewer and Critic prompts
**Plan-review checkpoint**
Update the prompt construction for the reviewer to include the intent if present. Similarly, pass the intent to the critic (`cli/internal/review/critic_systemone.go`) so System One evaluation takes the objective into account.

> **Review override: code review**

### Step 3: Testing & Verification
Ensure `go test` passes. Add tests in `main_test.go` and `run_test.go` validating that the intent is correctly passed and included in requests.

## Completion Criteria

- [ ] `--intent-file` flag is added.
- [ ] Intent is threaded to reviewer and critic.
- [ ] Prompts conditionally include the intent context.
- [ ] Tests pass via the contract test command.

## Do NOT
- Do NOT edit `.spine/`, `AGENTS.md`, `CLAUDE.md`, or `.gitnexus/`.

## Git Commit Convention
Prefix commits with `feat(SP-085): ` at step boundaries.
