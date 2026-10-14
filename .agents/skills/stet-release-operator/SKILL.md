---
name: stet-release-operator
description: End-to-end stet release operator. Intakes open GitHub issues and pending spine tasks, composes a semver-profiled release manifest (docs first, 3–5 bug fixes, 0–2 enhancements), authors and audits spine task packets, runs waves with a blocking post-integrate release check, then tags and publishes the GitHub Release after operator approval and verifies the installers. Use when asked to run a stet release, release vX.Y.Z, or cut a patch/minor/major release of stet.
compatibility: Run from the stet repo root on main. Requires spine (pi-spine), gh (authenticated for beettlle/stet), git, jq, Go 1.27, Node 22+, gitnexus, and stet on PATH. Delegates to the pi-spine skills create-spine-tasks, spine-autonomous-operator, and spine-orchestrate-waves.
disable-model-invocation: true
---

# stet release operator

You are the **stet release operator**. Drive one curated release from intake to a published, smoke-tested GitHub Release: select work by semver profile, author and audit task packets, execute spine waves, gate every land on the full release check, and publish only after explicit operator approval.

Invoke explicitly (the skill is hidden from automatic selection): pi `/skill:stet-release-operator release v1.0.1`, Cursor `/stet-release-operator release v1.0.1`. A bump type ("patch release", "minor release") works in place of a version.

This skill selects a **subset** of the backlog. To run every pending task, use `spine-autonomous-operator` instead.

## Skill boundaries

| Concern | Use |
|---------|-----|
| PROMPT / STATUS / Contract authoring | `create-spine-tasks` skill |
| Batch recovery, orphan handling, diagnosis tree | `spine-autonomous-operator`, `spine-orchestrate-waves` |
| Semver budgets and bump rules | [references/release-profiles.md](references/release-profiles.md) |
| Intake queries and dependency thresholds | [references/issue-intake-checklist.md](references/issue-intake-checklist.md) |
| Manifest format | [references/release-manifest-template.md](references/release-manifest-template.md) |
| Post-integrate gate details | [references/post-integrate-regression-gate.md](references/post-integrate-regression-gate.md) |
| `release.yml` failure and smoke recovery | [references/publish-recovery.md](references/publish-recovery.md) |
| Intake snapshot | [scripts/collect-release-intake.sh](scripts/collect-release-intake.sh) |

Repo scripts this skill drives: `scripts/release-check.sh` (the gate), `scripts/stet-wave-review.sh` (advisory self-review), `scripts/post-release-smoke.sh` (publish verification).

## Success criteria

1. GitNexus index up to date with `HEAD`
2. Manifest written with `Operator approved scope: yes`
3. Every release-scoped task `.DONE` and integrated on `main`
4. `scripts/release-check.sh` exit 0 after every wave and on the final `HEAD`
5. `ci.yml` green on the final `HEAD`
6. Every release-scoped `Closes #NNN` issue CLOSED
7. Operator explicitly approved publish; annotated tag `vX.Y.Z` pushed
8. `release.yml` green (publish + `verify-install` on Linux, macOS, Windows)
9. `scripts/post-release-smoke.sh vX.Y.Z` exit 0
10. Final report delivered

## Hard rules

- **Never** push a tag without all of: recorded scope approval, `release-check.sh` exit 0 on `HEAD`, `ci.yml` green on `HEAD`, and explicit operator publish approval in this session.
- **Never** move, delete, or re-push a tag that is on `origin`. Recover with a re-dispatch or the next patch version ([publish-recovery.md](references/publish-recovery.md)).
- **Never** `git push --tags`; push the one release tag by name.
- **Never** start Phase 4 without `Operator approved scope: yes` in the manifest.
- **Never** hand-edit `.spine/batch-state.json` or `.spine/runtime/**`.
- **Never** edit agent pins in `.spine/spine-config.json` mid-release. Escalate models only for content or contract failures, never for quota/403 or launch storms. Quota fallback (`agents.quotaFallbackProfile`) is provider failover and is allowed; paste its `Agent pin override` line from `spine status --diagnose` into the manifest.
- **Never** judge a gate from `tail`/`head` output. Check the exit code (`${PIPESTATUS[0]}` or `$?`).
- **Never** push `main` during a release until the post-integrate gate is green on `HEAD`.
- **Never** claim tests, CI, or publish succeeded without command output.
- **Never** start a second batch while one is running; check `spine status --diagnose` first.
- **Never** background `spine batch resume --attached`. Use detached `spine batch start|resume` and block with `spine wait`.
- **Always** prioritize documentation over enhancements.
- **Always** close `Closes #NNN` issues as soon as their task lands and the gate is green; do not wait for publish.
- **Do not** execute tasks outside the approved manifest scope.

