**Current Step:** Complete
**Status:** Done
**Last Updated:** 2026-10-08
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 1: Config and CLI

**Status:** Complete

- [x] `rules_annotate` key + env + tests
- [x] Shared client built when annotation is on; passed to start/run/rerun

## Step 2: Pipeline and docs

**Status:** Complete

- [x] Annotation after critic; fail-open single warning
- [x] Contract doc

## Step 3: Testing & Verification

**Status:** Complete

- [x] Default-off, enabled, error, dry-run tests
- [x] CLI client tests
- [x] Contract `testCommand`, full suite, coverage gate

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| 2026-10-08 | 1 | plan | skipped (in-worker reviewer spawn blocked; engine-owned after .DONE, SP-195) |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-10-08 | Wiring split from SP-027 library; gated on SP-028 validation set | Last wave |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-10-08 | Authored | From SP-020 follow-up table |
| 2026-10-08 | Step 1 done | `rules_annotate` TOML/env/override + tests; annotation client shares one `*systemone.Client` across critic/selector/annotator; wired into start/run/rerun |
| 2026-10-08 | Step 2 done | `annotateRuleConflictsForBatch` after critic in both pipeline branches (SP-020 caps: ≤32 questions/request, >64 findings skipped); one fail-open warning per run naming server URL, never the key; contract doc updated (config row, API-key note, `rule_conflict` stdout field, history verbatim note) |
| 2026-10-08 | Step 3 done | Contract testCommand PASS (vet + config 82.4% / run 82.2% / cmd 76.1%); full `go test ./...` PASS; coverage gate PASS (project 83.0%, all files ≥ 72%); extension `npm test` PASS + coverage guard PASS (extension untouched) |

## Backlog / Tech Debt

- Extension UI for `rule_conflict` (display the annotation) — not in this task's file scope; contract doc notes "the extension does not display it yet".

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
