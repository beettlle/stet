# Task: SP-066 — Bring review-process-internals up to date for v1.3.0

**Created:** 2026-10-22
**Size:** S

## Review Level: 0

**Assessment:** Docs-only; describes behavior on `main` after SP-058, SP-059, SP-064.
**Score:** 0/8 — Blast radius: 0, Pattern novelty: 0, Security: 0, Reversibility: 0

## Mission

`docs/review-process-internals.md` describes behavior that changes in v1.3.0. Update it to match the code on `main` (read the code; it wins over this text):

1. **Semantic hunk ID (glossary table, ~line 62, and the IDs bullet ~line 139):** JS/TS comment stripping applies to every extension in the shared list (`.js .jsx .mjs .cjs .ts .tsx`, via `jsts.IsExt` in `langFromPath`, SP-058), so `.jsx` comment-only edits no longer change the semantic hunk ID. Check `cli/internal/hunkid/hunkid.go`.
2. **JS/TS expand bullet (~line 226):** it names the list `jsts.Extensions`. It is now the accessor `jsts.Extensions()` returning a copy, with `jsts.IsExt` for membership (SP-064). Also mention that `hunkid` uses the same list. Check `cli/internal/jsts/ext.go`.
3. **RAG (the JS/TS resolver description; find it with `rg -n 'rag/js' docs/review-process-internals.md`):** candidate extraction skips true globals (`console`, `JSON`, `Math`, ...) always, but skips built-in method names (`get`, `set`, `filter`, `log`, ...) only when they appear as member calls (`x.get(`); bare calls, import/require-bound names, and identifiers passed as arguments are looked up (SP-059). If the doc has no JS RAG candidate description, add two or three sentences in the RAG section. Check `extractSymbols` in `cli/internal/rag/js/resolver.go`.

## Dependencies

- **Task:** SP-058 (`hunkid` uses `jsts.IsExt`)
- **Task:** SP-059 (member-call-only built-in skipping)
- **Task:** SP-064 (`jsts.Extensions()` accessor)

## Context to Read First

**Tier 3 (reference docs):**

- `docs/review-process-internals.md`
- `cli/internal/hunkid/hunkid.go`, `cli/internal/jsts/ext.go`, `cli/internal/rag/js/resolver.go`

## Environment

- **Workspace:** repo root
- **Services required:** None

## File Scope

- `docs/review-process-internals.md`

## Documentation Requirements

**Must Update:**

- `docs/review-process-internals.md` — the three areas in Mission

## Contract

| Field | Value |
|-------|-------|
| testCommand | `true` |
| fileScopeMustChange | `docs/review-process-internals.md` |
| fileScopeMustNotChange | `cli/**`, `extension/**`, `scripts/**`, `.review/**` |
| completionCriteria | `docs/review-process-internals.md` contains `jsts.Extensions()` and `.jsx` in the semantic hunk ID description, and describes member-call-only built-in skipping for JS RAG (check with one `rg -n` per term: `Extensions\(\)`, `member call`); no remaining text refers to `jsts.Extensions` as a variable; every statement matches the code on `main` |

## Steps

### Step 1: Hunk IDs, expand, and RAG sections

- [ ] Semantic hunk ID text covers `.jsx` via the shared list
- [ ] Expand bullet uses `jsts.Extensions()` and mentions `hunkid`
- [ ] JS RAG candidate rules described

### Step 2: Testing & Verification

- [ ] Contract `completionCriteria` `rg` checks pass
- [ ] `rg -l 'review-process-internals.md' cli/ scripts/` — note in STATUS whether any test references this doc

## Completion Criteria

- [ ] All steps complete
- [ ] Every statement matches the code on `main`

## Git Commit Convention

- **Docs:** `docs(SP-066): description`

## Do NOT

- Change code or rewrite unrelated sections
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
