# pi-spine v2 critic-eval dataset

Labeled dataset for tuning stet's System One critic threshold (`systemone_min_keep`,
default 0.5), built from the pi-spine release lines v2.27.1–v2.29.0 (tasks
SP-820..SP-845). pi-spine runs stet in its Contract gate; findings the release
dismissed are negatives (`drop`), findings it fixed are positives (`keep`).
SP-081 sweeps thresholds over this file with `stet critic-eval --dataset`.

## Files

| File | Contents |
|------|----------|
| `pi-spine-v2-dataset.jsonl` | 89 examples, one JSON object per line: `{"finding":{id,file,line,severity,category,confidence,message},"hunk":"…","label":"keep"\|"drop"}` in the `criticeval.LoadDataset` curated format (`cli/internal/criticeval/dataset.go`) |
| `pi-spine-v2-provenance.tsv` | one row per dataset line: `id, label, repo, task, commit, status_file, status_line, reason` |

The pi-spine checkout (`/Users/cdelgado/Documents/github/pi-spine`) was used
read-only; nothing was written or run there.

## Sources

1. **STATUS.md triage records** — `pi-spine/spine-tasks/SP-8NN-*/STATUS.md`
   (Discoveries and Execution Log) record every dismissal with reason
   (`false_positive`, `wrong_suggestion`, `already_correct`, `out_of_scope`),
   usually with the finding's short id, the claim, and the evidence it was wrong.
2. **`.reviews/contract-fail-*.log`** — the same task folders carry committed
   contract-failure logs embedding exact stet finding records in the form
   `<id>  <file>:<line>  <SEVERITY>  <message>`. Where a STATUS-named id (or an
   unambiguous STATUS triage claim) matches a log record, the row takes its
   file/line/severity/message from the log (message verbatim, or clipped at a
   sentence boundary and marked in provenance).
3. **Fix commits** — `keep` labels come from findings the release fixed rather
   than dismissed: `b8deb92a` (SP-820), `1ba31c36` (SP-821), `5a1024a0` +
   `f14fb499` (SP-822), `2ed04a88` (SP-827), `e7abf035` (SP-835), `d8b531d3`
   (SP-835 comment reword), `80ca9e3d` (SP-838), `b61b9fbb` (SP-845). Ids for
   the e7abf035/80ca9e3d/b61b9fbb keeps were recovered from the contract-fail
   logs (`7e9057f`, `360352d`, `8653ab9`, `85d0dd4`).
4. **Git history** — every `hunk` is a diff hunk rebuilt from the pi-spine
   repository: the cumulative task diff (`git diff <first-code-commit>^ <task-complete> -- <file>`),
   taking the hunk containing the finding's line. For `keep` rows the hunk is
   taken from the cumulative diff up to `fix-commit^`, i.e. the state where the
   defect still exists.

## Row rules applied

