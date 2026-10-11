**Current Step:** 3
**Status:** Complete
**Last Updated:** 2026-10-10
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

**Plan summary:**
- `scripts/model_eval.py` (stdlib only): injectable subprocess seams (`run_simple`, `run_stream`) so tests fake stet/git/clock; `HarnessError` → exit 1.
- `run`: pinned env constants, fixture loader/validator, temp-repo materialization (`git init` → before commit → after commit, `.txt` stripped), real-ref clone + detach, NDJSON parse (`(\d+) hunks to review`, finding, done; unknown ignored; malformed → failed), status order timeout > failed > zero_hunks > ok, meta/stderr/json writes, resume + `--retry-failed`, budget → exit 3 `remaining=<K>`, `model_eval: done; remaining=0`, `[i/total] …` progress lines.
- `score`: match rule (window; range span else line; no-line matches only whole-file bugs), per-model-tag metrics (recall mean/lists, by-category, unexpected w/ severity + clean breakdown, pairwise Jaccard on `(file, line//5)`, reliability, seconds means, `real_diff` with labels), deterministic JSON (sort_keys, indent 2, round4, trailing newline), `--out`, `--check` exit 2 with differing top-level keys.
- `export-blind`: deterministic sort→`random.Random(seed)` shuffle→`F001…`, TSV without model identity, key JSON, label ingestion with row-level errors.

---

## Step 1: `run`

**Status:** Complete

- [x] Fixture loading, temp repos, real-ref clones, pinned env
- [x] Stream parsing, status, meta, resume, budget, progress

## Step 2: `score` and `export-blind`

**Status:** Complete

- [x] Match rule, metrics, deterministic JSON, `--check`
- [x] Blind export and label ingestion

## Step 3: Testing & Verification

**Status:** Complete

- [x] Unit tests for every Step 3 bullet in PROMPT (73 tests, `pytest -q` green)
- [x] `--dry-run` integration test (real `stet` subprocess: `run` then `score`)
- [x] Contract `testCommand` (stet review exits 0; 4 stale findings triaged and dismissed)

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-10-10 | Stet contract raised 4 injection findings (SHA/ref/base_url validation + missing test) against the pre-hardening diff; all were addressed by `f945ac4` (`_validate_real_ref`, `_validate_base_url`, `TestInputValidation`) but persisted after the fix landed (known stale-hunk hazard, see CONTEXT.md SP-035) | Dismissed `already_correct` with code references; re-run of the contract reviewed zero new findings |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-10-10 | Authored | Phase 7 model benchmark (harness for SP-039) |
| 2026-10-10 | Step 1 | `scripts/model_eval.py` run subcommand + 36 tests; real `stet --dry-run` smoke test ok (hunks_total 1, meta correct) |
| 2026-10-10 | Step 2 | `score` + `export-blind`; 68 tests total; CLI chain smoke (run real-ref clone → score recall 1.0 → export-blind TSV) ok |
| 2026-10-10 | Step 3 | 73/73 pytest pass; contract `bash scripts/spine-stet-contract-run.sh lenient` exit 0 after dismissing 4 stale findings (`be015b6`, `38cdb7b`, `2b18bf5`, `85b00ab`) already fixed in `f945ac4`; `stet finish` ran |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
