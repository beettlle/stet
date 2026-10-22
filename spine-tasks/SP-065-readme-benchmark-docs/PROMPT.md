# Task: SP-065 — README: `stet benchmark` config source and end-to-end estimate

**Created:** 2026-10-22
**Size:** S

## Review Level: 0

**Assessment:** Docs-only; describes behavior on `main` after SP-060 and SP-061.
**Score:** 0/8 — Blast radius: 0, Pattern novelty: 0, Security: 0, Reversibility: 0

## Mission

`README.md` documents `stet benchmark` in the command table (~line 131) and the options list (~lines 152–153). After this release it should state two behaviors (read the code on `main`; it wins over this text):

1. **Config source (SP-061):** `stet benchmark` uses the same configuration as the other commands: inside a git repository it reads the repo `.review/config.toml` (provider, base URL, model) in addition to global config, environment, and flags; outside a repository it uses global config, environment, and flags only. Check `runBenchmark` in `cli/cmd/stet/main.go`.
2. **Estimate (SP-060):** the "Typical stet hunk ... estimated" line is approximate. For OpenAI-compatible providers the rate is end-to-end (prefill plus generation), and the estimate scales the measured request time to a typical hunk's prompt and output size. Check `estimateHunkSeconds` (or the helper SP-060 added) in `cli/internal/benchmark/benchmark.go`.

Add one or two sentences per behavior near the existing `stet benchmark` bullets. Do not restructure the README or change the performance table at the top.

## Dependencies

- **Task:** SP-060 (estimate behavior must exist)
- **Task:** SP-061 (repo config loading must exist)

## Context to Read First

**Tier 3 (reference docs):**

- `README.md` — `stet benchmark` rows and bullets
- `cli/cmd/stet/main.go` — `runBenchmark`
- `cli/internal/benchmark/benchmark.go` — estimate helper

## Environment

- **Workspace:** repo root
- **Services required:** None

## File Scope

- `README.md`

## Documentation Requirements

**Must Update:**

- `README.md` — the two behaviors in Mission

## Contract

| Field | Value |
|-------|-------|
| testCommand | `true` |
| fileScopeMustChange | `README.md` |
| fileScopeMustNotChange | `extension/**`, `.github/**`, `.review/**` |
| completionCriteria | `README.md` states that `stet benchmark` reads the repo `.review/config.toml` inside a repository, and that the typical-hunk estimate is approximate and end-to-end for OpenAI-compatible providers (check with `rg -n 'benchmark' README.md`); every statement matches the code on `main` |

## Steps

### Step 1: Benchmark section

- [ ] Config-source sentence
- [ ] Estimate sentence

### Step 2: Testing & Verification

- [ ] `rg -n 'benchmark' README.md` shows both additions
- [ ] `rg -l 'README.md' cli/ scripts/` — note in STATUS whether any test references the README (none expected)

## Completion Criteria

- [ ] All steps complete
- [ ] Every statement matches the code on `main`

## Git Commit Convention

- **Docs:** `docs(SP-065): description`

## Do NOT

- Change code, the README performance table, or unrelated sections
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
