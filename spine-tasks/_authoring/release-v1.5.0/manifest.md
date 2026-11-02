# Release manifest — v1.5.0

**Created:** 2026-11-01
**Last tag:** v1.4.0
**Target version:** v1.5.0
**Bump type / profile:** minor
**Operator approved scope:** yes (2026-11-01), including the third-enhancement override for #11.
**Worker model pin:** zai/glm-5.3-flash — do not change mid-release
**Agent pin override:** none

Intake snapshot: `/tmp/stet-release-intake-v1.5.0.txt` (HEAD `fbdb893`, 0 pending spine tasks, 12 open issues: 2 bug, 10 enhancement, 0 documentation).

Theme: **review precision**. Fix what the critic sees (#18), how it judges (#20), and where it draws the line (#34); prove that Cursor rules improve reviews and size their budget from data (#11); fix the wave-review trust bug (#5).

Already on `main` since v1.4.0 (outside spine; not counted against the budget):

- `7e0bb2b` docs(GitNexus): indexing details and impact-analysis guidelines in `AGENTS.md` / `CLAUDE.md`
- `fbdb893` chore(spine): model version and escalate policy profile in `.spine/spine-config.json` (operator change before the release started; not a mid-release pin edit)

Both are local only (`main` is 2 ahead of `origin/main`). They reach `origin/main` with the first post-integrate push (Phase 4.4).

Hygiene debt: none. Every `Closes #NNN` mapped to a `.DONE` task is closed. Open `Partial` mappings: #18 (SP-069) and #11 (SP-048), both completed by this release.

---

## Composition audit

| Bucket | Selected | Profile limit | Status |
|--------|----------|---------------|--------|
| Documentation | 0 standalone (docs ride inside SP-077, SP-080, SP-081, SP-083; SP-078/079 are research docs) | 2–4 | WARN (no `documentation` issues open) |
| Bug fixes | 2 tasks for 2 issues (#5, #18) | 3–5 | WARN: backlog has 2 bugs |
| Enhancements | 3 issues (#20, #34, #11) in 5 tasks | 1–2 | FAIL without override → operator override |
| **Total tasks** | 7 | 10–15 | WARN (below target; not inventing work) |

M/L check: SP-080, SP-082, and SP-083 are M; no L/XL. Minor allows S/M; at most one M enhancement is in flight per wave (SP-082 shares wave 0 only with S tasks).

**Profile audit:** PASS with operator override (third enhancement #11 approved by the operator 2026-11-01; most of #11 already shipped in `1a1782e`, and only its A/B acceptance item and budget tuning remain).

---

## Dependency drift

| Check | Result |
|-------|--------|
| `go list -u -m` | 4 indirect-only updates (go-md2man v2.0.7, pflag v1.0.10, yaml/v3 v3.0.6, check.v1) — no threshold |
| `govulncheck` | none (`go run golang.org/x/vuln/cmd/govulncheck@latest ./cli/...`, v1.9.0: "No vulnerabilities found.") |
| Toolchain / Actions pins | Go `1.27.x` (local go1.27.3), Node 22, actions `@v7` — current |
| Extension npm | vitest 2→5, typescript 5→7, @types/* behind; 0 audit findings — deferred (not shipped) |
| Action | deferred: deps OK |

---

## Selected tasks

| SP-ID | Issue | Bucket | Size | Title | Link |
|-------|-------|--------|------|-------|------|
| SP-077 | #5 | bug | S | Wave review shows only findings from the reviewed session | Closes #5 |
| SP-078 | #18 | bug | S | Measure the critic with expanded context (SP-831 off/on re-run) | Closes #18 (Partial if models unavailable) |
| SP-079 | #34 | enh | S | Critic-eval dataset from pi-spine release dismissals | Partial #34 |
| SP-080 | #20 | enh | M | System One critic rejects out-of-view "missing / undefined" claims | Closes #20 |
| SP-081 | #34 | enh | S | Sweep `systemone_min_keep` on the dataset and set the default | Closes #34 |
| SP-082 | #11 | enh | M | Rule-targeted model-eval cases and harness rule options | Partial #11 |
| SP-083 | #11 | enh | M | Rule-budget A/B eval and data-driven default | Closes #11 |

**Release scope ID:** `SP-077,SP-078,SP-079,SP-080,SP-081,SP-082,SP-083`

Dependencies: SP-078 → SP-077, SP-082 (LM Studio serialization); SP-080 → SP-078, SP-079; SP-081 → SP-079, SP-080; SP-083 → SP-081, SP-082.

---

## Gaps requiring new packets

| Issue | Bucket | Proposed SP-ID | Author with |
|-------|--------|----------------|-------------|
| #5 | bug | SP-077 | create-spine-tasks |
| #18 | bug | SP-078 | create-spine-tasks |
| #34 | enh | SP-079, SP-081 | create-spine-tasks |
| #20 | enh | SP-080 | create-spine-tasks |
| #11 | enh | SP-082, SP-083 | create-spine-tasks |

---

## Wave plan snapshot

```text
spine plan SP-077,SP-078,SP-079,SP-080,SP-081,SP-082,SP-083
7 task(s) · 5 wave(s) · maxParallel 2

Wave 0 · SP-077, SP-079 (round 1); SP-082 (round 2)
Wave 1 · SP-078
Wave 2 · SP-080
Wave 3 · SP-081
Wave 4 · SP-083

validate: 7 passed, 0 failed · analyze: 0 blocking, 0 warning
```

---

## Wave log

| Wave | Batch ID | Tasks landed | stet wave review (active findings) | release-check log | Exit | Pushed | Issues closed |
|------|----------|--------------|------------------------------------|-------------------|------|--------|---------------|
| 0 | 20261102T054035-ac43 | SP-077, SP-079, SP-082 | 15 (advisory; 1 ERROR false positive, rest repeats/nitpicks) | /tmp/stet-release-check-wave-0.log | 0 | `f2c09f9` (CI 41481420549 green) | #5 (auto-closed on push) |
| 1 | 20261102T162750-65d6 | SP-078 | 0 | /tmp/stet-release-check-wave-1.log | 0 | `a90293d` (CI 41510136666 green) | #18 |
| 2 | 20261102T192933-3d83 | SP-080 (one final-review retry) | 1 (test nit, not a defect) | /tmp/stet-release-check-wave-2.log | 0 | pending | none — #20 stays open: no wording passed the pre-registered rule (`Partial #20`) |

---

## Deferred backlog

| Item | Type | Rationale |
|------|------|-----------|
| #35 | enhancement | M, touches hot modules and the same critic files as #20; headline for v1.6.0 |
| #17 | enhancement | New report surface; outside the precision theme |
| #15 | enhancement | RAG performance; outside the precision theme, enhancement budget full |
| #14 | enhancement | Refactor; enhancement budget full |
| #13 | enhancement | Refactor (pairs with #12); enhancement budget full |
| #12 | enhancement | M refactor; enhancement budget full |
| #2 | enhancement | M/L prompt and output-format change; would confound the #11 and #34 measurements |
| Indirect Go drift | deps | Indirect-only; deps OK — deferred |
| Selector candidate pool (#11 follow-up) | enhancement | Out of scope in #11; file a follow-up issue |
| pi-spine `.review/config.toml` threshold guidance (#34) | docs | Lives in another repo; follow-up after SP-081 |

---

## Risks and blockers

- Live models: SP-078, SP-081, and SP-083 need LM Studio (and OpenJev for the critic). If unavailable, record here; #18, #34, #11 stay Partial and the release ships without them.
- `spine doctor` warned that swap is nearly exhausted (1.6 GiB free); run `lms unload --all` before the eval tasks.
- #11 eval cost: prior A/B hit a 45-minute cap at the 8192-token budget (33–461 s per case). SP-083 caps runs per variant; its wave (spine wave 4) uses a wait timeout above 4 h.
- Wave 0 recovery (batch `20261102T054035-ac43`): SP-077 and SP-082 finished their work but failed hand-off. Causes: (1) a transient reviewer failure (`google/gemini-3.1-pro-preview` exited in ~8 s with no artifact; a direct `pi` smoke test answered OK afterwards); (2) the contract script's `stet finish` after a clean worker run made the engine's contract re-check start a fresh full lane review (742 s > 600 s `contract.testTimeoutMs`, nondeterministic new findings; lane 1's baseline also covers SP-077's diff). Operator actions: triaged 36 findings in the lane 1 session (all false positives, wrong suggestions, or SP-077 code out of SP-082's scope; none real), verified the incremental contract passes, then `spine batch retry SP-077`, `spine batch retry SP-082`, and `SPINE_STET_NO_AUTO_FINISH=1 spine batch resume --force` so worker and engine contract runs share one open session. Follow-up for the final report: the contract script should not auto-finish under spine (or the engine should reuse the worker's verdict).
- Spine waves are 0-based: wave 0 (SP-077, SP-079, SP-082), 1 (SP-078), 2 (SP-080), 3 (SP-081), 4 (SP-083).
- SP-079 reconstructs hunks from pi-spine git history (read-only, absolute path `/Users/cdelgado/Documents/github/pi-spine`); unrecoverable rows are skipped and counted, never invented.
- #20 and #34 change what the critic drops; read wave-review counts for waves 2–4 with that in mind.
- #5 may touch hot modules (`cli/internal/run`, `cli/cmd/stet/main.go`): Review Level 2, serial lane.

---

## Release notes draft

```markdown
## stet v1.5.0

### Fixes
- Wave review and `stet list` no longer show findings from another clone's or worktree's session (#5)
- Critic verifies findings against the reviewer's expanded context; measured drop rate and latency recorded (#18)

### Improvements
- System One critic rejects "missing / undefined outside the shown code" claims (#20)
- `systemone_min_keep` tuned on a labeled dataset of real dismissals (#34)
- Cursor rule budget set from an A/B eval with rule-targeted cases (#11)

### Install
curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh | STET_RELEASE_TAG=v1.5.0 bash
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
- [ ] `git tag -a v1.5.0 --cleanup=verbatim -F release-notes.md` + `git push origin v1.5.0`
- [ ] `release.yml` run green (publish + verify-install ×3) — run: {url}
- [ ] `scripts/post-release-smoke.sh v1.5.0` exit 0
- [ ] Local `stet` replaced: `stet --version` reports `v1.5.0` (path: {command -v stet})
- [ ] `spine-tasks/CONTEXT.md` release note added
