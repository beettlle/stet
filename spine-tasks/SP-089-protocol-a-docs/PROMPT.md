# Task: SP-089 — protocol-a-docs

**Created:** 2026-11-21
**Size:** S
**Type:** llm

## Review Level: 0 (None)

**Risk:** Docs only; wrong contract text misleads operators enabling the selector.
**Score:** 1/8 — Blast radius: 1, Pattern novelty: 0, Security: 0, Reversibility: 0

## Mission

Partial #40 — Document Protocol A rule selection for release v1.7.0: `rules_selector_candidates` / `STET_RULES_SELECTOR_CANDIDATES` (`full` | `digest_fair`), `STET_RULES_SECTION_LOG`, and how the selector interacts with the 1000-token digest budget. Commit the completed A/B research note (and summaries) from the parked `protocol-a-wip` stash. Do **not** recommend raising default `rules_max_tokens` or enabling bare `full` on CursorRules-heavy repos.

## Dependencies

- **None** (may land in parallel with SP-088; product code lands in SP-090)

## Context to Read First

- `docs/cli-extension-contract.md` — Cursor rules / rules_selector sections
- `docs/review-process-internals.md` — review prompt assembly
- `docs/research/rules-budget-ab.md` — prior budget decision (cite, do not rewrite)
- Stash `protocol-a-wip` — restore `docs/research/rules-selector-protocol-a.md` and `docs/research/rules-selector-protocol-a/` (research artifacts); also any contract hunks already drafted in the stash for docs only

## Environment

- **Workspace:** `docs/`
- **Services required:** None

## File Scope

- `docs/cli-extension-contract.md`
- `docs/review-process-internals.md`
- `docs/research/rules-selector-protocol-a.md`
- `docs/research/rules-selector-protocol-a/`

## Contract

| Field | Value |
|-------|-------|
| `testCommand` | `true` |
| `fileScopeMustChange` | `docs/cli-extension-contract.md`, `docs/research/rules-selector-protocol-a.md` |
| `fileScopeMustNotChange` | `cli/**`, `extension/**`, `.github/**` |

## Steps

### Step 0: Restore and finalize research note
From git stash `protocol-a-wip` (message match), restore only `docs/research/rules-selector-protocol-a.md` and `docs/research/rules-selector-protocol-a/**`. Ensure the note states: critic-off H(v) winner = Protocol A (0.75), do not use `full` selector, keep selector off by default, do not raise default budget.

### Step 1: Contract + internals docs
Update `docs/cli-extension-contract.md` with `rules_selector_candidates` / `STET_RULES_SELECTOR_CANDIDATES` and `STET_RULES_SECTION_LOG` behavior. Update `docs/review-process-internals.md` briefly so the review pipeline description matches (selector digest_fair vs round-robin digests). Prefer adapting stash hunks for the contract file over rewriting from scratch.

### Step 2: Testing & Verification
Confirm `rg -n 'rules_selector_candidates|digest_fair|STET_RULES_SECTION_LOG' docs/cli-extension-contract.md` hits. Confirm research note path exists. Note: no test suite references these docs unless `rg` finds any — if none, record that in STATUS. Commit STATUS updates before `.DONE`. Do not commit `worker-usage.json` (gitignored).

## Completion Criteria

- [ ] Contract documents `digest_fair` vs `full` and section-log env
- [ ] Research note + artifacts committed under `docs/research/rules-selector-protocol-a*`
- [ ] Internals doc mentions Protocol A packing when selector is on

## Documentation Requirements

### Must Update
- `docs/cli-extension-contract.md`
- `docs/research/rules-selector-protocol-a.md`

### Check If Affected
- `docs/review-process-internals.md` (Must Update if behavior section exists — keep in File Scope)
- `README.md` — only if user-facing rules config is listed there

## Do NOT

- Do NOT change Go/product code (`cli/**`) — that is SP-090
- Do NOT change `scripts/model_eval.py` — that is SP-091
- Do NOT enable `rules_selector` in `.review/config.toml`
- Do NOT edit `.spine/`, `AGENTS.md`, `CLAUDE.md`, or `.gitnexus/`

## Git Commit Convention

Prefix commits with `docs(SP-089): ` at step boundaries.
