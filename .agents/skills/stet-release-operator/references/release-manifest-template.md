# Release manifest — v{TARGET}

Copy to `spine-tasks/_authoring/release-v{TARGET}/manifest.md` in Phase 2.

**Created:** {YYYY-MM-DD}
**Last tag:** v{CURRENT}
**Target version:** v{TARGET}
**Bump type / profile:** patch | minor | major
**Operator approved scope:** no | yes ({YYYY-MM-DD})
**Worker model pin:** {agents.worker.model from .spine/spine-config.json} — do not change mid-release
**Agent pin override:** none | yes ({date}, {reason}) — record before any mid-release pin change

---

## Composition audit

| Bucket | Selected | Profile limit | Status |
|--------|----------|---------------|--------|
| Documentation | {n} | {limit} | PASS / WARN |
| Bug fixes | {n} | 3–5 | PASS / WARN |
| Enhancements | {n} | {limit} | PASS / WARN / FAIL |
| **Total tasks** | {n} | {cap} | PASS / WARN |

**Profile audit:** PASS | PASS with operator override ({reason}) | FAIL (do not proceed)

---

## Dependency drift

| Check | Result |
|-------|--------|
| `go list -u -m` | {summary or "all current"} |
| `govulncheck` | {N reachable findings / none / not run: reason} |
| Toolchain / Actions pins | {current vs latest} |
| Extension npm | {summary} — deferred (not shipped) |
| Action | include SP-### / #NNN \| deferred: {reason} |

---

## Selected tasks

| SP-ID | Issue | Bucket | Size | Title | Link |
|-------|-------|--------|------|-------|------|
| SP-### | #NNN | doc / bug / enh | S/M | … | Closes / Partial |

**Release scope ID:** `SP-041,SP-042,...` (comma-separated, no spaces; used by `spine plan`, `spine batch start`, `spine run sequence`)

---

## Gaps requiring new packets

| Issue | Bucket | Proposed SP-ID | Author with |
|-------|--------|----------------|-------------|
| #NNN | bug | SP-### | create-spine-tasks |

---

## Wave plan snapshot

```text
(paste output of: spine plan <release-scope>)
```

---

## Wave log

| Wave | Batch ID | Tasks landed | stet wave review (active findings) | release-check log | Exit | Pushed | Issues closed |
|------|----------|--------------|------------------------------------|-------------------|------|--------|---------------|
| 0 | {id} | SP-### | {n} ({path}) | /tmp/stet-release-check-wave-0.log | 0 | yes | #NNN |

---

## Deferred backlog

| Item | Type | Rationale |
|------|------|-----------|
| #NNN | enhancement | Exceeds patch profile; next minor |

---

## Risks and blockers

- {e.g. #5 needs a multi-worktree repro; deferred}

---

## Release notes draft

Final copy goes to `spine-tasks/_authoring/release-v{TARGET}/release-notes.md` and becomes the annotated tag message (and therefore the GitHub Release body). No line may start with `#` unless the tag is created with `--cleanup=verbatim`.

```markdown
## stet v{TARGET}

### Fixes
- {user-visible change} (#NNN)

### Documentation
- {change} (#NNN)

### Install
curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh | STET_RELEASE_TAG=v{TARGET} bash
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
- [ ] `git tag -a v{TARGET} --cleanup=verbatim -F release-notes.md` + `git push origin v{TARGET}`
- [ ] `release.yml` run green (publish + verify-install ×3) — run: {url}
- [ ] `scripts/post-release-smoke.sh v{TARGET}` exit 0
- [ ] Local `stet` replaced: `stet --version` reports `v{TARGET}` (path: {command -v stet})
- [ ] `spine-tasks/CONTEXT.md` release note added
