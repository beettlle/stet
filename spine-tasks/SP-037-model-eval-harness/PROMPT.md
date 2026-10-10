# Task: SP-037 — Model comparison harness (`scripts/model_eval.py`)

**Created:** 2026-10-10
**Size:** S

## Review Level: 1

**Assessment:** New standalone Python script and its tests; no production CLI or extension code changes. Runs `stet` and `git` as subprocesses in temp directories only.
**Score:** 2/8 — Blast radius: 0, Pattern novelty: 1, Security: 1, Reversibility: 0

## Mission

Build a reusable, stdlib-only harness that runs stet with a chosen LLM over (a) seeded-bug fixture cases and (b) real commits, then scores the results so two models can be compared on the same inputs. SP-039 uses it to compare `qwen/qwen3-coder-next` and `qwen/qwen3.8-27b` in LM Studio; SP-038 authors the fixtures in parallel, so this task builds against the schema below and its own temp fixtures.

### Fixture schema (shared with SP-038 — do not change)

```
scripts/model-eval-cases/<case-id>/
  before/<relative/path>.txt   # committed first; harness strips the trailing .txt
  after/<relative/path>.txt    # full post-change files; a file only in after/ is an added file
  expected.json
```

`expected.json`:

```json
{
  "id": "go-off-by-one",
  "language": "go",
  "clean": false,
  "bugs": [
    {"file": "internal/page/page.go", "lines": [14, 16], "category": "correctness", "description": "slice end uses len+1"}
  ]
}
```

- `id` equals the directory name; `file` is the path **without** `.txt`, relative to the materialized repo root; `lines` is an inclusive 1-based `[start, end]` in the `after/` version.
- `clean: true` cases have `"bugs": []`; every finding on them is unexpected.

### Results layout (consumed by SP-039)

```
<out>/<model-tag>/cases/<case-id>/run<N>.json        # {"findings": [...]} parsed from the NDJSON stream
<out>/<model-tag>/cases/<case-id>/run<N>.meta.json   # see below
<out>/<model-tag>/cases/<case-id>/run<N>.stderr.log
<out>/<model-tag>/real/<sha>/run<N>.json | .meta.json | .stderr.log
```

- `model-tag` = model name with `/` replaced by `__` (e.g. `qwen__qwen3-coder-next`); dry-run uses the tag `dry-run`.
- `meta.json`: `{"model", "kind": "case"|"real", "target", "run", "status": "ok"|"failed"|"timeout"|"zero_hunks", "exit_code", "wall_seconds", "hunks_total", "findings_count", "stet_version", "env": {pinned STET_* vars}}`. No timestamps or absolute temp paths (keeps `score` output deterministic).

### Subcommands

**`run --model M --cases DIR --out DIR [--runs N=3] [--real-ref SHA ...] [--real-runs N=1] [--repo PATH=.] [--base-url URL] [--max-minutes F] [--review-timeout-minutes F=30] [--retry-failed] [--dry-run]`**

- `--runs` applies to fixture cases, `--real-runs` to each `--real-ref`. Order: all cases run 1, then run 2, …, then real refs (so an interrupted batch still has full coverage at fewer runs).

