# SP-062: `main_test.go`: one output-capture helper that drains and closes pipes — Status

**Current Step:** Step 3
**Status:** 🔄 In Progress — Step 2 complete
**Last Updated:** 2026-10-23
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

### Step 1: Capture helper
**Status:** ✅ Complete

- [x] Helper drains while running, restores via `defer`, closes all pipe ends
- [x] Helper tests (>128 KiB, separate streams, restore)
- [x] `runCLICapture` uses the helper

---

### Step 2: Migrate captures in main_test.go
**Status:** ✅ Complete

- [x] All `os.Pipe()` sites in `main_test.go` migrated; assertions unchanged

Migrated (34 test sites + 2 helpers): `TestRunCLI_doctorUnreachableExits2`, `TestRunCLI_benchmarkWithMockReturnsZero`, `TestRunCLI_benchmarkOpenAILabelsEndToEnd`, `TestRunCLI_benchmarkUsesRepoConfig`, `TestRunCLI_skillPrintsToStdout`, `TestRunCLI_startFromNonGitStderrHumanReadable`, `TestRunCLI_startDryRunExcludePatterns`, `TestRunCLI_strictnessInvalidExits1`, `TestRunCLI_rerunNoSessionFails`, `TestRunCLI_startBaselineNotAncestorPrintsClearMessage`, `TestRunCLI_startInvalidBaselineRefPrintsHint`, `TestRunCLI_startConcurrentLockPrintsClearMessage`, `TestRunCLI_startAllowDirtyProceedsWithWarning`, `TestRunCLI_statusNoSessionExitsNonZero`, `TestRunCLI_statusWithSessionPrintsFields`, `TestRunCLI_statusWithSessionPrintsSessionOptions`, `TestRunCLI_statusWithIdsPrintsFindingsWithIds`, `TestRunCLI_statusWithoutIdsUnchanged`, `TestRunCLI_listPrintsFindingsWithIds`, `TestRunCLI_listNoSessionExitsNonZero`, `TestRunCLI_listEmptyFindings`, `TestRunCLI_dismissPersistence`, `TestRunCLI_statusActiveCountAfterPartialDismiss`, `TestRunCLI_dismissByShortPrefix`, `TestRunCLI_optimizeNotConfiguredExitsNonZero`, `TestRunCLI_statsVolumeJSONInRepoWithNote`, `TestRunCLI_statsVolumeJSONWithGitAI`, `TestRunCLI_statsVolumeHumanWithGitAI`, `TestRunCLI_statsQualityJSONInRepo`, `TestRunCLI_statsEnergyJSONInRepo`, `TestPrintLLMUnreachable_timeoutHint`, `TestRunCLI_commitMsgPrintsSuggestion`, `TestPrintLLMUnreachable_emptyBaseURL`, `TestRunCLI_startSystemOneCriticDropsLowScoredFinding`; helpers `runCLICapture` (Step 1) and `runCLIWithOutput` (now takes `t *testing.T`; 16 call sites updated).

- `TestRunCLI_skillPrintsToStdout` lost its `t.Parallel()`: `captureOutput` documents that stream swapping is process-wide, so a parallel test must not use it (assertions unchanged; removal required by the helper contract — the reverse of the prohibited "mark tests parallel").

---

### Step 3: Testing & Verification
**Status:** ⬜ Not Started

- [ ] Coverage before/after recorded; after ≥ before
- [ ] No `os.Pipe()` in `main_test.go`
- [ ] Contract `testCommand` passing

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| Initial helper returned `stdoutBuf.String()` at a plain `return`, which evaluates before the deferred `drains.Wait()` — captured output was truncated (lost exactly what was still in flight). Fixed with named returns assigned inside the deferred cleanup after drains finish. | Fixed in `captureOutput` | `cli/cmd/stet/capture_test.go` |
| `os.Pipe()` also exists in `cli/internal/git/ignore_test.go` and `cli/internal/run/*_test.go` — outside this task's File Scope, left untouched (the "os.Pipe only in capture_test.go" criterion is met within `cli/cmd/stet`). | Logged | — |
| `TestRunCLI_skillPrintsToStdout` was `t.Parallel()` while swapping `os.Stdout`; un-marked it (assertions unchanged) because `captureOutput` must not be used from parallel tests. | Required by helper contract | `main_test.go` |
