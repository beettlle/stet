# Task: SP-081 — Sweep `systemone_min_keep` on the labeled dataset and set the default

**Created:** 2026-11-01
**Size:** S

## Review Level: 1

**Assessment:** One measurement run, a research note, and at most a one-constant default change with its test and doc row.
**Score:** 3/8 — Blast radius: 1, Pattern novelty: 0, Security: 0, Reversibility: 2 (default shift for every System One critic user) — Level 1 (the change is a single validated constant)

## Mission

Closes #34

`systemone_min_keep` (default `0.5`, `_defaultSystemOneMinKeep` in `cli/internal/config/config.go`) has never been tuned on labeled data. SP-079 built the dataset (`docs/research/critic-eval/pi-spine-v2-dataset.jsonl`), and SP-080 (#20) shipped the critic criteria this sweep must measure. Run `stet critic-eval` on the dataset with the System One backend, apply #34's pre-registered rule, and set the default from the result.

### Decision rule (pre-registered; apply mechanically from `critic-eval --json`)

`critic-eval --json` prints a `criticeval.Report` (fields `Counts`, `Thresholds`, `Recommended`, `Warnings`). It sweeps thresholds 0.1 to 0.9 and reports, per threshold, `DropsCaughtPct` (share of `drop` examples scored below it) and `KeepsLostPct` (share of `keep` examples scored below it). Its built-in `Recommended` value tolerates up to 10% keeps lost; #34's rule is stricter and is the one applied here.

1. **Reliability:** if the report carries the `recommendation unreliable` warning (fewer than 10 examples for either label), do **not** change the default. Record the table and the reason.
2. **Choose** `t*` = the highest threshold with `KeepsLostPct == 0` (no true positive lost). Ties do not arise (one value per step).
3. **Change the default** to `t*` only if `t* != 0.5` and either (a) `t* > 0.5` and `DropsCaughtPct(t*) > DropsCaughtPct(0.5)`, or (b) `t* < 0.5` because `0.5` loses a `keep` example. Otherwise keep `0.5`.

The sweep run and the applied rule close #34 even when the default stays `0.5`.

## Dependencies

- **Task:** SP-079 (dataset)
- **Task:** SP-080 (critic criteria under test)
- **External:** OpenJev (System One) at `http://127.0.0.1:8080`, nothing else using it during the run

## Context to Read First

**Tier 3 (reference docs):**

- `gh issue view 34 --repo beettlle/stet`
- `docs/research/critic-eval/README.md` (SP-079), `docs/research/critic-eval/criteria-sp080.md` (SP-080)
- `cli/internal/criticeval/sweep.go` (`ThresholdResult`, `Report`, `minReliableExamples`)
- `cli/internal/config/config.go` (`_defaultSystemOneMinKeep`), `cli/internal/config/config_test.go` (~line 1472)

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** OpenJev at `http://127.0.0.1:8080`; LM Studio for the contract stet review

## File Scope

- `docs/research/critic-eval/sweep-sp081.md` (new)
- `docs/research/critic-eval/sweep-sp081.json` (new: raw `critic-eval --json` output)
- `cli/internal/config/config.go` (only `_defaultSystemOneMinKeep`, only if the rule changes it)
- `cli/internal/config/config_test.go`
- `docs/cli-extension-contract.md` (only the `systemone_min_keep` row)

## Documentation Requirements

**Must Update:**

- `docs/research/critic-eval/sweep-sp081.md` — setup (stet commit, OpenJev model, samples), the full threshold table, the rule applied step by step, the decision, and a follow-up line for pi-spine's `.review/config.toml` guidance (not edited here)
- `docs/cli-extension-contract.md` — the `systemone_min_keep` row's default and a one-line pointer to the sweep note, only if the default changes

**Check If Affected:** `docs/review-process-internals.md` §7.10 — only if it states the threshold value (record under Discoveries; SP-080 owned that file this release).

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/config/... -count=1 && jq -e '.Thresholds' docs/research/critic-eval/sweep-sp081.json && grep -q '^## Decision' docs/research/critic-eval/sweep-sp081.md && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `docs/research/critic-eval/sweep-sp081.md`, `docs/research/critic-eval/sweep-sp081.json` |
| fileScopeMustNotChange | `cli/internal/review/**`, `cli/internal/criticeval/**`, `cli/internal/run/**`, `cli/internal/rules/**`, `cli/cmd/**`, `docs/research/critic-eval/pi-spine-v2-dataset.jsonl`, `extension/**`, `.review/**` |
| completionCriteria | `sweep-sp081.json` is the unedited `critic-eval --json` output; the note applies the rule step by step and ends with `## Decision`; if the default changes, `_defaultSystemOneMinKeep`, its config test, and the contract row all show the new value; result posted to #34 (`gh issue comment 34 --repo beettlle/stet --body-file …`, URL in STATUS); stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Run the sweep

- [ ] `go build -o "$TMPDIR/stet-sp081" ./cli/cmd/stet`; `STET_SYSTEMONE_BASE_URL=http://127.0.0.1:8080 "$TMPDIR/stet-sp081" critic-eval --dataset docs/research/critic-eval/pi-spine-v2-dataset.jsonl --json > docs/research/critic-eval/sweep-sp081.json`; on unreachable or unauthorized exit, Blocker and stop
- [ ] Commit the raw output (`docs(SP-081): critic-eval sweep output`)
- [ ] `spine_review_step` after the step

### Step 2: Apply the rule and set the default

- [ ] `sweep-sp081.md` with the table and the rule applied step by step; `## Decision`
- [ ] If the rule changes the default: `_defaultSystemOneMinKeep`, config test, and the contract row updated
- [ ] Post the decision table to #34
- [ ] `spine_review_step` after the step

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing
- [ ] Decision follows the pre-registered rule

## Git Commit Convention

- **Data / docs:** `docs(SP-081): description`
- **Default change:** `feat(SP-081): description`

## Do NOT

- Edit the dataset, the critic criteria, or the sweep code to change the outcome
- Change the threshold for any value the rule does not produce
- Edit pi-spine (`/Users/cdelgado/Documents/github/pi-spine`) or `.review/config.toml`
- Close or reopen #34 yourself (the release operator closes it after the wave lands)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
