# Research: Does the critic drop out-of-view false positives now that it sees the reviewer's expanded context? (SP-831 re-run)

**Task:** SP-078 · **Date:** 2026-11-02 · **Status:** Complete
**Question:** SP-069 (v1.4.0, commit `6de2150`) started passing the reviewer's prepared user prompt (expanded context plus RAG definitions, capped at 16384 chars) to both critics instead of the raw 4096-char hunk. Issue #18 asked for a re-run of the SP-831 comparison: does the System One critic now drop out-of-view false positives, and what is the latency change? This run is also the SP-080 (#20) tuning baseline, so every finding is labeled per-run in `labels.tsv`.

---

## Summary

**#18's acceptance condition holds: the expanded context measurably helps the critic, and no real findings were lost.** All 6 runs succeeded (3 critic-off, 3 critic-on). Every one of the 76 reported findings is false (39 off / 37 on; 44 of 76 assert facts about code outside the diff's hunks) — consistent with #18's original 22-of-23-false evidence. Using the pre-registered off-vs-on fallback (the stderr logs contain no critic kept/dropped counts), an average of **3.0 majority-reference false findings are absent per critic-on run** (2.0 even under the loosest family-level matching), clearing the "at least one per run on average" bar; there are **zero** real findings, so the no-real-loss condition holds vacuously. Latency: critic-on adds **+11.6 s/run on means** (+83.3 s on medians; the off run-1 mean is a cold-start outlier) — reported, not part of the rule. The critic's kept set introduces no new finding families, and it still keeps out-of-view claims about `compareTaskIds` internals, the `topoWaves` loop, and `discoverTasks` folder-name handling — the SP-080 tuning input.

**Verdict: #18 resolved** (per the pre-registered decision rule). The release operator closes the issue after this wave lands.

---

## Setup

**Pre-registered conditions** (from `spine-tasks/SP-078-critic-context-measurement/PROMPT.md`; unchanged during execution — no Amendments):

- **Harness:** `scripts/model_eval.py run` at worktree HEAD `f2c09f9ba927253fe3a8f252b6bb4f552db01912`; stet `v1.4.0`. Pinned env (recorded in each `run*.meta.json`): `STET_TEMPERATURE=0.1`, `STET_MAX_COMPLETION_TOKENS=4096`, `STET_NUM_CTX=65536`, `STET_CONTEXT_LIMIT=65536`, `STET_STRICTNESS=default`, `STET_RULES_SELECTOR=false`, `STET_RULES_ANNOTATE=false`, `STET_PROVIDER=openai`. Critic-on adds `STET_CRITIC_BACKEND=systemone`, `STET_CRITIC_ENABLED=true`, `STET_SYSTEMONE_MODEL=openjev-latest`, `STET_SYSTEMONE_TIMEOUT=90s`, `STET_SYSTEMONE_BASE_URL=http://127.0.0.1:8080`.
- **Strictness difference vs #18's original runs:** #18's evidence used `lenient`; this run uses the harness-pinned `default`. Recorded per the packet; the harness was not changed.
- **Source repo:** `/Users/cdelgado/Documents/github/pi-spine` (read-only; harness clones). **Real ref:** `37f22c1d854cf6e31520bf69b62c5eef2153811e` — `fix(SP-831): sort planner task IDs numerically (#352)`, 7 files, +115/−12, 16 hunks.
- **Model:** `qwen/qwen3.8-27b` via LM Studio at `http://127.0.0.1:1234`, loaded with `lms load qwen/qwen3.8-27b -c 65536 --parallel 1 --no-speculative-draft-mtp -y` (14.98 GiB, 46.8 s). Smoke request returned HTTP 200 / `OK`. OpenJev at `http://127.0.0.1:8080`: `/health` → 200 `{"status":"ok"}`. Nothing else used either server during the run (operator checklist).
- **Fixture cases:** none (empty `--cases` directory); `--real-runs 3` per side, each side into its own `--out`.

**Commands:**

```bash
# critic off
python3 scripts/model_eval.py run --model qwen/qwen3.8-27b --cases "$TMPDIR/sp078-empty-cases" \
  --out docs/research/critic-context-sp831/results-off \
  --repo /Users/cdelgado/Documents/github/pi-spine \
  --real-ref 37f22c1d854cf6e31520bf69b62c5eef2153811e --real-runs 3 --max-minutes 25
# critic on (same, plus)
  --out docs/research/critic-context-sp831/results-on --critic --systemone-base-url http://127.0.0.1:8080
```

All 6 runs exited `ok` on the first attempt; `--retry-failed` was not needed and no non-`ok` runs remain.

**Ground truth used for labeling** (code at the ref, read before labeling):

