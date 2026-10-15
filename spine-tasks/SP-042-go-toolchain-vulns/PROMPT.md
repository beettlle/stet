# Task: SP-042 — Build CI and release with the latest Go 1.27 patch

**Created:** 2026-10-14
**Size:** S

## Review Level: 0

**Assessment:** Two-line workflow config change; no Go code.
**Score:** 1/8 — Blast radius: 1, Pattern novelty: 0, Security: 0, Reversibility: 0

## Mission

`govulncheck ./cli/...` on go1.27.1 reports 8 reachable standard-library vulnerabilities (GO-2026-6617, -6613, -6611, -6610, -6608, -6607, -6605, -6603: `net/http`, `net/http/internal/http2`, `net/textproto`, `crypto/tls`) and 5 more in imported but uncalled code (GO-2026-6612, -6609, -6604, -6600, -6599). All are fixed in go1.27.2.

Both workflows set up Go with `go-version: "1.27.x"` and no `check-latest`. `actions/setup-go` then uses any cached 1.27.x on the runner, which can be 1.27.1, so release binaries may ship the vulnerable standard library.

Fix: in both `.github/workflows/ci.yml` and `.github/workflows/release.yml`, add `check-latest: true` to the `actions/setup-go` step so the runner always resolves the newest 1.27.x patch. Keep `go-version: "1.27.x"` and keep the two workflows in sync (each file has a comment saying so). Then add a step right after setup-go in each workflow that prints `go version` so every run records the toolchain used.

## Dependencies

- **None**

## Context to Read First

- `.github/workflows/ci.yml` — `release-check` job, setup-go step (~line 27)
- `.github/workflows/release.yml` — `publish` job, setup-go step (~line 50)

## Environment

- **Workspace:** repo root
- **Services required:** None

## File Scope

- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `rg -q 'check-latest: true' .github/workflows/ci.yml && rg -q 'check-latest: true' .github/workflows/release.yml && rg -q 'go-version: "1.27.x"' .github/workflows/ci.yml && rg -q 'go-version: "1.27.x"' .github/workflows/release.yml` |
| fileScopeMustChange | `.github/workflows/ci.yml`, `.github/workflows/release.yml` |
| fileScopeMustNotChange | `cli/**`, `extension/**`, `scripts/**`, `go.mod`, `go.sum`, `.review/**` |
| completionCriteria | Both setup-go steps have `go-version: "1.27.x"` and `check-latest: true`; both workflows print `go version` after setup-go; no other workflow changes |

## Steps

### Step 1: Update both workflows

- [ ] `ci.yml`: `check-latest: true` on setup-go; `go version` step after it
- [ ] `release.yml`: same change; the "keep in sync" comments stay accurate

### Step 2: Testing & Verification

- [ ] Run the Contract `testCommand`
- [ ] Both files are valid YAML: `ruby -ryaml -e 'YAML.load_file(ARGV[0])' .github/workflows/ci.yml` and the same for `release.yml` (or `python3 -c 'import sys,yaml; yaml.safe_load(open(sys.argv[1]))' <file>` if PyYAML is available)
- [ ] `git diff --stat` shows only the two workflow files

## Completion Criteria

- [ ] All steps complete
- [ ] Contract `testCommand` exits 0

## Git Commit Convention

- **Implementation:** `fix(SP-042): description`

## Do NOT

- Pin an exact patch version or switch to `go-version-file`
- Change Node setup, action versions, job structure, or any other step
- Add `govulncheck` to `scripts/release-check.sh` (out of scope; local toolchains may lag)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
