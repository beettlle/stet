# Release manifest — v1.7.0

**Created:** 2026-11-21
**Last tag:** v1.6.0
**Target version:** v1.7.0
**Bump type / profile:** minor
**Operator approved scope:** yes (2026-11-21)
**Worker model pin:** zai/glm-5.3-flash
**Agent pin override:** none

**Theme:** Protocol A rule-section selection (`digest_fair`) to improve review-criteria quality under the 1000-token rules budget (closes #40). Cite #11 (closed) as prior digests/budget work.

**WIP parking:** Protocol A implementation stashed as `stash@{0}: protocol-a-wip` before Phase 0 (required clean `main`). Packets land/adapt that work; do not push stash to `main` outside spine integrate.

---

## Composition audit

| Bucket | Selected | Profile limit | Status |
|--------|----------|---------------|--------|
| Documentation | 1 | 2–4 | PASS |
| Bug fixes | 1 | 3–5 | WARN: backlog has 0 labeled bugs; only govulncheck toolchain pin |
| Enhancements | 2 | 1–2 | PASS |
| **Total tasks** | 4 | 10–15 | PASS |

**Profile audit:** PASS with WARN (bug count &lt; 3 — small backlog; do not invent work)

---

## Dependency drift

| Check | Result |
|-------|--------|
| `go list -u -m` | Indirect only: go-md2man, pflag, yaml.in, check.v1 — deferred |
| `govulncheck` | **GO-2026-6859** (`net/http` MaxBytesReader header race) in stdlib **go1.27.3**, fixed in **go1.27.4**; reachable via ollama/openaicompat HTTP clients |
| Toolchain / Actions pins | Workflows use `go-version: "1.27.x"` (will float); local was 1.27.3. **Include SP-088** to pin `1.27.4` in ci.yml + release.yml |
| Extension npm | types/vitest/typescript drift — deferred (not shipped) |
| Action | include SP-088 (bug/hygiene); other deps deferred |

---

## Intake table

| Issue # | Labels | Mapped SP-* | Bucket | Profile fit | Notes |
|---------|--------|-------------|--------|-------------|-------|
| #40 | (none) | — (gap) | enh | minor ✓ | Customer: 1000-token budget drops ~187 sections; Protocol A |
| #2 | enhancement | — (gap) | enh | optional Partial | Cap suggestion; defer full prompt audit |
| #11 | — | closed | — | cite only | Digests + maxReviewRuleTokens=1000 already shipped |
| #12–14, #17 | enhancement | — | enh | defer | Exceed 1–2 enhancement slots |

---

## Selected tasks

| SP-ID | Issue | Bucket | Size | Title | Link |
|-------|-------|--------|------|-------|------|
| SP-088 | — | bug | S | Pin Go toolchain to 1.27.4 (GO-2026-6859) | Hygiene (govulncheck) |
| SP-089 | #40 | docs | S | Document Protocol A / `rules_selector_candidates` | Docs for Closes #40 |
| SP-090 | #40 | enh | M | Protocol A: digest_fair selector + wiring + section log | Closes #40 |
| SP-091 | #40 | enh | S | model_eval harness: selector flags + section-hit scoring | Supports #40 / research |

**Release scope ID:** `SP-088,SP-089,SP-090,SP-091`

**Not selected this release:** Partial #2 (suggestion cap) — defer to keep enhancements ≤2 product themes (Protocol A + harness). Operator may add SP-092 if they override.

---

## Gaps requiring new packets

| Issue | Bucket | Proposed SP-ID | Author with |
|-------|--------|----------------|-------------|
| GO-2026-6859 | bug | SP-088 | create-spine-tasks |
| #40 docs | docs | SP-089 | create-spine-tasks |
| #40 Protocol A | enh | SP-090 | create-spine-tasks (adapt `protocol-a-wip` stash) |
| #40 harness | enh | SP-091 | create-spine-tasks (adapt stash) |

---

## Wave plan snapshot

```text
Spine plan — ids
4 task(s) · 2 wave(s) · maxParallel 2

Wave 0 · 3 tasks · 2 rounds (queued by maxParallel)
  Round 1 (2 parallel):
    Lane 1: SP-088 — pin-go-1-27-4
    Lane 2: SP-089 — protocol-a-docs
  Round 2:
    Lane 1: SP-090 — protocol-a-selector

Wave 1 · 1 task
  Lane 1: SP-091 — protocol-a-harness

Start: spine batch start ids --wave 0
```

---

## Wave log

| Wave | Batch ID | Tasks landed | stet wave review (active findings) | release-check log | Exit | Pushed | Issues closed |
|------|----------|--------------|------------------------------------|-------------------|------|--------|---------------|
| 0 | 20261121T230955-1293 (salvage) | SP-088 | n/a | prior | 0 | yes | — |
| 0–1 | operator land after ae11 abort | SP-089, SP-090, SP-091 | skipped — workers mid-prompt | `/tmp/stet-release-check-v170-land.log` | 0 | yes (`7eaa90e`) | #40 closed |

---

## Deferred backlog

| Item | Type | Rationale |
|------|------|-----------|
| #2 (full) | enhancement | Compact JSON already shipped; suggestion cap / prompt audit deferred to keep enh ≤2 |
| #12 | enhancement | Exceeds minor enhancement budget |
| #13 | enhancement | Exceeds minor enhancement budget |
| #14 | enhancement | Exceeds minor enhancement budget |
| #17 | enhancement | Exceeds minor enhancement budget |
| Indirect Go module bumps | hygiene | No threshold for patch/minor inclusion |
| Extension npm | hygiene | Extension not shipped |
| Default `rules_selector=true` / `.review/config.toml` enable | product | Latency under critic-on (~3.5×); document enablement only |
| Raise default `rules_max_tokens` | product | SP-083 + Protocol A A/B reject larger default budget |

---

## Non-goals (v1.7.0)

- Raising `maxReviewRuleTokens` / default `rules_max_tokens`
- Enabling `rules_selector` by default or in repo `.review/config.toml`
- Recommending bare `full` selector on CursorRules-heavy repos
- Protocol B/C, reopening #11

---

## Publish checklist

- [x] All release-scoped tasks `.DONE` and on `main` (SP-088–091)
- [x] `scripts/release-check.sh` exit 0 on final HEAD (`/tmp/stet-release-check-v170-land.log`; re-run `/tmp/stet-release-check-v170-phase5.log` exit 0)
- [x] `ci.yml` green on final HEAD `e1b6cb9` (https://github.com/beettlle/stet/actions/runs/44970266205)
- [x] Every `Closes #NNN` CLOSED (#40)
- [x] Release notes written (`spine-tasks/_authoring/release-v1.7.0/release-notes.md`)
- [ ] Operator approved publish
- [ ] Tag `v1.7.0` pushed; `release.yml` green
- [ ] `post-release-smoke.sh v1.7.0` exit 0
- [ ] Local `stet --version` reports `v1.7.0`
