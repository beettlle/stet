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

## Optimizer and actionability

When `stet optimize` runs (Phase 6), the DSPy optimizer loads `.review/history.jsonl`. When history includes "not actionable" reasons (e.g. from the per-finding dismissal reasons in the history schema: `false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`), the optimizer should use them to down-weight similar patterns or refine the prompt toward higher actionability. The output is `.review/system_prompt_optimized.txt`, which the CLI uses when present (see [CLI–Extension Contract](cli-extension-contract.md) and implementation plan Phase 3.3). Feedback is recorded in `.review/history.jsonl` with optional reasons; see the "State storage and history" section in the contract doc.
