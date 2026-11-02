# SP-077: Wave review shows only findings from the reviewed session — Status

**Current Step:** 4 (Testing & Verification)
**Status:** 🔄 In Progress
**Last Updated:** 2026-11-01
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Reproduce and find the root cause
**Status:** ✅ Done

- [x] Dry-run repro (clone and worktree, disjoint diff) with leaked count recorded
- [x] Root cause recorded (file and function)

---

### Step 2: Fix
**Status:** ✅ Done

- [x] Fix at the root cause — `stet list` / `stet start --json` / `stet status` now filter findings to the session's reviewed diff (`cli/cmd/stet/main.go`: `sessionReviewedFiles`, `filterToSessionDiff`)

---

### Step 3: Regression test and CONTEXT note
**Status:** ✅ Done

- [x] Cross-session leak regression test (`TestRunCLI_listHidesFindingsFromOtherSessionDiff` + `TestFilterToSessionDiff_*`)
- [x] `spine-tasks/CONTEXT.md` #5 row marked resolved

---

### Step 4: Testing & Verification
**Status:** 🔄 In Progress

- [ ] Completion-criteria tests
- [x] Repro shows 0 leaked findings (dry-run shell repro, pre-fix 1 leaked → post-fix 0)
- [ ] Contract `testCommand` passes

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| Dry-run repro (2026-11-01, `$TMPDIR/sp077-repro.uygEod`): origin repo with disjoint branches side-a (fileA.txt) / side-b (fileB.txt). Leg 1 = harness analog: `git clone --no-hardlinks origin cloneA; git checkout --detach $SHA_A; stet start $SHA_A~1 --dry-run --json --stream` → 1 fileA.txt finding. Leg 2 = wave-review analog: `git worktree add --detach wtb $SHA_B; stet start $(git merge-base main $SHA_B) --allow-dirty --quiet --json --dry-run --strictness default` → 1 fileB.txt finding. Plant leg-1's finding into leg-2's `session.json` (this is exactly what a foreign-file reviewer finding produces — the evidence filters keep it). PRE-FIX `stet list`: 2 findings (1 leaked from the other session). POST-FIX `stet list`: 1 finding (0 leaked, own finding kept) | Repro recorded | `cli/cmd/stet/main_test.go` `TestRunCLI_listHidesFindingsFromOtherSessionDiff` |
| Root cause: NOT shared state. Verified each lead: (1) state dir is per worktree (`Config.EffectiveStateDir` on `git rev-parse --show-toplevel`); `.review/session.json` is gitignored so clones/worktrees never carry it; `refs/notes/stet` holds only usage stats; `history.jsonl`/digest cache/dismissal restore live in the state dir and only ever suppress findings; (2) `stet list` prints every non-dismissed finding in `session.json` with no diff scoping — but the session is replaced fresh by every `stet start` (`s.Findings = collected`); (3) `stet start` never appends (flock + fresh session); (4) the script's `git merge-base main $branch` matches the three-dot diff operators verify against — no change needed. Actual leak path: the evidence filters (`findings.FilterByAddedLines` / `FilterByHunkLines`, `cli/internal/findings/evidence.go` ~66/~107) deliberately KEEP findings whose `File` differs from the reviewed hunk's ("File mismatch → keep"), and RAG (default `rag_symbol_max_definitions = 10`) injects symbol definitions from any repo file into each hunk prompt — so the reviewer can anchor findings in files outside the session's diff, and those land in `session.json`. At incident time the SP-039 orch range legitimately contained `ea2124f`/`14fce09` (cli/internal/review + prompt); findings in `run.go`/`main.go`/`config.go` (0 hunks in range) came from this path. The issue's "empty cli diff" verification was made after main absorbed the orch branch | Fixed | `cli/cmd/stet/main.go` `filterToSessionDiff` |
| Fix: `stet list`, `stet start/run/rerun --json`+human output, and `stet status` now hide findings whose file is outside `git diff --name-only BaselineRef..LastReviewedAt` (fail-open when the session has no completed review or git fails). Dismissal and finish flows unchanged; thresholds compose on top | Implemented | `cli/cmd/stet/main.go` |
| `stet list` output semantics changed (findings outside the session diff are hidden); `docs/cli-extension-contract.md` is SP-080's file in this release — flagged here for SP-080 to fold in | Handoff | `spine-tasks/CONTEXT.md` #5 row, `stet list --help` Long text |

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-11-01 | Step 1: dry-run repro + lead-by-lead root cause investigation | Leak path found (evidence filters keep foreign-file findings; RAG supplies foreign file content) |
| 2026-11-01 | Step 2: `filterToSessionDiff` wired into `activeFindings`, all report writers, `runList`, `runStatus` | `go build ./cli/...` clean |
| 2026-11-01 | Step 3: regression tests + CONTEXT #5 row resolved | New tests pass; pre-fix binary shows the leak, post-fix does not |

---

## Blockers

*None*