- `compareTaskIds` (`src/tasks/packet/discover.mjs:46-59`) always returns a number: prefix `localeCompare` for distinct prefixes, numeric difference for distinct numbers, `0` only for identical conforming IDs, `-1`/`+1` when exactly one ID matches `TASK_ID_PARTS_RE`, and a `localeCompare` fallback when neither matches. It never returns `null`/`undefined`, and distinct strings never compare `0` — a valid total order.
- `topoWaves` (`src/planner/graph.mjs`): `const next = []` is declared inside the `while` loop; `zero` is sorted once before the loop; each array enters `waves` after its one and only sort — recorded waves are never re-sorted.
- `buildGraph` dedupes edges (`new Set` on deps plus an `includes` guard on dependents), so a node's indegree reaches `0` exactly once; duplicate pushes into `next` are impossible.
- `filterPendingTaskIds` pushes `task.taskId` (IDs, not paths) and sorts with `compareTaskIds` (changed in this diff).
- `discoverTasks` derives task IDs via `taskIdFromFolderName`, which strips the slug: folder `SP-1000-test` yields task ID `SP-1000`. The new tests in the diff pass.

---

## Results

### Per-run overview (16 hunks per run)

| Run | Critic | Status | Findings | Wall (s) |
|---|---|---|---|---|
| off run1 | off | ok | 12 | 392.9 |
| off run2 | off | ok | 13 | 151.8 |
| off run3 | off | ok | 14 | 163.8 |
| on run1 | System One (OpenJev) | ok | 13 | 266.5 |
| on run2 | System One (OpenJev) | ok | 11 | 229.7 |
| on run3 | System One (OpenJev) | ok | 13 | 247.1 |

**Critic counts:** the per-run `*.stderr.log` files contain no critic kept/dropped counts (only the rules-budget warning); the harness does not enable tracing, so SP-069's `Critic input: N chars` line is absent too. The pre-registered fallback — off-vs-on finding matching — is used for the drops condition below.

### Labels (full detail in `labels.tsv`, committed before the rule was applied)

| Metric | Critic off (3 runs) | Critic on (3 runs) |
|---|---|---|
| Findings reported | 39 (12/13/14) | 37 (13/11/13) |
| Labeled `real` | 0 | 0 |
| Labeled `false` | 39 | 37 |
| Of which out-of-view claims | 24 (62%) | 20 (54%) |

All 76 findings are false positives; none is a defect or a clearly worthwhile change. The false findings fall into 20 recurring families (labels.tsv `reason` column names the family per row):

1. **Comparator-behavior claims** (26 findings; deps.mjs:46/59, scope.mjs:77, pending.mjs:26): "not a total order", "returns 0/null/undefined for non-matching IDs", "pending array holds full file paths" — all contradicted by the actual `compareTaskIds`/`filterPendingTaskIds` code (Setup, bullets 1 and 4). One variant rambles and concludes "I will discard the finding … No change needed."
2. **`topoWaves` loop claims** (8 findings; graph.mjs:74/93/100): "zero mutated in place", "next is the same array pushed into waves and re-sorted", "duplicate IDs in next" — contradicted by the per-iteration `next` and `buildGraph` dedup (Setup, bullets 2 and 3). Three variants self-refute ("No defect here … No issue").
3. **Design/performance suggestions** (16 findings): cross-layer import of `compareTaskIds` from the declared single-source-of-truth module, regex-in-comparator and `includes()` micro-optimizations, "redundant" second sort pass, unsorted `depsByTask`, missing re-export in `scope.mjs:16` — none a defect, none clearly worthwhile (`compareTaskIds` is used internally at scope.mjs:77/:137 in the same diff).
4. **Test claims** (26 findings): wave-order "brittleness" (the tests verify exactly the sort this diff adds), "filterPendingTaskIds does not sort" (it does, in this diff), folder-name mismatch ("IDs become SP-1000-test"; slug-stripping makes them `SP-1000`, tests pass), DRY/sync-fs nitpicks.

Matching by file, line, and message substance: **every critic-on finding matches a critic-off finding family; the critic introduces no new false-finding families** (its input is the same reviewer output, filtered).

### Latency (not part of the decision rule)

| Statistic | Critic off | Critic on | Δ |
|---|---|---|---|
| Mean wall | 236.2 s | 247.8 s | **+11.6 s/run** |
| Median wall | 163.8 s | 247.1 s | **+83.3 s/run** |

Off run1 (392.9 s) is an outlier — the first review after the model load; the two later off runs took 151.8 s and 163.8 s. Run-to-run reviewer variance is large relative to the critic's cost, so treat the median-based +83 s as the more honest per-run estimate and the mean-based +11.6 s as the outlier-skewed floor. Per hunk (16/run), the median delta is ≈5.2 s. #18's original estimate was ~15–25 s/run of critic overhead; the expanded 16384-char input does not blow that up by an order of magnitude, but it is not free.

---

## Decision (pre-registered rule, applied mechanically)

**Condition 1 — Drops:** "across the 3 critic-on runs the critic drops at least one `false` finding per run on average (from the stderr critic counts, or off-vs-on findings where the logs lack counts)." The logs lack counts → off-vs-on matching. Reference set = labeled-false findings present in a majority (≥2 of 3) of critic-off runs; count reference findings with no match (file + line + message substance) in each critic-on run:

