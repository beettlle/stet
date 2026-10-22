# Task: SP-062 — `main_test.go`: one output-capture helper that drains and closes pipes

**Created:** 2026-10-22
**Size:** M

## Review Level: 1

**Assessment:** Test-only refactor in `cli/cmd/stet/`: one new helper file and a mechanical migration of about 37 `os.Pipe()` captures in one 4,300-line test file. No production code.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 0

## Mission

Closes #29

About 37 tests in `cli/cmd/stet/main_test.go` capture stdout or stderr with `os.Pipe()` (example: `TestRunCLI_benchmarkOpenAILabelsEndToEnd`):

```go
r, w, err := os.Pipe()
os.Stdout = w
got := runCLI([]string{"benchmark"})
_ = w.Close()
_, _ = io.Copy(&stdout, r)
```

Two problems:

1. The read end is closed in only about 7 of them, so each test leaks a file descriptor for the rest of the `go test` process.
2. The pipe is read only after `runCLI` returns. A command that writes more than the OS pipe buffer (64 KiB) blocks forever, and the test hangs until the `go test` timeout.

Fix:

1. Add `cli/cmd/stet/capture_test.go` with one helper, for example `captureOutput(t *testing.T, fn func()) (stdout, stderr string)`. It creates both pipes, swaps `os.Stdout` and `os.Stderr`, drains each read end in a goroutine **while** `fn` runs, and on return (use `defer`, so `t.Fatal` / panics inside `fn` also restore) restores the originals, closes the write ends, waits for the drain goroutines, and closes the read ends. Document in its comment that it must not be used from parallel tests (it swaps process-wide `os.Stdout` / `os.Stderr`).
2. Rewrite `runCLICapture` (~line 610) on top of the helper; keep its signature and its `getFindingsOut` handling.
3. Migrate every remaining `os.Pipe()` capture in `main_test.go` to the helper (or to `runCLICapture`). Keep each test's assertions unchanged.

## Dependencies

- **Task:** SP-061 (also edits `cli/cmd/stet/main_test.go`; its new benchmark test is migrated here too)

## Context to Read First

**Tier 3 (reference docs):**

- `cli/cmd/stet/main_test.go` — `runCLICapture` (~line 610) and all `os.Pipe()` sites (`rg -n 'os\.Pipe\(\)' cli/cmd/stet/main_test.go`)

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** None

## File Scope

- `cli/cmd/stet/capture_test.go`
- `cli/cmd/stet/main_test.go`

## Documentation Requirements

**Must Update:** none.

**Check If Affected:** none.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/cmd/stet/... -count=1 -cover` |
| fileScopeMustChange | `cli/cmd/stet/capture_test.go`, `cli/cmd/stet/main_test.go` |
| fileScopeMustNotChange | `cli/cmd/stet/main.go`, `extension/**`, `.review/**` |
| completionCriteria | A helper test writes more than 128 KiB to stdout through the helper and completes without hanging; `rg -n 'os\.Pipe\(\)' cli/cmd/stet/main_test.go` returns nothing; `os.Pipe()` appears only in `cli/cmd/stet/capture_test.go`; `go test ./cli/cmd/stet/... -count=1 -cover` passes and `cli/cmd/stet` coverage stays at or above its pre-change value |

## Steps

### Step 1: Capture helper

- [ ] `capture_test.go` helper drains while `fn` runs, restores via `defer`, closes all four pipe ends
- [ ] Helper tests: >128 KiB stdout completes; stdout and stderr captured separately; `os.Stdout` / `os.Stderr` restored afterwards
- [ ] `runCLICapture` rewritten on the helper
- [ ] `spine_review_step` after the step

### Step 2: Migrate captures in main_test.go

- [ ] Every `os.Pipe()` site in `main_test.go` migrated; assertions unchanged
- [ ] Unused imports removed (`io`, etc.) only where no longer used
- [ ] `spine_review_step` after the step

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Record `cli/cmd/stet` coverage before the change (`go test ./cli/cmd/stet/... -count=1 -cover` on the base) and after; after ≥ before
- [ ] `rg -n 'os\.Pipe\(\)' cli/cmd/stet/main_test.go` returns nothing
- [ ] Run Contract `testCommand`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Tests:** `test(SP-062): description`

## Do NOT

- Change `cli/cmd/stet/main.go` or any production code
- Change what a migrated test asserts, or mark tests parallel
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
