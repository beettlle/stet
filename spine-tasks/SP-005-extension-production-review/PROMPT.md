# Task: SP-005 — Extension production review path

**Created:** 2026-06-30
**Size:** M

## Review Level: 2

**Assessment:** Extension currently hardcodes dry-run; switching to real review affects CLI contract parsing and error handling.
**Score:** 4/8 — Blast radius: 1, Pattern novelty: 1, Security: 1, Reversibility: 1

## Mission

Fix the **Extension gaps** blocker: replace the hardcoded `stet start --dry-run` invocation in the Cursor extension with a **production review path** that calls the real CLI (`stet start` / incremental `stet run`) using JSON + streaming, surfacing progress and findings incrementally.

Support configuration for dry-run (developer/CI mode) without making it the default.

## Dependencies

- **None**

## Context to Read First

- `extension/src/extension.ts` — `stet.startReview` (currently `--dry-run`)
- `extension/src/cli.ts` — `spawnStet`, `spawnStetStream`
- `extension/src/parse.ts` — NDJSON stream events
- `docs/cli-extension-contract.md` — `--json`, `--stream`, exit codes
- `docs/implementation-plan.md` Phase 5

## Environment

- **Workspace:** `extension/` (TypeScript); integration with `cli/` binary on PATH

## File Scope

- `extension/src/extension.ts`
- `extension/src/cli.ts`
- `extension/src/cli.test.ts`
- `extension/src/parse.ts` (only if stream contract gaps found)
- `extension/package.json` (contributes.configuration for dry-run toggle, if added)
- `README.md` (extension usage note only)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd extension && npm run compile && npm test` |
| fileScopeMustChange | `extension/src/extension.ts` |
| fileScopeMustNotChange | `cli/**`, `scripts/**`, `spine-tasks/**` |
| completionCriteria | Default startReview uses real CLI args (no dry-run); streaming panel updates; Ollama unreachable surfaces exit code 2 message |

## Steps

### Step 0: Discovery

- [ ] Confirm current `extension.ts` dry-run args and panel flow
- [ ] Choose default CLI args: `start --quiet --json --stream` (and session-aware `run` if session exists — or document start-only for v1)
- [ ] Plan VS Code setting `stet.dryRun` (default false)

### Step 1: Production CLI invocation

- [ ] Replace hardcoded `--dry-run` with configurable real review path
- [ ] Handle active session: optionally detect via `stet status` and call `run` instead of `start` (or document start-only scope in STATUS)
- [ ] Preserve incremental NDJSON panel updates

### Step 2: Error handling and settings

- [ ] Map exit codes 1/2 to user messages (reuse `showCLIError`)
- [ ] Add workspace/user setting for dry-run mode; document in README extension section

### Step 3: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Update `cli.test.ts` / extension tests for new args
- [ ] Run `cd extension && npm run compile && npm test`

## Completion Criteria

- [ ] All steps complete
- [ ] Extension runs real review by default
- [ ] Dry-run available via setting for dev/CI
- [ ] Tests pass

## Git Commit Convention

- **Implementation:** `feat(SP-005): description`

## Do NOT

- Implement dismiss or filters (SP-006, SP-007)
- Change Go CLI review logic

---

## Amendments (Added During Execution)
