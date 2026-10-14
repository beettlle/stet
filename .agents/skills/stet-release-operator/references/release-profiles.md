# Release profiles (semver-driven scope)

Use in **Phase 0** to derive the bump type and in **Phase 2** to cap scope. stet has no `package.json` version: the current version is the newest `v*` tag.

```bash
git describe --tags --abbrev=0 --match 'v*'   # e.g. v1.0.0
```

## Derive bump type

| Last tag → Target | Bump type | Profile |
|-------------------|-----------|---------|
| `v1.0.0` → `v1.0.1` | patch | `patch` |
| `v1.0.0` → `v1.1.0` | minor | `minor` |
| `v1.0.0` → `v2.0.0` | major | `major` |

Reject a target that is not exactly one semver step above the last tag (for example `v1.0.0` → `v1.0.2`, or `v1.0.0` → `v1.1.1`) unless the operator overrides and the manifest records why.

When the operator gives only `patch` / `minor` / `major`, compute the next version from the last tag and record both in the manifest.

## Invocation parsing

| Operator says | Profile |
|---------------|---------|
| "release v1.0.1", "patch release", "hotfix release" | `patch` |
| "release v1.1.0", "minor release", "feature release" | `minor` |
| "release v2.0.0", "major release", "breaking release" | `major` |
| "run a stet release" (no version) | **Ask** for target version or bump type |

## Profile budgets

### Patch

Stability and correctness. Smallest window.

| Bucket | Target | Hard limits |
|--------|--------|-------------|
| Documentation | 1–2 small fixes | No new feature docs |
| Bug fixes | 3–5 | Primary focus; S-sized |
| Enhancements | **0** | Defer all; warn if operator insists without override |
| Total tasks | **5–8** | Warn if >8 without override |
| Max waves | 1–2 | Prefer one wave |
| Task sizes | **S only** | Split M/L before inclusion |

### Minor

New capability balanced with stability.

| Bucket | Target | Hard limits |
|--------|--------|-------------|
| Documentation | 2–4 | Include docs for the chosen enhancement |
| Bug fixes | 3–5 | Same as patch |
| Enhancements | **1–2** | At most one M, split into safe waves |
| Total tasks | **10–15** | Warn if >15 without override |
| Max waves | 2–4 | ≤4 M-sized tasks per wave |
| Task sizes | S/M | Split L/XL before inclusion |

### Major

Breaking changes (CLI flags, config keys, `.review/` layout, JSON output contract in `docs/cli-extension-contract.md`). Operator-defined epic scope.

| Bucket | Target | Hard limits |
|--------|--------|-------------|
| Documentation | Migration notes + README/contract pass | Required for every breaking change |
| Bug fixes | 3–5 critical | Prioritize regressions blocking migration |
| Enhancements | Multiple | Breaking changes need a migration path |
| Total tasks | Operator-defined | Manifest lists explicit epic boundaries |

**Require** explicit operator approval of epic scope before Phase 3. Never auto-select major scope from the backlog.

## Selection order (all profiles)

Apply in order; stop when the budget is full:

1. **Documentation** — `label:documentation`, docs-only pending SP-* tasks
2. **Bug fixes** — open `label:bug` with user impact; prefer already-tasked or quick S; **govulncheck findings on called symbols** insert here
3. **Enhancements / dependency hygiene** — only if the profile allows; dependency drift that crosses a threshold consumes an enhancement slot
4. **Defer** the rest with a one-line rationale (including "deps OK — deferred")

## Profile audit (must pass before Phase 3)

| Check | Patch | Minor | Major |
|-------|-------|-------|-------|
| Enhancement count | >0 without override → FAIL | >2 without override → FAIL | — |
| Bug count | <3 or >5 without override → WARN | <3 or >5 without override → WARN | <3 → WARN |
| Total tasks | >8 → WARN | >15 → WARN | — |
| M/L tasks | any without override → FAIL | L/XL → FAIL | — |
| Dependency drift recorded | WARN if missing | FAIL if missing | FAIL if missing |

Record `Profile audit: PASS`, `PASS with operator override`, or `FAIL (do not proceed)` in the manifest.

**Small backlog:** when fewer open bugs exist than the profile minimum, record `WARN: backlog has N bugs` and proceed with what exists; do not invent work to fill a bucket.

## Version ↔ tag alignment

The tag pushed in Phase 6 must equal the manifest target. If the operator changes the bump type at the publish gate, update the manifest and re-run the profile audit before tagging.
