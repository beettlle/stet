# Task: SP-061 — `stet benchmark` loads the repo `.review/config.toml`

**Created:** 2026-10-22
**Size:** S

## Review Level: 2

**Assessment:** Small change in `runBenchmark` in `cli/cmd/stet/main.go` (hot shared module) plus a CLI test.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 0, Security: 0, Reversibility: 1 — Level 2 for the hot module

## Mission

Closes #32

`runBenchmark` (`cli/cmd/stet/main.go` ~line 1854) is the only command that loads config with an empty repo root:

```go
cfg, err := config.Load(cmd.Context(), config.LoadOptions{RepoRoot: "", Overrides: overrides})
```

With `RepoRoot` empty, `config.Load` skips `<repo>/.review/config.toml`, so a repo configured for an OpenAI-compatible server (`provider = "openai"`, `openai_base_url`, `model`) is benchmarked against the default Ollama URL. `stet doctor` in the same directory honors the repo config.

Fix: resolve the repo root the way `runDoctor` does (~line 1939): get the working directory, call `git.RepoRoot(cwd)`, and fall back to `""` outside a git repo so `stet benchmark` still works anywhere. If `os.Getwd` fails, return the same `erruser` error `runDoctor` returns. Pass the repo root and the existing `overrides` to `config.Load`. Change nothing else in `runBenchmark`.

## Dependencies

- **None**

## Context to Read First

**Tier 3 (reference docs):**

- `cli/cmd/stet/main.go` — `runBenchmark` (~line 1854), `runDoctor` (~line 1939)
- `cli/cmd/stet/main_test.go` — `TestRunCLI_benchmarkOpenAILabelsEndToEnd` (~line 240) for the OpenAI mock server; `initRepo` and the `os.Chdir` pattern in `TestRunCLI_runNoNewHunksWarnsIgnoredFlags` (~line 631)
- `cli/internal/config/config.go` — `LoadOptions`, how repo config is read

## Environment

- **Workspace:** repo root (Go module root; packages under `cli/`)
- **Services required:** LM Studio and OpenJev for the stet contract step only

## File Scope

- `cli/cmd/stet/main.go`
- `cli/cmd/stet/main_test.go`

## Documentation Requirements

**Must Update:** none (SP-065 updates `README.md`).

**Check If Affected:** `docs/cli-extension-contract.md` — it does not document `stet benchmark` today; leave it unchanged.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `bash scripts/check-gofmt.sh && go vet ./cli/... && go test ./cli/cmd/stet/... -count=1 && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/cmd/stet/main.go`, `cli/cmd/stet/main_test.go` |
| fileScopeMustNotChange | `cli/internal/config/**`, `extension/**`, `.review/**` |
| completionCriteria | Inside a temp git repo whose `.review/config.toml` sets `provider = "openai"`, `openai_base_url` (pointing at an `httptest` server), and `model`, with no `STET_PROVIDER` / `STET_OPENAI_BASE_URL` / `STET_MODEL` in the environment, `stet benchmark` exits 0 and the mock server receives the configured model (test); outside a git repo `stet benchmark` still loads config (existing behavior); stet contract review exits 0 |

**Stet-gated:** the contract ends with `bash scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record stet reviewer bugs under Discoveries in STATUS.md. Never dismiss a real defect without fixing it.

## Steps

### Step 1: Resolve repo root in runBenchmark

- [ ] `runBenchmark` resolves the repo root like `runDoctor` and passes it to `config.Load`
- [ ] `spine_review_step` after the step

### Step 2: Testing & Verification

> ZERO test failures allowed.

- [ ] Test: temp repo with `.review/config.toml` (OpenAI provider, mock server URL, model); provider env vars cleared with `t.Setenv`; benchmark exits 0 and the server sees the configured model. Isolate global config the same way neighboring config tests do. Do not run the test in parallel (it changes cwd).
- [ ] Run Contract `testCommand` (includes the stet review; triage as described in the Contract)

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Fix:** `fix(SP-061): description`
- **Tests:** `test(SP-061): description`

## Do NOT

- Change the benchmark output, estimate, or `cli/internal/benchmark` (SP-060 owns it)
- Refactor other `config.Load` call sites or the existing `os.Pipe` test captures (SP-062 owns the capture refactor)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
