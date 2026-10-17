# Task: SP-056 — Bring review-process-internals up to date for v1.2.0

**Created:** 2026-10-17
**Size:** S

## Review Level: 0

**Assessment:** Docs-only; describes behavior on `main` after SP-051, SP-053, SP-054.
**Score:** 0/8 — Blast radius: 0, Pattern novelty: 0, Security: 0, Reversibility: 0

## Mission

`docs/review-process-internals.md` describes behavior that changed in v1.2.0. Update it to match the code on `main` (read the code; it wins over this text):

1. **§7.3 Cursor rules (~line 208):** the "Append" bullet says rules are ordered always-apply first and truncated to a token budget. Now (commit `1a1782e`): rules are reduced to review digests (heading, `❌ Bad:`, `**Detect:**` lines per section; full content when a rule has no markers), cached in `<state_dir>/rules-digest-cache.json`, glob-matched rules come before `alwaysApply`, the budget is `rules_max_tokens` or 15% of `context_limit` clamped to [1000, 8192], cuts happen at section boundaries, and dropped sections produce one warning per run and a trace entry. Check `AppendCursorRules` / `AppendRuleSections` in `cli/internal/prompt/prompt.go` and `cli/internal/rules/digest*.go`.
2. **Expand bullet (~line 224):** the JS/TS list reads `.js`, `.jsx`, `.ts`, `.tsx`. It is now the shared list in `cli/internal/jsts/ext.go` (`.js .jsx .mjs .cjs .ts .tsx`), used by both expand and the JS RAG resolver (SP-051).
3. **Post-filters (§ around "Abstention", ~line 252, and the one-paragraph summary ~line 49):** add the praise-only filter (`FilterPraiseOnly`, SP-053: `info`/`nitpick`, no suggestion, praise wording; skipped when the FP kill list is skipped) and replace the evidence filter description with the added-line rule (SP-054: kept on or within 3 lines of an added line; pure-deletion hunks use the whole-hunk range).
4. **Truncated responses (~line 242, "On malformed JSON response..."):** describe the current behavior in `cli/internal/review/truncation.go`: continuation rounds are skipped for empty content or a degenerate tail, total output per hunk is capped at 2× the completion cap, and complete finding objects are salvaged from a truncated array.

## Dependencies

- **Task:** SP-051 (shared JS/TS extension list must exist)
- **Task:** SP-053 (praise filter must exist)
- **Task:** SP-054 (added-line evidence filter must exist)

## Context to Read First

**Tier 3 (reference docs):**

- `docs/review-process-internals.md`
- `cli/internal/prompt/prompt.go`, `cli/internal/rules/digest.go`, `cli/internal/rules/digestcache.go`
- `cli/internal/jsts/ext.go`, `cli/internal/findings/praise.go`, `cli/internal/findings/evidence.go`, `cli/internal/review/truncation.go`

## Environment

- **Workspace:** repo root
- **Services required:** None

## File Scope

- `docs/review-process-internals.md`

## Documentation Requirements

**Must Update:**

- `docs/review-process-internals.md` — the four areas in Mission

## Contract

| Field | Value |
|-------|-------|
| testCommand | `true` |
| fileScopeMustChange | `docs/review-process-internals.md` |
| fileScopeMustNotChange | `cli/**`, `extension/**`, `scripts/**`, `.review/**` |
| completionCriteria | `docs/review-process-internals.md` contains each of `rules-digest-cache.json`, `FilterPraiseOnly`, `.mjs`, and `degenerate` (check with one `rg -n` per term); no remaining text says rules are ordered always-apply first; evidence filter text describes the added-line rule |

## Steps

### Step 1: Rules and expand sections

- [ ] §7.3 Append bullet rewritten for digests, cache, ordering, budget, warning
- [ ] JS/TS expand bullet uses the shared extension list

### Step 2: Post-filters and truncation

- [ ] Praise filter and added-line evidence rule in the post-filter list and the summary paragraph
- [ ] Truncated-response handling described

### Step 3: Testing & Verification

- [ ] Contract `completionCriteria` `rg` check passes
- [ ] `rg -l 'review-process-internals.md' cli/ scripts/` — note in STATUS whether any test references this doc

## Completion Criteria

- [ ] All steps complete
- [ ] Every statement matches the code on `main`

## Git Commit Convention

- **Docs:** `docs(SP-056): description`

## Do NOT

- Edit code, tests, or scripts
- Edit `docs/cli-extension-contract.md` or `README.md` (SP-048 owns those)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