| Majority-reference false finding | on run1 | on run2 | on run3 |
|---|---|---|---|
| deps.mjs:46 total-order claim | ✓ | ✓ | ✓ |
| deps.mjs:59 comparator-0 claim | ✓ | ✓ | ✓ |
| graph.mjs:42 redundant-sort-pass claim | ✗ | ✗ | ✗ |
| graph.mjs:74 aliasing/self-refute claim | ✗ | ✗ | ✓ |
| graph.mjs:93 recorded-wave re-sort claim | ✗ | ✗ | ✗ |
| pending.mjs:7 cross-layer import | ✓ | ✓ | ✓ |
| pending.mjs:26 null-comparator claim | ✓ | ✓ | ✓ |
| scope.mjs:16 missing re-export | ✓ | ✗ | ✓ |
| scope.mjs:77 comparator claim | ✓ | ✓ | ✓ |
| scope.mjs:137 `includes()` micro-opt | ✓ | ✓ | ✓ |
| graph.test.mjs:14 brittleness claim | ✓ | ✓ | ✓ |
| pending-scope.test.mjs no-sort claim | ✓ | ✓ | ✓ |
| scope.test.mjs folder-name claim | ✓ | ✓ | ✓ |
| **Missing per run** | **3** | **4** | **2** |

Average = **3.0 false findings missing per critic-on run ≥ 1 → condition 1 HOLDS.** (Under the loosest file+line family-level matching, where the four different graph.mjs:42 messages count as one family and off run1's one-off defect-claim at :74 counts as present, the average is 2.0 — still ≥ 1. Both readings agree.)

**Condition 2 — No real loss:** "no `real` finding present in a majority of critic-off runs is missing from a majority of critic-on runs." Zero findings are labeled `real` (0 of 39 off, 0 of 37 on), so there is no real finding that could be lost → condition 2 HOLDS vacuously.

**Both conditions hold → #18 resolved.** The result table is posted to #18 (URL in STATUS.md); the Mission line keeps `Closes #18`. The release operator closes the issue after the wave lands.

---

## Out-of-view claims still kept

Claims whose decisive evidence lies outside the diff's hunks (marked `out_of_view=yes` in `labels.tsv`) that appear in critic-on reports — the concrete tuning input for SP-080 (#20):

1. **`compareTaskIds` internals** (13 of 37 on findings; deps.mjs:46/59, pending.mjs:26, scope.mjs:77): "returns 0/null for non-matching IDs", "not a total order". The implementation lives in `src/tasks/packet/discover.mjs` (outside the diff); the RAG-expanded prompt does not settle these claims. Candidate criterion: a code check that drops comparator-behavior assertions whose subject function is not in the diff (cf. pi-spine #33's symbol-existence check).
2. **`topoWaves` loop state** (on run1 graph.mjs:93 "duplicates"; on run3 graph.mjs:74 "aliasing/fragile"): refuted by `const next = []` inside the loop and `buildGraph` dedup — code in the same function/file but outside the raw hunks. The expanded context demonstrably does not always reach these (two of three on runs still lack the refutation-driven drop).
3. **`discoverTasks` behavior** (3 on findings; scope.test.mjs:75/80/81): "taskId derived from PROMPT.md content with folder fallback", "test relies on filesystem enumeration order" — refuted by `taskIdFromFolderName` slug-stripping and explicit sorts in `discover.mjs`.
4. **Comparator/graph internals for performance claims** (on run2 graph.mjs:42 regex-in-comparator; on run3 includes-guard O(n²); on run1 unsorted `depsByTask`): subject code outside the diff.

24 of 39 critic-off findings (62%) and 20 of 37 critic-on findings (54%) carry `out_of_view=yes` overall.

---

## Threats to validity

1. **Drops are inferred, not observed.** With no critic counts in the logs, "absent from the on report" conflates critic drops with reviewer stochasticity (temperature 0.1 is not determinism). The pre-registered rule accepts this proxy; the +13→+12.33 naive per-run totals (−0.67) show why the finding-level match, not raw counts, is the usable signal. Future runs should capture SP-069's `Critic input: N chars` trace (or a critic kept/dropped counter) to observe drops directly.
2. **Strictness differs from #18's original evidence** (`default` here vs `lenient` in #18's runs), as pre-registered; absolute finding counts are not comparable to the original table — only the within-run off/on comparison is.
3. **Sample size:** 3 runs per side of a single 7-file diff with one model pair (qwen3.8-27b reviewer, OpenJev System One critic). Family-level repetition across runs mitigates but does not eliminate noise.
4. **Labeling is single-author** (this worker), mechanical per the rule's `real` = "a defect or a clearly worthwhile change" bar; borderline performance/style items (e.g. `includes()` → Set) were labeled `false` with the reasoning recorded in `labels.tsv` for re-review.
5. **Latency confound:** off run1's cold start (392.9 s) skews the mean; machine load during the runs was not independently monitored beyond the exclusivity checklist.
6. **One missed real issue:** no run (either side) flagged that the diff left `filterPendingTaskIds`'s JSDoc saying "Pending task IDs sorted lexicographically" while the sort is now numeric-aware (`src/planner/pending.mjs`, docstring above the changed line). Minor but real — recorded for SP-080's recall baseline.
