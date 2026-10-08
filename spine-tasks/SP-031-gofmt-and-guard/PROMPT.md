# Task: SP-031 — gofmt the CLI and guard it in the build script

**Created:** 2026-10-08
**Size:** S

## Review Level: 1

**Assessment:** Mechanical formatting across many files plus a one-line build guard; no logic changes.
**Score:** 2/8 — Blast radius: 2, Pattern novelty: 0, Security: 0, Reversibility: 0

## Mission

Bring every Go file under `cli/` to `gofmt` formatting and make `scripts/build-all.sh` (the spine `testing.build` command) fail when any Go file is unformatted, so formatting drift is caught at the integrate gate.

On 2026-10-08 `gofmt -l cli/` listed 35 files (including `cli/cmd/stet/main.go` and `main_test.go` from SP-023's struct-literal alignment, plus long-standing drift in `benchmark`, `findings`, `history`, `ollama`, `openaicompat`, `prompt`, `rag/*`, `review`, `rules`, `session`, `stats`). Re-run `gofmt -l` at task start — the list may differ after earlier waves.

**Guard requirements:**

- New `scripts/check-gofmt.sh` (executable, `set -euo pipefail`, resolves the repo root like `build-all.sh`): runs `gofmt -l .` inside `cli/`; if output is non-empty, prints `gofmt: unformatted Go files:` followed by the list and exits non-zero; otherwise exits 0 silently.
- `scripts/build-all.sh` calls `scripts/check-gofmt.sh` before the Go build. Keep its existing build steps unchanged.

## Dependencies

- **Task:** SP-029 (edits `cli/internal/rules/rules.go`; format after it lands)
- **Task:** SP-030 (edits many `_test.go` files; format after it lands)
- **Task:** SP-032 (edits `cli/internal/systemone/client.go`; format after it lands)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md` — tech-debt row "`gofmt` in `cli/cmd/stet/`"

**Tier 3 (reference docs):**

- `scripts/build-all.sh`

## Environment

- **Workspace:** repo root and `cli/` Go module
- **Services required:** none

## File Scope

- `cli/**/*.go` (formatting only)
- `scripts/check-gofmt.sh` (new)
- `scripts/build-all.sh`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && grep -q check-gofmt.sh scripts/build-all.sh && bash -n scripts/build-all.sh && cd cli && go build ./... && go vet ./...` |
| fileScopeMustChange | `scripts/check-gofmt.sh`, `scripts/build-all.sh`, `cli/cmd/stet/main.go` |
| fileScopeMustNotChange | `extension/**`, `docs/**`, `go.mod`, `go.sum` |
| completionCriteria | `gofmt -l` under `cli/` prints nothing; `build-all.sh` fails on unformatted files; diff is whitespace/formatting only (`git diff -w --stat` on Go files shows no semantic changes); full Go test suite passes |

## Steps

### Step 1: Format and guard

- [ ] Run `gofmt -w .` inside `cli/`; confirm `git diff -w` on Go files is empty or only gofmt-equivalent rewrites (no hand edits)
- [ ] Add `scripts/check-gofmt.sh` and call it from `scripts/build-all.sh` per Mission
- [ ] Prove the guard: temporarily mis-indent a scratch `.go` file under `cli/`, confirm `scripts/check-gofmt.sh` fails with the file listed, then delete the scratch file (record in STATUS; do not commit it)

### Step 2: Testing & Verification

> ZERO test failures allowed.

- [ ] Run Contract `testCommand`
- [ ] Run `cd cli && go test ./... -count=1 -cover`
- [ ] Coverage gate: `cd cli && go test ./... -coverprofile=../coverage.out -count=1 && cd .. && bash scripts/check-coverage.sh coverage.out` (then `git checkout -- coverage.out`)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Formatting:** `style(SP-031): gofmt cli`
- **Guard:** `build(SP-031): fail build on unformatted Go files`

## Do NOT

- Make any non-formatting change to Go code
- Change `scripts/test-all.sh`, `scripts/coverage-all.sh`, or `.spine/spine-config.json`
- Run `goimports` or other tools that reorder imports beyond `gofmt`
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