- `finding.message`: the claim as recorded — quoted verbatim from the
  contract-fail log when available (clipped at ~400 chars with `…` when the
  reviewer's message rambles; provenance notes the truncation), otherwise a
  faithful one-sentence paraphrase of the STATUS triage text including the
  recorded triage rationale. Claims are never sharpened or softened.
- `finding.file` / `line`: from the contract-fail log when available, otherwise
  from STATUS or located in the final/pre-fix file by the claim's subject.
  Mined lines reflect the log-time file state, which matches the state the
  reconstructed diff hunk shows.
- `severity` / `category` / `confidence`: from STATUS when given
  (SP-829: severity `error`, confidence `0.95`), else `warning` / `correctness`
  / `0.8`. Log severities map ERROR→`error`, WARNING→`warning`,
  INFO/NITPICK→`info`. Category is `test` for test-infra fixes, `style` for
  ordering/formatting, `documentation` for comment fixes.
- `finding.id`: `pi-spine-<SP-ID>-<short finding id>` where the id is recorded;
  otherwise `pi-spine-<SP-ID>-n` (synthetic, provenance notes it).
- One row per finding; grouped STATUS records without per-finding ids are
  emitted as one representative row per recoverable claim family, and the
  sibling findings are counted as skips below.
- Findings whose file, claim, or hunk could not be recovered are never
  invented; they are counted as skips by reason.

## Counts

**89 rows: 78 `drop`, 11 `keep`.** Both labels clear `minReliableExamples` (10).

| Source of row | drop | keep |
|---------------|------|------|
| message quoted/clipped from contract-fail log | 40 | 5 |
| message paraphrased from STATUS triage | 38 | 6 |

| Task | drop | keep | | Task | drop | keep |
|------|------|------|---|------|------|------|
| SP-820 | 2 | 2 | | SP-832 | 6 | 0 |
| SP-821 | 6 | 1 | | SP-834 | 5 | 0 |
| SP-822 | 0 | 2 | | SP-835 | 5 | 3 |
| SP-824 | 6 | 0 | | SP-837 | 3 | 0 |
| SP-826 | 6 | 0 | | SP-838 | 3 | 1 |
| SP-827 | 10 | 1 | | SP-842 | 3 | 0 |
| SP-828 | 3 | 0 | | SP-845 | 8 | 1 |
| SP-829 | 1 | 0 | | SP-831 | 11 | 0 |

## Skips: 139 findings counted, by reason

| Task | Skipped | Reason |
|------|---------|--------|
| SP-820 | 8 | round-1 dismissals recorded as a group (import/unused-symbol/spread-order claims) without per-finding ids, files, or hunks; no log round |
| SP-821 | 11 | 9 stale-hunk reads dismissed as a group; 1 `runWorker .DONE` no-op claim (target file outside the task diff); 1 "intentional defensive fallback" (unattributable) |
| SP-822 | 2 | `9cd6778`/`27a8259` (configResult claims, engine contract-fail round): per-id disposition not recorded in STATUS |
| SP-829 | 1 | `f21f4ab`: missing-`fs` claim re-raised against the `.DONE:1` marker; no git hunk recoverable |
| SP-831 | 8 | ids from an earlier unlogged round (`e1bf2c2 7cdfeaf e8bae2f 3d2ac90 aad5d2d e7f975c a571dba e809cdf`): no per-finding file/line recorded |
| SP-832 | 9 | 3 remainder of the 9-finding dismissal not claim-matched + 6 logged ids (`151482b 1952f78 ac1d7df 6c77082 c302cdb d438bf8`) without recorded disposition |
| SP-835 | 19 | 1 "hypothetical `ultra`" noise finding (no matching log record) + 18 logged ids from other rounds without recorded STATUS disposition |
| SP-836 | 4 | dismissed with no ids, claims, or files in STATUS; no log round; evidence lost with `.review` state |
| SP-837 | 5 | 2 wrong_suggestion without recorded claims + 3 without recoverable targets (symbol-defined-in-module, imports-used, `withProject`-awaited) |
| SP-838 | 4 | logged ids (`0fcd748 1d10e0a 91a54d4 600791e`) without recorded disposition; `600791e` targets an SP-820 file |
| SP-840 | 12 | 6 STATUS-dismissed out_of_scope findings on SP-838's files (hunks not in SP-840's commits) + 6 logged ids without recorded disposition |
| SP-841 | 6 | 2 findings (`ad3236c`, `33e9ca0`) raised on SP-839 files outside SP-841's diff + 4 logged ids without recorded disposition |
| SP-842 | 3 | logged ids (`f8a6c91 c145046 1b17196`) outside STATUS's three recorded claims |
| SP-843 | 10 | 13 findings dismissed as a group with 3 claim families; 3 representative rows emitted |
| SP-844 | 19 | 6 dismissed hunk-visibility false positives without ids + 11 logged ids dismissed as a group (STATUS REVISE record) + 2 "real fixed" findings not attributable to ids/hunks |
| SP-845 | 18 | round-1 remainder (13 findings, 4 claims + cosmetic fix matched) = 8; round-2 remainder (16 findings, 6 claims matched, 2 merged) = 10 |
| **Total** | **139** | |

## Threats to validity

- **Paraphrased messages.** 44 of 89 messages are one-sentence paraphrases of
  STATUS triage text (the original reviewer message did not survive); 45 are
  quoted from contract-fail logs, of which 29 could be kept fully verbatim and
  16 are clipped at a sentence boundary. Paraphrase may shift nuance.
- **Reconstructed hunks.** pi-spine keeps no `.review/history.jsonl` and its
  `refs/notes/stet` holds only counts, so every hunk is rebuilt from cumulative
  task diffs. The reviewer's actual prompt shadow (hunk window, truncation —
  itself the cause of several dismissed findings) is not exactly reproducible;
  hunks may be larger or shifted relative to what the reviewer saw.
- **Single labeler.** One curator mapped STATUS/log records to labels, guided by
  the release operator's own triage verdicts; no second labeling pass or
  inter-rater check was performed.
- **Original prompt context unavailable.** The rules profile, diff shaping, and
  strictness in force at review time are only partially recorded; rows do not
  condition on them.
- **Label provenance is post-hoc triage.** `drop` labels reflect the release
  operator's dismissal decisions (some supported by lint/typecheck/test
  evidence, others by argument); `already_correct` and `out_of_scope` rows are
  included as `drop` per the task packet even though `LoadFromState` treats
  those reasons as ambiguous for state-derived datasets.
- **Line-number drift.** Mined `line` values are log-time positions; where a
  later in-task commit moved code (e.g. SP-824's LOC-cap compaction), the hunk
  is the nearest final-state hunk, which may differ slightly from the flagged
  region.
- **Round overlap.** Repeated review rounds logged the same finding ids with
  evolving severity; rows take the first (most severe) record, and duplicates
  across rounds are merged rather than double-counted.
