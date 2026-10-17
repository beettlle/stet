# Task: SP-054 — Evidence filter keeps only findings near added lines

**Created:** 2026-10-17
**Size:** S

## Review Level: 2

**Assessment:** New evidence rule in `findings`, call-site change in `cli/internal/run/run.go` (hot shared module).
**Score:** 4/8 — Blast radius: 2, Pattern novelty: 1, Security: 0, Reversibility: 1 — Level 2 for the hot module

## Mission

Closes #21

`FilterByHunkLines` in `cli/internal/findings/evidence.go` keeps any finding whose line or range overlaps the whole hunk range, including the unchanged context lines around the edit. Findings about code the diff didn't change therefore survive.

Change the evidence rule so a finding on the hunk's file is kept only when its line (or any line of its range) is **on, or within `addedLineWindow` lines of, an added (`+`) line** in the hunk, using new-file line numbers. `addedLineWindow` is a named constant set to `3`.

- Compute added new-file line numbers from `hunk.RawContent`: parse the `@@ -a,b +c,d @@` header for the new start (reuse `expand.HunkLineRange` or the same header regex), then walk the body: `+` lines advance the new-file counter and are added; ` ` (context) lines advance it; `-` lines don't; `\ No newline at end of file` is ignored.
- Keep the current rules unchanged: file mismatch → keep; file-only (`Line == 0`, no `Range`) → keep; invalid range (`Start > End`) → drop.
- A hunk with no added lines (pure deletion) → fall back to the current whole-hunk overlap rule, so deletion reviews still report.
- Prefer a new function (for example `FilterByAddedLines(list []Finding, filePath string, added []int, hunkStart, hunkEnd int) []Finding`) plus a small exported helper that extracts added lines from raw hunk content; keep `FilterByHunkLines` if other callers or tests still need it.
- Call it at the `FilterByHunkLines` sites in `cli/internal/run/run.go` (~lines 500, 581). For the dry-run canned-findings path (~line 1364), check where `cannedFindingsForHunks` places its findings; if they sit on added lines, use the new filter there too, otherwise keep the old filter there and note why in STATUS. Update the trace label to `Evidence (added lines ±3)`.

## Dependencies

- **Task:** SP-053 (also edits the same post-filter blocks in `cli/internal/run/run.go`; serialize to avoid merge conflicts)

## Context to Read First

**Tier 3 (reference docs):**

- `cli/internal/findings/evidence.go`, `evidence_test.go`
- `cli/internal/expand/expand.go` — `HunkLineRange`, `hunkHeaderRegex`
- `cli/internal/run/run.go` — post-filter blocks and `cannedFindingsForHunks`

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/findings/evidence.go`
- `cli/internal/findings/evidence_test.go`
- `cli/internal/run/run.go`
- `cli/internal/run/run_test.go`

## Documentation Requirements

**Must Update:** none (SP-056 updates `docs/review-process-internals.md`)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/findings/... ./cli/internal/expand/... ./cli/internal/run/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/findings/evidence.go`, `cli/internal/findings/evidence_test.go`, `cli/internal/run/run.go` |
| fileScopeMustNotChange | `cli/cmd/**`, `cli/internal/findings/praise.go`, `extension/**`, `scripts/**`, `.review/**` |
| completionCriteria | A finding on a context line more than 3 lines from any added line is dropped (test); findings on or within 3 lines of an added line are kept (test); pure-deletion hunks keep the old rule (test); existing evidence tests pass or are updated with a stated reason; stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Added-line extraction and filter

- [ ] Helper returns added new-file line numbers from raw hunk content
- [ ] Added-line evidence filter with `addedLineWindow = 3` and the deletion fallback
- [ ] `spine_review_step` after the step

### Step 2: Wire into run

- [ ] Live-review call sites use the new filter; dry-run path decided and noted; trace label updated
- [ ] `spine_review_step` after the step

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Helper tests: mixed `+`/` `/`-` body, `\ No newline` line, missing count in header (`@@ -1 +1 @@`)
- [ ] Filter tests: context line 4+ away dropped; line exactly 3 away kept; range overlapping the window kept; file mismatch and file-only kept; pure deletion fallback
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing; `evidence.go` ≥ 72% line coverage

## Git Commit Convention

- **Feature:** `feat(SP-054): description`
- **Tests:** `test(SP-054): description`

## Do NOT

- Change abstention, the FP kill list, or the praise filter
- Make the window configurable
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
