# Release manifest — v1.4.0

**Created:** 2026-10-24
**Last tag:** v1.3.0
**Target version:** v1.4.0
**Bump type / profile:** minor
**Operator approved scope:** no. Packets were authored ahead of approval at operator request ("just create the spine tasks, don't run until it is approved"). Do not start Phase 4 until the operator says "approve release scope".
**Worker model pin:** zai/glm-5.3-flash — do not change mid-release
**Agent pin override:** none

Intake snapshot: `/tmp/stet-release-intake-v1.4.0.txt` (HEAD `e35636b`, 0 pending spine tasks, 18 open issues: 5 bug, 13 enhancement, 0 documentation).

Already on `main` since v1.3.0 (outside spine; not counted against the budget):

- `e35636b` feat(cli): `stet list --min-severity` / `--min-confidence` (first half of #37; documented in `docs/cli-extension-contract.md`)
- `1ea4f48` chore(spine): record v1.3.0 release

Both commits are local only (`main` is 2 ahead of `origin/main`); the operator chose not to push them yet. They must be on `origin/main` before the first post-integrate push (Phase 4.4).

Hygiene debt: none. Every `Closes #NNN` mapped to a `.DONE` task is already closed. Open `Partial` mappings: #19 (SP-055), #25 (SP-053), #11 (SP-048) — #19 and #25 are completed by this release.

---

## Composition audit

| Bucket | Selected | Profile limit | Status |
|--------|----------|---------------|--------|
| Documentation | 3 (SP-074, SP-075, SP-076) | 2–4 | PASS (no `documentation` issues open; all three document this release's changes) |
| Bug fixes | 5 tasks for 4 issues (#36, #19, #18, #38 ×2) | 3–5 | PASS |
| Enhancements | 2 issues (#33, #37; SP-073 also closes #25) | 1–2 | PASS |
| **Total tasks** | 10 | 10–15 | PASS |

M/L check: three M (SP-068, SP-069, SP-070); no L/XL. Minor allows S/M. The only M enhancement is SP-070. PASS.

**Profile audit:** PASS

---

## Dependency drift

| Check | Result |
|-------|--------|
| `go list -u -m` | 4 indirect-only updates (go-md2man v2.0.7, pflag v1.0.10, yaml/v3 v3.0.5, check.v1) — no threshold |
| `govulncheck` | 1 reachable stdlib finding: GO-2026-6809 (`crypto/x509`, trace `run.Run` → `fmt.Fprintf` → `x509.Certificate.Verify`), found in go1.27.2, fixed in go1.27.3. Same finding as v1.3.0; mitigated without a task: both workflows use `go-version: "1.27.x"` with `check-latest: true` (SP-042), and the v1.3.0 `release.yml` run built with go1.27.3. Verify again in Phase 6. Operator: the local toolchain is still go1.27.2; update it. |
| Toolchain / Actions pins | Go `1.27.x` + `check-latest` (local go1.27.2), Node 22, actions `@v7` — current |
| Extension npm | vitest 2→5, typescript 5→7, @types/* behind; 0 audit findings — deferred (not shipped) |
| Action | deferred: indirect-only drift; stdlib finding covered by the workflow pin |

---

## Selected tasks

| SP-ID | Issue | Bucket | Size | Title | Link |
|-------|-------|--------|------|-------|------|
| SP-067 | #36 | bug | S | JS/TS import block reaches top-level and test-callback hunks | Closes #36 |
| SP-068 | #19 | bug | M | JS RAG resolves imported names in the module they come from | Closes #19 |
| SP-069 | #18 | bug | M | Critic sees the reviewer's expanded context and RAG definitions | Partial #18 |
| SP-071 | #38 | bug | S | Line re-mapping library for dismissed findings | Partial #38 |
| SP-072 | #38 | bug | S | Dismissals follow line shifts in `stet start` and `stet run` | Partial #38 |
| SP-070 | #33 | enh | M | Drop "unused / undefined / not imported" findings the file disproves | Closes #33 |
| SP-073 | #37, #25 | enh | S | Spine contract fails only on actionable stet findings | Closes #37, Closes #25 |
| SP-074 | #38 | doc | S | Dismissals follow line shifts; keep history across `.review` cleanup | Closes #38 |
| SP-075 | — | doc | S | README `stet list` thresholds and contract gating | — |
| SP-076 | — | doc | S | `review-process-internals.md` for v1.4.0 | — |

**Release scope ID:** `SP-067,SP-068,SP-069,SP-070,SP-071,SP-072,SP-073,SP-074,SP-075,SP-076`

Notes:

- **#18 stays open (Partial).** SP-069 delivers the code (both critics receive the prepared user prompt, capped at 16384 characters). The second acceptance item, re-running the SP-831 comparison and recording the latency change, needs a live model and OpenJev; it is an operator measurement, not a lane task.
- **#38** closes when SP-074 lands; SP-074 depends on SP-072, which depends on SP-071, so all three parts land first. The issue's optional message-similarity fallback is not included.
- **#25** closes with SP-073: the prompt rule (option 1) is already in `DefaultSystemPrompt`, the deterministic backstop (option 2) landed in SP-053, and SP-073 records the contract policy (option 3).
- **Gate policy decision (SP-073):** `--min-severity warning --min-confidence 0.8`, overridable with `SPINE_STET_MIN_SEVERITY` / `SPINE_STET_MIN_CONFIDENCE`. #37 asked to pick values from the critic eval data (#34), which does not exist yet; this value comes from the `stet list` help text. Approving the scope approves this policy. After SP-073 lands, the operator updates the "Contract convention" bullet in `spine-tasks/CONTEXT.md` and recommends the same change to pi-spine (#37 follow-up 3).
- **#36 evidence caveat:** the issue cites the prompt shadow in `session.json`, which stores only the raw hunk, so SP-067 starts with regression tests to establish the real failure before fixing.
- Review levels: SP-069, SP-070, SP-072 are Level 2 (`cli/internal/run/run.go`; SP-072 also changes session dismissal behavior). Other code tasks Level 1; docs Level 0.
- SP-071 lands an unexported library with no caller until SP-072 (wave 2). Both ship in this release.

---

## Gaps requiring new packets

All ten tasks above were gaps; packets authored 2026-10-24. `spine tasks validate`: 10 passed, 0 failed (exit 0). `spine tasks analyze`: 0 blocking, 0 warning (exit 0).

---

## Wave plan snapshot

```text
Spine plan — ids
10 task(s) · 4 wave(s) · maxParallel 2

Wave 0 · 5 tasks · 3 rounds (queued by maxParallel)
  Round 1 (2 parallel):
    Lane 1: SP-067 — JS/TS import block reaches top-level and test-callback hunks
    Lane 2: SP-068 — JS RAG resolves imported names in the module they come from
  Round 2 (2 parallel):
    Lane 1: SP-069 — Critic sees the reviewer's expanded context and RAG definitions
    Lane 2: SP-071 — Line re-mapping library for dismissed findings
  Round 3:
    Lane 1: SP-073 — Spine contract fails only on actionable stet findings

Wave 1 · 2 tasks · 2 lanes in parallel
  Lane 1: SP-070 — Drop "unused / undefined / not imported" findings the file disproves
  Lane 2: SP-075 — Docs: README `stet list` thresholds and contract gating

Wave 2 · 1 task
  Lane 1: SP-072 — Dismissals follow line shifts in `stet start` and `stet run`

Wave 3 · 2 tasks · 2 lanes in parallel
  Lane 1: SP-074 — Docs: dismissals follow line shifts; keep history across `.review` cleanup
  Lane 2: SP-076 — Bring review-process-internals up to date for v1.4.0
```

`fileScopeMustNotChange` lists exclude every path a same-wave task changes, so a reused lane cannot fail a contract on a wave-mate's diff. Edges: `run.go` serialization SP-069 → SP-070 → SP-072; SP-072 needs SP-071's `line_remap.go`; docs tasks follow the code they document.

---

## Wave log

| Wave | Batch ID | Tasks landed | stet wave review (active findings) | release-check log | Exit | Pushed | Issues closed |
|------|----------|--------------|------------------------------------|-------------------|------|--------|---------------|

---

## Deferred backlog

| Item | Type | Rationale |
|------|------|-----------|
| #5 | bug | Root cause unknown; needs a cross-worktree repro investigation before it can be sized |
| #18 (remainder) | bug | Live-model latency and SP-831 re-run after SP-069 lands (operator measurement) |
| #20 | enhancement | Enhancement budget full; re-measure the critic after SP-069 first |
| #34 | enhancement | Enhancement budget full; would inform the SP-073 thresholds in a later release |
| #35 | enhancement | Enhancement budget full; touches `main.go` and the review/critic prompts |
| #17 | enhancement | Enhancement budget full; M |
| #15 | enhancement | Enhancement budget full; touches `rag/js/resolver.go` (SP-068 owns it this release) |
| #14, #13, #12 | enhancement | Refactors; enhancement budget full |
| #11 (remainder) | enhancement | Waiting on the A/B eval |
| #2 | enhancement | Epic-sized token reduction |
| Go indirect drift | deps | Indirect-only; no threshold fired |
| Extension npm drift | deps | Extension not shipped |

---

## Risks and blockers

- Scope approval pending. Do not start Phase 4 until the operator says "approve release scope".
- `main` is 2 commits ahead of `origin/main` (`1ea4f48`, `e35636b`); push before the first wave lands.
- Uncommitted files from another session are in the working tree (`AGENTS.md` modified, `.claude/` and `CLAUDE.md` untracked). Spine preflight and `spine batch start` need a clean tree; resolve before Phase 4.
- Three tasks edit `cli/internal/run/run.go` in sequence (SP-069 → SP-070 → SP-072), which stretches the plan to 4 waves.
- SP-069 raises critic input from ≤4096 to ≤16384 characters per finding/hunk; expect higher critic latency in lane contracts once it lands.
- Lane reuse can leave a lane without `.review/spine-stet-baseline.ref` (v1.2.0 / v1.3.0 tech debt); stet-gated contracts may need the baseline restored (wave 0 reuses lane 1 for SP-069 and SP-073).
- Local toolchain is go1.27.2 (GO-2026-6809). Release binaries are built in CI with the latest 1.27.x; confirm the version in the `release.yml` log.

---

## Release notes draft

```markdown
## stet v1.4.0

### Fixes
- JS/TS reviews include the file's imports for top-level and test-callback hunks, so imported names are no longer reported as missing (#36)
- JS/TS symbol lookup resolves imported names in the module they are imported from (#19)
- The critic now judges findings with the same expanded context and symbol definitions the reviewer saw (#18)
- Dismissed findings stay dismissed when edits above them shift their lines, in `stet start` and `stet run` (#38)

### Features
- Findings that claim a symbol is unused, undefined, or not imported are dropped when the file shows otherwise, before the critic runs (#33)
- `stet list --min-severity` / `--min-confidence` hide findings below a threshold; the spine contract script now fails only on `warning`+ findings with confidence ≥ 0.8 (#37, #25)

### Documentation
- How to keep review history across `.review` cleanups; dismissal re-mapping (#38)
- README: `stet list` thresholds and contract gating
- Review process internals updated for v1.4.0

### Install
curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh | STET_RELEASE_TAG=v1.4.0 bash
```

---

## Publish checklist (Phases 5–6)

- [ ] All release-scoped tasks `.DONE` on `main`
- [ ] Post-integrate `scripts/release-check.sh` exit 0 after every wave (logs above)
- [ ] `scripts/release-check.sh` exit 0 on final `HEAD` ({sha}) — log: {path}
- [ ] `ci.yml` green on final `HEAD` — run: {url}
- [ ] `git status` clean; `main` equals `origin/main`
- [ ] Release-scoped `Closes #NNN` issues all CLOSED (#19, #25, #33, #36, #37, #38)
- [ ] Release notes written and reviewed
- [ ] Operator approved publish ({date})
- [ ] `git tag -a v1.4.0 --cleanup=verbatim -F release-notes.md` + `git push origin v1.4.0`
- [ ] `release.yml` run green (publish + verify-install ×3) — run: {url}
- [ ] `scripts/post-release-smoke.sh v1.4.0` exit 0
- [ ] Local `stet` replaced: `stet --version` reports `v1.4.0` (path: {command -v stet})
- [ ] `spine-tasks/CONTEXT.md` release note added
