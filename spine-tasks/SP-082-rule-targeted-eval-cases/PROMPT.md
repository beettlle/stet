# Task: SP-082 — Rule-targeted model-eval cases and harness rule options

**Created:** 2026-11-01
**Size:** M

## Review Level: 1

**Assessment:** Test fixtures plus two opt-in harness flags in `scripts/model_eval.py`; no stet product code.
**Score:** 3/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 1

## Mission

Partial #11

Commit `1a1782e` shipped Cursor rule review digests (file-specific rules first, budget `rules_max_tokens`, default 15% of `context_limit` clamped to [1000, 8192]). The A/B eval posted on #11 showed **no recall gain**, because every planted defect in `scripts/model-eval-cases/` is caught by generic review without any rules, and the scaled budget cost 2–6x latency. Before SP-083 can size the budget, the eval needs (1) cases where only a rule identifies the defect, and (2) harness control over which rules a case repo has and what budget stet uses.

### Part A — harness flags (`scripts/model_eval.py run`)

- `--rules-dir DIR`: copy every `DIR/*.mdc` into `<case repo>/.cursor/rules/` as part of the **before** commit in `materialize_case_repo`, so the rules are present but not in the reviewed diff. Fixture cases only; real refs keep their own tree. Default: no rules (current behavior unchanged).
- `--rules-max-tokens N`: set `STET_RULES_MAX_TOKENS=N` in the pinned env (`pinned_env`). Default: unset, so stet uses its scaled default (current behavior unchanged).
- Record both values in each run's `meta.json` (`_build_meta`).
- Unit tests in `scripts/tests/test_model_eval.py`: rules land in the before commit and not in the after diff; env carries the budget only when set; meta records both.

### Part B — rule-targeted cases (`scripts/model-eval-cases/`)

Add 4 seeded cases and 2 clean controls, using the existing layout (`before/` and `after/` trees with a `.txt` suffix on every file, and `expected.json`):

- `go-rule-*` ×2 and `py-rule-*` ×2 (seeded): each defect violates one specific section of `.cursor/rules/go-1-21-development-standards.mdc` or `.cursor/rules/python-3-development-standards.mdc` whose `❌ Bad:` or `**Detect:**` line names it. It must be something a generic reviewer would usually accept as working code, for example `context.Background()` created inside a request path (Go §1.3 Context Propagation), a mutable package-level global without the `_` prefix (Go §2.1), or the Python rules' equivalents. Avoid defects the generic categories already catch (crashes, injection, races, swallowed errors).
- `go-rule-clean`, `py-rule-clean` (clean controls): a refactor of similar size that follows the same rules.
- Each seeded bug in `expected.json` uses `"category": "rule"` and adds `"rule": "<rule file>#<section number>"`. The harness ignores extra bug keys; the case tests must accept them.
- Follow the existing fixture constraints in `scripts/tests/test_model_eval_cases.py`: 30–120 lines per file, no hint words (`bug`, `todo`, `fixme`, `xxx`, `hack`, `unsafe`), and the diff-geometry checks. Extend `EXPECTED_IDS` and `CATEGORIES` (add `rule`) for the new cases.

## Dependencies

- **None**

## Context to Read First

**Tier 3 (reference docs):**

- `gh issue view 11 --repo beettlle/stet` (the A/B comment)
- `scripts/model_eval.py` — `materialize_case_repo` (~line 339), `pinned_env` (~line 295), `_build_meta` (~line 531), `build_parser`
- `scripts/tests/test_model_eval.py`, `scripts/tests/test_model_eval_cases.py`
- `scripts/model-eval-cases/go-swallowed-error/` (layout example)
- `.cursor/rules/go-1-21-development-standards.mdc`, `.cursor/rules/python-3-development-standards.mdc` (sections with `❌ Bad:` / `**Detect:**` lines)
- `cli/internal/rules/digest.go` — what reaches the prompt from each rule (heading, `❌ Bad:`, `**Detect:**`)

## Environment

- **Workspace:** repo root
- **Services required:** none (no LLM; SP-083 runs the eval)

## File Scope

- `scripts/model_eval.py`
- `scripts/tests/test_model_eval.py`
- `scripts/tests/test_model_eval_cases.py`
- `scripts/model-eval-cases/go-rule-*/**` (new)
- `scripts/model-eval-cases/py-rule-*/**` (new)
- `scripts/model-eval-cases/go-rule-clean/**` (new)
- `scripts/model-eval-cases/py-rule-clean/**` (new)

## Documentation Requirements

**Must Update:** none (each seeded case documents its rule in `expected.json`; SP-083 writes the research note).

**Check If Affected:** `docs/efficacy-tests.md` — only if it lists the fixture cases (record under Discoveries; do not edit).

## Contract

| Field | Value |
|-------|-------|
| testCommand | `python3 -m pytest scripts/tests/test_model_eval.py scripts/tests/test_model_eval_cases.py -q && python3 scripts/model_eval.py run --cases scripts/model-eval-cases --out /tmp/sp082-dry --runs 1 --dry-run --rules-dir .cursor/rules --rules-max-tokens 2000 --max-minutes 10 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `scripts/model_eval.py`, `scripts/tests/test_model_eval.py`, `scripts/tests/test_model_eval_cases.py` |
| fileScopeMustNotChange | `cli/**`, `extension/**`, `.review/**`, `.cursor/**`, `docs/research/**`, `scripts/model-eval-cases/go-swallowed-error/**`, `scripts/model-eval-cases/go-clean-refactor/**`, `scripts/model-eval-cases/ts-clean-rename/**` |
| completionCriteria | Both flags work and default to current behavior (tests); 4 seeded rule cases and 2 clean controls pass the case tests; each seeded bug names its rule section; a `--dry-run` harness pass over all cases with `--rules-dir .cursor/rules` exits 0; existing cases unchanged; stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). The seeded cases contain deliberate rule violations: dismiss findings on them as `out_of_scope` with the note "seeded fixture". Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect in harness code without fixing it.

## Steps

### Step 1: Harness flags

- [ ] `--rules-dir` and `--rules-max-tokens` on `run`; rules in the before commit only; meta records both; defaults unchanged
- [ ] Unit tests for both flags
- [ ] `spine_review_step` after the step

### Step 2: Seeded rule cases

- [ ] `go-rule-*` ×2 and `py-rule-*` ×2 with `"category": "rule"` and a `"rule"` key per bug
- [ ] `spine_review_step` after the step

### Step 3: Clean controls and case tests

- [ ] `go-rule-clean`, `py-rule-clean`; `EXPECTED_IDS` and `CATEGORIES` extended; case tests pass
- [ ] `spine_review_step` after the step

### Step 4: Testing & Verification

> ZERO test failures allowed.

- [ ] Tests for every case in Contract `completionCriteria`
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Feature:** `feat(SP-082): description`
- **Fixtures / tests:** `test(SP-082): description`

## Do NOT

- Change stet code, rule files under `.cursor/rules/`, or existing fixture cases
- Change scoring, the decision window, or pinned env values other than adding the optional budget
- Run the LLM eval (SP-083 does)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
