# Task: SP-086 — critic-missing-code-claims

## Mission
Closes #20

System One critic accepts claims that something is missing or undefined outside the shown code. Tune criteria and check the threshold.
Prior effort in SP-080 attempted this but no wording passed the evaluation. Now we need to revisit the prompt/criteria for the System One critic to successfully reject out-of-view "missing / undefined" claims.

## Dependencies

- **None**

## Context to Read First

- `docs/research/systemone-rules-compliance.md`

## File Scope

- `cli/internal/review/critic_systemone.go`
- `cli/internal/review/critic_systemone_test.go`
- `cli/internal/criticeval/sweep.go`
- `cli/internal/criticeval/dataset.go`

## Contract

| Field | Value |
|-------|-------|
| `testCommand` | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/review/... ./cli/internal/criticeval/... -count=1` |
| `fileScopeMustChange` | `cli/internal/review/critic_systemone.go` |
| `fileScopeMustNotChange` | `.review/**`, `spine-tasks/**` |

## Steps

### Step 0: Tune System One criteria for out-of-view claims
**Plan-review checkpoint**
Update `cli/internal/review/critic_systemone.go` to adjust the evaluation prompt/criteria. The critic must accurately reject existence/undefined claims if the relevant code is outside the provided context (e.g., imported modules or unshown files). 

> **Review override: code review**

### Step 1: Evaluate against dataset
Use or update `cli/internal/criticeval/sweep.go` to ensure the new criteria successfully passes the test cases where out-of-view claims should be rejected without regressing valid in-view missing-code claims.

### Step 2: Testing & Verification
Ensure `go test` passes. Add or update unit tests in `critic_systemone_test.go` to cover the out-of-view rejection logic.

## Completion Criteria

- [ ] System One prompt/criteria is tuned to reject out-of-view existence claims.
- [ ] Evaluation demonstrates improvement over baseline.
- [ ] Tests pass via the contract test command.

## Do NOT
- Do NOT edit `.spine/`, `AGENTS.md`, `CLAUDE.md`, or `.gitnexus/`.

## Git Commit Convention
Prefix commits with `feat(SP-086): ` at step boundaries.
