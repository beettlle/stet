# Release manifest — v1.1.0

**Created:** 2026-10-14
**Last tag:** v1.0.0
**Target version:** v1.1.0
**Bump type / profile:** minor
**Operator approved scope:** yes (2026-10-14; no swaps; SP-043 uses #7 option 1 — `findings:` = active)
**Worker model pin:** zai/glm-5.3-flash — do not change mid-release
**Agent pin override:** none

---

## Composition audit

| Bucket | Selected | Profile limit | Status |
|--------|----------|---------------|--------|
| Documentation | 1 (#6) | 2–4 | WARN: backlog has 1 documentation issue |
| Bug fixes | 5 (Go stdlib vulns, #7, #8, #9, #10) | 3–5 | PASS |
| Enhancements | 0 | 1–2 | WARN: #2's selected part (compact JSON, drop `cursor_uri`/`evidence_lines`) already shipped in v1.0.0 (`14fce09`); SP-048 dropped in Phase 3 |
| **Total tasks** | 7 | 10–15 (warn >15) | PASS |

**Profile audit:** PASS (documentation and enhancement WARNs are small-backlog warnings; no work invented)

**Scope amendment (2026-10-14, Phase 3):** SP-048 (Partial #2) dropped: its change is already on `main` and in v1.0.0. #10 is split into SP-046 + SP-047 with no edge between them (no data dependency); close #10 when both land.

---

## Dependency drift

| Check | Result |
|-------|--------|
| `go list -u -m` | Indirect only: go-md2man v2.0.6→v2.0.7, pflag v1.0.9→v1.0.10, yaml/v3 v3.0.4→v3.0.5, check.v1 — deferred |
| `govulncheck` | 8 reachable (GO-2026-6617, -6613, -6611, -6610, -6608, -6607, -6605, -6603) + 5 uncalled (GO-2026-6612, -6609, -6604, -6600, -6599); all Go stdlib at go1.27.1, all fixed in go1.27.2 |
| Toolchain / Actions pins | Workflows use `go-version: "1.27.x"` without `check-latest`; runners can resolve a cached 1.27.1. `go.mod` has `go 1.22`, no toolchain line. Actions at v7 (current) |
| Extension npm | @types/node, @types/vscode, vitest/coverage-v8 5.x, typescript 7.x behind; 0 audit findings — deferred (not shipped) |
| Action | include SP-042 (pin release/CI toolchain to go1.27.2+) |

---

## Selected tasks

| SP-ID | Issue | Bucket | Size | Title | Link |
|-------|-------|--------|------|-------|------|
| SP-041 | #6 | doc | S | Update suppression header in review-process-internals and roadmap docs | Closes #6 |
| SP-042 | — | bug | S | CI and release setup-go `check-latest: true` (Go 1.27.2 fixes 13 stdlib vulns) | — |
| SP-043 | #7 | bug | S | `stet status` `findings:` = active count; contract doc matches | Closes #7 |
| SP-044 | #9 | bug | S | `git.Create` writes `.gitignore` (`*`) in an in-repo worktree root | Closes #9 |
| SP-045 | #8 | bug | M | `stet start` restores user dismissals from history for unchanged files | Closes #8 |
| SP-046 | #10 | bug | S | Include the JS/TS import block in expanded hunk context | Partial #10 |
| SP-047 | #10 | bug | S | Prompt: out-of-view imports/definitions are not evidence | Partial #10 |

**Release scope ID:** `SP-041,SP-042,SP-043,SP-044,SP-045,SP-046,SP-047`

Serialization (shared files, not data deps):

- SP-041 after SP-045 (`docs/review-process-internals.md`)
- SP-043 after SP-045 (`docs/cli-extension-contract.md`)
- SP-044 after SP-043 (`docs/cli-extension-contract.md`)

Stet gate levels: SP-043 lenient, SP-044 default, SP-045 strict, SP-046 default, SP-047 lenient; SP-041 and SP-042 have no stet gate.

---

## Gaps requiring new packets

| Issue | Bucket | Proposed SP-ID | Author with |
|-------|--------|----------------|-------------|
| #6 | doc | SP-041 | create-spine-tasks |
| (govulncheck) | bug | SP-042 | create-spine-tasks |
| #7 | bug | SP-043 | create-spine-tasks |
| #9 | bug | SP-044 | create-spine-tasks |
| #8 | bug | SP-045 | create-spine-tasks |
| #10 | bug | SP-046, SP-047 | create-spine-tasks |

---

## Wave plan snapshot

```text
Spine plan — ids
7 task(s) · 3 wave(s) · maxParallel 2

Wave 0 · 4 tasks · 2 rounds (queued by maxParallel)
  Round 1 (2 parallel):
    Lane 1: SP-042 — Build CI and release with the latest Go 1.27 patch
    Lane 2: SP-045 — Dismissals survive `stet finish`
  Round 2 (2 parallel):
    Lane 1: SP-046 — Include JS/TS imports in expanded hunk context
    Lane 2: SP-047 — Prompt: out-of-view imports and definitions are not evidence

Wave 1 · 2 tasks · 2 lanes in parallel
  Lane 1: SP-041 — Update suppression header in docs
  Lane 2: SP-043 — `stet status` reports active findings

Wave 2 · 1 task
  Lane 1: SP-044 — Keep the review worktree out of `git add`
```

`spine tasks validate`: 7 passed, 0 failed. `spine tasks analyze`: 0 blocking, 0 warning.

---

## Wave log

| Wave | Batch ID | Tasks landed | stet wave review (active findings) | release-check log | Exit | Pushed | Issues closed |
|------|----------|--------------|------------------------------------|-------------------|------|--------|---------------|
| 0 | `20261015T053242-6e48` | SP-042, SP-045, SP-046, SP-047 (merge `76987c0`); post-integrate fix `5cd4430` (SP-045 restored dismissals never set `category`, so location match never fired) | 9: 1 real fixed (c64b3e5 → `5cd4430`), 1 real low-priority noted (d5b453c: empty `funcSrc` drops JS/TS imports), 7 false positives. Recovery: SP-047 lane false positive 5f28cf7 dismissed, `spine batch retry SP-047` | `/tmp/stet-release-check-wave-0.log` | 0 | `5cd4430`, `ci.yml` run 38698351507 green | #8, #10 (SP-046 + SP-047) |
| 1 | `20261015T072451-ec43` | SP-041, SP-043 (merge `ada554c`) | 2, both false positives (97ef93f: `activeFindingsInSession` read as a duplicate `activeFindings`; eec4a55: `status` writes to `os.Stdout`, not `getFindingsOut`). Recovery: SP-043 final code review `fetch failed` (`code_review_spawn_failed`) after contract green, `spine batch retry SP-043` | `/tmp/stet-release-check-wave-1.log` | 0 | `ada554c`, `ci.yml` run 38718202288 green | #6, #7 |
| 2 | `20261015T101054-9b9a` | SP-044 (merge `9cdb6fc`) | 2: af96d3e false positive (`*` in the parent `.gitignore` ignores the worktree; `TestCreate_WorktreeRootNotStaged` proves it); ef2ab56 handle-leak claim false, but a failed write leaves an empty `.gitignore` that later `O_EXCL` calls skip — real, low priority, deferred | `/tmp/stet-release-check-wave-2.log` | 0 | `9cdb6fc`, `ci.yml` run 38721407947 green (go1.27.2) | #9 |

---

## Deferred backlog

| Item | Type | Rationale |
|------|------|-----------|
| #3 | bug | Fixes 1/2/3/5 shipped in v1.0.0 (`ea2124f`). Remaining: fix 6 (distinct "reasoning used the budget" error from `reasoning_content` in `openaicompat`), optional fix 7. Bug budget full; S-sized; first candidate for v1.1.1 |
| #4 | bug | Bug budget full; benchmark/run-summary display only, no review correctness impact |
| #5 | bug | Needs a multi-clone/worktree repro first; may share root cause with #8 session state — investigate after SP-045 lands |
| #2 (remainder) | enhancement | Proposals 1–2 shipped in v1.0.0 (`14fce09`). Remaining: suggestion-length cap, prompt audit (proposal 3), before/after token measurement on a live model server |
| Wave review d5b453c | bug | `extractJSTSImports` gets no imports when `funcSrc` is empty; destructured `const {a} = require()` also stops the scan. Low priority |
| Wave review ef2ab56 | bug | `ensureWorktreeRootIgnored` leaves an empty `.gitignore` if `WriteString` fails (disk full); later calls skip it via `O_EXCL`. Remove the file on write failure. Low priority |
| Indirect Go module drift | hygiene | No threshold fires |
| Extension npm drift | hygiene | Extension not shipped |

---

## Risks and blockers

- SP-045 touches `cli/internal/run/run.go` (hot module, ~2000 lines): Review Level 2, stet `strict`. Finding IDs hash model text, so restore matches by ID or file+category+line overlap, only for files unchanged since the dismissal; `already_correct` (auto-dismiss) is never restored. A false match could hide a real new finding at the same location and category.
- SP-043 touches `cli/cmd/stet/main.go` (hot module): Review Level 2. Changing `findings:` semantics affects scripts that parse `stet status` (pi-spine already counts `stet list`); contract doc updated in the same task.
- SP-045 must leave history-based prompt suppression (`suppression_enabled`) unchanged.
- Stet-gated contracts need LM Studio (`127.0.0.1:1234`) and OpenJev (`127.0.0.1:8080`) running; batch env per `spine-tasks/CONTEXT.md` Execution policy 9 (`HEADROOM_URL=http://127.0.0.1:9`, `SPINE_STET_NO_AUTO_FINISH=1`, `PYTHONDONTWRITEBYTECODE=1`).
- SP-042 local verification still runs on go1.27.1; the fix is confirmed by `govulncheck` in CI only if the workflow prints the resolved Go version.
- `gitnexus analyze` (default mode) injects AGENTS.md/CLAUDE.md/.claude content; use `gitnexus analyze --index-only` during this release to keep the tree clean.

---

## Release notes draft

```markdown
## stet v1.1.0

### Fixes
- `stet status` now reports active findings only, matching the docs (#7)
- Dismissals survive `stet finish`; a new `stet start` re-applies them to matching findings in unchanged files (#8)
- stet writes a `.gitignore` into `.review/worktrees/` so the review worktree is never committed as a gitlink (#9)
- JS/TS hunks include the file's imports, and the prompt tells the reviewer not to report missing imports or `await` it cannot see (#10)
- Release binaries are built with the latest Go 1.27 patch (1.27.2+), which fixes 13 standard library vulnerabilities

### Documentation
- Docs describe the current "Previously dismissed false positives" prompt section (#6)

### Install
curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh | STET_RELEASE_TAG=v1.1.0 bash
```

---

## Publish checklist (Phases 5–6)

- [x] All release-scoped tasks `.DONE` on `main` (SP-041..SP-047)
- [x] Post-integrate `scripts/release-check.sh` exit 0 after every wave (logs above)
- [x] `scripts/release-check.sh` exit 0 on last code `HEAD` (`9cdb6fc`) — log: `/tmp/stet-release-check-wave-2.log`; re-run on the final `HEAD` in the Phase 5 report
- [x] `ci.yml` green on last code `HEAD` — run: https://github.com/beettlle/stet/actions/runs/38721407947; final `HEAD` run in the Phase 5 report
- [x] `git status` clean; `main` equals `origin/main` (after pushing this commit)
- [x] Release-scoped `Closes #NNN` issues all CLOSED (#6, #7, #8, #9, #10)
- [x] Release notes written: `spine-tasks/_authoring/release-v1.1.0/release-notes.md`
- [x] Operator approved publish (2026-10-17) — Phase 5 re-run on `a03742d`: `release-check.sh` exit 0 (`/tmp/stet-release-check-v1.1.0.log`), `ci.yml` run https://github.com/beettlle/stet/actions/runs/38721762145
- [x] `git tag -a v1.1.0 --cleanup=verbatim -F release-notes.md` + `git push origin v1.1.0` (tag at `a03742d`)
- [x] `release.yml` run green (publish + verify-install ×3) — run: https://github.com/beettlle/stet/actions/runs/39073998149
- [x] `scripts/post-release-smoke.sh v1.1.0` exit 0
- [x] `spine-tasks/CONTEXT.md` release note added
