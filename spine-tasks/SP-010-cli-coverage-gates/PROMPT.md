# Task: SP-010 — Raise CLI coverage to AGENTS.md gates

**Created:** 2026-07-01
**Size:** L

## Review Level: 2

**Assessment:** Test-only and targeted production-path coverage across many CLI packages; no feature change but broad blast radius.
**Score:** 4/8 — Blast radius: 2, Pattern novelty: 0, Security: 0, Reversibility: 2

## Mission

Close the **pre-existing** CLI coverage gap identified during SP-001 verification: `scripts/check-coverage.sh` must **PASS** (project ≥ 77% line coverage, every file ≥ 72%) after `go test ./... -coverprofile=coverage.out` in `cli/`.

Do not lower thresholds in `AGENTS.md` or `scripts/check-coverage.sh`. Prefer meaningful tests on error paths and business logic over trivial line-hitting.

## Dependencies

- **None**

## Context to Read First

- `AGENTS.md` — coverage requirements (77% project, 72% per file)
- `scripts/check-coverage.sh` — gate script
- SP-001 final review (`.reviews/final-20260701T233730.md`) — baseline ~71.6% project gap
- Packages below threshold as of 2026-07-01 verification: `cmd/stet` (~64%), `internal/config` (~66%), `internal/run` (~70%), `internal/ollama` (~55%), `internal/openaicompat` (~37%), `internal/rag/go` (~47%), others per baseline report

## Environment

- **Workspace:** `cli/` Go module

## File Scope

- `cli/**/*_test.go` (new or extended tests only)
- `scripts/check-coverage.sh` (only if gate script has a bug; do not weaken thresholds)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./... -coverprofile=coverage.out && bash ../scripts/check-coverage.sh coverage.out` |
| fileScopeMustChange | `cli/internal/run/run_test.go` |
| fileScopeMustNotChange | `extension/**`, `scripts/optimize.py` |
| completionCriteria | `scripts/check-coverage.sh` PASS on full CLI coverprofile; no production behavior change |

## Steps

### Step 0: Baseline and target list

- [ ] Run `cd cli && go test ./... -coverprofile=coverage.out && bash ../scripts/check-coverage.sh coverage.out`; capture FAIL output
- [ ] List packages/files under 72% and project total; prioritize by gap size and review criticality (`run`, `config`, `review`, LLM backends)
- [ ] Record baseline in STATUS Discoveries

### Step 1: Core pipeline packages

- [ ] Add tests for `internal/run`, `internal/config`, `internal/review` until each file ≥ 72%
- [ ] Re-run check-coverage; note remaining failures

### Step 2: LLM and RAG backends

- [ ] Add tests for lowest packages (`internal/ollama`, `internal/openaicompat`, `internal/rag/*` as needed) until per-file ≥ 72%
- [ ] Re-run check-coverage after each package group

### Step 3: cmd/stet and project total

- [ ] Raise `cmd/stet` coverage (integration-style tests where unit tests are impractical)
- [ ] Confirm project total ≥ 77% and all files ≥ 72%

### Step 4: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Run Contract `testCommand` — must exit 0
- [ ] Confirm no production code changes except test hooks if absolutely required (document in STATUS)

## Completion Criteria

- [ ] All steps complete
- [ ] `scripts/check-coverage.sh` PASS on full CLI coverprofile
- [ ] No threshold weakening

## Git Commit Convention

- **Implementation:** `test(SP-010): description`

## Do NOT

- Lower 77%/72% thresholds
- Add coverage via excluded generated files or `//nolint` without tests
- Change review or exclude-pattern behavior (SP-001 scope)
