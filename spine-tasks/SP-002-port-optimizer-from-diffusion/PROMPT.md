# Task: SP-002 — Port optimizer sidecar from diffusion branch

**Created:** 2026-06-30
**Size:** M

## Review Level: 1

**Assessment:** Merge/port work with an existing, tested implementation on `diffusion`; low novelty, moderate integration surface.
**Score:** 3/8 — Blast radius: 1, Pattern novelty: 0, Security: 1, Reversibility: 1

## Mission

Bring the **existing** SDRE optimizer sidecar from the `diffusion` branch into `main` — do **not** reimplement rule-based optimization or duplicate completed spine work (`SP-004`, `SP-012`, `SP-027` on `diffusion` are already `.DONE` there).

After port: `stet optimize` runs `scripts/optimize.py`, reads `.review/history.jsonl`, and writes `.review/system_prompt_optimized.txt` per the sidecar contract.

## Dependencies

- **None**

## Context to Read First

- `diffusion` branch: `scripts/optimize.py`, `scripts/optimizer/**`, `scripts/tests/test_optimize_integration.py`, `optimizer-requirements.txt`
- `diffusion`: `specs/001-implement-sdre-docs/contracts/optimizer-sidecar.md`
- `cli/cmd/stet/main.go` — `runOptimize` (already invokes `STET_OPTIMIZER_SCRIPT`)
- `docs/implementation-plan.md` Phase 4.1 / Phase 6.10 (optimizer docs)
- Completed on `diffusion`: `spine-tasks/SP-004-optimizer-entrypoint/`, `SP-012-rule-based-optimizer/`, `SP-027-pin-corpus-optimize/` (reference only; do not re-execute)

## Environment

- **Workspace:** stet monorepo
- **Python:** 3.x; `pip install -r optimizer-requirements.txt` (stdlib-only V0 on diffusion)
- **Go:** existing `stet optimize` tests in `cli/cmd/stet/main_test.go`

## File Scope

- `scripts/optimize.py`
- `scripts/optimizer/**`
- `scripts/tests/**`
- `optimizer-requirements.txt`
- `specs/001-implement-sdre-docs/contracts/optimizer-sidecar.md`
- `docs/cli-extension-contract.md` (optimizer section only, if absent)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `python3 -m pytest scripts/tests/test_optimize_integration.py scripts/tests/test_history_loader.py scripts/tests/test_rule_based.py scripts/tests/test_prompt_validator.py -q && cd cli && go test ./cmd/stet/... -run optimize -count=1` |
| fileScopeMustChange | `scripts/optimize.py`, `optimizer-requirements.txt` |
| fileScopeMustNotChange | `cli/**`, `extension/**` |
| completionCriteria | Sidecar contract satisfied; empty history no-op exit 0; valid dismissals write optimized prompt; Go `stet optimize` integration tests pass |

## Steps

### Step 0: Discovery on diffusion

- [ ] Check out or diff `diffusion` vs `main` for optimizer paths listed in File Scope
- [ ] Confirm `SP-004` / `SP-012` / `SP-027` deliverables exist on `diffusion` and note commit range to cherry-pick
- [ ] Verify `runOptimize` on `main` already shells out to `STET_OPTIMIZER_SCRIPT`; document default path to set (`python3 scripts/optimize.py`)

### Step 1: Port sidecar and tests

- [ ] Cherry-pick or copy optimizer files from `diffusion` without reimplementing rule-based logic
- [ ] Add `optimizer-requirements.txt` and port contract doc if missing
- [ ] Ensure default config/docs point operators at `scripts/optimize.py`

### Step 2: Wire defaults and documentation

- [ ] Document `stet optimize` usage in `docs/cli-extension-contract.md` (when to run, inputs, outputs, exit codes)
- [ ] Confirm `.review/system_prompt_optimized.txt` precedence matches `cli/internal/prompt/prompt.go`

### Step 3: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Run Python tests listed in Contract `testCommand`
- [ ] Run Go optimize tests: `cd cli && go test ./cmd/stet/... -run optimize -count=1`
- [ ] Manual smoke: fixture `history.jsonl` with dismissals → `stet optimize` writes optimized prompt

## Completion Criteria

- [ ] All steps complete
- [ ] Optimizer ported from `diffusion`; no duplicate reimplementation of rule-based optimizer
- [ ] Contract tests pass

## Git Commit Convention

- **Implementation:** `feat(SP-002): description`
- **Port/merge:** `chore(SP-002): port optimizer from diffusion`

## Do NOT

- Reimplement optimizer logic from scratch
- Duplicate `SP-004` / `SP-012` / `SP-027` work already done on `diffusion`
- Add DSPy dependency in this task (deferred on diffusion to later phase)
- Modify `extension/**`

---

## Amendments (Added During Execution)
