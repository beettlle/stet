# Release manifest — v1.3.0

**Created:** 2026-10-22
**Last tag:** v1.2.0
**Target version:** v1.3.0
**Bump type / profile:** minor
**Operator approved scope:** yes (2026-10-22). Packets were authored ahead of approval at operator request; operator then said "approve release scope".
**Worker model pin:** zai/glm-5.3-flash — do not change mid-release
**Agent pin override:** none

Intake snapshot: `/tmp/stet-release-intake-v1.3.0.txt` (HEAD `d91a357`, 0 pending spine tasks, 19 open issues: 8 bug, 11 enhancement, 0 documentation).

Already on `main` since v1.2.0 (outside spine; not counted against the budget):

- `f21d397` feat(model_eval): System One critic configuration and environment variables
- `a43bf01` docs(README): performance metrics and local model results

Hygiene debt: none. Every `Closes #NNN` mapped to a `.DONE` task is already closed.

---

## Composition audit

| Bucket | Selected | Profile limit | Status |
|--------|----------|---------------|--------|
| Documentation | 2 (SP-065, SP-066) | 2–4 | PASS (no `documentation` issues open; both document this release's behavior changes) |
| Bug fixes | 5 (#26, #28, #31, #32, #29) | 3–5 | PASS |
| Enhancements | 2 (#30, #27) | 1–2 | PASS |
| **Total tasks** | 9 | 10–15 | PASS (one below the target; no work invented to fill it) |

M/L check: one M (SP-062, mechanical test migration in one file); no L/XL. PASS.

**Profile audit:** PASS

---

## Dependency drift

| Check | Result |
|-------|--------|
| `go list -u -m` | 4 indirect-only updates (go-md2man v2.0.7, pflag v1.0.10, yaml/v3 v3.0.5, check.v1) — no threshold |
| `govulncheck` | 1 reachable stdlib finding: GO-2026-6809 (`crypto/x509`, trace `run.Run` → `fmt.Fprintf` → `x509.Certificate.Verify`), found in go1.27.2, fixed in go1.27.3. Mitigated without a task: both workflows use `go-version: "1.27.x"` with `check-latest: true` (SP-042), so CI and release binaries build with go1.27.3. Verify in Phase 6 that the `release.yml` "go version" step prints go1.27.3 or later. Operator: update the local toolchain to go1.27.3. |
| Toolchain / Actions pins | Go `1.27.x` + `check-latest` (local go1.27.2), Node 22, actions `@v7` — current |
| Extension npm | vitest 2→5, typescript 5→7, @types/* behind; 0 audit findings — deferred (not shipped) |
| Action | deferred: indirect-only drift; stdlib finding covered by the workflow pin |

---

## Selected tasks

| SP-ID | Issue | Bucket | Size | Title | Link |
|-------|-------|--------|------|-------|------|
| SP-058 | #26 | bug | S | `hunkid`: `.jsx` comment stripping via `jsts.IsExt` | Closes #26 |
| SP-059 | #28 | bug | S | JS RAG: skip built-in method names only as member calls | Closes #28 |
| SP-060 | #31 | bug | S | Benchmark: end-to-end hunk estimate grows with prompt size | Closes #31 |
| SP-061 | #32 | bug | S | `stet benchmark` loads the repo `.review/config.toml` | Closes #32 |
| SP-062 | #29 | bug | M | `main_test.go`: one output-capture helper that drains pipes and closes them | Closes #29 |
| SP-063 | #30 | enh | S | Ignore `test_output.txt`; release check fails when one is tracked | Closes #30 |
| SP-064 | #27 | enh | S | `jsts.Extensions()` returns a copy; unexport the slice | Closes #27 |
| SP-065 | — | doc | S | README: `stet benchmark` config source and end-to-end estimate | — |
| SP-066 | — | doc | S | `review-process-internals.md` for v1.3.0 (`.jsx` hunk IDs, JS RAG built-ins, `jsts.Extensions()`) | — |

**Release scope ID:** `SP-058,SP-059,SP-060,SP-061,SP-062,SP-063,SP-064,SP-065,SP-066`

Notes:

- SP-062 depends on SP-061 (both edit `cli/cmd/stet/main_test.go`; SP-062 migrates the test SP-061 adds).
- SP-064 depends on SP-059 (both edit `cli/internal/rag/js/resolver.go`).
- SP-065 depends on SP-060 and SP-061 (documents their behavior).
- SP-066 depends on SP-058, SP-059, SP-064 (documents their behavior).
- SP-061 edits `cli/cmd/stet/main.go`: Review Level 2 (hot module). All other code tasks are Level 1; SP-063 and the docs tasks are Level 0.
- SP-060 stays inside `cli/internal/benchmark/`; the printer in `main.go` already says "~Ns estimated", so no label change is needed there.
- SP-058 changes semantic hunk IDs for `.jsx` files once: existing sessions re-review `.jsx` hunks one time after upgrading.

---

## Gaps requiring new packets

All nine tasks above were gaps; packets authored 2026-10-22. `spine tasks validate`: 9 passed, 0 failed (exit 0). `spine tasks analyze`: 0 blocking, 0 warning (exit 0).

---

## Wave plan snapshot

```text
Spine plan — ids
9 task(s) · 3 wave(s) · maxParallel 2

Wave 0 · 5 tasks · 3 rounds (queued by maxParallel)
  Round 1 (2 parallel):
    Lane 1: SP-058 — `.jsx` hunks get JS comment stripping in semantic hunk IDs
    Lane 2: SP-059 — JS RAG skips built-in method names only as member calls
  Round 2 (2 parallel):
    Lane 1: SP-060 — Benchmark: end-to-end hunk estimate grows with prompt size
    Lane 2: SP-061 — `stet benchmark` loads the repo `.review/config.toml`
  Round 3:
    Lane 1: SP-063 — Ignore `test_output.txt`; release check fails when one is tracked

Wave 1 · 3 tasks · 2 rounds (queued by maxParallel)
  Round 1 (2 parallel):
    Lane 1: SP-062 — `main_test.go`: one output-capture helper that drains and closes pipes
    Lane 2: SP-064 — `jsts.Extensions()` returns a copy; unexport the slice
  Round 2:
    Lane 1: SP-065 — README: `stet benchmark` config source and end-to-end estimate

Wave 2 · 1 task
  Lane 1: SP-066 — Bring review-process-internals up to date for v1.3.0
```

`fileScopeMustNotChange` lists exclude every path a same-wave task changes, so a reused lane cannot fail a contract on a wave-mate's diff.

---

## Wave log

| Wave | Batch ID | Tasks landed | stet wave review (active findings) | release-check log | Exit | Pushed | Issues closed |
|------|----------|--------------|------------------------------------|-------------------|------|--------|---------------|
| 0 | `20261022T234114-7def` | SP-058, SP-059, SP-060, SP-061, SP-063 (merge `4517870`) | 12 (`/tmp/stet-wave-review-v1.3.0-w0.json`): all false positives or low nits (compile/"test will fail" claims disproven by green build and tests; `t.Parallel` race claim wrong for Go's sequential-then-parallel ordering; `estimateHunkSeconds` exists); none blocking | `/tmp/stet-release-check-wave-0.log` | 0 | yes ([CI 39881744558](https://github.com/beettlle/stet/actions/runs/39881744558) green) | #26, #28, #30, #31, #32 (closed without comment; operator OK needed to post comments) |
| 1 | `20261023T010609-ac5e` | SP-062, SP-064, SP-065 (merge `9f29686`) | 5 (`/tmp/stet-wave-review-v1.3.0-w1.json`): compile-error/"breaking API" claims on `main_test.go` and `jsts`, all disproven by the green release check; none blocking | `/tmp/stet-release-check-wave-1.log` | 0 | yes ([CI 39898183776](https://github.com/beettlle/stet/actions/runs/39898183776) green) | #27, #29 (with comments) |

Wave 1 recovery: SP-064 failed contract verification twice on one stet false positive (`ext_test.go:15`, claimed the mutation-isolation test was flawed for mutating a copy) while Go tests, vet, and gofmt passed. Finding dismissed, recorded in SP-064 STATUS, contract re-run exit 0, then `spine batch retry SP-064` (escalatePolicy moved the retry to the `anthropic` profile) passed. SP-062's final reviewer (gemini) hung for 53 minutes with no output; terminated it and the engine's spawn retry returned PASS in about 7 minutes. Removed worker scratch file `.tmp_test` committed by SP-062 (`7055f39`).

Wave 0 recovery: SP-058 and SP-059 failed contract verification on stet review false positives while their Go tests passed (tests, vet, and gofmt were green each time). SP-058: 3 false positives dismissed in the lane, then `spine batch retry` passed. SP-059: first failure was 1+ false positive; on retry the lane had lost its stet session and `.review/spine-stet-baseline.ref` (SP-061's contract ran later in the same lane), so the contract exited "no session". Restored the ref to the lane base `700da3c`, re-ran the contract (8 false positives, all disproven by build/tests, dismissed), then `spine batch retry SP-059` passed. No packet or model-pin changes. The dry run's generic "max 4 tasks per wave" cap warned on wave 0 (5 S tasks); proceeded since the stet profile caps M-sized tasks only.

---

## Deferred backlog

| Item | Type | Rationale |
|------|------|-----------|
| #5 | bug | Needs investigation and a cross-worktree repro; not S-sized yet |
| #18 | bug | M; edits `cli/internal/run/run.go` (hot module) and both critic backends; pairs with #20 |
| #19 (remainder) | bug | Resolving import-bound names in the `from` module is M; follow-up |
| #20 | enhancement | Enhancement budget full; measure together with #18 |
| #25 (remainder) | enhancement | Option 3 (`stet list --min-severity`, contract policy) is a maintainer decision |
| #17 | enhancement | Enhancement budget full; M |
| #15 | enhancement | Enhancement budget full; builds on #14 |
| #14, #13, #12 | enhancement | Refactors; enhancement budget full |
| #11 (remainder) | enhancement | Waiting on the A/B eval |
| #2 | enhancement | Epic-sized token reduction |
| Go indirect drift | deps | Indirect-only; no threshold fired |
| Extension npm drift | deps | Extension not shipped |

---

## Risks and blockers

- Scope approval pending. Do not start Phase 4 until the operator says "approve release scope".
- Local toolchain is go1.27.2 (GO-2026-6809). Release binaries are built in CI with the latest 1.27.x; confirm the version in the `release.yml` log.
- Lane reuse can leave a lane without `.review/spine-stet-baseline.ref` (v1.2.0 tech debt); stet-gated contracts may need the baseline restored.

---

## Release notes draft

```markdown
## stet v1.3.0

### Fixes
- `stet benchmark` now reads the repository's `.review/config.toml`, so it benchmarks the configured provider and model (#32)
- `stet benchmark`: the typical-hunk estimate for OpenAI-compatible providers now accounts for prompt size (#31)
- Comment-only edits in `.jsx` files no longer trigger a re-review (#26)
- JS/TS RAG looks up repo functions named like built-in methods (`get`, `set`, `log`, `find`, ...) when they are called directly or imported (#28)
- CLI tests drain captured output while commands run and close their pipes (#29)

### Maintenance
- `test_output.txt` is ignored, and the release check fails if one is tracked (#30)
- The JS/TS extension list is read-only for callers (#27)

### Documentation
- README: `stet benchmark` configuration source and estimate
- Review process internals updated for v1.3.0

### Install
curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh | STET_RELEASE_TAG=v1.3.0 bash
```

---

## Publish checklist (Phases 5–6)

- [ ] All release-scoped tasks `.DONE` on `main`
- [ ] Post-integrate `scripts/release-check.sh` exit 0 after every wave (logs above)
- [ ] `scripts/release-check.sh` exit 0 on final `HEAD` ({sha}) — log: {path}
- [ ] `ci.yml` green on final `HEAD` — run: {url}
- [ ] `git status` clean; `main` equals `origin/main`
- [ ] Release-scoped `Closes #NNN` issues all CLOSED
- [ ] Release notes written and reviewed
- [ ] Operator approved publish ({date})
- [ ] `git tag -a v1.3.0 --cleanup=verbatim -F release-notes.md` + `git push origin v1.3.0`
- [ ] `release.yml` run green (publish + verify-install ×3) — run: {url}; `go version` step reports go1.27.3+
- [ ] `scripts/post-release-smoke.sh v1.3.0` exit 0
- [ ] Local `stet` replaced: `stet --version` reports `v1.3.0` (path: {command -v stet})
- [ ] `spine-tasks/CONTEXT.md` release note added
