# Task: SP-087 — batch-git-grep-rag

## Mission
Closes #15

RAG symbol lookup currently runs up to 30 sequential `git grep` processes per hunk. This is slow and scales poorly. Batch the lookups into a single `git grep` call or significantly reduce the number of spawned processes.

## Dependencies

- **None**

## Context to Read First

- None

## File Scope

- `cli/internal/rag/symbol.go`
- `cli/internal/rag/symbol_test.go`
- `cli/internal/rag/grep.go`
- `cli/internal/rag/grep_test.go`
- `cli/internal/git/git.go`

## Contract

| Field | Value |
|-------|-------|
| `testCommand` | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/rag/... ./cli/internal/git/... -count=1` |
| `fileScopeMustChange` | `cli/internal/rag/symbol.go`, `cli/internal/git/git.go` |
| `fileScopeMustNotChange` | `.review/**`, `spine-tasks/**` |

## Steps

### Step 0: Batch git grep RAG queries
**Plan-review checkpoint**
Update `cli/internal/git/git.go` and `cli/internal/rag/` (likely `symbol.go` or `grep.go`) to batch search patterns. Instead of invoking `git grep` in a loop for each symbol, construct a combined regex query (`git grep -e "pattern1" -e "pattern2" ...` or similar) to fetch definitions in a single call.

> **Review override: code review**

### Step 1: Parse batched results
Adapt the output parser to map the batched `git grep` output back to the respective symbols so the RAG context continues to map definitions to the correct queried symbol.

### Step 2: Testing & Verification
Ensure `go test` passes. Add tests verifying that batched queries return the identical mapped definitions as the sequential version, and that process count is minimized.

## Completion Criteria

- [ ] RAG symbol lookup issues a single (or very few) `git grep` calls instead of up to 30.
- [ ] Definitions are correctly parsed and mapped back to the symbols.
- [ ] Tests pass via the contract test command.

## Do NOT
- Do NOT edit `.spine/`, `AGENTS.md`, `CLAUDE.md`, or `.gitnexus/`.

## Git Commit Convention
Prefix commits with `feat(SP-087): ` or `refactor(SP-087): ` at step boundaries.
