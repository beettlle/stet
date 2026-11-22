# Task: SP-091 — protocol-a-harness

**Created:** 2026-11-21
**Size:** S
**Type:** llm

## Review Level: 1 (Plan Only)

**Risk:** Eval harness only; wrong pinning can skew research runs, not production reviews.
**Score:** 3/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 0

## Mission

Partial #40 — Extend `scripts/model_eval.py` so release/research can reproduce the Protocol A A/B: `--rules-selector`, `--rules-selector-candidates {full,digest_fair}`, always pin `STET_RULES_SECTION_LOG=1`, parse section labels into meta, and score `section_hit_mean` / `section_hit_by_case` (continued-file stem match for SP-082 `expected.json` drift). Prefer adapting the `protocol-a-wip` stash for the listed scripts only.

## Dependencies

- **Task:** SP-090 (section-log marker and candidates config must exist on `main`)

## Context to Read First

- `scripts/model_eval.py` — `pinned_env`, `RunOptions`, `cmd_run`, `build_model_summary`
- `scripts/tests/test_model_eval.py` — pinned env / run args helpers
- `scripts/model-eval-cases/go-rule-context/expected.json` — `rule` field shape
- Stash `protocol-a-wip` — restore `scripts/model_eval.py` and `scripts/tests/test_model_eval.py` hunks

## Environment

- **Workspace:** `scripts/`
- **Services required:** None for unit tests

## File Scope

- `scripts/model_eval.py`
- `scripts/tests/test_model_eval.py`

## Contract

| Field | Value |
|-------|-------|
| `testCommand` | `PYTHONPATH=. python3 -m pytest scripts/tests/test_model_eval.py::TestPinnedEnv scripts/tests/test_model_eval.py::TestSectionHit -q` |
| `fileScopeMustChange` | `scripts/model_eval.py`, `scripts/tests/test_model_eval.py` |
| `fileScopeMustNotChange` | `cli/**`, `extension/**`, `.github/**` |

## Steps

### Step 0: Harness flags + section-hit scoring
**Plan-review checkpoint**
Add `--rules-selector` / `--rules-selector-candidates`, pin env (including System One URL when selector on and critic off), parse `stet: rules_sections` stderr into `meta["rules_sections"]`, implement `section_hit` / `parse_rules_sections`, and emit `section_hit_mean` in score summaries. Load optional `rule` on Bug from expected.json.

### Step 1: Testing & Verification
Add/adjust unit tests for pinned env and section-hit (exact + continued-file). Run Contract `testCommand`. If pytest path differs on the lane, use an equivalent `python3 -m pytest` with `PYTHONPATH=.` that the worker verifies locally — update STATUS with the command that passed. Commit STATUS updates before `.DONE`. Do not commit `worker-usage.json` (gitignored).

## Completion Criteria

- [ ] Flags pin `STET_RULES_SELECTOR` / `STET_RULES_SELECTOR_CANDIDATES` / `STET_RULES_SECTION_LOG`
- [ ] Score reports `section_hit_mean`
- [ ] Unit tests for section-hit matcher pass

## Documentation Requirements

### Must Update
- None

### Check If Affected
- `docs/research/rules-selector-protocol-a.md` — already authored in SP-089

## Do NOT

- Do NOT change `cli/**` product code
- Do NOT re-run the full 6-cell LLM A/B unless the operator asks (unit tests only)
- Do NOT edit `.spine/`, `AGENTS.md`, `CLAUDE.md`, or `.gitnexus/`

## Git Commit Convention

Prefix commits with `feat(SP-091): ` or `test(SP-091): ` at step boundaries.
