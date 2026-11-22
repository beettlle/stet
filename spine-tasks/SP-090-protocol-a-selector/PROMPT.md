# Task: SP-090 — protocol-a-selector

**Created:** 2026-11-21
**Size:** M
**Type:** llm

## Review Level: 2 (Plan and Code)

**Risk:** Touches shared review pipeline (`cli/internal/run`, `cli/cmd/stet/main.go`) and System One selector path; wrong wiring can fail-open to digests or send bad choice options.
**Score:** 6/8 — Blast radius: 2, Pattern novelty: 1, Security: 0, Reversibility: 1

## Mission

Closes #40

Ship Protocol A for the rule-section selector: when `rules_selector` is on, candidate mode `digest_fair` builds digest sections (file-specific-first, round-robin into ≤64), choice option text prefers `**Detect:**` lines, and selection honors `rules_max_tokens` / `ReviewRuleBudget` (no hardcoded-only fill). Default `rules_selector_candidates` is **`digest_fair`**. Keep `rules_selector` **off** by default. Emit `STET_RULES_SECTION_LOG=1` stderr markers for included/selected sections. Prefer adapting the parked `protocol-a-wip` stash for the listed File Scope paths (code only — not docs/scripts owned by SP-089/SP-091).

## Dependencies

- **None** (docs SP-089 may land earlier or in parallel in a prior wave; do not edit docs)

## Context to Read First

- `cli/internal/rules/selector.go` — SelectSections / maxSelectorOptions
- `cli/internal/rules/criteria.go` — BuildReviewCriteria / digest round-robin
- `cli/internal/run/run.go` — selectRuleSectionsForHunk
- `docs/research/rules-selector-protocol-a.md` — if present after SP-089; else stash note
- Stash `protocol-a-wip` — restore code paths listed in File Scope

## Environment

- **Workspace:** `cli/`
- **Services required:** None for unit tests (OpenJev not required for contract tests)

## File Scope

- `cli/internal/rules/selector.go`
- `cli/internal/rules/selector_test.go`
- `cli/internal/config/config.go`
- `cli/internal/run/run.go`
- `cli/internal/review/review.go`
- `cli/cmd/stet/main.go`

## Contract

| Field | Value |
|-------|-------|
| `testCommand` | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/rules/... ./cli/internal/config/... ./cli/internal/review/... ./cli/internal/run/... -count=1` |
| `reviewCommand` | `bash scripts/spine-stet-contract-run.sh default SP-090` |
| `fileScopeMustChange` | `cli/internal/rules/selector.go`, `cli/internal/config/config.go`, `cli/internal/run/run.go` |
| `fileScopeMustNotChange` | `docs/**`, `scripts/**`, `extension/**`, `.github/**` |

## Steps

### Step 0: Protocol A candidate building + SelectSections
**Plan-review checkpoint**
Land `BuildSelectorCandidates` / `NormalizeSelectorCandidates` (`full` | `digest_fair`), Detect option text, and `SelectSections` signature updates with unit tests (fair 64 includes late file-specific digests; persona-only excluded; Detect in options).

> **Review override: code review**

### Step 1: Config + pipeline wiring
Wire `rules_selector_candidates` / `STET_RULES_SELECTOR_CANDIDATES` through config (default `digest_fair`), `reviewPipelineOpts` / `StartOptions` / `RunOptions`, and `cli/cmd/stet/main.go`. In `selectRuleSectionsForHunk`, build candidates via mode, pass `ReviewRuleBudget`, set Detect options for `digest_fair`. In `PrepareHunkPrompt`, honor budget for AppendRuleSections and log `stet: rules_sections kind=…` when `STET_RULES_SECTION_LOG=1` (labels joined with ` | `).

### Step 2: Testing & Verification
Run Contract `testCommand`. Ensure selector tests cover digest_fair fairness and Detect option text. Do not enable selector in `.review/config.toml`. Commit STATUS updates before `.DONE`. Do not commit `worker-usage.json` (gitignored).

## Completion Criteria

- [ ] `digest_fair` is the default candidates mode when selector is on
- [ ] Selector honors rules token budget; Detect option text for digest_fair
- [ ] Section-log stderr marker works when env is `1`
- [ ] Contract tests pass; reviewCommand advisory/blocking per spine policy

## Documentation Requirements

### Must Update
- None (owned by SP-089)

### Check If Affected
- `docs/cli-extension-contract.md` — if you discover missing keys, note in STATUS Discoveries for SP-089; do not edit docs here

## Do NOT

- Do NOT set `rules_selector = true` by default or in `.review/config.toml`
- Do NOT raise `maxReviewRuleTokens` / default `rules_max_tokens`
- Do NOT edit `scripts/model_eval.py` or `docs/**`
- Do NOT edit `.spine/`, `AGENTS.md`, `CLAUDE.md`, or `.gitnexus/`

## Git Commit Convention

Prefix commits with `feat(SP-090): ` at step boundaries.
