# Release manifest — v1.2.0

**Created:** 2026-10-17
**Last tag:** v1.1.0
**Target version:** v1.2.0
**Bump type / profile:** minor
**Operator approved scope:** no
**Worker model pin:** zai/glm-5.3-flash — do not change mid-release
**Agent pin override:** none

Intake snapshot: `/tmp/stet-release-intake-v1.2.0.txt` (HEAD `7a73a1e`, 0 pending spine tasks, 21 open issues).

Already on `main` since v1.1.0 (landed outside spine, gated by `release-check.sh` exit 0 on `7a73a1e`, `ci.yml` run 39074239474 green). Operator decision (2026-10-17): these ship in v1.2.0 but **do not count** against the profile budget.

- `1a1782e` feat(rules): cached runtime review digest, file-specific-first ordering, `rules_max_tokens` (#11 except the A/B eval)
- `bfa3b86` feat(expand): `.mjs`/`.cjs` enclosing-function context (#16, expand half only)

---

## Composition audit

| Bucket | Selected | Profile limit | Status |
|--------|----------|---------------|--------|
| Documentation | 2 | 2–4 | PASS |
| Bug fixes | 6 | 3–5 | WARN: 6 > 5 — operator override ("add more if possible", 2026-10-17) |
| Enhancements | 3 (#25, #21, #23) | 1–2 | FAIL without override — operator override ("Add #23 to the release", 2026-10-17) |
| **Total tasks** | 11 | 10–15 | PASS |

**Profile audit:** PASS with operator override (6 bug fixes and 3 enhancements; operator asked to add more scope and explicitly added #23)

---

## Dependency drift

| Check | Result |
|-------|--------|
| `go list -u -m` | 4 indirect-only updates (go-md2man, pflag, yaml/v3, check.v1) |
| `govulncheck` | none (`No vulnerabilities found.`) |
| Toolchain / Actions pins | Go `1.27.x` (local go1.27.2), Node 22, actions `@v7` — current |
| Extension npm | vitest 2→5, typescript 5→7, @types/* behind; 0 audit findings — deferred (not shipped) |
| Action | deferred: deps OK, indirect-only drift |

---

## Selected tasks

| SP-ID | Issue | Bucket | Size | Title | Link |
|-------|-------|--------|------|-------|------|
| SP-048 | #11 | doc | S | Document `rules_max_tokens` / `STET_RULES_MAX_TOKENS` and the rule digest in `docs/cli-extension-contract.md` and `README.md` | Partial #11 |
| SP-049 | #22 | bug | S | Contract script recovery `stet start` passes `--strictness` | Closes #22 |
| SP-050 | #24 | bug | S | `jstsDeclLineRange`: arrow branch only before the declaration's own body brace | Closes #24 |
| SP-051 | #16 | bug | S | One shared JS/TS extension list for `expand` and `rag/js`; `.jsx` gets RAG definitions | Closes #16 |
| SP-052 | #4 | bug | S | `openaicompat` client-side wall-clock timing; benchmark and run summary show end-to-end throughput | Closes #4 |
| SP-053 | #3 | bug | S | Salvage complete findings from a truncated array; skip continuation on a degenerate tail | Partial #3 |
| SP-054 | #25 | enh | S | Prompt rule against praise-only findings + deterministic praise filter for `info`/`nitpick` without suggestion | Partial #25 |
| SP-055 | #21 | enh | S | Evidence filter keeps only findings on or within N lines of an added line | Closes #21 |
| SP-056 | #19 | bug | S | JS RAG: extract function references passed as arguments and import/require-bound names; skip built-in method names | Partial #19 |
| SP-057 | — | doc | S | Bring `docs/review-process-internals.md` up to date: rule digest ordering, JS/TS extension list, praise filter, added-line evidence filter, truncation salvage | — |
| SP-058 | #23 | enh | S | `stet run` warns on stderr (even with `--quiet`) when review-affecting flags are set and zero hunks are reviewed | Closes #23 |

**Release scope ID:** `SP-048,SP-049,SP-050,SP-051,SP-052,SP-053,SP-054,SP-055,SP-056,SP-057,SP-058`

Notes:

- SP-051 depends on SP-050 (both edit `cli/internal/expand/expand_jsts.go`).
- SP-056 depends on SP-051 (both edit `cli/internal/rag/js/resolver.go`). It leaves #19's "resolve in the `from` module" part open.
- SP-055 depends on SP-054 (both wire filters in `cli/internal/run/run.go`). Both are Review Level 2 (hot module).
- SP-052 edits `cli/cmd/stet/main.go` (`printRunSummary`): Review Level 2.
- SP-053 leaves #3 fix 3 (whole-hunk output budget) open.
- SP-054 implements #25 options 1 and 2; option 3 (`stet list --min-severity`, contract policy) is a maintainer decision and stays open.
- SP-057 depends on SP-051, SP-053, SP-054, SP-055 (documents their behavior). SP-048 and SP-057 edit different files.
- SP-058 depends on SP-052 (`cli/cmd/stet/main.go`) and SP-055 (`cli/internal/run/run.go`, zero-hunk path near line 1739). Review Level 2 (hot modules).
- #11 stays open after SP-048 until the A/B eval is posted.

---

## Gaps requiring new packets

All eleven tasks above are gaps (no existing SP-*). Author with `create-spine-tasks`, starting at `Next Task ID: SP-048`.

---

## Wave plan snapshot

```text
(pending Phase 3: spine plan SP-048,SP-049,SP-050,SP-051,SP-052,SP-053,SP-054,SP-055,SP-056,SP-057,SP-058)
```

---

## Wave log

| Wave | Batch ID | Tasks landed | stet wave review (active findings) | release-check log | Exit | Pushed | Issues closed |
|------|----------|--------------|------------------------------------|-------------------|------|--------|---------------|

---

## Deferred backlog

| Item | Type | Rationale |
|------|------|-----------|
| #5 | bug | Needs investigation and repro across worktrees/clones; not S-sized yet |
| #18 | bug | M; edits `cli/internal/run/run.go` (hot module) and both critic backends; next minor |
| #19 (remainder) | bug | Resolving import-bound names in the `from` module is M; follow-up after SP-056 |
| #20 | enhancement | Enhancement budget full; pairs with #18 |
| #17 | enhancement | Enhancement budget full; M |
| #15 | enhancement | Enhancement budget full; touches `lookupDefinitions` with #19 |
| #14, #13, #12 | enhancement | Refactors; enhancement budget full |
| #2 | enhancement | Epic-sized token reduction; next minor |
| Go indirect drift | deps | Indirect-only; no threshold fired |
| Extension npm drift | deps | Extension not shipped |

---

## Publish checklist (Phases 5–6)

- [ ] All release-scoped tasks `.DONE` on `main`
- [ ] Post-integrate `scripts/release-check.sh` exit 0 after every wave
- [ ] `scripts/release-check.sh` exit 0 on final `HEAD`
- [ ] `ci.yml` green on final `HEAD`
- [ ] `git status` clean; `main` equals `origin/main`
- [ ] Release-scoped `Closes #NNN` issues all CLOSED (#4, #16, #21, #22, #23, #24)
- [ ] Release notes written: `spine-tasks/_authoring/release-v1.2.0/release-notes.md`
- [ ] Operator approved publish
- [ ] `git tag -a v1.2.0 --cleanup=verbatim -F release-notes.md` + `git push origin v1.2.0`
- [ ] `release.yml` run green (publish + verify-install ×3)
- [ ] `scripts/post-release-smoke.sh v1.2.0` exit 0
- [ ] `spine-tasks/CONTEXT.md` release note added
