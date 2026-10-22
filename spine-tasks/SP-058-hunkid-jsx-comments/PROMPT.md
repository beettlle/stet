# Task: SP-058 — `.jsx` hunks get JS comment stripping in semantic hunk IDs

**Created:** 2026-10-22
**Size:** S

## Review Level: 1

**Assessment:** One-line switch change in `cli/internal/hunkid/hunkid.go` plus a test; reuses `jsts.IsExt`.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 0, Security: 0, Reversibility: 1 (semantic IDs for `.jsx` change once)

## Mission

Closes #26

`langFromPath` in `cli/internal/hunkid/hunkid.go` (~line 77) maps `.js .mjs .cjs .ts .tsx` to `"js"` for comment stripping but not `.jsx`. A comment-only edit in a `.jsx` file therefore changes its semantic hunk ID and re-triggers review; for the other JS/TS extensions it does not.

`cli/internal/jsts/ext.go` is the single JS/TS extension list (SP-051). Make `hunkid` use it:

1. In `langFromPath`, keep lowercasing the extension first (`jsts.IsExt` is case-sensitive), then return `"js"` when `jsts.IsExt(ext)` is true. Remove the hardcoded `.js .mjs .cjs .ts .tsx` case. Keep the other languages unchanged.
2. Add tests:
   - A `.jsx` hunk whose only change is a comment keeps the same `SemanticHunkID`.
   - An upper-case extension (`.JSX` or `.TS`) is still treated as JS.

Expected one-time effect: sessions that already hold `.jsx` hunks see new semantic IDs once and re-review those hunks. No migration is needed.

## Dependencies

- **None**

## Context to Read First

**Tier 3 (reference docs):**

- `cli/internal/hunkid/hunkid.go` — `langFromPath`, `codeOnly`, `SemanticHunkID`
- `cli/internal/hunkid/hunkid_test.go` — `TestSemanticHunkID` (~line 96) for the test pattern
- `cli/internal/jsts/ext.go` — `IsExt`

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/hunkid/hunkid.go`
- `cli/internal/hunkid/hunkid_test.go`

## Documentation Requirements

**Must Update:** none (SP-066 updates `docs/review-process-internals.md`).

**Check If Affected:** none.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/hunkid/... -count=1 && bash scripts/spine-stet-contract-run.sh lenient` |
| fileScopeMustChange | `cli/internal/hunkid/hunkid.go`, `cli/internal/hunkid/hunkid_test.go` |
| fileScopeMustNotChange | `cli/internal/jsts/**`, `cli/internal/expand/**`, `extension/**`, `.review/**` |
| completionCriteria | `.jsx` comment-only change keeps the semantic hunk ID (test); upper-case JS/TS extension still maps to `"js"` (test); `rg -n '"[.]jsx"' cli --glob '!*_test.go'` matches only `cli/internal/jsts/ext.go`; stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh lenient`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Use jsts.IsExt in langFromPath

- [ ] Hardcoded JS/TS case replaced by `jsts.IsExt` on the lowercased extension
- [ ] `spine_review_step` after the step

### Step 2: Testing & Verification

> ZERO test failures allowed.

- [ ] `.jsx` comment-only test and upper-case extension test added
- [ ] `rg -n '"[.]jsx"' cli --glob '!*_test.go'` shows only `cli/internal/jsts/ext.go`
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Fix:** `fix(SP-058): description`
- **Tests:** `test(SP-058): description`

## Do NOT

- Change `jsts` (SP-064 owns it) or any other language mapping
- Change `StrictHunkID` or `StableFindingID`
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
