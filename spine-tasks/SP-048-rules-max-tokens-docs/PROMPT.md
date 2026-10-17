# Task: SP-048 — Document the rule digest and `rules_max_tokens`

**Created:** 2026-10-17
**Size:** S

## Review Level: 0

**Assessment:** Docs-only; describes behavior already on `main` (commit `1a1782e`).
**Score:** 0/8 — Blast radius: 0, Pattern novelty: 0, Security: 0, Reversibility: 0

## Mission

Partial #11

Commit `1a1782e` added cached Cursor rule review digests, file-specific-first rule ordering, and a scaled rule token budget, but no user-facing doc mentions them. Document them in the configuration contract and the README.

Facts to document (verify each against the code before writing; the code wins if this text disagrees):

- Config key `rules_max_tokens`, env `STET_RULES_MAX_TOKENS`, flag `--rules-max-tokens` on `stet start` and `stet run` (`cli/internal/config/config.go` ~lines 29, 125–127, 630; `cli/cmd/stet/main.go` ~lines 298, 516).
- Default `0` means 15% of `context_limit`, clamped to [1000, 8192] tokens. A positive value overrides it. Precedence: config file, then env, then CLI flag.
- Each rule is reduced to a review digest: per section, the heading, `❌ Bad:` lines, and `**Detect:**` lines; sections with neither are dropped; a rule with no markers falls back to its full content (`cli/internal/rules/digest.go`).
- Digests are cached in `<state_dir>/rules-digest-cache.json`, keyed by absolute rule path and invalidated by size, modification time, and sha256 (`cli/internal/rules/digestcache.go`). A corrupt or missing cache is rebuilt and never fails the review.
- Rules matched by glob come before `alwaysApply` rules. The budget cuts only at section boundaries; dropped sections produce one warning per run and are listed in the trace.

## Dependencies

- **None**

## Context to Read First

**Tier 3 (reference docs):**

- `docs/cli-extension-contract.md` — configuration table (~lines 120–150)
- `README.md` — configuration and flags sections (~lines 100–140)
- `cli/internal/rules/digest.go`, `cli/internal/rules/digestcache.go`, `cli/internal/config/config.go`

## Environment

- **Workspace:** repo root
- **Services required:** None

## File Scope

- `docs/cli-extension-contract.md`
- `README.md`

## Documentation Requirements

**Must Update:**

- `docs/cli-extension-contract.md` — add a `rules_max_tokens` / `STET_RULES_MAX_TOKENS` row to the configuration table and a short "Cursor rules in the review prompt" paragraph (digest, cache file, ordering, budget, warning)
- `README.md` — one line or table row pointing to `rules_max_tokens` and the contract section

## Contract

| Field | Value |
|-------|-------|
| testCommand | `true` |
| fileScopeMustChange | `docs/cli-extension-contract.md`, `README.md` |
| fileScopeMustNotChange | `cli/**`, `extension/**`, `scripts/**`, `.review/**` |
| completionCriteria | `rg -n 'rules_max_tokens' docs/cli-extension-contract.md README.md` matches both files; contract doc names `STET_RULES_MAX_TOKENS`, `--rules-max-tokens`, `rules-digest-cache.json`, the 15% / [1000, 8192] default, and file-specific-first ordering |

## Steps

### Step 1: Contract doc

- [ ] Configuration table row for `rules_max_tokens` / `STET_RULES_MAX_TOKENS`
- [ ] Paragraph on rule digests, cache, ordering, budget, and dropped-section warning

### Step 2: README

- [ ] Mention `rules_max_tokens` with a link to the contract section

### Step 3: Testing & Verification

- [ ] `rg -n 'rules_max_tokens|STET_RULES_MAX_TOKENS|rules-digest-cache.json' docs/cli-extension-contract.md README.md` shows the new text
- [ ] `rg -l 'cli-extension-contract.md' cli/ scripts/` — no tests assert on these docs (note result in STATUS)

## Completion Criteria

- [ ] All steps complete
- [ ] Every documented default and precedence rule matches the code

## Git Commit Convention

- **Docs:** `docs(SP-048): description`

## Do NOT

- Edit any code, test, or script
- Document behavior that is not on `main`
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
