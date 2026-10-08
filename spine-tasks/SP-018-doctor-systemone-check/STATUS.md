**Current Step:** Complete (Steps 1–2)
**Status:** Done — awaiting batch engine final review
**Last Updated:** 2026-10-08
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 1: Implement doctor check

**Status:** Complete

- [x] System One check in `runDoctor`
- [x] Sentinel errors mapped to messages and exit codes

Implementation: new `checkSystemOne(ctx, cfg)` helper in `cli/cmd/stet/main.go`, called at the end of `runDoctor` (after the LLM checks succeed). No-op when `!cfg.CriticEnabled || cfg.CriticBackend != "systemone"`. Builds `systemone.NewClient(cfg.SystemOneBaseURL, cfg.SystemOneAPIKey, nil)`:
- `Health` + `ErrUnreachable` → stderr `System One server unreachable at <base_url>`, exit 2.
- `Health`/`Models` + `ErrUnauthorized` → stderr hint to set `STET_SYSTEMONE_API_KEY`, exit 2.
- Other `Health`/`Models` errors → error text to stderr, exit 1 (mirrors the existing generic LLM failure path).
- Model missing from `Models` → stderr `System One model "<model>" not found at <base_url>` + `Available models: ...` (with `(none)` for an empty list), exit 1.
- Success → stdout `System One OK` and `System One model: <model>`.
- No `POST /v1/systemone`; API key never printed (client redacts errors; helper never logs the key).

## Step 2: Testing and verification

**Status:** Complete

- [x] `TestRunCLI_doctorSystemOne*` cases per PROMPT — OK, unreachable (closed port), health 401, models 401, model missing, critic disabled / backend llm (zero System One requests)
- [x] Contract `testCommand` (`cd cli && go test ./cmd/stet/... -count=1 -cover -run doctor` → ok) and full `cd cli && go test ./... -cover` (`cli/cmd/stet` ok; see Discoveries for unrelated flaky packages)
- [x] `main.go` per-file coverage 75.1% ≥ 72% (baseline was 74.8%; coverage improved)

Additional verification: `go build ./...`, `go vet ./cmd/stet/...`, `gofmt` clean on added code, `go test -race -run TestRunCLI_doctor` ok, new tests assert no `POST /v1/systemone` and that the API key never appears in output.

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| 2026-10-08 | 1 | plan | skipped in-worker (SP-195: nested reviewer spawn blocked; batch engine runs final review after .DONE) |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-09-30 | Decomposition pass: dependencies changed SP-017 → SP-021 + SP-023 | Health/Models now come from SP-021; last prior `main.go` change is SP-023 |
| 2026-10-08 | Pre-existing `gofmt` violations in `cli/cmd/stet/main.go` (lines ~1473–1618, SP-023 region) and `main_test.go` (lines ~2237, ~2347) | Not introduced by SP-018 (verified on baseline); added code is gofmt-clean. Tech debt logged in `spine-tasks/CONTEXT.md`; left untouched to keep the diff scoped |
| 2026-10-08 | Full `go test ./...` has pre-existing flaky failures in git-subprocess/TempDir-heavy tests (`internal/stats`, `internal/run`, `internal/git`) under parallel package load; different packages fail each run, all pass with `-p 4`, in isolation, and on baseline (stash-verified) | Not caused by SP-018; `cli/cmd/stet` passes every run (75.1% coverage). Candidate for a hygiene task |
| 2026-10-08 | System One client retries unreachable connections with capped exponential backoff (~1s+2s+4s), so the doctor unreachable test takes ~7s | Accepted: helpers `_initialBackoff`/`_maxBackoff` are package-private to `internal/systemone`, not shorten-able from `package main` tests |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-09-30 | Authored | Packet created |
| 2026-10-08 | Step 1 | `checkSystemOne` helper added; `runDoctor` returns it after LLM checks; build + vet pass |
| 2026-10-08 | Review | Plan review skipped per SP-195 (in-worker spawn blocked; `spawnFailed: false`) |
| 2026-10-08 | Step 2 | 6 new `TestRunCLI_doctorSystemOne*` tests; contract testCommand ok; full suite ok (modulo pre-existing flakes documented above); main.go coverage 75.1% |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes
