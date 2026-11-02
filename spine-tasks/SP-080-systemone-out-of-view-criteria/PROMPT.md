# Task: SP-080 — System One critic rejects out-of-view "missing / undefined" claims

**Created:** 2026-11-01
**Size:** M

## Review Level: 2

**Assessment:** Changes what the System One critic drops on every review that enables it; behavior change in a shared review path validated against labeled data.
**Score:** 5/8 — Blast radius: 2, Pattern novelty: 1, Security: 0, Reversibility: 2 (silent precision/recall shift for every critic user) — Level 2

## Mission

Closes #20

The System One critic asks one yes/no question per finding (`cli/internal/review/critic_systemone.go`):

- true: `"The finding is correct and actionable for this code."`
- false: `"The finding is incorrect, unsupported by this code, or not actionable."`

**Mission (amended 2026-11-02): `Partial #20`** — none of the three validated
wordings passed the pre-registered rule; see Amendments.

Claims such as "X is not defined / not imported / not exported / unused" about code outside the shown context are hard for the model to call false, so the critic keeps them (pi-spine release: SP-823, SP-826, SP-829, SP-830, SP-831, SP-832, SP-837, SP-843). SP-069 (#18) gave the critic the reviewer's expanded context; SP-070 (#33) drops existence claims the file itself disproves. This task rewords the criteria so that **a claim that something is absent, undefined, or wrong in code the shown context does not include counts as unsupported (false)**, and validates the wording on labeled data before shipping it.

### Inputs from wave 1

- `docs/research/critic-context-sp831.md` (SP-078): which out-of-view claims the critic still keeps with expanded context, and their critic scores.
- `docs/research/critic-eval/pi-spine-v2-dataset.jsonl` (SP-079): labeled `keep` / `drop` findings.

### Validation rule (pre-registered; apply mechanically)

Build two stet binaries, `old` (current `main`) and `new` (this branch), and run `stet critic-eval --dataset docs/research/critic-eval/pi-spine-v2-dataset.jsonl --json` with each against the same OpenJev server. At the default threshold 0.5:

1. **Precision on drops:** `new` drops at least as many `drop` examples as `old`, and strictly more of the out-of-view ones (rows whose message claims something is missing, undefined, unused, not imported, or not exported).
2. **No real loss:** `new` drops no `keep` example that `old` keeps.

- Both hold → ship the new wording.
- Otherwise try at most two more wordings (record each and its numbers in the research note **before** trying the next). If none passes, keep the current criteria, record the results, and change the Mission line to `Partial #20` in an Amendment.
- Do not change `systemone_min_keep` here (SP-081 sweeps it after this lands).

## Dependencies

- **Task:** SP-078 (critic baseline with expanded context)
- **Task:** SP-079 (labeled dataset)
- **External:** OpenJev (System One) at `http://127.0.0.1:8080` for the validation runs

## Context to Read First

**Tier 3 (reference docs):**

- `gh issue view 20 --repo beettlle/stet` (proposal and acceptance)
- `cli/internal/review/critic_systemone.go`, `cli/internal/review/critic_systemone_test.go`
- `cli/internal/systemone/` — `NoulCriteria` request shape
- `cli/internal/criticeval/sweep.go` — report fields (`--json`)
- `docs/research/critic-context-sp831.md`, `docs/research/critic-eval/README.md` (wave 1 outputs)
- `docs/review-process-internals.md` §7.10 (critic behavior)

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** OpenJev at `http://127.0.0.1:8080` (validation), LM Studio for the contract stet review

## File Scope

- `cli/internal/review/critic_systemone.go`
- `cli/internal/review/critic_systemone_test.go`
- `docs/research/critic-eval/criteria-sp080.md` (new: wordings tried, numbers, decision)
- `docs/cli-extension-contract.md`
- `docs/review-process-internals.md`

## Documentation Requirements

**Must Update:**

- `docs/cli-extension-contract.md` — where the System One critic is described (the `--critic-backend` paragraph near the top or the `critic_backend` row): one sentence on what the critic now treats as unsupported
- `docs/review-process-internals.md` — §7.10 System One critic bullet: the criteria text
- `docs/research/critic-eval/criteria-sp080.md` — as listed in File Scope

**Check If Affected:** `README.md` — only if it quotes the critic criteria (record under Discoveries; do not edit).

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/review/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `docs/research/critic-eval/criteria-sp080.md`, `docs/review-process-internals.md` |
| fileScopeMustNotChange | `cli/internal/run/**`, `cli/internal/config/**`, `cli/internal/rules/**`, `cli/internal/criticeval/**`, `cli/internal/review/critic.go`, `cli/internal/review/rules_annotation.go`, `cli/cmd/**`, `docs/research/critic-eval/pi-spine-v2-dataset.jsonl`, `extension/**`, `.review/**` |
| completionCriteria | The research note records old and new `critic-eval --json` results at 0.5 and applies the validation rule; if the new wording ships, a test asserts each System One question carries the new true/false criteria and the rules-annotation request is unchanged; docs describe the shipped criteria; stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Baseline with the current criteria

- [ ] `go build -o "$TMPDIR/stet-old" ./cli/cmd/stet` on the unchanged branch; `"$TMPDIR/stet-old" critic-eval --dataset docs/research/critic-eval/pi-spine-v2-dataset.jsonl --json` saved and summarized in `criteria-sp080.md`, with the out-of-view rows identified
- [ ] `spine_review_step` after the step

### Step 2: New criteria wording

- [ ] Reword `systemOneCriteriaTrue` / `systemOneCriteriaFalse` so claims of absence or incorrectness about code not in the shown context are false; keep both strings short and free of hunk text
- [ ] Unit test: questions carry the new criteria; annotation path unchanged
- [ ] `spine_review_step` after the step

### Step 3: Validate and document

- [ ] Build `new`, run the same `critic-eval`, apply the validation rule (up to two more wordings if needed, each recorded first)
- [ ] Update `docs/cli-extension-contract.md` and `docs/review-process-internals.md` §7.10 for the shipped criteria
- [ ] `spine_review_step` after the step

### Step 4: Testing & Verification

> ZERO test failures allowed.

- [ ] Tests for every case in Contract `completionCriteria`
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing
- [ ] Decision follows the pre-registered validation rule

## Git Commit Convention

- **Feature:** `feat(SP-080): description`
- **Tests:** `test(SP-080): description`
- **Docs:** `docs(SP-080): description`

## Do NOT

- Change `systemone_min_keep`, the LLM critic (`critic.go`), the reviewer prompt, or the rules-annotation criteria
- Edit the SP-079 dataset to make a wording pass
- Add a new deterministic filter (SP-070 already covers existence claims the file disproves)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)