- **Case review:** `tempfile.mkdtemp()` → `git init`; write `before/` (strip `.txt`), commit; write `after/`, commit; run `stet start HEAD~1 --json --stream` with `cwd` = temp repo; then `stet finish`; remove the temp dir in `finally`. Commit with `git -c user.name=model-eval -c user.email=model-eval@localhost -c commit.gpgsign=false` so no global git config is needed.
- **Real ref:** `git clone --quiet --no-hardlinks <repo> <tmp>`; `git -C <tmp> checkout --quiet --detach <SHA>`; `stet start <SHA>~1 --json --stream`; `stet finish`; remove. Never run stet inside `--repo` itself (lane worktrees already hold an open stet session; clones also keep `refs/notes/stet` and `.review/history.jsonl` untouched).
- **Pinned environment** (set explicitly on every stet subprocess; env beats `.review/config.toml`, so a cloned repo's tracked config cannot change these). Name them as module-level constants:
  `STET_PROVIDER=openai`, `STET_OPENAI_BASE_URL` (from `--base-url`, default `http://127.0.0.1:1234/v1`), `STET_MODEL=<M>`, `STET_TEMPERATURE=0.1` (never 0 — the OpenAI-compat client omits a zero temperature, so LM Studio's UI value would apply), `STET_MAX_COMPLETION_TOKENS=8192`, `STET_NUM_CTX=32768`, `STET_CONTEXT_LIMIT=32768`, `STET_STRICTNESS=default`, `STET_CRITIC_ENABLED=false`, `STET_RULES_SELECTOR=false`, `STET_RULES_ANNOTATE=false`, `STET_TIMEOUT=30m`.
- **Stream parsing:** stdout is NDJSON. `{"type":"progress","msg":"<N> hunks to review"}` gives `hunks_total`; `{"type":"finding","data":{...}}` appends a finding; `{"type":"done"}` ends. Ignore unknown event types; a malformed line marks the run `failed` (do not crash the batch).
- **Status:** non-zero exit → `failed`; subprocess timeout (`--review-timeout-minutes`) → `timeout` (kill the process group, still attempt `stet finish`); `hunks_total == 0` → `zero_hunks`. Failed runs still write `meta.json` and the stderr log.
- **Resume:** skip any run whose `meta.json` exists; with `--retry-failed`, re-run those whose status is not `ok`.
- **Budget:** before starting each review, if elapsed > `--max-minutes`, stop and exit **3** after printing `model_eval: budget reached; remaining=<K>`. Exit 0 with `model_eval: done; remaining=0` when everything is complete. Exit 1 on usage/setup errors (missing cases dir, `stet`/`git` not on PATH, invalid `expected.json`).
- **Progress:** print one line per review to stdout (`[i/total] <model-tag> <kind> <target> run<N> -> <status> <findings> findings <secs>s`) so spine sees activity.
- `--dry-run` passes `--dry-run` to `stet start` (no LLM; canned findings) for tests and smoke checks.

**`score --cases DIR --results DIR [--labels TSV --blind-key JSON] [--out FILE] [--check FILE] [--window 3]`**

- **Match rule:** a finding catches a bug when `finding.file == bug.file` and the finding's line span intersects `[start - window, end + window]`. The span is `range.start..range.end` when `range` is present, else `line..line`; findings with no line match on file only when the bug spans the whole file (document this edge in a docstring).
- **Per model** (keyed by `model-tag`), output:
  - `recall_mean`: mean over runs of (caught bugs / total bugs) across non-clean cases; also `bugs_caught_all_runs`, `bugs_caught_some_runs`, `bugs_missed_all_runs` (lists of `case-id`).
  - `recall_by_category`: `{category: {caught_mean, total}}`.
  - `unexpected`: count of findings matching no bug (includes everything on clean cases), mean per run, broken down by severity (`error`, `warning`, `info`, `nitpick`) and with `on_clean_cases` separately.
  - `repeatability_jaccard_mean`: per case, Jaccard over sets of `(file, line // 5)` between every pair of runs; mean across cases (cases with < 2 ok runs are excluded and counted).
  - `reliability`: counts of `ok`, `failed`, `timeout`, `zero_hunks`.
  - `seconds_per_case_mean` (ok case runs only) and `seconds_per_real_ref_mean`.
  - With `--labels`/`--blind-key`: `real_diff`: per-label counts and `actionable_rate` = actionable / labeled.
- Output JSON is deterministic: `sort_keys=True`, `indent=2`, floats rounded to 4 places, trailing newline.
- `--check FILE`: compute the summary, compare to `FILE` as parsed JSON; exit 0 if equal, else print a short diff of differing top-level keys and exit 2.

**`export-blind --results DIR --out TSV --key JSON [--seed 20261010]`**

- Collect every finding from `*/real/*/run*.json`, shuffle with `random.Random(seed)`, assign `anon_id` `F001…`, and write TSV columns `anon_id, ref, file, line, severity, category, message, suggestion, label, notes` (`label`/`notes` empty). The model tag must not appear anywhere in the TSV. Write `{anon_id: {"model_tag", "ref", "run", "index"}}` to the key file.
- Labels TSV accepted by `score` is the same file with `label` filled with one of `actionable`, `false_positive`, `wrong_suggestion`, `out_of_scope`; unknown labels or missing `anon_id`s in the key are errors naming the row.

## Dependencies

- **None** (SP-038 fixtures land in parallel; do not read `scripts/model-eval-cases/`)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md` — "Stet in spine (dogfooding)"

**Tier 3 (reference docs):**

- `docs/cli-extension-contract.md` — `--stream` NDJSON events, finding JSON shape, env var table
- `cli/internal/findings/finding.go` — `Finding` (`file`, `line`, `range{start,end}`, `severity`, `category`)
- `cli/internal/openaicompat/client.go` — `chatRequest` (`temperature` is `omitempty`)
- `docs/efficacy-tests.md` — A1 (model swap), A4 (repeatability), H1 (blind triage), H2 (fixture benchmark)
- `scripts/tests/conftest.py`, `scripts/optimize.py` — existing Python style in `scripts/`

## Environment

- **Workspace:** repo root
- **Services required:** none for tests (`--dry-run` integration test needs `stet` on PATH and skips otherwise). LM Studio at `http://127.0.0.1:1234` only for the stet contract step.
- **Python:** 3.14, stdlib only (no new dependencies; `pytest` is already available)

## File Scope

- `scripts/model_eval.py` (new)
- `scripts/tests/test_model_eval.py` (new)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `python3 -m pytest scripts/tests/test_model_eval.py -q && bash scripts/spine-stet-contract-run.sh lenient` |
| fileScopeMustChange | `scripts/model_eval.py`, `scripts/tests/test_model_eval.py` |
| fileScopeMustNotChange | `cli/**`, `extension/**`, `.review/**`, `scripts/model-eval-cases/**`, `scripts/optimizer/**`, `scripts/spine-*.sh` |
| completionCriteria | `run`, `score`, and `export-blind` behave as specified; tests cover every bullet in Step 3; `--dry-run` integration test passes when `stet` is on PATH; stet contract review exits 0 |

## Steps

### Step 1: `run`

- [ ] Fixture loading/validation, temp-repo materialization (strip `.txt`), real-ref clones, pinned env constants
- [ ] NDJSON parsing, status classification, meta/stderr files, resume, `--retry-failed`, budget exit 3, progress lines

### Step 2: `score` and `export-blind`

- [ ] Match rule, per-model metrics, deterministic JSON, `--check` exit 2
- [ ] Blind export (no model identity in TSV), labels + key ingestion with row-level errors

### Step 3: Testing & Verification

> ZERO test failures allowed. `pytest-cov` is not installed, so cover error paths explicitly instead of quoting a coverage number.

- [ ] Match window: finding exactly at `start - window` and `end + window` matches; one line beyond does not; `range` overlap; wrong file never matches
- [ ] Clean case: all findings counted unexpected (and in `on_clean_cases`)
- [ ] Recall mean / all-runs / some-runs / missed lists on a hand-built results tree with exact expected numbers
- [ ] Jaccard: identical runs → 1.0; disjoint → 0.0; single-run case excluded
- [ ] Stream parsing: progress hunk count, findings, unknown event ignored, malformed line → `failed`, zero hunks → `zero_hunks`
- [ ] Resume skips existing meta; `--retry-failed` re-runs non-ok; budget reached → exit 3 with `remaining=` line (inject a fake runner/clock rather than sleeping)
- [ ] `--check` equal → 0, mismatch → 2; output byte-identical across two invocations
- [ ] `export-blind`: deterministic for a seed; TSV contains no model tag; labels with unknown label / unknown `anon_id` → error naming the row
- [ ] Invalid `expected.json` (missing field, `lines` start > end, `clean: true` with bugs) → exit 1 with the case id
- [ ] Integration (skip if `stet` not on PATH): `run --dry-run --runs 1` on a one-case temp fixture produces `ok` meta with `hunks_total >= 1`, then `score` runs on it
- [ ] Run Contract `testCommand` (includes the stet review; triage per "Stet in spine" in CONTEXT.md)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Implementation:** `feat(SP-037): description`
- **Tests:** `test(SP-037): description`

## Do NOT

- Add third-party Python dependencies
- Run stet inside the repo or lane worktree (always temp repos/clones)
- Read or create files under `scripts/model-eval-cases/` (SP-038 owns them)
- Call LM Studio from tests (use `--dry-run` or fake runners)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
