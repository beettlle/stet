**Current Step:** Step 0: Not started
**Status:** Ready
**Last Updated:** 2026-09-30
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

## Step 0: Confirm labeling against real data

**Status:** Not Started

- [ ] Inspect real history/session schema and confirm label rules
- [ ] Record rule changes in Discoveries

> ⚠️ Hydrate: adjust Step 1 checkboxes if label sources differ from PROMPT.

## Step 1: Dataset and scoring (`criticeval` package)

**Status:** Not Started

- [ ] Load labeled examples grouped by hunk
- [ ] Score with `VerifyFindingsSystemOne`; latency per request
- [ ] Threshold sweep, percentiles, recommendation

## Step 2: `stet critic-eval` subcommand

**Status:** Not Started

- [ ] Command and flags
- [ ] Human and JSON output; empty-data error

## Step 3: Testing and verification

**Status:** Not Started

- [ ] Fixture-based exact sweep tests
- [ ] Edge cases per PROMPT
- [ ] `main_test.go` command test
- [ ] Contract `testCommand` and full `go test ./... -cover`
- [ ] `criticeval.go`, `main.go` coverage ≥ 72%

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| | | | |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-09-30 | History stores `prompt_context` only for dismissed findings; positives must come from `session.json` `finding_prompt_context` or `--dataset` | Label sources in PROMPT reflect this (authoring-time finding) |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-09-30 | Authored | Packet created |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
