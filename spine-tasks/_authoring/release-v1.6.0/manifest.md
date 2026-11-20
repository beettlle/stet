# Release manifest — v1.6.0

**Created:** 2026-11-19
**Last tag:** v1.5.0
**Target version:** v1.6.0
**Bump type / profile:** minor
**Operator approved scope:** yes (2026-11-19)
**Worker model pin:** zai/glm-5.3-flash
**Agent pin override:** none

---

## Composition audit

| Bucket | Selected | Profile limit | Status |
|--------|----------|---------------|--------|
| Documentation | 0 | 1-2 | PASS (none open) |
| Bug fixes | 1 | 3–5 | PASS |
| Enhancements | 3 | ~1-2 (max 3) | PASS |
| **Total tasks** | 4 | ~3-4 per wave | PASS |

**Profile audit:** PASS

---

## Dependency drift

| Check | Result |
|-------|--------|
| `go list -u -m` | All indirect updates, no major drift |
| `govulncheck` | not run: govulncheck not installed |
| Toolchain / Actions pins | Local go 1.27.3. Actions use 1.27.x. |
| Extension npm | Typescript/Vitest out of date — deferred (not shipped) |
| Action | deferred: no critical vulnerability or strict requirement to bump |

---

## Selected tasks

| SP-ID | Issue | Bucket | Size | Title | Link |
|-------|-------|--------|------|-------|------|
| SP-084 | #39 | bug | S | v1.4.0 false positives: unused-import claim not filtered | Closes #39 |
| SP-085 | #35 | enh | S/M | Pass task intent into the review and critic | Closes #35 |
| SP-086 | #20 | enh | S/M | System One critic accepts claims that something is missing outside shown code | Closes #20 |
| SP-087 | #15 | enh | S | RAG symbol lookup sequential git grep processes batching | Closes #15 |

**Release scope ID:** `SP-084,SP-085,SP-086,SP-087`

---

## Gaps requiring new packets

| Issue | Bucket | Proposed SP-ID | Author with |
|-------|--------|----------------|-------------|
| #39 | bug | SP-084 | create-spine-tasks |
| #35 | enh | SP-085 | create-spine-tasks |
| #20 | enh | SP-086 | create-spine-tasks |
| #15 | enh | SP-087 | create-spine-tasks |

---

## Wave plan snapshot

```text
Spine plan — ids
4 task(s) · 3 wave(s) · maxParallel 2

Wave 0 · 2 tasks · 2 lanes in parallel
  Lane 1: SP-084 — unused-import-false-positive
  Lane 2: SP-087 — batch-git-grep-rag

Wave 1 · 1 task
  Lane 1: SP-086 — critic-missing-code-claims

Wave 2 · 1 task
  Lane 1: SP-085 — pass-task-intent
```

---

## Wave log

| Wave | Batch ID | Tasks landed | stet wave review (active findings) | release-check log | Exit | Pushed | Issues closed |
|------|----------|--------------|------------------------------------|-------------------|------|--------|---------------|
| 0 | 20261120T173358-5019 | SP-084, SP-087 | review failed (System One unreachable) | /tmp/stet-release-check-wave-0.log | 0 | yes | #39, #15 |
| 1 | 20261120T181408-9651 | SP-086 | 3 (WARNINGs, advisory) | /tmp/stet-release-check-wave-1.log | 0 | yes | #20 |

---

## Deferred backlog

| Item | Type | Rationale |
|------|------|-----------|
| #17 | enhancement | Exceeds minor profile limits (max 3 enhancements) |
| #14 | enhancement | Exceeds minor profile limits (max 3 enhancements) |
| #13 | enhancement | Exceeds minor profile limits (max 3 enhancements) |
| #12 | enhancement | Exceeds minor profile limits (max 3 enhancements) |
| #2 | enhancement | Exceeds minor profile limits (max 3 enhancements) |

---

## Risks and blockers

- none

---

## Release notes draft

```markdown
## stet v1.6.0

### Fixes
- Fix v1.4.0 false positives where unused-import claims weren't filtered when used in other hunks (#39)

### Enhancements
- Pass task intent into the review and critic via --intent-file (#35)
- Improve System One critic to avoid accepting claims for code outside the shown context (#20)
- Optimize RAG symbol lookup by batching git grep processes (#15)

### Install
curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh | STET_RELEASE_TAG=v1.6.0 bash
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
- [ ] `git tag -a v1.6.0 --cleanup=verbatim -F release-notes.md` + `git push origin v1.6.0`
- [ ] `release.yml` run green (publish + verify-install ×3) — run: {url}
- [ ] `scripts/post-release-smoke.sh v1.6.0` exit 0
- [ ] Local `stet` replaced: `stet --version` reports `v1.6.0` (path: {command -v stet})
- [ ] `spine-tasks/CONTEXT.md` release note added
