# Task: SP-047 — Prompt: out-of-view imports and definitions are not evidence

**Created:** 2026-10-14
**Size:** S

## Review Level: 1

**Assessment:** One sentence added to the default system prompt; tests updated.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 0, Security: 0, Reversibility: 1 (review behavior)

## Mission

Partial #10

The reviewer reports defects that the reviewed file disproves: "`fs` is not imported" when the import sits above the hunk, and "missing `await`" on a call to a synchronous helper whose definition is not in view. Review step 1 of `DefaultSystemPrompt` (`cli/internal/prompt/prompt.go` ~line 45) already says to assume identifiers defined outside the hunk are valid and not to report "undefined" or "not declared". It does not cover imports or async-ness.

Change: extend review step 1 of `DefaultSystemPrompt` with one sentence, for example:

> Code outside the hunk is not evidence of absence: do not report a missing import, require, or include for a module the hunk uses, and do not report a missing `await` or other call-convention issue for a function whose definition is not shown.

Keep the rest of the default prompt unchanged. Repo prompts (`.review/prompt.md`) and the optimized prompt (`system_prompt_optimized.txt`) are out of scope.

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh lenient`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Dependencies

- **None**

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md` — "Stet in spine (dogfooding)" section

**Tier 3 (reference docs):**

- `cli/internal/prompt/prompt.go` — `DefaultSystemPrompt` review steps
- `cli/internal/prompt/prompt_test.go` — assertions on default prompt content

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/prompt/prompt.go`
- `cli/internal/prompt/prompt_test.go`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/prompt/... ./cli/internal/review/... -count=1 && bash scripts/spine-stet-contract-run.sh lenient` |
| fileScopeMustChange | `cli/internal/prompt/prompt.go`, `cli/internal/prompt/prompt_test.go` |
| fileScopeMustNotChange | `cli/internal/expand/**`, `cli/internal/run/**`, `cli/cmd/**`, `extension/**`, `scripts/**`, `.review/**`, `go.mod`, `go.sum` |
| completionCriteria | `DefaultSystemPrompt` step 1 tells the model not to report missing imports or missing `await` for out-of-view code; the rest of the prompt is unchanged; prompt tests assert the new sentence; stet contract review exits 0 |

## Steps

### Step 1: Prompt sentence

- [ ] Extend review step 1 of `DefaultSystemPrompt` with the out-of-view sentence

### Step 2: Testing & Verification

> ZERO test failures allowed.

- [ ] `prompt_test.go`: assert the default prompt contains the new import and `await` wording
- [ ] `rg -n 'Verify variables and functions exist' cli --glob '*_test.go'` — update any exact-text assertion the edit breaks
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in Mission)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Implementation:** `fix(SP-047): description`
- **Tests:** `test(SP-047): description`

## Do NOT

- Rewrite other parts of the default prompt or change the output schema
- Edit `cli/internal/expand/**` (SP-046 owns JS/TS context)
- Edit `.review/config.toml`, `.review/prompt.md`, or `scripts/**`
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
