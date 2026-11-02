# Task: SP-079 — Critic-eval dataset from pi-spine release dismissals

**Created:** 2026-11-01
**Size:** S

## Review Level: 1

**Assessment:** Data curation and a provenance note; no production code. Reads another local repository (read-only).
**Score:** 2/8 — Blast radius: 0, Pattern novelty: 1, Security: 0, Reversibility: 1

## Mission

Partial #34

pi-spine runs stet with the System One critic at the default `systemone_min_keep = 0.5`, which has never been tuned on labeled data. Build the labeled dataset that SP-081 sweeps. Output: a JSONL file in the `stet critic-eval --dataset` format, one example per line:

```json
{"finding":{"id":"…","file":"…","line":12,"severity":"warning","category":"correctness","confidence":0.8,"message":"…"},"hunk":"…","label":"drop"}
```

`criticeval.LoadDataset` (`cli/internal/criticeval/dataset.go`) rejects invalid JSON, labels other than `keep`/`drop`, and empty hunks. See `cli/internal/criticeval/testdata/curated/valid.jsonl` for a valid file. `stet critic-eval` reports a recommendation only when both labels have at least 10 examples (`minReliableExamples` in `cli/internal/criticeval/sweep.go`).

### Sources (read-only)

pi-spine checkout: `/Users/cdelgado/Documents/github/pi-spine`. Do **not** write anything there.

1. **`drop` examples:** stet findings the pi-spine release (SP-820..SP-845) dismissed as `false_positive`, `wrong_suggestion`, `already_correct`, or `out_of_scope`. They are recorded in `spine-tasks/SP-8NN-*/STATUS.md` (Discoveries and Execution Log), usually with the finding's short ID, the claim, and the evidence it was wrong. Find them with `rg -n 'false_positive|wrong_suggestion|already_correct|out_of_scope' /Users/cdelgado/Documents/github/pi-spine/spine-tasks/SP-8[234]*/STATUS.md`.
2. **`keep` examples:** stet findings that were real and fixed instead of dismissed. In pi-spine these include the JSDoc comma (SP-838, `80ca9e3d`), stale `workerModel` state (SP-835, `e7abf035`), and test cleanup not in `finally`; also rows such as "fixed in <sha>" in the same STATUS files. If fewer than 10 `keep` rows result, add fixed stet findings recorded in this repo's `spine-tasks/SP-04*..SP-07*/STATUS.md` (same rules, source repo stet).
3. **Hunks:** pi-spine has no `.review/history.jsonl`, and its `refs/notes/stet` notes hold only counts. Rebuild each `hunk` from git: find the task commits (`git -C <repo> log --oneline --grep 'SP-8NN'`), then take the diff hunk for the finding's file around its line (`git -C <repo> show <sha> -- <file>`). For a fixed (`keep`) finding, use the hunk **before** the fix commit, where the defect still exists.

### Row rules

- `finding.message`: the claim as recorded in STATUS, quoted when STATUS quotes it, otherwise a faithful one-sentence paraphrase. Never sharpen or soften the claim.
- `finding.file` / `line`: from STATUS or the hunk; `severity`, `category`, `confidence` from STATUS when given, otherwise `warning`, `correctness`, `0.8`.
- `finding.id`: `<repo>-<SP-ID>-<short finding id or n>` (unique).
- Skip any row whose file, claim, or hunk cannot be recovered. Never invent a finding. Count skips by reason.
- One row per finding; when STATUS groups several IDs under one claim, emit one row per ID only if each has its own hunk, otherwise one row.

## Dependencies

- **None**

## Context to Read First

**Tier 3 (reference docs):**

- `gh issue view 34 --repo beettlle/stet`
- `cli/internal/criticeval/dataset.go` (`LoadDataset`, curated line format) and `testdata/curated/valid.jsonl`
- `cli/internal/criticeval/sweep.go` (`minReliableExamples`, recommendation rule)

## Environment

- **Workspace:** repo root
- **Services required:** none (no LLM; `stet critic-eval` is **not** run here, SP-081 runs it)

## File Scope

- `docs/research/critic-eval/pi-spine-v2-dataset.jsonl` (new)
- `docs/research/critic-eval/pi-spine-v2-provenance.tsv` (new: `id`, `label`, `repo`, `task`, `commit`, `status_file`, `status_line`, `reason`)
- `docs/research/critic-eval/README.md` (new: sources, row rules, counts per label and source, skips by reason, threats to validity)

## Documentation Requirements

**Must Update:** `docs/research/critic-eval/README.md` — as listed in File Scope.

**Check If Affected:** none.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `jq -c . docs/research/critic-eval/pi-spine-v2-dataset.jsonl > /dev/null && grep -c '"label":"keep"' docs/research/critic-eval/pi-spine-v2-dataset.jsonl && grep -c '"label":"drop"' docs/research/critic-eval/pi-spine-v2-dataset.jsonl && go test ./cli/internal/criticeval/... -count=1 -run TestLoadDataset` |
| fileScopeMustChange | `docs/research/critic-eval/pi-spine-v2-dataset.jsonl`, `docs/research/critic-eval/pi-spine-v2-provenance.tsv`, `docs/research/critic-eval/README.md` |
| fileScopeMustNotChange | `cli/**`, `scripts/**`, `extension/**`, `.review/**`, `docs/research/critic-context-sp831/**` |
| completionCriteria | Every dataset line parses with `jq` and loads with `criticeval.LoadDataset` (verified by a throwaway Go program or test run outside the repo, output recorded in STATUS); every row has a provenance row; README states counts per label and source and skips by reason; both labels have ≥ 10 rows, or README and STATUS record `WARN: <label> has N < 10 examples` |

No stet contract gate: this task's diff is data and Markdown only.

## Steps

### Step 1: Harvest `drop` rows from pi-spine

- [ ] Every dismissal in SP-820..SP-845 STATUS files extracted with its hunk, or counted as skipped with a reason
- [ ] `spine_review_step` after the step

### Step 2: Harvest `keep` rows

- [ ] Fixed findings from pi-spine (and from stet SP-04*..SP-07* if needed to reach 10), each with its pre-fix hunk
- [ ] `spine_review_step` after the step

### Step 3: Provenance and README

- [ ] `pi-spine-v2-provenance.tsv` row for every dataset line; README with counts, skips, and threats to validity (paraphrased messages, reconstructed hunks, single labeler, original prompt context unavailable)
- [ ] `spine_review_step` after the step

### Step 4: Testing & Verification

> ZERO test failures allowed.

- [ ] Load check with `criticeval.LoadDataset` recorded in STATUS (counts printed)
- [ ] Run Contract `testCommand`
- [ ] Confirm nothing outside File Scope changed (`git status`)

## Completion Criteria

- [ ] All steps complete
- [ ] Contract `testCommand` passes

## Git Commit Convention

- **Data / docs:** `docs(SP-079): description`

## Do NOT

- Write, commit, or run stet in `/Users/cdelgado/Documents/github/pi-spine`
- Run `stet critic-eval` or any LLM (SP-081 does the sweep)
- Invent findings, hunks, or labels; relabel a dismissal as `keep`
- Change `cli/internal/criticeval/**` (if the format cannot express a row, record it under Discoveries)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
