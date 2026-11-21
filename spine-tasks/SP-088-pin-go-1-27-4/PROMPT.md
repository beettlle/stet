# Task: SP-088 — pin-go-1-27-4

**Created:** 2026-11-21
**Size:** S
**Type:** llm

## Review Level: 0 (None)

**Risk:** Workflow pin only; CI/release use setup-go. Wrong pin breaks CI until fixed.
**Score:** 1/8 — Blast radius: 1, Pattern novelty: 0, Security: 0, Reversibility: 0

## Mission

Pin the Go toolchain used by CI and release to **1.27.4** to clear **GO-2026-6859** (`net/http` MaxBytesReader header race in go1.27.3, fixed in go1.27.4). `govulncheck ./cli/...` reported reachable traces via ollama/openaicompat HTTP clients.

## Dependencies

- **None**

## Context to Read First

- `.github/workflows/ci.yml` — `setup-go` / `go-version`
- `.github/workflows/release.yml` — same pin (must match ci.yml)

## Environment

- **Workspace:** `.github/workflows/`
- **Services required:** None

## File Scope

- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`

## Contract

| Field | Value |
|-------|-------|
| `testCommand` | `rg -n 'go-version: \"1\\.27\\.4\"' .github/workflows/ci.yml .github/workflows/release.yml` |
| `fileScopeMustChange` | `.github/workflows/ci.yml`, `.github/workflows/release.yml` |
| `fileScopeMustNotChange` | `cli/**`, `extension/**`, `spine-tasks/**` |

## Steps

### Step 0: Pin go-version to 1.27.4
In both `.github/workflows/ci.yml` and `.github/workflows/release.yml`, change `go-version: "1.27.x"` to `go-version: "1.27.4"`. Keep any comments that explain why the pin is explicit (vs go.mod). Do not change `node-version` or other Actions majors.

### Step 1: Testing & Verification
Run the Contract `testCommand`. Confirm both workflow files contain `go-version: "1.27.4"` and no remaining `1.27.x` go-version lines for setup-go.

## Completion Criteria

- [ ] Both workflows pin Go **1.27.4**
- [ ] Contract `testCommand` exits 0

## Documentation Requirements

### Must Update
- None (workflow pin only)

### Check If Affected
- None

## Do NOT

- Do NOT bump Actions majors (`checkout@v7`, etc.)
- Do NOT change `go.mod` `go 1.22` minimum unless required for the pin
- Do NOT edit `.spine/`, `AGENTS.md`, `CLAUDE.md`, or `.gitnexus/`

## Git Commit Convention

Prefix commits with `fix(SP-088): ` at step boundaries.
