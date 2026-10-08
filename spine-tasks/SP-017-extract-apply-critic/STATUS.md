**Current Step:** Complete
**Status:** Done — completion criteria met
**Last Updated:** 2026-09-30
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

## Step 1: Extract helper

**Status:** Complete

- [x] `applyCritic` with current logic
- [x] Both call sites replaced
- [x] Diff confined to critic blocks

## Step 2: Testing and verification

**Status:** Complete

- [x] Existing critic tests unchanged and passing (`internal/review` ok, untouched)
- [x] Direct helper test: `TestApplyCritic` — 8 subtests (disabled guard, empty batch, keep-all, drop-rejected, generate-error + trace + wrap, same-model KeepAlive/MaxCompletionTokens, different-model no KeepAlive, parse-retry); `applyCritic` at 100% statement coverage
- [x] Contract `testCommand` ok (77.1%); `go test ./... -cover` green ×3 consecutive runs (flaky TempDir cleanup hits random packages ~1-in-3 runs; see Discoveries)
- [x] `run.go` file coverage 77.1% ≥ 72%

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| 2026-09-30 | 1 | plan | SKIPPED — nested reviewer spawn blocked in pi worker (SP-195); engine reviews after worker success |
| 2026-09-30 | 2 | plan | SKIPPED — same in-worker spawn block (SP-195) |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| 2026-09-30 | Decomposition pass: original M "pipeline wiring" split into SP-017 (refactor), SP-022 (System One branch in `run.go`), SP-023 (CLI flag, client construction, docs) | Refactor has no deps and can run in the first wave; docs kept with the flag (SP-023) so they never describe an unavailable option |
| 2026-09-30 | `run.go` and `run_test.go` are not gofmt-clean at baseline (const/struct alignment in untouched sections) | Did not reformat — would expand the diff beyond critic blocks; new code is gofmt-conformant |
| 2026-09-30 | **Known flaky baseline:** `internal/run` intermittently fails one random test with `TempDir RemoveAll cleanup: unlinkat …/.git: directory not empty` (macOS). Seen at baseline (`TestRun_pipeline_multipleHunks…`) and after the change (`TestStart_dryRun_noHunks`); passes on rerun. Unrelated to critic. | Treat this cleanup error as known flake; rerun the suite before declaring failure |
| 2026-09-30 | Flake affects any package whose tests build git temp repos (`internal/run`, `internal/stats`, `cmd/stet`); test bodies pass, only `t.TempDir()` cleanup fails | Candidate hygiene fix (retry/robust initRepo teardown) — out of SP-017 scope |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| 2026-09-30 | Authored | Packet created as pipeline wiring (M) |
| 2026-09-30 | Split | Narrowed to refactor; backend → SP-022, CLI/docs → SP-023 |
| 2026-09-30 | Baseline | Suite green (1 env flake); `run.go` file coverage 73.8%, `runReviewPipeline` 52.4% |
| 2026-09-30 | Step 1 done | `applyCritic` extracted; diff confined to the two critic blocks + helper; coverage 75.0% file / 61.4% `runReviewPipeline` |
| 2026-09-30 | Step 2 done | `TestApplyCritic` (8 subtests, fake llm.Client); `applyCritic` 100%; contract cmd ok; full suite green ×3; `run.go` 77.1% ≥ 72% |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes

- SP-022 can add the System One backend inside `applyCritic` alone (one place, one call site pair already collapsed).
- Test fake `fakeCriticLLM` implements the full `llm.Client` interface (4 methods), not just `CriticClient`, so it is reusable for future pipeline tests.
