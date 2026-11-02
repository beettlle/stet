# Task: SP-078 — Measure the critic with the reviewer's expanded context (SP-831 off/on re-run)

**Created:** 2026-11-01
**Size:** S

## Review Level: 1

**Assessment:** Measurement and a research document; no production code. Needs exclusive use of LM Studio and OpenJev for roughly 30–60 minutes.
**Score:** 2/8 — Blast radius: 0, Pattern novelty: 1, Security: 0, Reversibility: 1

## Mission

Closes #18

Issue #18 reported that both critic backends judged findings against the raw hunk only, so the System One critic kept every out-of-view false positive on pi-spine's SP-831 diff (critic dropped 0 of 5 and 0 of 6; 22 of 23 findings false). SP-069 (v1.4.0) now passes the reviewer's prepared user prompt (expanded context plus RAG definitions) to both critics, capped at 16384 characters. The code half of #18 is done; this task does the issue's remaining acceptance item: **re-run the SP-831 comparison and record whether the critic now drops out-of-view false positives, and the latency change.**

This is also the baseline SP-080 (#20, critic criteria) tunes against, so record per-finding detail, not only totals.

### Operator checklist (done by the human before the wave starts)

The worker cannot do these; if Step 0 shows any is not done, record a Blocker and stop.

1. LM Studio server at `http://127.0.0.1:1234` with `qwen/qwen3.8-27b` loadable (the `.review/config.toml` model).
2. OpenJev (System One) server at `http://127.0.0.1:8080` serving `openjev-latest`.
3. Nothing else using LM Studio or OpenJev during the run (no other eval task in flight).

### Fixed run conditions (pre-registered)

- Harness: `scripts/model_eval.py run` with its pinned env (temperature 0.1, `max_completion_tokens` 4096, context 65536, strictness `default`, rules selector and annotate off). #18's original runs used `lenient`; record that difference in the doc, do not change the harness.
- Source repo: `--repo /Users/cdelgado/Documents/github/pi-spine` (read-only; the harness clones it).
- Real ref: `37f22c1d854cf6e31520bf69b62c5eef2153811e` (`fix(SP-831): sort planner task IDs numerically`, 7 files, +115/−12).
- Fixture cases: none (pass an empty directory as `--cases`).
- Runs: 3 critic-off, 3 critic-on (`--critic --systemone-base-url http://127.0.0.1:8080`), `--real-runs 3`, each into its own `--out` directory.
- Model: `qwen/qwen3.8-27b`, loaded with `lms load qwen/qwen3.8-27b -c 65536 --parallel 1 --no-speculative-draft-mtp -y`.

### Decision rule (pre-registered; apply mechanically)

Label every critic-off finding once, by reading the SP-831 diff and the code at that ref: `real` (a defect or a clearly worthwhile change) or `false` (`false_positive`, `wrong_suggestion`, or `out_of_scope`; add a one-line reason and mark out-of-view claims such as "X is not defined / not exported / not imported"). Match critic-on findings to these labels by file, line, and message; label any new ones the same way.

1. **Drops:** across the 3 critic-on runs the critic drops at least one `false` finding per run on average (from the stderr critic counts, or off-vs-on findings where the logs lack counts).
2. **No real loss:** no `real` finding present in a majority of critic-off runs is missing from a majority of critic-on runs.

- Both hold → **#18 resolved**: post the result table to #18 and keep `Closes #18`.
- Otherwise → **#18 stays open**: change the Mission line to `Partial #18` in an Amendment, post the table to #18, and list the out-of-view claims the critic still keeps (input for SP-080).
- Latency (seconds per run, critic-on minus critic-off) is reported but is not part of the rule.

## Dependencies

- **Task:** SP-077, SP-082 (serialization only: their contract reviews use LM Studio, and this measurement needs exclusive use)
- **External:** LM Studio, OpenJev, and `stet` on PATH per the operator checklist

## Context to Read First

**Tier 3 (reference docs):**

- `scripts/model_eval.py` (`run --help`; results layout, `*.stderr.log` per run)
- `spine-tasks/SP-069-critic-expanded-context/PROMPT.md` — what changed for #18
- `gh issue view 18 --repo beettlle/stet` — the original evidence table
- `docs/research/model-comparison-qwen3-coder-next-vs-qwen3.8-27b.md` — house style for research docs

## Environment

- **Workspace:** repo root
- **Services required:** LM Studio at `http://127.0.0.1:1234`, OpenJev at `http://127.0.0.1:8080` (exclusive use)

## File Scope

- `docs/research/critic-context-sp831.md` (new)
- `docs/research/critic-context-sp831/preflight.txt` (new)
- `docs/research/critic-context-sp831/labels.tsv` (new)
- `docs/research/critic-context-sp831/results-off/**` (new, harness output)
- `docs/research/critic-context-sp831/results-on/**` (new, harness output)

## Documentation Requirements

**Must Update:** `docs/research/critic-context-sp831.md` — sections Summary, Setup, Results (per-run findings, critic counts, labels, latency), Decision (rule applied condition by condition), Out-of-view claims still kept, Threats to validity.

**Check If Affected:** none.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `test -s docs/research/critic-context-sp831/labels.tsv && grep -q '^## Decision' docs/research/critic-context-sp831.md && grep -q '^## Out-of-view claims still kept' docs/research/critic-context-sp831.md` |
| fileScopeMustChange | `docs/research/critic-context-sp831.md`, `docs/research/critic-context-sp831/labels.tsv`, `docs/research/critic-context-sp831/preflight.txt` |
| fileScopeMustNotChange | `cli/**`, `scripts/**`, `extension/**`, `.review/**`, `docs/research/critic-eval/**` |
| completionCriteria | 3 critic-off and 3 critic-on runs of the SP-831 ref recorded (non-`ok` runs retried once and reported); every finding labeled; the doc applies the decision rule verbatim; the result table is posted to #18 (`gh issue comment 18 --repo beettlle/stet --body-file …`) with the comment URL in STATUS |

No stet contract gate: this task's diff is Markdown, TSV, and harness logs only.

## Steps

### Step 0: Preflight

Write all output to `docs/research/critic-context-sp831/preflight.txt`.

- [ ] `stet --version`, `git rev-parse HEAD`, `lms ps`, `git -C /Users/cdelgado/Documents/github/pi-spine cat-file -e 37f22c1d854cf6e31520bf69b62c5eef2153811e^{commit}`
- [ ] Load the model per Fixed run conditions; smoke request to `http://127.0.0.1:1234/v1/chat/completions` ("Reply with OK.", `max_tokens` 64) returns HTTP 200
- [ ] OpenJev reachable at `http://127.0.0.1:8080` (any 2xx or a documented health response); otherwise Blocker "OpenJev not running" and stop

### Step 1: Run critic off and critic on

- [ ] `mkdir -p "$TMPDIR/sp078-empty-cases"`; critic off: `python3 scripts/model_eval.py run --model qwen/qwen3.8-27b --cases "$TMPDIR/sp078-empty-cases" --out docs/research/critic-context-sp831/results-off --repo /Users/cdelgado/Documents/github/pi-spine --real-ref 37f22c1d854cf6e31520bf69b62c5eef2153811e --real-runs 3 --max-minutes 25` (repeat until exit 0; exit 3 = budget reached)
- [ ] Critic on: same command with `--out docs/research/critic-context-sp831/results-on --critic --systemone-base-url http://127.0.0.1:8080`
- [ ] `--retry-failed` once per side; record remaining non-`ok` runs in Discoveries
- [ ] Commit results (`docs(SP-078): SP-831 critic off/on results`)

### Step 2: Label, decide, and write the doc

- [ ] `labels.tsv` (columns: `side`, `run`, `file`, `line`, `message`, `label`, `reason`, `out_of_view`) committed before applying the rule
- [ ] Write `docs/research/critic-context-sp831.md`; apply the decision rule; post the table to #18
- [ ] `spine_review_step` after the step

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Run Contract `testCommand`
- [ ] Confirm nothing outside File Scope changed (`git status`)

## Completion Criteria

- [ ] All steps complete
- [ ] Contract `testCommand` passes
- [ ] Decision is explicit and follows the pre-registered rule

## Git Commit Convention

- **Results / docs:** `docs(SP-078): description`

## Do NOT

- Change the decision rule, run counts, model, ref, or pinned settings after seeing results (if something forces a change, add an Amendment explaining why **before** re-running)
- Edit `.review/config.toml`, LM Studio presets, critic code, or `scripts/model_eval.py`
- Write anything inside `/Users/cdelgado/Documents/github/pi-spine`
- Close or reopen #18 yourself (the release operator closes it after the wave lands)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
