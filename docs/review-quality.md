# Review Quality and Actionability

## What we mean by actionable

A finding is **actionable** when the reported issue is real (not already fixed or by design), the suggestion is correct and safe, and the change is within project scope. Developers should be able to apply the suggestion or fix the issue without reverting correct behavior. See [PRD §2 Goals](PRD.md#2-goals-and-non-goals) (Actionable findings) and [PRD §3f Review actionability](PRD.md#3f-advanced-features-shadowing-streaming-health).

## Prompt guidelines

- Report only **actionable** issues: do not suggest reverting intentional changes, adding code that already exists, or changing behavior that matches documented design.
- Prefer **fewer, high-confidence** findings over volume; avoid speculative or low-signal suggestions.

## Common false positives (examples)

Examples from Stet self-review; keep this list brief and update as patterns emerge:

- Suggesting adding targets to `PHONY` when they are already listed in `PHONY`.
- Suggesting "return default" when the code intentionally returns an error for read failures.
- Misreading coverage profile (e.g. statement vs count) and flagging uncovered code that is covered.
- Flagging possible nil dereference when a nil check exists earlier in the flow.
- Suggesting "restore full hint" when the hint was intentionally aligned to the documented contract.
- Tests that set `findingsOut` and restore in `t.Cleanup()` without `t.Parallel()`: flagging "test interference" is redundant when the isolation convention is documented and followed.
- Findings that point at **generated coverage report HTML** (e.g. `extension/coverage/lcov-report/*.html`) but whose message/suggestion refer to TypeScript or source code (e.g. "Unused import", "Unreachable code"). The model is inferring from embedded or referenced source; the reported file is the HTML report, not the source file. Excluding `coverage/` from the diff avoids these; document here for prompt/optimizer awareness.

## Known false positive patterns (curated)

Structured entries for prompt lessons, optimizer feedback, and future filtering. Schema: see [Schema for false positive entries](#schema-for-false-positive-entries) below.

| Category      | Message pattern                         | Reason         | Note                                                                 |
|---------------|-----------------------------------------|----------------|----------------------------------------------------------------------|
| maintainability | Using t.Cleanup instead of defer for test | false_positive | Full defer→t.Cleanup migration completed; suggestion is redundant    |
| testing      | Test uses global variable 'findingsOut' which may cause test interference | already_correct | Isolation ensured: tests that set findingsOut do not use t.Parallel() and restore via t.Cleanup(); comment documents the convention. |
| testing      | Missing dependency jest / @types/jest / ts-jest after removing Jest | false_positive | Extension uses Vitest only; Jest was intentionally removed; no residual refs. |
| testing      | Use jest.fn() instead of vi.fn() for consistency | wrong_suggestion | Project uses Vitest; vi.fn() is correct. |
| (various)    | Findings in `**/coverage/**` (e.g. lcov-report HTML) about "unused import", "unreachable code", "TypeScript in HTML" | out_of_scope | File under review is generated coverage output; suggestion targets source. Exclude `coverage/` from diff to avoid. |

## Schema for false positive entries

For future tooling (optimizer, filter, prompt injection), each curated entry uses:

| Field              | Purpose                                | Example                                                  |
|--------------------|----------------------------------------|----------------------------------------------------------|
| `category`         | Narrow matches                         | `maintainability`                                        |
| `message_pattern`  | Match similar future findings          | `Using t.Cleanup instead of defer`                       |
| `reason`           | Why not actionable; use `history` schema constants | `false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope` |
| `note`             | Short explanation for prompt / docs    | `Code already uses t.Cleanup; suggestion redundant`      |

Optional enriched fields when available: `finding_id`, `file`, `line`, `suggestion_substring`, `recorded_at`. See `cli/internal/history/schema.go` for dismissal reason constants.

## Improvement backlog

Deferred items from post–Vitest migration review; consider when touching the extension or coverage config:

- **Coverage threshold:** Project rule is 77% global (AGENTS.md). Optionally consider raising to 80–85% in `extension/vitest.config.ts` for stricter coverage.
- **Coverage report differences:** After switching from Jest to Vitest’s v8 provider, lcov/html reports and counts (e.g. FNF, LH, BRH) differ slightly. Treat as expected tooling difference; optionally verify no real code paths were dropped.
- **Consistency:** Any remaining style nits (e.g. callback extraction pattern in tests) can be aligned when touching the file.
- **Review noise from coverage:** After adding diff exclusion for `coverage/`, re-run self-review and confirm no findings on `extension/coverage/` paths. If the model still suggests "improvements" for test mocks (e.g. TreeItem, MarkdownString, createTreeView), treat as optional test-quality improvements; document here so future optimizer/prompt work can consider down-weighting style nits on test files if desired.
- **Extension small cleanups (optional):** From self-review: simplify `cursor_uri` check (done); consolidate `setFindings([])` (done); consider adding a brief comment in openFinding when falling back from cursor_uri fragment to finding.line/range for clarity.

## Optimizer and actionability

When `stet optimize` runs (Phase 6), the DSPy optimizer loads `.review/history.jsonl`. When history includes "not actionable" reasons (e.g. from the per-finding dismissal reasons in the history schema: `false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`), the optimizer should use them to down-weight similar patterns or refine the prompt toward higher actionability. The output is `.review/system_prompt_optimized.txt`, which the CLI uses when present (see [CLI–Extension Contract](cli-extension-contract.md) and implementation plan Phase 3.3). Feedback is recorded in `.review/history.jsonl` with optional reasons; see the "State storage and history" section in the contract doc.
