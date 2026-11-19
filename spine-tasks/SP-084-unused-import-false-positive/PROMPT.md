# Task: SP-084 — unused-import-false-positive

## Mission
Closes #39

stet v1.4.0 raised two false-positive findings during a review:
1. An "unused import" existence claim wasn't filtered because the use was in another hunk, but still in the same file. The existence filter `cli/internal/findings/existence.go` must correctly see uses across the entire file and drop the finding.
2. The critic misread the callee's return value when the callee is imported. We must improve the critic's ability to avoid ERROR severity for behavior of imported functions without evidence.

Fix the existence filter to drop unused-import claims when the symbol is used anywhere in the file. Address the callee-return error by ensuring the critic avoids ERROR without evidence from the callee's body.

## Dependencies

- **None**

## Context to Read First

- None

## File Scope

- `cli/internal/findings/existence.go`
- `cli/internal/findings/existence_test.go`
- `cli/internal/review/critic_systemone.go`
- `cli/internal/review/critic_systemone_test.go`

## Contract

| Field | Value |
|-------|-------|
| `testCommand` | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/findings/... ./cli/internal/review/... -count=1` |
| `fileScopeMustChange` | `cli/internal/findings/existence.go`, `cli/internal/review/critic_systemone.go` |
| `fileScopeMustNotChange` | `.review/**`, `spine-tasks/**` |

## Steps

### Step 0: Fix existence claim filter for unused imports
**Plan-review checkpoint**
Update `cli/internal/findings/existence.go` to ensure that "unused import" existence claims are filtered out if the imported symbol is used anywhere in the file, regardless of hunk boundaries. Update `existenceClaimDisproven` or related logic. Add test cases in `existence_test.go` matching the issue description.

> **Review override: code review**

### Step 1: Adjust critic criteria for imported callee returns
Update `cli/internal/review/critic_systemone.go` (or related system one prompts) so the critic does not emit ERROR severity for claims about a callee's return value if that callee's body is not in the context.

### Step 2: Testing & Verification
Verify that the `testCommand` passes. Ensure `go test` covers the new existence filter test cases and critic behavior test cases.

## Completion Criteria

- [ ] Unused import claims are correctly dropped if the symbol is used elsewhere in the file.
- [ ] Critic correctly avoids raising ERROR for imported callee return claims without evidence.
- [ ] Tests pass via the contract test command.

## Do NOT
- Do NOT edit `.spine/`, `AGENTS.md`, `CLAUDE.md`, or `.gitnexus/`.

## Git Commit Convention
Prefix commits with `feat(SP-084): `, `fix(SP-084): `, or `test(SP-084): ` at step boundaries.
