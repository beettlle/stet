# Review Quality and Actionability

The PRD defines “actionable” and review goals in [PRD.md](PRD.md) §2 and §3f; this document expands with prompt guidelines and curated lessons (e.g. common false positives).

## What we mean by actionable

A finding is **actionable** when the reported issue is real (not already fixed or by design), the suggestion is correct and safe, and the change is within project scope. Developers should be able to apply the suggestion or fix the issue without reverting correct behavior. See [PRD §2 Goals](PRD.md#2-goals-and-non-goals) (Actionable findings) and [PRD §3f Review actionability](PRD.md#3f-advanced-features-shadowing-streaming-health).

## Prompt guidelines

- Report only **actionable** issues: do not suggest reverting intentional changes, adding code that already exists, or changing behavior that matches documented design.
- Prefer **fewer, high-confidence** findings over volume; avoid speculative or low-signal suggestions.
- **Diff interpretation:** Any custom or optimized system prompt (e.g. from `stet optimize`) should preserve or replicate the default’s diff-interpretation rule: review the resulting code (the + side) and the change; do not report issues that exist only in the removed lines (-) and are already fixed by the added lines (+). This keeps actionability consistent.

## Common false positives (examples)

Examples from Stet self-review; keep this list brief and update as patterns emerge:

- Suggesting adding targets to `PHONY` when they are already listed in `PHONY`.
- Suggesting "return default" when the code intentionally returns an error for read failures.
- Misreading coverage profile (e.g. statement vs count) and flagging uncovered code that is covered.
- Flagging possible nil dereference when a nil check exists earlier in the flow.
- Suggesting "restore full hint" when the hint was intentionally aligned to the documented contract.
- Tests that set `findingsOut` and restore in `t.Cleanup()` without `t.Parallel()`: flagging "test interference" is redundant when the isolation convention is documented and followed.
- Findings that point at **generated coverage report HTML** (e.g. `extension/coverage/lcov-report/*.html`) but whose message/suggestion refer to TypeScript or source code (e.g. "Unused import", "Unreachable code"). The model is inferring from embedded or referenced source; the reported file is the HTML report, not the source file. Excluding `coverage/` from the diff avoids these; document here for prompt/optimizer awareness.
- **Dry-run category Maintainability:** Suggesting to "revert" or "consider if" changing dry-run finding category from Style to Maintainability. Per implementation plan 4.5.1 the CLI intentionally emits `category: maintainability` (and `confidence: 1.0`) for dry-run; no change needed.
- **Copy-for-Chat / extension tests:** Suggesting that unit tests use a "mock or configurable workspace root" when testing a pure function that takes `(finding, workspaceRoot)`. A fixed path (e.g. `/workspace/repo`) is normal and sufficient; no change needed.
- **Copy-for-Chat implementation:** Suggesting "implementation may use different casing" for admonition text (WARNING, NOTE) when the implementation explicitly returns those strings and tests assert them. Case-insensitive matching is not required.
- **Copy-for-Chat implementation:** Suggesting that `linePartForLabel` fallback `'1'` and `lineForFragment` fallback `1` are "confusing." Both are the same intentional fallback for missing line/range; consistent and correct.
- **Copy-for-Chat implementation:** Suggesting to "break into smaller helper functions" when the function already delegates to `lineForFragment`, `linePartForLabel`, `severityToAdmonition`, `categoryToTitle`. Further splitting is optional style, not a correctness issue.
- **This file (review-quality.md):** Findings that suggest reclassifying or "verifying" entries in the curated false-positive table (e.g. "The false_positive classification may be incorrect", "Verify that the implementation..."). This document is human-curated for the optimizer; do not report meta-review of the table as actionable.
- **Dismiss command naming:** Suggesting that function names `newDismissCmd` or `runDismiss` should be "consistent with command usage 'dismiss'" by renaming them to `newApproveCmd` / `runApprove`. The suggestion is reversed: the command is `dismiss` and the functions already match; renaming to Approve would make them inconsistent. Dismiss as wrong_suggestion.
- **Finish review / package.json:** "Missing 'when' clause for context menu item" for `stet.copyFindingForChat`. The context menu contribution under `view/item/context` already has `when: "view == stetFindings && viewItem == finding"`. Dismiss as false_positive.
- **Finish review / runFinishReview:** Suggesting that the function should show "visual feedback" or that the doc should "clarify caller responsibility." By design the function only clears the panel; the caller in extension.ts shows "Stet: Review finished." and showCLIError. JSDoc already says "Caller should show success message or call showCLIError." Dismiss as false_positive.

## Known false positive patterns (curated)

Structured entries for prompt lessons, optimizer feedback, and future filtering. Schema: see [Schema for false positive entries](#schema-for-false-positive-entries) below.

| Category      | Message pattern                         | Reason         | Note                                                                 |
|---------------|-----------------------------------------|----------------|----------------------------------------------------------------------|
| maintainability | Using t.Cleanup instead of defer for test | false_positive | Full defer→t.Cleanup migration completed; suggestion is redundant    |
| testing      | Test uses global variable 'findingsOut' which may cause test interference | already_correct | Isolation ensured: tests that set findingsOut do not use t.Parallel() and restore via t.Cleanup(); comment documents the convention. |
| testing      | Missing dependency jest / @types/jest / ts-jest after removing Jest | false_positive | Extension uses Vitest only; Jest was intentionally removed; no residual refs. |
| testing      | Use jest.fn() instead of vi.fn() for consistency | wrong_suggestion | Project uses Vitest; vi.fn() is correct. |
| (various)    | Findings in `**/coverage/**` (e.g. lcov-report HTML) about "unused import", "unreachable code", "TypeScript in HTML" | out_of_scope | File under review is generated coverage output; suggestion targets source. Exclude `coverage/` from diff to avoid. |
| maintainability / documentation | Findings in `**/coverage/**` (lcov-report HTML, lcov.info) about "coverage percentage/values updated", "line coverage counts", "Coverage data appears to have been updated" | out_of_scope | File under review is generated coverage output; metrics change when tests change. Exclude `coverage/` from diff to avoid. |
| testing | Test uses hardcoded workspace root which may not match actual implementation | false_positive | Pure-function unit tests often use a fixed path; implementation takes workspaceRoot as argument. |
| testing | Test expects specific admonition text 'WARNING' / 'NOTE' but implementation may use different casing | wrong_suggestion | Implementation explicitly returns WARNING and NOTE; tests are correct. |
| testing | Test uses hardcoded category 'best_practice' but implementation may have different handling for underscores | false_positive | Implementation correctly capitalizes and replaces underscores; test verifies that. |
| style | linePartForLabel / lineForFragment fallback '1' could be confusing | false_positive | Same intentional fallback for missing line; consistent. |
| style | buildCopyForChatBlock is quite long; consider breaking into smaller helper functions | wrong_suggestion | Function already uses helpers (lineForFragment, linePartForLabel, severityToAdmonition, categoryToTitle). |
| style | Consider more descriptive variable name than 'root' (e.g. repoRoot, workspaceRoot) | false_positive | Low-signal style nit; document so optimizer can down-weight. |
| maintainability | Error message text is hardcoded; consider constant or localized string | false_positive | Acceptable for small extension; localization out of scope for v1. |
| maintainability | Consider adding a comment explaining why contextValue is being set | false_positive | Optional documentation; not a correctness issue. |
| (various) | Findings in `docs/review-quality.md` that the false_positive/wrong_suggestion classification "may be incorrect" or that suggest "verify that the implementation..." for table entries | out_of_scope | This file is curated; do not suggest reclassifying or re-verifying table entries as actionable. |
| style | Function name 'newDismissCmd' should be consistent with command usage 'dismiss' (suggest renaming to newApproveCmd) | wrong_suggestion | Command is `dismiss`; newDismissCmd is correct. Suggestion would make name inconsistent. |
| style | Function name 'runDismiss' should be consistent with command usage 'dismiss' (suggest renaming to runApprove) | wrong_suggestion | Command is `dismiss`; runDismiss is correct. Suggestion would make name inconsistent. |
| documentation | Roadmap: add intro, clarify cognitive complexity, note on circular deps, research topics timeline/priority | false_positive | Optional doc polish; defer or dismiss as out_of_scope if not doing roadmap edits. |
| design | Missing 'when' clause for context menu item (for a command that already has when on its menu contribution) | false_positive | Context menu entry under view/item/context already has when (e.g. view == stetFindings && viewItem == finding). Model may point at command or wrong line. |
| maintainability / documentation | runFinishReview (or similar) "only calls provider.clear() without visual feedback" or "doc should clarify caller handles messages" | false_positive | By design the function does not show UI; caller shows success/error. JSDoc already states caller responsibility. |
| maintainability | Extension streaming (cli.ts, extension.ts): "onClose called twice", "Potential duplicate resolution in spawnStetStream", "race condition in finding accumulation" or "concurrent access to findingsProvider" | already_correct | One-shot guard in finish() prevents double onClose/resolve; count shown only after await; single-threaded event loop. |
| security / correctness | Extension streaming (parse.ts): "Missing validation for required fields in finding data", "Potential denial of service through large JSON", "unbounded line length" or "unbounded JSON parsing" | already_correct | parseStreamEvent validates finding data and enforces MAX_STREAM_LINE_LENGTH; user-facing error is generic. |
| (various) | Finding flags an issue in the removed (-) lines when the added (+) lines fix that issue | already_correct | Prompt instructs to review resulting code and not report issues fixed by the change. |

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
- **Git intent warning:** When the repo is in a detached HEAD (or similar) state, `git rev-parse --abbrev-ref HEAD` can exit 128 and Stet logs "Warning: could not retrieve Git intent (branch/commit): ... using placeholder." For future improvement: consider a clearer user-facing message or document expected behavior when not on a branch (e.g. in CLI help or contract doc).

### Self-review batch (RAG/config/run validation)

A batch of self-review findings (RAG config options without CLI flags, int64→int overflow in config, RunOptions RAG validation) were all actionable or optional improvements; none were false positives or hallucinations, so no new entries were added to the curated false-positive tables.

## Optimizer and actionability

When `stet optimize` runs (Phase 6), the DSPy optimizer loads `.review/history.jsonl`. When history includes "not actionable" reasons (e.g. from the per-finding dismissal reasons in the history schema: `false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`), the optimizer should use them to down-weight similar patterns or refine the prompt toward higher actionability. The output is `.review/system_prompt_optimized.txt`, which the CLI uses when present (see [CLI–Extension Contract](cli-extension-contract.md) and implementation plan Phase 3.3). Feedback is recorded in `.review/history.jsonl` with optional reasons; see the "State storage and history" section in the contract doc.
