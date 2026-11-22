# Task: SP-089 — protocol-a-docs

**Created:** 2026-11-21
**Size:** S
**Type:** llm
**Profile:** hard

## Review Level: 0 (None)

**Risk:** Docs only; wrong contract text misleads operators enabling the selector.
**Score:** 1/8 — Blast radius: 1, Pattern novelty: 0, Security: 0, Reversibility: 0

## Mission

Partial #40 — Document Protocol A rule selection for release v1.7.0: `rules_selector_candidates` / `STET_RULES_SELECTOR_CANDIDATES` (`full` | `digest_fair`), default **`digest_fair`**, `STET_RULES_SECTION_LOG`, and how the selector interacts with the 1000-token digest budget. The research note and summary JSON files are **already on `main`** under `docs/research/rules-selector-protocol-a*`. Do **not** restore or commit any `results-*` trees. Do **not** recommend raising default `rules_max_tokens` or enabling bare `full` on CursorRules-heavy repos.

## Dependencies

- **None**

## Context to Read First

- `docs/cli-extension-contract.md` — Cursor rules / rules_selector sections
- `docs/review-process-internals.md` — §7.3 Cursor rules
- `docs/research/rules-selector-protocol-a.md` — A/B decision (already landed)
- Stash `protocol-a-wip` — **docs/cli-extension-contract.md only** (adapt the `rules_selector_candidates` / `STET_RULES_SECTION_LOG` hunks; change default from `full` to `digest_fair`)

## Environment

- **Workspace:** `docs/`
- **Services required:** None

## File Scope

- `docs/cli-extension-contract.md`
- `docs/review-process-internals.md`

## Contract

| Field | Value |
|-------|-------|
| `testCommand` | `true` |
| `fileScopeMustChange` | `docs/cli-extension-contract.md`, `docs/review-process-internals.md` |
| `fileScopeMustNotChange` | `cli/**`, `extension/**`, `.github/**`, `scripts/**` |

## Steps

### Step 0: Contract + section-log docs
Adapt stash hunks into `docs/cli-extension-contract.md`: add `rules_selector_candidates` / `STET_RULES_SELECTOR_CANDIDATES` with **default `digest_fair`** (Protocol A), document `full`, and document `STET_RULES_SECTION_LOG=1`. Do **not** `git stash` restore of `docs/research/rules-selector-protocol-a/**` (especially never `results-*/**`).

### Step 1: Internals §7.3
Update `docs/review-process-internals.md` §7.3 so selector packing mentions `digest_fair` vs `full`, budget via `ReviewRuleBudget`, and the section-log env. Cross-link `docs/research/rules-selector-protocol-a.md`.

### Step 2: Testing & Verification
Confirm `rg -n 'rules_selector_candidates|digest_fair|STET_RULES_SECTION_LOG' docs/cli-extension-contract.md docs/review-process-internals.md` hits. Confirm `docs/research/rules-selector-protocol-a.md` exists on disk. Commit STATUS before `.DONE`. Do not commit `worker-usage.json` (gitignored).

## Completion Criteria

- [ ] Contract documents `digest_fair` (default) vs `full` and section-log env
- [ ] Internals doc mentions Protocol A packing when selector is on
- [ ] Research note already present (do not add results trees)

## Documentation Requirements

### Must Update
- `docs/cli-extension-contract.md`
- `docs/review-process-internals.md`

### Check If Affected
- `docs/research/rules-selector-protocol-a.md` — already on main; do not rewrite
- `README.md` — only if user-facing rules config is listed there

## Do NOT

- Do NOT restore or commit `docs/research/rules-selector-protocol-a/results-*/**`
- Do NOT change Go/product code (`cli/**`) — that is SP-090
- Do NOT change `scripts/model_eval.py` — that is SP-091
- Do NOT enable `rules_selector` in `.review/config.toml`
- Do NOT edit `.spine/`, `AGENTS.md`, `CLAUDE.md`, or `.gitnexus/`

## Git Commit Convention

Prefix commits with `docs(SP-089): ` at step boundaries.
