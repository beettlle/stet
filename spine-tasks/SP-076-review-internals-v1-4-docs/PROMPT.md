# Task: SP-076 — Bring review-process-internals up to date for v1.4.0

**Created:** 2026-10-24
**Size:** S

## Review Level: 0

**Assessment:** Docs-only; describes behavior on `main` after SP-067, SP-068, SP-069, SP-070, SP-072.
**Score:** 0/8 — Blast radius: 0, Pattern novelty: 0, Security: 0, Reversibility: 0

## Mission

`docs/review-process-internals.md` describes behavior that changes in v1.4.0. Update it to match the code on `main` (read the code; it wins over this text). Read each predecessor's STATUS.md Discoveries first; they may record behavior details for this doc.

1. **JS/TS expand bullet (~line 226):** the `## File imports` section is included for top-level and test-callback hunks too, and is truncated rather than dropped when over budget (SP-067). Check `expandJSTSHunk` in `cli/internal/expand/expand_jsts.go`.
2. **JS/TS RAG candidates (~line 237):** import/require-bound names with a relative module specifier resolve to the exported declaration in that module (extension and `index` resolution, repo-root bound), falling back to `git grep` for bare packages or misses (SP-068). Check `cli/internal/rag/js/resolver.go`.
3. **Post-filters (§7, the numbered list ~line 255):** add the existence-claim filter: what claims it detects, that it drops a finding only when the file proves every backticked identifier, that it runs after the evidence filter and before the critic in every strictness mode, and its trace line (SP-070). Check `cli/internal/findings/existence.go` and `runReviewPipeline`. Update the one-paragraph summary (~line 49) post-filter list to include it.
4. **Critic (~lines 263–265):** both backends now receive the reviewer's prepared user prompt (expanded context plus RAG definitions), capped at the critic context limit, instead of the raw hunk (SP-069). Check `applyCritic` / `applyCriticSystemOne` in `cli/internal/run/run.go` and the cap in `cli/internal/review/critic.go`.
5. **§8.1 Dismissal restoration (~line 291) and the `stet run` skip:** replace the "Changed-file filter" bullet with the re-mapping behavior (one `git diff -U0` per `DiffRef`; spans shifted through edits; dropped only when their own lines changed), and describe the run-time re-mapping from `LastReviewedAt` (SP-071, SP-072). Check `cli/internal/run/line_remap.go`, `restore_dismissals.go`, and `filterHunksWithDismissedFindings`.

## Dependencies

- **Task:** SP-067 (import block for top-level hunks)
- **Task:** SP-068 (JS RAG module resolution)
- **Task:** SP-069 (critic context)
- **Task:** SP-070 (existence-claim filter)
- **Task:** SP-072 (dismissal re-mapping)

## Context to Read First

**Tier 3 (reference docs):**

- `docs/review-process-internals.md`
- `spine-tasks/SP-067-jsts-imports-top-level/STATUS.md`, `spine-tasks/SP-068-js-rag-import-module/STATUS.md`, `spine-tasks/SP-069-critic-expanded-context/STATUS.md`, `spine-tasks/SP-070-existence-claim-filter/STATUS.md`, `spine-tasks/SP-072-dismissals-follow-line-shifts/STATUS.md` — Discoveries
- Code paths named in Mission

## Environment

- **Workspace:** repo root
- **Services required:** None

## File Scope

- `docs/review-process-internals.md`

## Documentation Requirements

**Must Update:**

- `docs/review-process-internals.md` — the five areas in Mission

## Contract

| Field | Value |
|-------|-------|
| testCommand | `true` |
| fileScopeMustChange | `docs/review-process-internals.md` |
| fileScopeMustNotChange | `cli/**`, `extension/**`, `scripts/**`, `.review/**`, `README.md` |
| completionCriteria | One `rg -n` hit each in `docs/review-process-internals.md` for `existence`, `line_remap`, and the critic context cap constant name; no remaining text says the critic sees only the raw hunk or that restore keeps only unchanged files (`rg -n -i 'unchanged since its record'` finds nothing); every statement matches the code on `main` |

## Steps

### Step 1: Expand, RAG, and post-filter sections

- [ ] Areas 1–3 from Mission

### Step 2: Critic and dismissal sections

- [ ] Areas 4–5 from Mission

### Step 3: Testing & Verification

- [ ] Contract `completionCriteria` `rg` checks pass
- [ ] `rg -l 'review-process-internals.md' cli/ scripts/` — note in STATUS whether any test references this doc

## Completion Criteria

- [ ] All steps complete
- [ ] Every statement matches the code on `main`

## Git Commit Convention

- **Docs:** `docs(SP-076): description`

## Do NOT

- Change code or rewrite unrelated sections
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`

---

## Amendments (Added During Execution)
