# Task: SP-019 — Critic evaluation dataset loader

**Created:** 2026-09-30
**Size:** S

## Review Level: 1

**Assessment:** Read-only loader over local review history and session files; labeling rules decide evaluation quality.
**Score:** 3/8 — Blast radius: 0, Pattern novelty: 2, Security: 1, Reversibility: 0

## Mission

Build the labeled dataset that a later evaluation (SP-024, SP-025) uses to choose the System One critic threshold (`systemone_min_keep`). Create package `cli/internal/criticeval` with a loader that turns local review data into examples labeled **keep** (real finding) or **drop** (false positive), grouped by hunk.

**Background (System One / OpenJev):** stet's optional System One critic (OpenJev, [razorback16/openjev](https://github.com/razorback16/openjev); same wire API as hosted Jev) scores each finding with P(yes) = "this finding is correct" and keeps it when P(yes) ≥ `systemone_min_keep`. Choosing that threshold needs labeled past findings with the code they were raised on.

**Label sources** (confirm against real data in Step 0):

- **drop:** from `history.ReadRecords(stateDir)` (`cli/internal/history/append.go`), dismissals whose `reason` is `false_positive` or `wrong_suggestion` and that have a non-empty `prompt_context`; the finding comes from the same record's `review_output` by `finding_id`.
- **keep (weak label):** active findings in `.review/session.json` (`cli/internal/session`) that are not in `dismissed_ids` and have an entry in `finding_prompt_context`. History stores hunk content only for dismissed findings, so history alone cannot supply positives.
- **Curated set:** `LoadDataset(path)` reads JSONL lines `{"finding": {...}, "hunk": "...", "label": "keep"|"drop"}`; when used, it replaces history/session.
- **Excluded:** `already_correct` (auto-dismiss reuses it for fixed issues, so it is ambiguous) and `out_of_scope`.

**API surface (adjust names to local style):**

```go
type Label string // "keep" | "drop"
type Example struct { Finding findings.Finding; Label Label; Source string }
type HunkGroup struct { Hunk string; Examples []Example }
type Dataset struct { Groups []HunkGroup; Counts map[Label]int; BySource map[string]int }

func LoadFromState(stateDir string) (*Dataset, error)
func LoadDataset(path string) (*Dataset, error)
```

- Group by exact hunk text so each group becomes one System One request later.
- De-duplicate by finding ID; if an ID has conflicting labels, drop it and count it under a `conflict` source.
- Read-only: never write to `.review/`.

## Dependencies

- **None**

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `cli/internal/history/schema.go` — `Record`, `Dismissal`, reason constants
- `cli/internal/history/append.go` — `ReadRecords`
- `cli/internal/session/session.go` — `Load`, `DismissedIDs`, `FindingPromptContext`, `Findings`
- `cli/internal/findings/finding.go` — `Finding`
- `AGENTS.md` — coverage gates

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none

## File Scope

- `cli/internal/criticeval/dataset.go` (new)
- `cli/internal/criticeval/dataset_test.go` (new)
- `cli/internal/criticeval/testdata/**` (new fixtures)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./internal/criticeval/... -count=1 -cover` |
| fileScopeMustChange | `cli/internal/criticeval/dataset.go`, `cli/internal/criticeval/dataset_test.go` |
| fileScopeMustNotChange | `cli/internal/history/**`, `cli/internal/session/**`, `cli/internal/review/**`, `cli/cmd/**`, `extension/**` |
| completionCriteria | `LoadFromState` and `LoadDataset` produce grouped, labeled, de-duplicated examples per the rules; read-only; tests pass with ≥72% coverage on `dataset.go` |

## Steps

### Step 0: Confirm labeling against real data

- [ ] Inspect a real `.review/history.jsonl` and `session.json` (schema only; do not commit user data) and confirm the label rules above
- [ ] Record any rule changes in STATUS Discoveries

> ⚠️ Hydrate: adjust Step 1 checkboxes if the label sources differ from the PROMPT.

### Step 1: Implement loaders

- [ ] `LoadFromState`: drop labels from history, keep labels from session, exclusions applied
- [ ] `LoadDataset`: curated JSONL with validation (unknown label, empty hunk → error naming the line number)
- [ ] Grouping by hunk, de-duplication, conflict handling, counts

### Step 2: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Synthetic fixtures under `testdata/` (history with each reason, session with active/dismissed findings, curated JSONL valid and invalid)
- [ ] Cases: exclusions ignored, missing `prompt_context` skipped, conflicting labels, empty state dir, grouping by identical hunk
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Per-file coverage ≥ 72% for `dataset.go`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing
- [ ] Loader never writes to `.review/`

## Git Commit Convention

- **Implementation:** `feat(SP-019): description`
- **Tests:** `test(SP-019): description`

## Do NOT

- Call System One or compute metrics (SP-024)
- Add a CLI command (SP-025)
- Modify history or session schemas
- Commit real user history or code as fixtures
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
