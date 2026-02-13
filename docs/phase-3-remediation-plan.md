# Phase 3 Remediation Plan

This document consolidates issues from the Phase 0–3 brutal audit and the Doctor vs Start Ollama unreachable report. It is the single source of truth for a coding LLM to implement all fixes.

## 1. Overview

- **Purpose:** Close Phase 3.2 gap (token estimation not wired), fix Doctor/Start Ollama confusion, and optionally harden diff filtering.
- **Target:** Coding LLM; each issue has file paths, line references, and concrete change instructions.
- **Verification:** After changes, run `go test ./cli/... -cover` and manual test (doctor ; start with prefixed vs exported env).

## 2. Issue A: Phase 3.2 — Token estimation not wired

**Requirement (from [implementation-plan.md](implementation-plan.md) 3.2):** Estimate prompt (and optional response) tokens; configurable context limit and warn threshold; **warn when over threshold**.

**Current state:** [cli/internal/tokens/estimate.go](../cli/internal/tokens/estimate.go) defines `Estimate(prompt)` and `WarnIfOver(promptTokens, responseReserve, contextLimit, warnThreshold)`. Config has `ContextLimit` and `WarnThreshold`. Neither [cli/internal/run/run.go](../cli/internal/run/run.go) nor [cli/internal/review/review.go](../cli/internal/review/review.go) imports `tokens` or uses these values.

**Required changes:**

- **run package:** Add `ContextLimit` and `WarnThreshold` to `StartOptions` and `RunOptions` in [cli/internal/run/run.go](../cli/internal/run/run.go). When calling the LLM path (i.e. when `!opts.DryRun` and there are to-review hunks), before the first `review.ReviewHunk` call (or once per hunk if warning is per-hunk), compute estimated tokens for the prompt that will be sent (system + user prompt for that hunk). Use `tokens.Estimate(systemPrompt+userPrompt)` and a response reserve (e.g. `tokens.DefaultResponseReserve`). Call `tokens.WarnIfOver(..., opts.ContextLimit, opts.WarnThreshold)`. If the result is non-empty, write it to stderr (or a logger if present). Either warn once per run (e.g. using max estimated prompt across hunks) or once per hunk; implementation plan does not specify, so either is acceptable if documented.
- **CLI:** In [cli/cmd/stet/main.go](../cli/cmd/stet/main.go), when building `run.StartOptions` and `run.RunOptions`, set `ContextLimit: cfg.ContextLimit` and `WarnThreshold: cfg.WarnThreshold` from loaded config.
- **Tests:** Add or extend tests in [cli/internal/run/run_test.go](../cli/internal/run/run_test.go) to assert that when a mock prompt would exceed the threshold, a warning string is produced (e.g. capture stderr and check for `WarnIfOver` output). Ensure coverage remains ≥ 72% per file and project ≥ 77%.

## 3. Issue B: Doctor vs Start — Ollama unreachable message and optional upfront check

**Requirement:** When `stet start` or `stet run` fails with Ollama unreachable, the message must include the **base URL** used. Optionally, fail fast by running the same reachability check as doctor before worktree/partition work.

**Current state:**

- **Doctor** ([cli/cmd/stet/main.go](../cli/cmd/stet/main.go) around line 256–257): On `ollama.ErrUnreachable` prints the URL.
- **Start** (same file, lines 128–129): On `ollama.ErrUnreachable` prints a generic message with **no URL**.
- **Run** (same file, lines 185–186): Same generic message, no URL.
- **run.Start** and **run.Run**: No upfront Ollama check; first failure surfaces in the review loop.

**Required changes:**

1. **Include base URL in start/run unreachable message (mandatory)**  
   In [cli/cmd/stet/main.go](../cli/cmd/stet/main.go): In `runStart` and `runRun`, when `errors.Is(err, ollama.ErrUnreachable)`, change the `fmt.Fprintf` to include `cfg.OllamaBaseURL`, e.g. `"Ollama unreachable at %s. Is the server running? For local: ollama serve.\n", cfg.OllamaBaseURL`.

