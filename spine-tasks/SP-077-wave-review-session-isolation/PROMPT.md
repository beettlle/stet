# Task: SP-077 — Wave review shows only findings from the reviewed session

**Created:** 2026-11-01
**Size:** S

## Review Level: 2

**Assessment:** Bug fix in review-session state; the fix may land in `cli/cmd/stet/main.go` (hot shared module) or the session package, so Level 2.
**Score:** 4/8 — Blast radius: 2, Pattern novelty: 1, Security: 0, Reversibility: 1 — Level 2 for the hot module

## Mission

Closes #5

On 2026-10-11, `bash scripts/stet-wave-review.sh orch/spine-20261011T202931-a926` listed 28 active findings, 21 of them in `cli/` files, although `git diff main...orch/spine-20261011T202931-a926 -- cli` was empty. The `cli/` findings matched commits that `scripts/model_eval.py run` had just reviewed in temporary clones (SP-034 `3ee3055`, SP-033 `2326c04` / `e9b2e61`). Until this is fixed, wave-review counts after a model-eval run cannot be trusted (`spine-tasks/CONTEXT.md`, tech-debt row dated 2026-10-11 (SP-039)).

`stet-wave-review.sh` runs `stet start <merge-base> --allow-dirty --quiet --json --strictness default` in a fresh `git worktree add --detach` directory, then `stet list`. By default stet keeps state in `<git rev-parse --show-toplevel>/.review` (`Config.EffectiveStateDir` in `cli/internal/config/config.go`), which should already be per worktree. So the leak comes from something the code reads outside that directory, or from how `stet start` / `stet list` select findings. Find it, fix it, and pin it with a regression test.

Leads to check (verify each; do not assume):

1. Anything `stet start` or `stet list` reads outside the state dir: `refs/notes/stet` (shared by all worktrees of one repository), `history.jsonl`, restored dismissals (`cli/internal/run/restore_dismissals.go`), the digest cache, a global or env `state_dir` (`STET_STATE_DIR`), or a path derived from `--git-common-dir`.
2. Whether `stet list` filters findings to the current session's diff (base ref to HEAD plus dirty files), or prints every finding in `session.json`.
3. Whether `stet start` on a worktree with an existing session (or a stale `.review/session.json` that `git worktree add` or the harness clones could carry) appends to old findings instead of starting fresh.
4. Whether the script itself picks the wrong base (`git merge-base main "$branch"` against a stale local `main`) so the reviewed range includes commits outside the wave.

## Dependencies

- **None**

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md` — "Stet in spine (dogfooding)" and the 2026-10-11 tech-debt row for #5

**Tier 3 (reference docs):**

- `scripts/stet-wave-review.sh`
- `cli/cmd/stet/main.go` — `runStart`, `runList` (~line 1444), state-dir resolution
- `cli/internal/session/session.go`, `cli/internal/config/config.go` (`EffectiveStateDir`)
- `scripts/model_eval.py` — `git clone --quiet --no-hardlinks` of the source repo (~line 361), to reproduce the clone side

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** none for the fix (`stet start --dry-run` avoids the LLM); LM Studio only for the contract stet review

## File Scope

- `cli/cmd/stet/main.go`
- `cli/cmd/stet/main_test.go`
- `cli/internal/session/session.go`
- `cli/internal/session/session_test.go`
- `scripts/stet-wave-review.sh`
- `spine-tasks/CONTEXT.md` (remove or mark resolved the #5 caveat row only)

## Documentation Requirements

**Must Update:** `spine-tasks/CONTEXT.md` — mark the 2026-10-11 #5 tech-debt row resolved by SP-077 with the root cause in one sentence.

**Check If Affected:** `docs/cli-extension-contract.md` — only if `stet list` output semantics change (record under Discoveries; SP-080 owns that file in this release).

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/session/... ./cli/cmd/stet/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `spine-tasks/CONTEXT.md` |
| fileScopeMustNotChange | `cli/internal/review/**`, `cli/internal/rules/**`, `cli/internal/criticeval/**`, `scripts/model_eval.py`, `scripts/model-eval-cases/**`, `docs/research/**`, `extension/**`, `.review/**` |
| completionCriteria | Step 1 records a reproduction (commands and the leaked finding count) and the root cause in STATUS Discoveries; a regression test reviews one range in one worktree or clone, then a disjoint range in another, and asserts the second `stet list` shows no finding from the first; the original repro shows 0 leaked findings after the fix; stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Reproduce and find the root cause

- [ ] Reproduce with `--dry-run` (no LLM): in a scratch repo, review a range in a clone and in a worktree, then run the wave-review flow on a branch with a disjoint diff; record commands and leaked count in Discoveries
- [ ] Root cause identified against the leads in Mission and recorded in Discoveries (file and function)
- [ ] `spine_review_step` after the step

### Step 2: Fix

- [ ] Smallest fix at the root cause (state isolation, list filtering, fresh-session start, or the script's base selection); if the script was at fault, fix it there and keep stet unchanged
- [ ] `spine_review_step` after the step

### Step 3: Regression test and CONTEXT note

- [ ] Go test at the session or command level reproducing the cross-session leak (fails before the fix, passes after)
- [ ] `spine-tasks/CONTEXT.md` #5 row marked resolved by SP-077
- [ ] `spine_review_step` after the step

### Step 4: Testing & Verification

> ZERO test failures allowed.

- [ ] Tests for every case in Contract `completionCriteria`
- [ ] Re-run the Step 1 repro: 0 leaked findings
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Fix:** `fix(SP-077): description`
- **Tests:** `test(SP-077): description`

## Do NOT

- Change review, critic, rules, or prompt behavior
- Change the default `state_dir` location for normal (non-worktree) repos without recording why in Amendments
- Run stet against the lane worktree itself for the repro (use a scratch repo under `$TMPDIR`)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
