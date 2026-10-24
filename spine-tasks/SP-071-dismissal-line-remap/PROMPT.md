# Task: SP-071 — Line re-mapping library for dismissed findings

**Created:** 2026-10-24
**Size:** S

## Review Level: 1

**Assessment:** New self-contained file in `cli/internal/run/` (one `git diff` call per ref pair plus a pure parser); no callers yet. SP-072 wires it in.
**Score:** 3/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 1

## Mission

Partial #38

Dismissed findings resurface when edits above them shift their lines: the stored location no longer overlaps the re-reviewed hunk, and `stet start` drops dismissals whose file changed at all (`filterUnchangedFiles` in `cli/internal/run/restore_dismissals.go`). Both paths need to translate a line span recorded at one commit into the same span at another commit.

Add `cli/internal/run/line_remap.go`:

1. `loadLineRemaps(ctx, repoRoot, fromRef, toRef string, files []string) (lineRemaps, error)` runs **one** `git diff --no-color --no-renames -U0 <fromRef> <toRef> -- <files...>` (use `git.MinimalEnv()` and `cmd.Dir = repoRoot`, as `changedFilesSince` does) and parses the `@@ -a,b +c,d @@` headers per file. An empty `files` list makes no git call. Bound the parse: ignore files not in `files`.
2. `(lineRemaps) remapSpan(file string, start, end int) (newStart, newEnd int, ok bool)`:
   - File absent from the diff: identity (`ok = true`).
   - Every line of `start..end` lies outside the changed old ranges: shift by the cumulative `(+d) - (-b)` delta of all hunks that end before `start`; `ok = true`.
   - Any line of the span falls inside a changed old range (modified or deleted): `ok = false` (the dismissed code itself changed, so the dismissal no longer applies).
   - Handle pure insertions (`-a,0`) and pure deletions (`+c,0`) per git's `-U0` conventions, where `a` / `c` name the line before the insertion point.
   - A file deleted at `toRef`: `ok = false`.
3. Keep the parser a pure function over the diff text so tests can cover it without git, plus one test against a temp repo.

## Dependencies

- **None**

## Context to Read First

**Tier 3 (reference docs):**

- `cli/internal/run/restore_dismissals.go` — `changedFilesSince`, `dismissalSpan`, `findingSpan` for conventions (1-based inclusive spans)
- `cli/internal/run/restore_dismissals_test.go` — temp-repo test helpers
- `cli/internal/git/` — `MinimalEnv`

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/internal/run/line_remap.go`
- `cli/internal/run/line_remap_test.go`

## Documentation Requirements

**Must Update:** none.

**Check If Affected:** none.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/run/... -run 'Remap' -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/run/line_remap.go`, `cli/internal/run/line_remap_test.go` |
| fileScopeMustNotChange | `cli/internal/run/restore_dismissals.go`, `cli/internal/run/restore_dismissals_test.go`, `cli/cmd/**`, `extension/**`, `.review/**` |
| completionCriteria | Parser table tests (test names contain `Remap`): lines above, inside, and below a modification; N lines inserted above shift the span by N; lines deleted above shift by −N; insertion directly after the span keeps it; a span overlapping a changed line returns `ok=false`; multiple hunks accumulate; file absent from the diff is identity; deleted file returns `ok=false`; one temp-repo test runs `loadLineRemaps` end to end; stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Diff parser and `remapSpan`

- [ ] Pure `-U0` hunk-header parser and span remapping with the rules in Mission
- [ ] `spine_review_step` after the step

### Step 2: `loadLineRemaps` git call

- [ ] One `git diff` per ref pair for all requested files; empty list makes no call
- [ ] `spine_review_step` after the step

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Tests for every case in Contract `completionCriteria`
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Feature:** `feat(SP-071): description`
- **Tests:** `test(SP-071): description`

## Do NOT

- Call the new code from `run.go` or `restore_dismissals.go` (SP-072 does)
- Add message-similarity matching
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
