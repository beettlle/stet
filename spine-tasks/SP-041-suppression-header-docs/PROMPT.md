# Task: SP-041 — Update suppression header in docs

**Created:** 2026-10-14
**Size:** S

## Review Level: 0

**Assessment:** Docs-only wording fix; no code.
**Score:** 0/8 — Blast radius: 0, Pattern novelty: 0, Security: 0, Reversibility: 0

## Mission

Closes #6

SP-040 renamed the suppression section of the system prompt. The constant `suppressionExamplesHeader` in `cli/internal/prompt/prompt.go` is now:

```text
## Previously dismissed false positives (do not report or discuss)
```

Two docs still describe the old header, "Do not report issues similar to":

- `docs/review-process-internals.md` line ~217
- `docs/roadmap.md` lines ~151, ~155 and ~159

Update both docs to the current header text, and describe the section as previously dismissed false positives that the model must not report or discuss, not as hints. In `docs/roadmap.md` keep the historical plan structure (Option A / Option B, acceptance criteria); change only the header wording and the framing. Do not change any other doc content.

## Dependencies

- **Task:** SP-045 (also edits `docs/review-process-internals.md`; serialize to avoid merge conflicts)

## Context to Read First

- `cli/internal/prompt/prompt.go` — `suppressionExamplesHeader`, `AppendSuppressionExamples`, `AppendPromptShadows` (read only)

## Environment

- **Workspace:** repo root
- **Services required:** None

## File Scope

- `docs/review-process-internals.md`
- `docs/roadmap.md`

## Documentation Requirements

**Must Update:**

- `docs/review-process-internals.md`
- `docs/roadmap.md`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `true` |
| fileScopeMustChange | `docs/review-process-internals.md`, `docs/roadmap.md` |
| fileScopeMustNotChange | `cli/**`, `extension/**`, `scripts/**`, `.review/**` |
| completionCriteria | `rg -n 'Do not report issues similar to' docs/review-process-internals.md docs/roadmap.md` returns no matches; both docs name the "Previously dismissed false positives (do not report or discuss)" section |

## Steps

### Step 1: Update both docs

- [ ] `docs/review-process-internals.md`: suppression bullet uses the current header and the dismissed-false-positive framing
- [ ] `docs/roadmap.md`: the three references use the current header; plan structure unchanged

### Step 2: Testing & Verification

- [ ] `rg -n 'Do not report issues similar to' docs/review-process-internals.md docs/roadmap.md` returns nothing
- [ ] `rg -n 'Previously dismissed false positives' docs/review-process-internals.md docs/roadmap.md` shows the new text
- [ ] `rg -l 'review-process-internals.md|roadmap.md' cli --glob '*_test.go'` — run any hits (expected: none)

## Completion Criteria

- [ ] All steps complete
- [ ] Old header text gone from both docs

## Git Commit Convention

- **Docs:** `docs(SP-041): description`

## Do NOT

- Edit `cli/**`, `extension/**`, `scripts/**`, or `.review/**`
- Rewrite unrelated sections of either doc
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
