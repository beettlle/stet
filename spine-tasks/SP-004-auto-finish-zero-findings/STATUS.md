**Current Step:** Complete
**Status:** Complete
**Last Updated:** 2026-07-01
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

## Step 0: Design opt-in default and triggers

**Status:** Complete

- [x] Default and config keys — opt-in: `auto_finish_zero_findings = false`; enable via config `true` or `STET_AUTO_FINISH_ZERO=1`; CLI flag `--auto-finish-zero`
- [x] Trigger points — end of `stet start`, `stet run`, `stet rerun`, and `stet commitmsg --commit-and-review` when active findings count is 0
- [x] Dry-run policy — excluded (dry-run is CI/dev mode; session left open)

## Step 1: CLI auto-finish path

**Status:** Complete

- [x] Config/env/flag
- [x] Finish helper invocation (`run.MaybeAutoFinish`, `tryAutoFinishZero` in main)
- [x] CLI tests

## Step 2: Extension behavior

**Status:** Complete

- [x] Zero-findings auto-finish in extension (`maybeAutoFinishAfterReview`, `--auto-finish-zero` passthrough)
- [x] Explicit finish when findings remain (unchanged `stet.finishReview` command)
- [x] Extension tests

## Step 3: Testing and verification

**Status:** Complete

- [x] Full test command (`go test` + `npm run compile` + `npm test` — all pass)
- [x] Contract doc update (`docs/cli-extension-contract.md`)

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| | | | |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-07-01 | Active findings = session findings minus dismissed IDs; auto-finish must use that count, not raw stream count | Extension checks `stet list` / CLI uses `ActiveFindingCount` |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-07-01 | Step 0 | Design locked: opt-in default, dry-run excluded |
| 2026-07-01 | Step 1–3 | Implementation and tests complete |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |
