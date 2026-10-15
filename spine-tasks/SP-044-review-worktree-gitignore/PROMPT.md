# Task: SP-044 — Keep the review worktree out of `git add`

**Created:** 2026-10-14
**Size:** S

## Review Level: 1

**Assessment:** Write one ignore file next to worktrees stet already creates; git package only.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 0

## Mission

Closes #9

By default stet creates its review worktree inside the reviewed repo at `.review/worktrees/stet-<short-sha>` (`cli/cmd/stet/main.go` ~line 1215 sets the default root; `git.PathForRef` in `cli/internal/git/worktree.go` derives the path). stet adds no ignore rule, so in repos that don't ignore `.review/`, `git add -A` stages the nested checkout as a gitlink (mode 160000). pi-spine committed one to `main` this way.

Fix in `git.Create` (`cli/internal/git/worktree.go`): after it creates the worktree parent directory (`os.MkdirAll(parent, ...)`, ~line 103) and before `git worktree add`, ensure `<parent>/.gitignore` exists when `parent` is inside `repoRoot`:

- "Inside" means `filepath.Rel(repoRoot, parent)` succeeds and does not start with `..` (compare cleaned absolute paths; resolve symlinks the same way on both sides so macOS `/var` vs `/private/var` temp dirs don't break the check).
- Content: a one-line comment saying stet wrote it, then `*`. That ignores every review worktree and the `.gitignore` itself, so it never shows up in `git status`.
- Never overwrite an existing `.gitignore` there: create it with `os.OpenFile(..., os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)` and treat `os.ErrExist` as success.
- A failure to write the file is a warning on stderr (`warning: could not write <path>: <err>`), not an error: worktree creation must still succeed.
- Worktree roots outside the repo (`worktree_root` config) get no file.

Put the helper in a new file `cli/internal/git/ignore.go` (for example `ensureWorktreeRootIgnored(repoRoot, root string) error`) and call it from `Create`.

Docs: `docs/cli-extension-contract.md` (~line 213) says "The `.review/` directory is in `.gitignore` by default". That is true only in the stet repo. Replace it with: stet writes `.review/worktrees/.gitignore` so review worktrees are never staged; other state files (`session.json`, `lock`, `history.jsonl*`) are not ignored automatically, and the recommended repo `.gitignore` rules are `.review/*`, `!.review/config.toml`, `!.review/prompt.md`.

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Dependencies

- **Task:** SP-043 (also edits `docs/cli-extension-contract.md`; serialize to avoid merge conflicts)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md` — "Stet in spine (dogfooding)" section

**Tier 3 (reference docs):**

- `cli/internal/git/worktree.go` — `PathForRef`, `Create`
- `cli/internal/git/worktree_test.go` — existing `Create` tests and temp-repo helpers

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only (unit tests use temp git repos)

## File Scope

- `cli/internal/git/ignore.go`
- `cli/internal/git/ignore_test.go`
- `cli/internal/git/worktree.go`
- `docs/cli-extension-contract.md`

## Documentation Requirements

**Must Update:**

- `docs/cli-extension-contract.md` — `.review/` layout paragraph (~line 213)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/git/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/git/ignore.go`, `cli/internal/git/ignore_test.go`, `cli/internal/git/worktree.go`, `docs/cli-extension-contract.md` |
| fileScopeMustNotChange | `cli/cmd/**`, `cli/internal/run/**`, `extension/**`, `scripts/**`, `.review/**`, `go.mod`, `go.sum` |
| completionCriteria | In a temp repo with no ignore rules, `Create` with the default root followed by `git add -A` stages nothing under `.review/worktrees/`; existing `.gitignore` untouched; roots outside the repo get no file; write failure only warns; doc paragraph accurate; stet contract review exits 0 |

## Steps

### Step 1: Ignore helper and wiring

- [ ] `ignore.go`: inside-repo check, write-if-missing, `*` content
- [ ] `Create` calls it after `MkdirAll(parent)`; failure prints a warning and continues

### Step 2: Docs

- [ ] `docs/cli-extension-contract.md` `.review/` paragraph replaced as described in Mission

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Regression test (issue acceptance): temp repo without ignore rules, `Create(repo, filepath.Join(repo, ".review", "worktrees"), "HEAD")`, then `git add -A` and `git ls-files -s` → no `.review/worktrees` entries and no mode-160000 entry
- [ ] Existing `.gitignore` in the root is left byte-for-byte unchanged
- [ ] Root outside the repo → no `.gitignore` written
- [ ] Helper on a read-only directory (skip on Windows) returns an error; `Create` prints the warning instead of failing (cover by calling the helper directly; a read-only root also blocks `git worktree add`)
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in Mission)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing; new file `ignore.go` ≥ 72% line coverage

## Git Commit Convention

- **Implementation:** `fix(SP-044): description`
- **Tests:** `test(SP-044): description`

## Do NOT

- Write any file outside the worktree root directory (no edits to the repo's own `.gitignore` or `.review/.gitignore`)
- Change the default worktree location or `PathForRef`
- Edit `cli/cmd/**` or `cli/internal/run/**`
- Edit `.review/config.toml` or `scripts/**`
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