**Amendment 1 (2026-11-02) — Mission is `Partial #20`:** the pre-registered
validation rule (docs/research/critic-eval/criteria-sp080.md) was applied
mechanically to three wordings (initial + two more, each recorded before its
run), at threshold 0.5 against `stet-old` = main `a90293d`:

| Wording | cond 1 (≥ drops, strictly more out-of-view) | cond 2 (no new keep loss) |
|---|---|---|
| 1 ("verifiable from the code shown") | ✓ 38≥36, OOV 8>7 | ✗ loses SP-822-7ac810c, SP-835-bf03c29 |
| 2 ("correctness depends on code not shown") | ✓ 49≥36, OOV 13>7 | ✗ loses SP-821-n7, SP-822-7ac810c, SP-835-bf03c29 |
| 3 ("symbol asserted undefined/unused/… that the excerpt cannot decide") | ✗ OOV 7→7 | ✗ loses SP-822-7ac810c |

None passes → per the rule, the current criteria stay and this task delivers
the validated baseline, the recorded rewording attempts, and drift-guard tests
pinning the shipped criteria. #20's premise is confirmed on the labeled
dataset (12 of 19 out-of-view rows score 0.79–0.999 under the shipped
criteria); the remaining lever is the SP-081 `systemone_min_keep` sweep.
