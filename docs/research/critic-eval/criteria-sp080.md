# SP-080: System One criteria wording — validation record

**Task:** SP-080 (#20) · **Date:** 2026-11-02 · **Status:** In progress
**Question:** Does rewording the System One critic's true/false criteria so that
absence claims about code outside the shown context count as unsupported raise
precision on `drop` examples without losing any `keep` example the old criteria
kept?

## Pre-registered protocol (fixed before any `new`-criteria run)

- **Binaries:** `old` = current `main` (`a90293d`, built unchanged:
  `go build -o $TMPDIR/stet-old ./cli/cmd/stet`); `new` = this branch after the
  wording change. Both run `stet critic-eval --dataset
  docs/research/critic-eval/pi-spine-v2-dataset.jsonl --json` from the repo
  root, so both load the same `.review/config.toml`
  (`systemone_base_url=http://127.0.0.1:8080`, `systemone_model=openjev-latest`,
  `systemone_timeout=90s`, `systemone_samples` default 1,
  `systemone_min_keep` default 0.5). OpenJev `/health` → 200 before both runs;
  LM Studio up for the later contract review. Per-example P(yes) scores come
  from an uncommitted `criticeval` test harness (`TestZZScoreDump`) run in each
  source tree with the same dataset, same server, samples 1 — 82 hunk groups, 89
  examples per run.
- **Out-of-view classifier (mechanical, fixed now):** a row is out-of-view iff
  its `message` matches, case-insensitively:
  `(missing|undefined|not defined|never defined|unused|not used|never used|not imported|never imported|not exported|never exported|undeclared|not declared|no import|not available|does not exist|doesn't exist|cannot find|no such|absent|lacks|without (an? )?(import|definition|export))`
  and does not match `missing a separator` (the one in-view punctuation claim,
  `pi-spine-SP-838-8653ab9`, labeled `keep`). Result: **19 out-of-view rows,
  all labeled `drop`** (ids listed at the bottom; dataset unchanged).
- **Validation rule (pre-registered in PROMPT.md; applied mechanically at
  threshold 0.5):**
  1. `new` drops at least as many `drop` rows as `old`, and strictly more of
     the 19 out-of-view rows.
  2. `new` drops no `keep` row that `old` keeps.
  Both hold → ship. Otherwise: at most two more wordings, each recorded here
  **before** it is tried; if none passes, keep the current criteria and change
  the Mission line to `Partial #20` in an Amendment. `systemone_min_keep`
  stays 0.5 (SP-081 sweeps it later).

## Baseline: old criteria (current main)

`$TMPDIR/sp080-old-report.json` (full sweep). At **0.5**:
`DropsCaughtPct 46.15%` → **36/78** drop rows dropped; `KeepsLostPct 18.18%` →
**2/11** keep rows lost. Per-example dump: `$TMPDIR/sp080-old-scores.jsonl`.

Keep rows lost by old (these are *allowed* to stay lost — rule 2 only forbids
new losses):

| Score | Row | Message |
|---|---|---|
| 0.227 | `pi-spine-SP-820-3295372` | provider-message fallback attributes an unrelated earlier marker line to the terminal 4xx |
| 0.385 | `pi-spine-SP-838-8653ab9` | JSDoc description for `providerCode` missing a separator (in-view punctuation claim) |

**Old drops 7/19 out-of-view rows.** The 12 out-of-view rows old keeps — the
concrete tuning target (scores 0.79–0.999; the critic agrees with the absence
claims because the shown context never shows the refuting code):

| Score | Row | Claim |
|---|---|---|
| 0.947 | `pi-spine-SP-827-cc988af` | `reviewLevel` may be `undefined`/`null`, not validated |
| 0.982 | `pi-spine-SP-829-5f9106b` | `fs` used without importing it |
| 0.999 | `pi-spine-SP-831-74dc631` | `compareTaskIds` import unused in the hunk |
| 0.939 | `pi-spine-SP-831-131e1ec` | `compareTaskIds` not defined/imported in visible context |
| 0.963 | `pi-spine-SP-831-0404db1` | same, "not defined or imported in the provided context" |
| 0.905 | `pi-spine-SP-831-3ebc640` | same |
| 0.996 | `pi-spine-SP-831-1757410` | same |
| 0.881 | `pi-spine-SP-832-4e406f4` | `resolveEffectiveReviewer` import unused |
| 0.796 | `pi-spine-SP-835-4b4b746` | `resolveWorkerBackend` import unused |
| 0.998 | `pi-spine-SP-835-6cc7bd9` | `prepareWorkerAttempt` import unused |
| 0.983 | `pi-spine-SP-838-7005bc1` | `ZAI_1308` not defined in added code or imports |
| 0.991 | `pi-spine-SP-845-8e13031` | implementation change not shown in the diff |

(The other 7 out-of-view rows old already drops: SP-821-n1, SP-821-n3,
SP-826-06d4730, SP-826-9dafa80, SP-827-d7b530c, SP-831-7cfc4f4,
SP-838-27f9724.)

## Wordings tried (record before each run)

| # | Wording (true / false) | drop rows dropped | keep rows lost | OOV drops /19 | Verdict |
|---|------------------------|-------------------|----------------|---------------|---------|
| 0 (baseline) | "The finding is correct and actionable for this code." / "The finding is incorrect, unsupported by this code, or not actionable." | 36/78 | 2/11 | 7 | baseline |
| 1 | "The finding is correct and actionable, and every claim it makes is verifiable from the code shown." / "The finding is incorrect, not actionable, or claims that code is missing, undefined, unused, not imported, or not exported in code the shown context does not include." | 38/78 | 2/11 | 8 | **FAIL** — condition 2: newly loses `SP-822-7ac810c` (0.612→0.244, "read without a size bound" — absence verifiable in the shown lines) and `SP-835-bf03c29` (0.862→0.0, comment/code mismatch in view); also re-keeps 4 drops old caught. OOV delta only 7→8; 11 OOV rows still kept (0.5–0.98). |
| 2 | "The finding is correct and actionable, and the code shown is enough to verify it." / "The finding is incorrect, not actionable, or its correctness depends on code the shown excerpt does not include — such as whether a symbol is defined, imported, used, or exported elsewhere." | 49/78 | 5/11 | 13 | **FAIL** — condition 1 passes with a large margin (OOV 7→13), but condition 2 fails: newly loses `SP-821-n7` (0.631→0.409, hardcoded-id claim in view), `SP-822-7ac810c` (0.612→0.075), `SP-835-bf03c29` (0.862→0.0). The broad "correctness depends on code not shown" clause made the decider conservative on every finding (keepsLost 45.45% vs old 18.18%). |
| 3 | "The finding is correct and actionable, and the shown code supports it." / "The finding is incorrect, not actionable, or asserts a symbol is undefined, unused, not imported, or not exported when the shown excerpt lacks the definition, import, or usage that would decide it." | 37/78 | 2/11 | 7 | **FAIL** — condition 1 fails (OOV 7→7, not strictly more; the symbol-existence rows stayed at 0.83–0.99 — the model reads "not used in the provided hunk" as settled by the excerpt) and condition 2 fails (`SP-822-7ac810c` 0.612→0.401). 5 drops old caught re-kept. |

**Lesson from v1** (recorded before drafting v2): a blanket "verifiable from the
shown code" clause makes the model treat absence claims it can *partially*
check in the excerpt ("unused in the provided hunk", "not defined in the
provided context") as verified, while punishing claims that are verifiable in
the shown lines themselves ("no size bound on this read"). v2 targets the
decisive property instead: whether the claim's correctness **depends on code
the excerpt does not include** (the rest of the file / other files).

**Lesson from v2** (recorded before drafting v3): scoping by *dependence on
unshown code* works for the symbol-existence family (OOV drops 7→13) but the
phrase "its correctness depends on code the shown excerpt does not include"
generalizes: the model second-guesses claims fully checkable in the shown
lines (hardcoded id, missing size bound, comment/code mismatch), losing three
keeps. v3 restricts the false clause to the concrete family itself — a symbol
asserted undefined / unused / not imported / not exported that the shown
excerpt cannot decide — leaving all other claim types under the original
criteria.

## Decision

**None of the three wordings passes the pre-registered rule — the current
criteria stay.** Applied mechanically at threshold 0.5:

| Wording | cond 1 (≥ drops, strictly more OOV) | cond 2 (no new keep loss) | Ship? |
|---|---|---|---|
| 1 | ✓ (38≥36, 8>7) | ✗ (loses 7ac810c, bf03c29) | no |
| 2 | ✓ (49≥36, 13>7) | ✗ (loses 821-n7, 7ac810c, bf03c29) | no |
| 3 | ✗ (OOV 7→7) | ✗ (loses 7ac810c) | no |

Per PROMPT.md: at most two more wordings after the first — all three failed —
so the shipped criteria remain the originals ("The finding is correct and
actionable for this code." / "The finding is incorrect, unsupported by this
code, or not actionable."), the Mission line becomes `Partial #20` via
Amendment, and SP-081 still sweeps `systemone_min_keep` on this dataset.

Observed pattern (for any future retuning attempt): OpenJev reliably scores
symbol-existence claims about the excerpt itself ("unused in the provided
hunk", "not defined in the provided context") 0.8–0.99 — it treats the excerpt
as the whole world, so criteria text alone did not flip those rows without
also suppressing in-view keeps (the binding constraint: 0f88739 sits exactly
at 0.500 under the old criteria, and 7ac810c/57dde22/bf03c29/821-n7 are
in-view keeps that every aggressive wording dropped). The v2 wording is the
best drop-precision candidate ever measured (49/78, OOV 13/19) but fails the
no-real-loss rule; a future task could revisit it together with reviewer-prompt
changes (out of SP-080 scope).

## Out-of-view row ids (classifier output, fixed)

`pi-spine-SP-821-n1`, `pi-spine-SP-821-n3`, `pi-spine-SP-826-06d4730`,
`pi-spine-SP-826-9dafa80`, `pi-spine-SP-827-cc988af`, `pi-spine-SP-827-d7b530c`,
`pi-spine-SP-829-5f9106b`, `pi-spine-SP-831-0404db1`, `pi-spine-SP-831-131e1ec`,
`pi-spine-SP-831-1757410`, `pi-spine-SP-831-3ebc640`, `pi-spine-SP-831-74dc631`,
`pi-spine-SP-831-7cfc4f4`, `pi-spine-SP-832-4e406f4`, `pi-spine-SP-835-4b4b746`,
`pi-spine-SP-835-6cc7bd9`, `pi-spine-SP-838-27f9724`, `pi-spine-SP-838-7005bc1`,
`pi-spine-SP-845-8e13031`