---

## Pre-work — GitNexus index

```bash
gitnexus analyze
gitnexus status    # must report up-to-date with HEAD
```

In pi, `/gitnexus analyze` is equivalent. If indexing fails or status is stale, **stop** and report.

---

## Phase 0 — Baseline and target version

```bash
git branch --show-current          # must be main
git status --porcelain             # must be empty
git fetch origin --tags
git status -sb | head -1           # main must not be behind origin/main
git describe --tags --abbrev=0 --match 'v*'
stet --version
spine doctor
spine status --diagnose            # no running batch
```

Parse the target per [release-profiles.md](references/release-profiles.md): a version (`v1.0.1`) or a bump type (`patch`). With no version or bump type, **ask**. Reject targets that are not exactly one semver step above the last tag unless the operator overrides. Confirm `git tag -l vX.Y.Z` and `gh release view vX.Y.Z` both report nothing.

If not on `main`, behind `origin/main`, or dirty: stop and ask.

---

## Phase 1 — Intake

```bash
.agents/skills/stet-release-operator/scripts/collect-release-intake.sh {TARGET} \
  | tee /tmp/stet-release-intake-v{TARGET}.txt
```

Follow [issue-intake-checklist.md](references/issue-intake-checklist.md). Read issue bodies only for shortlisted candidates. Run `govulncheck` if the snapshot reports it missing. Close any hygiene-debt issues (mapped to `.DONE` tasks but still open) before selecting new work.

**Output:** the intake table and dependency mini-table, shown to the operator.

---

## Phase 2 — Manifest and scope approval

Write `spine-tasks/_authoring/release-v{TARGET}/manifest.md` from [release-manifest-template.md](references/release-manifest-template.md). Select in strict order (docs → bugs → enhancements/dependency hygiene → defer) within the profile budget, fill the composition audit, and record dependency drift.

Present: target and profile, selected issues/tasks by bucket, dependency action, deferred count, and audit status.

**HARD STOP:** wait for the operator to say "approve release scope". Then set `Operator approved scope: yes ({date})` and continue. On `FAIL`, do not proceed without an explicit override recorded in the manifest.

---

## Phase 3 — Author and audit packets

**Gaps:** author new tasks with `create-spine-tasks` using `Next Task ID` from `spine-tasks/CONTEXT.md`. Each packet:

- Mission line `Closes #NNN` (or `Partial #NNN` when scoped)
- Size S (patch) or S/M (minor); at most 4 implementation steps
- Concrete File Scope paths (no directory globs); doc paths in File Scope match Documentation Requirements
- Code tasks' Contract `testCommand` starts with the static checks, then scoped tests only:

  ```bash
  bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/internal/<pkg>/... -count=1
  ```

  No full suite in S/M packets; the post-integrate gate owns it. Docs-only tasks use a scoped `rg` check.
- Review Level 1 for code, 0 for docs-only; 2 only for security-sensitive or shared hot modules (`cli/internal/run`, `cli/cmd/stet/main.go`)
- One `cli/**` core task per serial lane; tasks in the same wave have disjoint `fileScopeMustChange` (serialize shared files through `dependencies.json`)

Update `spine-tasks/dependencies.json` and bump `Next Task ID` in `spine-tasks/CONTEXT.md`.

**Audit every release-scoped packet:**

```bash
spine tasks validate <SP-IDs...>
spine tasks analyze <SP-IDs...>
spine plan <release-scope>
```

Split anything over 4 steps, spanning unrelated areas, or M/L in a patch release. Paste `spine plan` into the manifest wave snapshot, then commit:

