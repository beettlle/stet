# Task: SP-033 — Wire the rule-section selector behind `rules_selector`

**Created:** 2026-10-08
**Size:** M

## Review Level: 1

**Assessment:** Cross-module wiring (config, review prompt prep, run pipeline, CLI, docs) of an opt-in feature that sends hunk text to the configured System One server.
**Score:** 3/8 — Blast radius: 2, Pattern novelty: 0, Security: 1, Reversibility: 0

## Mission

Let users opt into the SP-026 rule-section selector so each hunk's review prompt gets the most relevant rule sections instead of a tail-truncated concatenation. **Off by default; with it off, behaviour and prompts must be byte-identical to today.**

**Behaviour:**

- **Config:** new bool `rules_selector` (TOML) / `STET_RULES_SELECTOR` (env; same 1/true/yes/on parsing as `STET_CRITIC_ENABLED`), default `false`. Reuses `systemone_base_url`, `systemone_model`, and env-only `STET_SYSTEMONE_API_KEY`. Follow the existing config precedence and validation patterns (`Config`, file struct, env, `Overrides` if critic keys have one).
- **CLI (`cli/cmd/stet/main.go`):** build one `*systemone.Client` when `rules_selector` is true (and not `--dry-run`) or when the System One critic is enabled — never two clients. Extend `systemOneRunOptions` (or a sibling helper) and pass the selector client to start/run/rerun options. No new flag.
- **Prompt prep (`cli/internal/review/review.go`):** `PrepareHunkPrompt` gains a way to receive pre-selected sections (e.g. a `ruleSections []rules.Section` parameter or an options struct); when non-nil it uses `prompt.AppendRuleSections(…, rules.MaxRuleTokens)`, otherwise `prompt.AppendCursorRules` exactly as today. Trace output lists selected `Source: Heading` entries when sections are used.
- **Pipeline (`cli/internal/run/run.go`):** in the hunk preparer, when the selector client is non-nil and not dry-run: candidates = `rules.FilterRules(cursorRules, hunk.FilePath)` → `rules.SplitSections` each → `rules.SelectSections(ctx, client, model, hunk.RawContent, sections, rules.MaxRuleTokens)`. On selector error, fall back to default rule assembly for that hunk and print **one** warning per run to stderr naming the server URL (no API key), e.g. `stet: rules selector unavailable at <url>: <err>; using default rule truncation`. Selection must not fail the review.
- **Docs (`docs/cli-extension-contract.md`, Configuration / Config schema):** document `rules_selector` / `STET_RULES_SELECTOR`, that it reuses the `systemone_*` keys, one request per hunk, fail-open fallback, and that hunk text is sent to the server.

## Dependencies

- **Task:** SP-026 (`SplitSections`, `SelectSections`, `AppendRuleSections`)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `cli/cmd/stet/main.go` — `systemOneRunOptions`, start/run/rerun option assembly
- `cli/internal/run/run.go` — pipeline preparer calling `review.PrepareHunkPrompt`; `SystemOneClient` option fields
- `cli/internal/review/review.go` — `PrepareHunkPrompt`
- `cli/internal/config/config.go` — `critic_backend` / `systemone_*` keys (pattern to follow)
- `cli/internal/rules/selector.go` (SP-026)
- `docs/research/systemone-rules-compliance.md` — Q5

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** none (tests use fakes / `httptest`)

## File Scope

- `cli/internal/config/config.go`
- `cli/internal/config/config_test.go`
- `cli/internal/review/review.go`
- `cli/internal/review/review_test.go`
- `cli/internal/run/run.go`
- `cli/internal/run/run_test.go`
- `cli/cmd/stet/main.go`
- `cli/cmd/stet/main_test.go`
- `docs/cli-extension-contract.md`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go vet ./... && go test ./internal/config/... ./internal/review/... ./internal/run/... ./cmd/stet/... -count=1 -cover` |
| fileScopeMustChange | `cli/internal/config/config.go`, `cli/internal/review/review.go`, `cli/internal/run/run.go`, `cli/cmd/stet/main.go`, `docs/cli-extension-contract.md` |
| fileScopeMustNotChange | `cli/internal/rules/**`, `cli/internal/prompt/**`, `cli/internal/systemone/**`, `cli/internal/review/rules_annotation.go`, `cli/internal/findings/**`, `extension/**` |
| completionCriteria | Default-off prompts identical to before (test asserts equality); enabled path makes one selector request per hunk and uses ranked sections; selector error falls back with a single stderr warning without the API key; dry-run never contacts the server; per-file coverage ≥ 72% for every changed Go file |

## Steps

### Step 1: Config

- [ ] `rules_selector` key, env var, default false, tests for file/env precedence and parsing

### Step 2: Prompt prep and pipeline

- [ ] `PrepareHunkPrompt` accepts pre-selected sections; default path unchanged
- [ ] Preparer runs the selector when enabled; fail-open with one warning per run

### Step 3: CLI and docs

- [ ] Single shared System One client construction; selector client passed to start/run/rerun
- [ ] `docs/cli-extension-contract.md` updated

### Step 4: Testing & Verification

> ZERO test failures allowed.

- [ ] Default-off: system prompt equal to pre-change output for a fixture hunk
- [ ] Enabled with fake decider: one request per hunk, ranked sections in prompt; error → fallback + exactly one warning; dry-run → zero requests
- [ ] CLI: client built when selector on, when critic=systemone, once when both, never when neither
- [ ] Run Contract `testCommand`, then `cd cli && go test ./... -cover`
- [ ] Coverage gate: `cd cli && go test ./... -coverprofile=../coverage.out -count=1 && cd .. && bash scripts/check-coverage.sh coverage.out` (then `git checkout -- coverage.out`)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Implementation:** `feat(SP-033): description`
- **Tests:** `test(SP-033): description`
- **Docs:** `docs(SP-033): description`

## Do NOT

- Enable the selector by default or add a CLI flag
- Change selector, prompt-section, or systemone library behaviour (SP-026/SP-032)
- Print or log the API key
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
