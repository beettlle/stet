# Task: SP-063 — Ignore `test_output.txt`; release check fails when one is tracked

**Created:** 2026-10-22
**Size:** S

## Review Level: 0

**Assessment:** One `.gitignore` rule and one guard step in `scripts/release-check.sh`; no Go code.
**Score:** 1/8 — Blast radius: 1, Pattern novelty: 0, Security: 0, Reversibility: 0

## Mission

Closes #30

During v1.2.0, spine workers committed `test_output.txt` twice (repo root in SP-052, removed in `38e66cf`; `cli/test_output.txt` with 4,721 lines of `go test` output, removed in `7020f31`). Workers redirect test output to that file, and the lane commit picks it up. `.gitignore` covers `*.out` and `coverage.*` but not this file.

1. **Ignore it.** In `.gitignore`, under the "Code coverage profiles and other test artifacts" block, add `test_output.txt` (no leading slash, so it matches in any directory).
2. **Guard it.** In `scripts/release-check.sh`, add a step right after the "coverage artifact guard" step, using the existing `step` / `pass` helpers, named "test output artifact guard". It fails (non-zero exit, message naming the tracked paths and the fix `git rm --cached <path>`) when `git ls-files` lists any path whose basename is `test_output.txt`, and passes otherwise. Keep the script's `set -euo pipefail` semantics: a "no match" result must not abort the script.
3. Update the header comment's `Steps:` line in `release-check.sh` to include the new guard.

## Dependencies

- **None**

## Context to Read First

**Tier 3 (reference docs):**

- `.gitignore`
- `scripts/release-check.sh` — `step` / `pass` helpers, "coverage artifact guard" step
- `scripts/check-untracked-coverage.sh` — style for a tracked-artifact check

## Environment

- **Workspace:** repo root
- **Services required:** None

## File Scope

- `.gitignore`
- `scripts/release-check.sh`

## Documentation Requirements

**Must Update:** none.

**Check If Affected:** `AGENTS.md` describes the release gate steps; do not edit it (harness-maintained). Note in STATUS Discoveries if its description is now incomplete.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash -n scripts/release-check.sh && git check-ignore -q --no-index test_output.txt && git check-ignore -q --no-index cli/test_output.txt && rg -q 'test output artifact guard' scripts/release-check.sh` |
| fileScopeMustChange | `.gitignore`, `scripts/release-check.sh` |
| fileScopeMustNotChange | `extension/**`, `.github/**`, `.review/**` |
| completionCriteria | `test_output.txt` is ignored at the root and in `cli/`; `git ls-files` lists no `test_output.txt`; the new release-check step fails when a `test_output.txt` is tracked and passes otherwise (verified manually in a scratch clone or by running only the guard logic); `release-check.sh` parses (`bash -n`) |

## Steps

### Step 1: Ignore rule and release-check guard

- [ ] `.gitignore` rule added in the test-artifacts block
- [ ] "test output artifact guard" step after the coverage artifact guard; header `Steps:` line updated

### Step 2: Testing & Verification

- [ ] Run Contract `testCommand`
- [ ] Guard fails when a `test_output.txt` is tracked: in a throwaway clone (`git clone -q . /tmp/sp063-check`), force-add one (`git add -f test_output.txt`) and run only the guard logic; record the output in STATUS. Delete the clone afterwards.
- [ ] Guard passes on the lane worktree (`git ls-files` has no `test_output.txt`)
- [ ] `git diff --stat` shows only `.gitignore` and `scripts/release-check.sh`

## Completion Criteria

- [ ] All steps complete
- [ ] Contract `testCommand` exits 0

## Git Commit Convention

- **Chore:** `chore(SP-063): description`

## Do NOT

- Run the full `scripts/release-check.sh` inside the lane (the post-integrate gate owns it)
- Ignore `*.txt` broadly or change other ignore rules
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