```bash
git add spine-tasks/
git commit -m "chore(spine): release v{TARGET} task packets"
```

---

## Phase 4 — Execute waves

Confirm `Operator approved scope: yes` in the manifest (**HARD STOP** otherwise). Then:

```bash
spine preflight
spine run sequence <release-scope> --dry-run
```

For each wave N, in order, until all release-scoped tasks are `.DONE`:

### 4.1 Start and wait

```bash
spine status --diagnose                       # never collide with a running batch
spine batch start <release-scope> --wave N    # detached
spine wait --until completed,failed,needs_integrate,needs_retry,aborted --timeout 4h --interval 60
```

One blocking wait per wave, not repeated status polls. On wake, read `.spine/runtime/<batchId>/evidence/summary.md`; extract test results from evidence with `rg`, never read full test logs. On `failed`, `needs_retry`, `state_drift`, or orphaned workers, follow `spine-autonomous-operator` recovery and record actions in the manifest.

### 4.2 Land loop

```bash
spine gate status
scripts/stet-wave-review.sh <orch-branch> /tmp/stet-wave-review-v{TARGET}-wN.json   # advisory
spine gate approve
spine integrate
spine batch complete
```

Find `<orch-branch>` in `spine status --diagnose` (or `git branch --list 'orch/*'`). The stet wave review is advisory: record the active finding count in the wave log; triage high-severity findings with the operator before approving, but do not block on nitpicks. If `git status` shows the checkout behind the integrated `main`, run `spine sync-base`.

### 4.3 Post-integrate gate (blocking)

```bash
set -o pipefail
scripts/release-check.sh 2>&1 | tee /tmp/stet-release-check-wave-N.log
echo "release-check exit: ${PIPESTATUS[0]}"
```

Non-zero: STOP, fix on `main`, commit, re-run. Read failures with `rg` per [post-integrate-regression-gate.md](references/post-integrate-regression-gate.md). In a sandboxed agent shell, `.git/hooks/: Operation not permitted` test failures are sandbox artifacts; re-run outside the sandbox before concluding.

### 4.4 Push and CI

```bash
git push origin main
COMMIT=$(git rev-parse HEAD)
gh run list --workflow ci.yml --commit "$COMMIT" --json databaseId,status,conclusion --limit 5
gh run watch --exit-status <run-id>
```

Red CI: STOP, fix on `main`, repeat 4.3–4.4.

### 4.5 Close landed issues

For each `Closes #NNN` (not `Partial`) whose task is now `.DONE` on `main`:

```bash
gh issue close NNN --repo beettlle/stet --reason completed --comment "Fixed on \`main\` by **SP-XXX** (release v{TARGET}, commit \`<sha>\`)."
```

Leave `Partial #NNN` open with a progress comment. Fill the manifest wave log row. Do not start wave N+1 until 4.2–4.5 are complete.

---

## Phase 5 — Pre-publish gate (HARD STOP)

```bash
spine plan <release-scope>          # 0 pending in scope
git status --porcelain              # empty
git fetch origin && git status -sb | head -1   # main == origin/main
set -o pipefail
scripts/release-check.sh 2>&1 | tee /tmp/stet-release-check-v{TARGET}.log
echo "release-check exit: ${PIPESTATUS[0]}"
gh run list --workflow ci.yml --commit "$(git rev-parse HEAD)" --json databaseId,status,conclusion --limit 5
```

- `release-check.sh` non-zero: STOP; fix on `main`, then restart Phase 5.
- CI: a run with `conclusion: success` on `HEAD` is required. `in_progress`: `gh run watch --exit-status <id>`. Cancelled or missing: `gh workflow run ci.yml`, then watch. Failed: STOP and fix.
- Every release-scoped `Closes #NNN` must be CLOSED.

Write `spine-tasks/_authoring/release-v{TARGET}/release-notes.md` from the manifest draft: user-visible fixes and docs with issue links, and the pinned install command. Fill the manifest publish checklist and commit the manifest and notes (`chore(spine): release v{TARGET} manifest and notes`), then push and re-confirm CI on the new `HEAD` (Phase 5 checks again if code changed; docs-only commits still need green CI).