2. **Optional: Upfront Ollama check in run.Start and run.Run**  
   In [cli/internal/run/run.go](../cli/internal/run/run.go):  
   - **Start:** After clean worktree and lock, **before** `git.Create`, if `!opts.DryRun`, call `client.Check(ctx, opts.Model)`. On error return immediately.  
   - **Run:** After `scope.Partition`, when `len(part.ToReview) > 0` and `!opts.DryRun`, before the client/loop, call `client.Check(ctx, opts.Model)`. On failure return the error.

3. **Docs / UX:** Add a note (in this doc or [cli-extension-contract.md](cli-extension-contract.md)): for pipelines or multiple commands, `STET_OLLAMA_BASE_URL` and other `STET_*` variables must be **exported** so every `stet` invocation sees the same config. Command-prefixed env (e.g. `VAR=value cmd1 ; cmd2`) only applies to the first command.

## 4. Issue C (optional): Diff filterByPatterns — filepath.Match error

**Location:** [cli/internal/diff/diff.go](../cli/internal/diff/diff.go), line 137.

**Current code:** `ok, _ = filepath.Match(p, filepath.Base(path))` — the error is discarded. Malformed patterns are treated as no match.

**Recommended change:** Use `ok, err := filepath.Match(p, filepath.Base(path))`. If `err != nil`, skip this pattern (don’t exclude on malformed pattern) and continue. Optionally document that malformed patterns are ignored.

## 5. Implementation order and checklist

- Implement in this order: (1) Issue B.1 (URL in messages), (2) Issue A (token wiring), (3) Issue B.2 (optional upfront check), (4) Issue B.3 (docs note), (5) Issue C (optional diff).
- After code changes: run `go build ./cli/cmd/stet` and `go test ./cli/... -cover`; ensure project coverage ≥ 77% and each file ≥ 72%.
- Manual check: `STET_OLLAMA_BASE_URL="http://<remote>:11434" ./bin/stet doctor ; ./bin/stet start HEAD~1` — start should fail with a message that includes the URL (e.g. localhost). With `export STET_OLLAMA_BASE_URL="..."`, both commands should use the same URL.

## 6. Reference: key file and symbol summary

| Area             | File                                                                 | Symbols / lines                                                                             |
|------------------|----------------------------------------------------------------------|---------------------------------------------------------------------------------------------|
| Token estimation | [cli/internal/tokens/estimate.go](../cli/internal/tokens/estimate.go) | `Estimate`, `WarnIfOver`, `DefaultResponseReserve`                                         |
| Run options      | [cli/internal/run/run.go](../cli/internal/run/run.go)                | `StartOptions`, `RunOptions`; add `ContextLimit`, `WarnThreshold`                          |
| CLI start/run    | [cli/cmd/stet/main.go](../cli/cmd/stet/main.go)                       | `runStart`, `runRun`; ErrUnreachable handling at 128–130, 185–187                           |
| Doctor           | [cli/cmd/stet/main.go](../cli/cmd/stet/main.go)                       | `runDoctor`; ErrUnreachable at 256–257 with URL                                             |
| Ollama client    | [cli/internal/ollama/client.go](../cli/internal/ollama/client.go)    | `Check(ctx, model)`, `ErrUnreachable`                                                      |
| Diff filter      | [cli/internal/diff/diff.go](../cli/internal/diff/diff.go)             | `filterByPatterns` (113–147); line 137                                                      |

## Out of scope

- Phase 4+ features (status, dismiss, git note, etc.).
- Extension changes (Phase 5).
- Changing config load order or adding new config keys beyond using existing `ContextLimit` and `WarnThreshold`.

## Summary

This document is the single place a coding LLM uses to implement the Phase 3.2 token warning, the Doctor/Start Ollama message and optional upfront check, and the optional diff error handling.
