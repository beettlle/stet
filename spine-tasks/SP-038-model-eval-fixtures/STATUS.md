**Current Step:** Complete
**Status:** Done — all completion criteria met
**Last Updated:** 2026-10-10
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 1: Go cases

**Status:** Done

- [x] 6 buggy Go cases + `go-clean-refactor`

## Step 2: TypeScript and Python cases

**Status:** Done

- [x] 3 buggy TS cases + `ts-clean-rename`
- [x] 3 buggy Python cases

## Step 3: Testing & Verification

**Status:** Done

- [x] Structural test over all 14 cases
- [x] Contract `testCommand`

---

## Contract Verification

| Check | Result |
|-------|--------|
| `python3 -m pytest scripts/tests/test_model_eval_cases.py -q` | 86 passed, 14 skipped (0.09s) |
| 14 case ids | all present, `expected.json` id == dir name |
| Bug lines in diff + control edit ≥ 10 lines away | asserted per buggy case by `test_bug_lines_in_diff_and_control_edit_far_away` |
| File scope | only `scripts/model-eval-cases/**` + `scripts/tests/test_model_eval_cases.py` touched |

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-10-10 | `scripts/tests/__pycache__/` and `scripts/optimizer/__pycache__/` `.pyc` files are tracked; pytest runs dirty the tree | Logged as tech-debt row in `spine-tasks/CONTEXT.md`; restored tracked `.pyc` to HEAD before each commit to keep diffs scoped |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-10-10 | Authored | Phase 7 model benchmark (fixtures for SP-039) |
| 2026-10-10 | Step 1 done | 7 Go cases (6 buggy + `go-clean-refactor`), diff geometry verified |
| 2026-10-10 | Step 2 done | 3 TS + 3 Python cases, diff geometry verified |
| 2026-10-10 | Step 3 done | `scripts/tests/test_model_eval_cases.py` added; contract command: 86 passed, 14 skipped; full `scripts/tests/`: 129 passed, 14 skipped |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