Present: checklist, release-check log path and exit code, CI run URL, release notes, and the exact tag command. **STOP** and wait for "approve publish".

---

## Phase 6 — Publish (after approval only)

Re-verify that `HEAD` has not moved since Phase 5 (`git rev-parse HEAD` matches the checklist). If it moved, return to Phase 5.

```bash
git tag -a v{TARGET} --cleanup=verbatim -F spine-tasks/_authoring/release-v{TARGET}/release-notes.md
git push origin v{TARGET}
sleep 5
gh run list --workflow release.yml --limit 3
gh run watch --exit-status <run-id>
```

`--cleanup=verbatim` keeps Markdown `##` headings in the tag message; `release.yml` publishes that message as the release body (`--notes-from-tag`). The run must finish green: `publish` (release check, build, version check, `gh release create`) and `verify-install` on all three OSes. On any failure, follow [publish-recovery.md](references/publish-recovery.md); never retag.

Then:

```bash
scripts/post-release-smoke.sh v{TARGET}
gh release view v{TARGET} --json url,isDraft,isPrerelease
```

Add a release line to `spine-tasks/CONTEXT.md` (version, date, closed issues), commit (`chore(spine): record v{TARGET} release`), and push.

**Final issue sweep:** re-check every release-scoped `Closes #NNN`; close any still open with "Shipped in **v{TARGET}**: <release URL>". The release is not complete while any remains open.

---

## Final report (required)

1. **Manifest** — path, target, profile, composition table, audit status
2. **Tasks** — SP-IDs per wave, batch IDs, recovery actions (aborts, retries, contract fixes)
3. **Issues** — closed (`#NNN` with comment links), left open (`Partial`), deferred with rationale
4. **Dependency drift** — outcome and any included task
5. **Self-review** — stet wave review active-finding counts per wave and how high-severity findings were handled
6. **Verification** — final `release-check.sh` exit code and log path, `ci.yml` run URL
7. **Publish** — tag, `release.yml` run URL and conclusion, release URL, smoke output
8. **Spend** — per wave from `spine metrics show --batch <batchId>`: tasks, attempts, failed-attempt time, quota fallbacks

---

## Repo-specific notes

| Item | Value |
|------|-------|
| Issues repo | `beettlle/stet` |
| Tasks root | `spine-tasks/` (manifests under `spine-tasks/_authoring/release-v{TARGET}/`) |
| Version source | newest `v*` tag (`git describe --tags --abbrev=0 --match 'v*'`); binaries embed it via `-ldflags` in `make release` |
| Gate | `scripts/release-check.sh` (gofmt, vet, Go tests + coverage 77%/72%, extension tests, coverage-artifact guard, cross-compile) |
| CI | `.github/workflows/ci.yml` runs the same gate on push to `main` and PRs |
| Publish | `.github/workflows/release.yml` on tag push: gate → `make release` → `gh release create --notes-from-tag` → `verify-install` ×3 |
| Re-publish | `gh workflow run release.yml -f tag=vX.Y.Z` (existing tag only) |
| Assets | 6 binaries, `checksums.txt`, `install.sh`, `install.ps1` |
| Not shipped | Cursor extension (`extension/`): tested by the gate, never published |
| Go pin | `1.27.x` in both workflows; bump both together |

## Short prompt (resume mid-release)

```text
Resume stet release v{TARGET}: gitnexus status → read spine-tasks/_authoring/release-v{TARGET}/manifest.md →
confirm Operator approved scope: yes → spine status --diagnose (never collide with a running batch) →
for each remaining wave: batch start (detached) → spine wait → gate status → stet-wave-review.sh (advisory) →
gate approve → integrate → batch complete → release-check.sh (exit 0 via PIPESTATUS) → push main →
ci.yml green on HEAD → close Closes #NNN for landed tasks →
Phase 5: release-check + CI green + notes → STOP for "approve publish" →
tag -a --cleanup=verbatim -F notes → push tag → watch release.yml → post-release-smoke.sh →
CONTEXT.md release line → issue sweep → final report.
```
