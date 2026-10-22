# stet — Context

**Last Updated:** 2026-10-22
**Status:** Active
**Next Task ID:** SP-067

---

## Current State

**Wave 0 landed on `main` (manual merge, batch `20260701T232227`):** SP-001, SP-002, SP-003, SP-004, SP-005, SP-008 marked `.DONE`. Remaining feature backlog: SP-006, SP-007, SP-009.

**Hygiene backlog (SP-010–SP-013):** From 2026-07-01 `.DONE` verification — coverage gates, contract testCommand typos, wave-plan revision, coverage-artifact guard. Run **SP-011, SP-012, SP-013** before the next feature batch; **SP-010** (L) can run in parallel or after wave 1.

**pi-spine v1.2.0:** `contract.mode: required`; preflight `prelanded-file-scope` warns when `fileScopeMustChange` paths already on `main`. Upstream: serial-lane cumulative contract verify ([#62](https://github.com/beettlle/pi-spine/issues/62)), manual-merge detection ([#73](https://github.com/beettlle/pi-spine/issues/73)), DirtyWorktree guard ([#74](https://github.com/beettlle/pi-spine/issues/74)).

### Wave-0 failure modes (batch `20260701T232227`)

1. **Serial-lane contract cross-contamination** — SP-001, SP-002, and SP-003 ran sequentially in lane-1. Final contract review diffs the **cumulative lane worktree** against `main`, so SP-002/SP-003 saw SP-001 deliverables outside their `fileScopeMustChange` and exhausted review (`review_exhausted`). **Fix:** at most one `cli/**` core task per serial lane; use disjoint parallel waves (the documented SP-002 ∥ SP-003 ∥ SP-005 ∥ SP-008 layout), not `spine batch start pending --wave 0` when scopes overlap.

2. **DirtyWorktree from tracked `extension/coverage/`** — Vitest coverage output was tracked on `main`, blocking batch complete with DirtyWorktree. Fixed in commit `175c1a4`; guard in SP-013.

3. **Manual land after batch failure** — Work landed on `main` via operator merge while spine batch status remained failed. Use `spine batch complete --detect-manual-merge` (see [Operator recovery](#operator-recovery)).

### GitHub issues ([beettlle/stet](https://github.com/beettlle/stet/issues))

| Issue | Type | Task | Status |
|-------|------|------|--------|
| [#1](https://github.com/beettlle/stet/issues/1) Configurable file exclude/include patterns | enhancement | SP-001 | Done |

### Phase 1 — CLI core (remaining)

| Task | Summary | Status | Deps |
|------|---------|--------|------|
| SP-009 | Python hunk expansion | Ready | SP-008 (done) |

### Phase 2 — Extension production UX (remaining)

| Task | Summary | Status | Deps |
|------|---------|--------|------|
| SP-006 | Extension dismiss findings | Ready | SP-005 (done) |
| SP-007 | Extension confidence and category filters | Ready | SP-006 |

### Phase 3 — Hygiene (post wave-0 verification)

| Task | Summary | Status | Deps |
|------|---------|--------|------|
| SP-010 | Raise CLI coverage to AGENTS.md gates (77%/72%) | Ready | — |
| SP-011 | Fix spine task contract testCommand typos | Ready | — |
| SP-012 | Revise spine wave plan after wave-0 lessons | Ready | — |
| SP-013 | Guard against extension/coverage re-tracking | Ready | — |

### Phase 4 — System One critic (OpenJev)

Optional critic backend that verifies findings with a System One decision server (OpenJev, [razorback16/openjev](https://github.com/razorback16/openjev); same wire API as hosted Jev) in one request per hunk, instead of one LLM generation per finding. Off by default (`critic_backend = "llm"`).

| Task | Summary | Size | Status | Deps |
|------|---------|------|--------|------|
| SP-014 | Config keys: `critic_backend`, `systemone_*`, env-only API key | S | Ready | — |
| SP-015 | `cli/internal/systemone` client core (`Decide`, errors, retries, key redaction) | S | Ready | — |
| SP-016 | Batch verifier `review.VerifyFindingsSystemOne` | S | Ready | SP-015 |
| SP-017 | Extract `applyCritic` helper in `run.go` (no behaviour change) | S | Ready | — |
| SP-018 | `stet doctor` System One check | S | Ready | SP-021, SP-023 |
| SP-019 | Critic eval dataset loader (`criticeval/dataset.go`) | S | Ready | — |
| SP-020 | Research: rules compliance via System One | M | **Deferred** | SP-025 |
| SP-021 | System One client `Health` / `Models` | S | Ready | SP-015 |
| SP-022 | System One branch in `applyCritic` + run options | S | Ready | SP-016, SP-017 |
| SP-023 | `--critic-backend` flag, client wiring in `main.go`, docs | S | Ready | SP-014, SP-022 |
| SP-024 | Critic eval scoring and threshold sweep (`criticeval/sweep.go`) | S | Ready | SP-016, SP-019 |
| SP-025 | `stet critic-eval` command | S | Ready | SP-018, SP-024 |

Decomposition (2026-09-30): the original M packets were split where each slice is independently testable and the split enables parallel work. SP-015 → SP-015 + SP-021; SP-017 → SP-017 + SP-022 + SP-023; SP-019 → SP-019 + SP-024 + SP-025. SP-014 and SP-016 were re-sized to S; SP-020 stays M. Reasons are recorded in each packet's STATUS Discoveries.

### Phase 5 — Hygiene and rules-compliance follow-ups (2026-10-08)

From the tech-debt backlog below and the SP-020 research follow-up table (`docs/research/systemone-rules-compliance.md`). SP-032 was added because the System One client modelled only `noul` questions; SP-033/SP-034 split the pipeline wiring out of SP-026/SP-027 so libraries and wiring stay S-sized with disjoint scopes.

| Task | Summary | Size | Status | Deps |
|------|---------|------|--------|------|
| SP-026 | Rule-section selector library (`rules/selector.go`, `prompt.AppendRuleSections`) | S | Done | SP-029, SP-031, SP-032 |
| SP-027 | Rule-conflict annotation library (`review/rules_annotation.go`, `Finding.RuleConflict`) | S | Done | SP-026, SP-031, SP-032 |
| SP-028 | Rule-pairs labeled dataset (≥ 100 pairs) and evaluator (`criticeval/rulepairs.go`) | S | Done | — |
| SP-029 | `rules.FilterRules` `**` glob support | S | Done | — |
| SP-030 | Isolate git-heavy tests from git-ai daemon (`internal/testutil`) | M | Done | — |
| SP-031 | `gofmt` the CLI + guard in `scripts/build-all.sh` | S | Done | SP-029, SP-030, SP-032 |
| SP-032 | System One client `choice` questions (`DecideChoice`) | S | Done | — |
| SP-033 | Wire selector behind `rules_selector` | M | Done | SP-026 |
| SP-034 | Wire annotation behind `rules_annotate` | S | Done | SP-027, SP-028, SP-033 |

Landed 2026-10-08 in waves W0 SP-028 ∥ SP-029 ∥ SP-030 ∥ SP-032 → W1 SP-031 → W2 SP-026 → W3 SP-027 ∥ SP-033 → W4 SP-034.

### Phase 6 — Stet in spine (dogfooding, 2026-10-09)

First packets whose contracts review their own diff with stet (see [Stet in spine](#stet-in-spine-dogfooding)).

| Task | Summary | Size | Status | Deps |
|------|---------|------|--------|------|
| SP-035 | `stet doctor` probes System One when `rules_selector` / `rules_annotate` are on | S | Done | — |
| SP-036 | `systemone_timeout` / `STET_SYSTEMONE_TIMEOUT` (default 10s, `[1s, 30m]`) | S | Done | SP-035 |

Waves: W0 SP-035 → W1 SP-036 (both edit `main.go`). After SP-036 landed, the operator set `systemone_timeout = "90s"` and `critic_enabled = true` in `.review/config.toml` (2026-10-11, with the SP-039 model switch).

### Phase 7 — Model benchmark (2026-10-10)

`qwen/qwen3-coder-next` (repo default in `.review/config.toml` until SP-039) does not fit in memory alongside OpenJev; `qwen/qwen3.8-27b` does. Measure what stet loses with the 27B model on seeded-bug fixtures and real stet commits, then recommend a model with a pre-registered decision rule.

| Task | Summary | Size | Status | Deps |
|------|---------|------|--------|------|
| SP-037 | Harness `scripts/model_eval.py` (`run` / `score` / `export-blind`, stdlib only) | S | Done | SP-040 |
| SP-038 | 14 seeded-bug and clean fixture cases under `scripts/model-eval-cases/` | S | Done | — |
| SP-039 | Run both models, blind-judge real diffs, write `docs/research/model-comparison-qwen3-coder-next-vs-qwen3.8-27b.md` | M | Done | SP-037, SP-038 |

Waves: **W0** SP-037 ∥ SP-038 (disjoint scopes; SP-037 is stet-gated `lenient`, SP-038 has no stet gate) → **W1** SP-039 alone.

**SP-039 operator checklist (before `spine batch start SP-039`):** stop OpenJev; no other batch running; identical LM Studio inference presets for both models (Repeat Penalty off, Top P 0.9, Top K 40, Min P off, Context Overflow "Stop at Limit", Limit Response Length off, Reasoning Parsing on, empty System Prompt, Structured Output off, default prompt template); LM Studio server up at `http://127.0.0.1:1234`. SP-039 swaps the loaded model with `lms` and takes ≈2–3 h; it reloads `qwen/qwen3-coder-next` at 65536 when done.

SP-038 landed in batch `20261010T182206-c796`. SP-037 was skipped there: its stet contract failed with `parse failed (after retry): invalid character 'L'` because the reviewer wrote prose before a fenced JSON block. SP-037 now depends on SP-040.

SP-037 landed in batch `20261010T215242-59b9`. SP-039 landed in batch `20261011T202931-a926` after an aborted first run (`20261011T174423-6acb`). That run led to stet fixes for issues #2 and #3, a 64k context / 4096 output cap harness pin, and 27b with thinking off (SP-039 PROMPT Amendments; pre-fix data in `docs/research/model-eval/baseline-pre-fix/`). **Outcome:** all four decision-rule conditions hold, so the doc recommends `qwen/qwen3.8-27b` as stet's default; the noise condition passed by 0.006 and both models scored 0 actionable on the real diffs. The operator then switched `.review/config.toml` to `qwen/qwen3.8-27b` with the OpenJev critic on (2026-10-11).

### Phase 8 — Stet reviewer robustness (2026-10-10)

| Task | Summary | Size | Status | Deps |
|------|---------|------|--------|------|
| SP-040 | Recover findings JSON from prose responses; JSON-only retry; suppression/shadow sections framed as dismissed false positives | S | Done | — |

Waves: **R0** SP-040 → reinstall `stet` from `main` (`cd cli && go build -o ~/.local/bin/stet ./cmd/stet`) → SP-037 → SP-039.

### Phase 9 — Release v1.1.0 (2026-10-14)

Manifest: `spine-tasks/_authoring/release-v1.1.0/manifest.md`.

| Task | Summary | Size | Status | Deps |
|------|---------|------|--------|------|
| SP-041 | Docs: current suppression header (#6) | S | Done | SP-045 |
| SP-042 | CI/release setup-go `check-latest: true` (Go 1.27.2 stdlib vulns) | S | Done | — |
| SP-043 | `stet status` `findings:` = active count (#7) | S | Done | SP-045 |
| SP-044 | `.gitignore` in in-repo worktree root (#9) | S | Done | SP-043 |
| SP-045 | Restore dismissals from history on `stet start` (#8) | M | Done | — |
| SP-046 | JS/TS hunk context includes file imports (#10, partial) | S | Done | — |
| SP-047 | Prompt: out-of-view imports/definitions are not evidence (#10, partial) | S | Done | — |

Edges are shared-file serialization (`docs/review-process-internals.md`, `docs/cli-extension-contract.md`), not data deps.

**Released:** v1.1.0 on 2026-10-17, tagged at `a03742d` (closes #6, #7, #8, #9, #10). Release: https://github.com/beettlle/stet/releases/tag/v1.1.0

### Phase 10 — Release v1.2.0 (2026-10-17)

Manifest: `spine-tasks/_authoring/release-v1.2.0/manifest.md`.

| Task | Summary | Size | Status | Deps |
|------|---------|------|--------|------|
| SP-048 | Docs: rule digest and `rules_max_tokens` (#11, partial) | S | Done | — |
| SP-049 | Contract script recovery `stet start` uses strictness (#22) | S | Done | — |
| SP-050 | JS/TS expand: arrow callback no longer truncates function scope (#24) | S | Done | — |
| SP-051 | One JS/TS extension list for expand and RAG; `.jsx` RAG (#16) | S | Done | SP-050 |
| SP-052 | OpenAI-compatible benchmark / run summary throughput (#4) | S | Done | — |
| SP-053 | Drop praise-only `info`/`nitpick` findings (#25, partial) | S | Done | — |
| SP-054 | Evidence filter: findings near added lines only (#21) | S | Done | SP-053 |
| SP-055 | JS RAG: function references, import-bound names, skip built-ins (#19, partial) | S | Done | SP-051 |
| SP-056 | Docs: `review-process-internals.md` for v1.2.0 behavior | S | Done | SP-051, SP-053, SP-054 |
| SP-057 | `stet run` warns when review flags have no effect (#23) | S | Done | SP-048, SP-052, SP-054 |

Edges are shared-file serialization (`expand_jsts.go`, `rag/js/resolver.go`, `run.go`, `main.go`, `docs/cli-extension-contract.md`) except SP-056, which documents its dependencies' behavior.

**Released:** v1.2.0 on 2026-10-18, tagged at `f93dce2` (closes #4, #16, #21, #22, #23, #24; partial #11, #19, #25). Also ships `1a1782e` (rule digests, `rules_max_tokens`) and `bfa3b86` (`.mjs`/`.cjs` expansion). Release: https://github.com/beettlle/stet/releases/tag/v1.2.0

Wave 0 (batch `20261017T222827-fb0f`) landed SP-048, SP-049, SP-050, SP-052, SP-053. Tech debt found:

- A lane reused for a second task has no `.review/spine-stet-baseline.ref`, so `scripts/spine-stet-contract-run.sh` exits 1 until the worker recreates it (SP-052 hit this after SP-049 in lane 2). `scripts/spine-worktree-setup.sh` should write the baseline per task, not only per lane.
- `cli/cmd/stet/main_test.go` benchmark throughput test never closes the `os.Pipe` read end (fd leak in tests only).

Wave 1 (batch `20261018T182409-7c0e`) landed SP-051, SP-054. Tech debt found:

- `jsts.Extensions` (`cli/internal/jsts/ext.go`) is an exported mutable slice; a caller could corrupt it. Unexport it behind an accessor that returns a copy.

Wave 2 (batch `20261018T211721-5f62`) landed SP-055, SP-056, SP-057. Tech debt found:

- `jsBuiltinNames` (`cli/internal/rag/js/resolver.go`) skips generic names (`get`, `set`, `add`, `log`, `find`, ...), so repo functions with those names never get RAG definitions. Consider skipping them only as member calls (`x.get(`), not bare calls.
- Workers committed `test_output.txt` twice this release (repo root in SP-052, `cli/` in wave 2). Add `test_output.txt` to `.gitignore` or have the contract reject it.

### Phase 11 — Release v1.3.0 (2026-10-22)

Manifest: `spine-tasks/_authoring/release-v1.3.0/manifest.md`. Scope approved 2026-10-22.

| Task | Summary | Size | Status | Deps |
|------|---------|------|--------|------|
| SP-058 | `hunkid`: `.jsx` comment stripping via `jsts.IsExt` (#26) | S | Ready | — |
| SP-059 | JS RAG: skip built-in method names only as member calls (#28) | S | Ready | — |
| SP-060 | Benchmark: end-to-end hunk estimate grows with prompt size (#31) | S | Ready | — |
| SP-061 | `stet benchmark` loads the repo `.review/config.toml` (#32) | S | Ready | — |
| SP-062 | `main_test.go` output-capture helper drains and closes pipes (#29) | M | Ready | SP-061 |
| SP-063 | Ignore `test_output.txt`; release check guard (#30) | S | Ready | — |
| SP-064 | `jsts.Extensions()` returns a copy (#27) | S | Ready | SP-059 |
| SP-065 | Docs: README `stet benchmark` config source and estimate | S | Ready | SP-060, SP-061 |
| SP-066 | Docs: `review-process-internals.md` for v1.3.0 | S | Ready | SP-058, SP-059, SP-064 |

Edges are shared-file serialization (`main_test.go`, `rag/js/resolver.go`) except SP-065 and SP-066, which document their dependencies' behavior.

### Completed (wave 0 + follow-on)

SP-001, SP-002, SP-003, SP-004, SP-005, SP-008 — `.DONE` on `main`.

---

## Execution policy

1. **Preflight** before every batch: `spine preflight`.
2. **One CLI core task per serial lane** — never chain SP-001-style `cli/**` tasks in one lane; use disjoint parallel batches. `spine plan pending` may still serialize SP-009 + SP-010 in lane-2 — **do not use** `pending --wave N` when that happens; start named task batches instead.
3. **Parallel lanes** only when `fileScopeMustChange` paths are disjoint (see waves below).
4. **Land loop:** `bash scripts/stet-wave-review.sh orch/spine-<batchId>` (advisory) → `spine gate approve` → `spine integrate` → `spine batch complete` (or `complete --detect-manual-merge` after manual lane merge).
5. **Never** ban `spine-tasks/**` in `fileScopeMustNotChange`.
6. **Pre-landed paths:** Narrow `fileScopeMustChange` to new deliverables when preflight warns.
7. **Before extension batches:** run `scripts/check-untracked-coverage.sh` once SP-013 lands.
8. **Prefer named batches** over `spine batch start pending --wave N` when tasks share `cli/**`, `extension/**`, or dependency chains that the auto-planner serializes unsafely.
9. **Batch env:** export `HEADROOM_URL=http://127.0.0.1:9` for `spine batch start` / `resume` until the pi-headroom `thought_signature` ticket is fixed (the proxy strips Gemini's `thought_signature`, so reviewer spawns fail through it). Also export `SPINE_STET_NO_AUTO_FINISH=1` (see the SP-035 hazard below) and `PYTHONDONTWRITEBYTECODE=1` (lanes created before `b66603b` still track `__pycache__`, and a dirty `.pyc` fails `fileScopeMustNotChange`).

---

## Stet in spine (dogfooding)

Spine lanes review their own diff with the locally built `stet` binary, using the repo `.review/config.toml` (LM Studio `qwen/qwen3.8-27b` with thinking off at `http://127.0.0.1:1234`; System One critic on via local OpenJev at `http://127.0.0.1:8080`, `systemone_timeout = "90s"`). OpenJev must be running for lane contracts and wave review; otherwise set `STET_CRITIC_ENABLED=false`. Secrets stay env-only (`STET_SYSTEMONE_API_KEY`).

- **Lane setup:** `.spine/spine-config.json` `worktreeSetupHook` runs `scripts/spine-worktree-setup.sh`, which opens a stet session at the pre-worker HEAD and writes `.review/spine-stet-baseline.ref`. It skips (exit 0) when `stet` is not on PATH.
- **Contract convention:** append the review to the task's tests: `<tests> && cd .. && bash scripts/spine-stet-contract-run.sh <level>`. The script restores a lost session from the baseline ref, runs `stet run --strictness <level>`, fails with triage instructions when `stet list` has active findings, and runs `stet finish` at zero findings.
- **Level by task:** S tasks with low risk → omit or `lenient`; M tasks → `default`; review / critic / rag / systemone / security / config / session code → `strict`.
- **Time budget:** the contract `testCommand` has a 10-minute wall clock (not configurable). A first review of a small Go diff took ~3m20s, re-runs after dismissals are incremental (<1s). Use `maxParallel` 2 for stet-gated waves so lanes do not starve LM Studio.
- **Triage:** fix real defects; dismiss others with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`); record stet reviewer bugs under the task's STATUS Discoveries.
- **Known hazard (SP-035):** `stet finish` clears the session, so re-running a contract after a passing run starts a fresh review that resurfaces dismissed findings and can invent new ones (the reviewer repeated its `RulesAnnotator` hallucination verbatim). Engine-side re-verification of a finished contract needs re-triage; persisting dismissals across sessions or `SPINE_STET_NO_AUTO_FINISH=1` for verification runs would fix this.
- **Wave review (operator):** before `spine gate approve`, run `bash scripts/stet-wave-review.sh orch/spine-<batchId>`. It reviews the orch branch against its merge-base with `main` in a throwaway worktree, writes JSON to `/tmp/stet-wave-<branch>.json`, and prints active findings. Advisory: exits 0 for any finding count, non-zero only when stet fails. The worktree reads the branch's own `.review/config.toml`; override with env (for example `STET_CRITIC_ENABLED=false`) if that config is stale.
- **Packet scope:** stet-gated packets list `.review/**` in `fileScopeMustNotChange`; `.review/*` except `config.toml` is gitignored.
- **Known hazard (SP-040):** the reviewer hallucinates Go syntax defects on gofmt-clean files when a diff adds a new file (31 "indentation is inconsistent" + 4 "malformed import" ERRORs on `extract_json.go`, all disproven by `gofmt -l`/build/vet), and re-reports findings from a stale diff hunk after a fix lands (e.g. "unused import" for an import the new hunk uses). Triage evidence: gofmt/build/vet/test output beats reviewer claims; dismiss with that reasoning. Tech debt: `docs/review-process-internals.md` and `docs/roadmap.md` still reference the old "Do not report issues similar to" suppression header (SP-040 renamed it; docs were outside that packet's file scope). Tracked in [#6](https://github.com/beettlle/stet/issues/6).

---

## Suggested batch waves (remaining work)

| Wave | Tasks | Lanes | Notes |
|------|-------|-------|-------|
| H0 | SP-011, SP-012, SP-013 | up to 3 | Hygiene; disjoint scopes; run before feature wave |
| 1 | SP-009 | 1 | Python expand; `cli/internal/expand/` only |
| 2a | SP-006 | 1 | Extension dismiss; `findingsPanel*` |
| 2b | SP-007 | 1 | After SP-006 — same `findingsPanel.ts` |
| H1 | SP-010 | 1 | Coverage gates (L); `cli/**/*_test.go` only |
| S1 | SP-014, SP-015, SP-017 | 3 | No deps; disjoint (`config/`, `systemone/`, `run/`) |
| S2 | SP-016, SP-021, SP-019 | 3 | Disjoint (`review/critic_systemone*`, `systemone/`, `criticeval/dataset*`); SP-019 has no deps and may also go earlier |
| S3 | SP-022, SP-024 | 2 | Disjoint (`run/`, `criticeval/sweep*`) |
| S4 | SP-023 | 1 | Flag, client wiring, docs; `main.go` |
| S5 | SP-018 | 1 | Doctor check; `main.go` |
| S6 | SP-025 | 1 | `stet critic-eval`; `main.go` |
| — | SP-020 | — | Deferred research; schedule manually |
| M0 | SP-037, SP-038 | 2 | Model benchmark harness ∥ fixtures; disjoint (`scripts/model_eval.py` vs `scripts/model-eval-cases/`) |
| M1 | SP-039 | 1 | Model benchmark run; exclusive LM Studio; operator checklist in Phase 7 |
| R0 | SP-040 | 1 | Stet parser fix; reinstall `stet` before SP-037 |

**Start commands:**

```bash
spine batch start SP-011 SP-012 SP-013
spine batch start SP-009
spine batch start SP-006
spine batch start SP-007
spine batch start SP-010
spine batch start SP-014 SP-015 SP-017
spine batch start SP-016 SP-021 SP-019
spine batch start SP-022 SP-024
spine batch start SP-023
spine batch start SP-018
spine batch start SP-025
spine batch start SP-037 SP-038
spine batch start SP-039
```

### Tech debt / hygiene backlog (found during execution)

The three rows below are scheduled as SP-031 (gofmt), SP-030 (flaky git tests), and SP-029 (`**` globs) in Phase 5.

| Found | Task area | Detail |
|-------|-----------|--------|
| 2026-10-08 (SP-018) | `gofmt` in `cli/cmd/stet/` | Pre-existing gofmt violations in `cli/cmd/stet/main.go` (~lines 1473–1618, SP-023 struct-literal alignment) and `main_test.go` (~lines 2237, 2347). Not fixed by SP-018 to keep its diff scoped; run `gofmt -w cli/cmd/stet/` in a hygiene task and consider adding a `gofmt -l` CI guard |
| 2026-10-08 (SP-018) | Flaky git-subprocess tests | Full `go test ./...` intermittently fails different git/TempDir-heavy tests (`internal/stats`, `internal/run`, `internal/git`) under parallel package load on macOS; pass in isolation and with `-p 4`. Candidate hygiene task: serialize git-heavy tests or harden TempDir cleanup |
| 2026-10-08 (SP-020) | `rules.FilterRules` `**` glob support | `filepath.Match` does not support `**`; frontmatter globs like `**/*.go` (most language rule files) never match multi-segment paths (nor bare filenames, since the pattern requires a `/`). Only `alwaysApply` rules are injected today — measured in `docs/research/systemone-rules-compliance.md`. Candidate SP-029 (S): support recursive `**` or normalize globs at load, with table tests |
| 2026-10-10 (SP-038) | Tracked `__pycache__` under `scripts/` | `scripts/tests/__pycache__/*.pyc` and `scripts/optimizer/__pycache__/*.pyc` are tracked in git (`.gitignore` only covers `scripts/__pycache__/`), so every pytest run dirties the worktree. **Fixed** in `b66603b` (untracked, `__pycache__/` ignored) |
| 2026-10-11 (SP-039) | `stet benchmark` tokens/s on OpenAI-compat | `stet benchmark` reports `Eval rate: 0.0 t/s` against LM Studio: it reads Ollama-style eval-duration fields the OpenAI-compatible client does not populate. Candidate S task: time the request client-side for `provider = "openai"`. Tracked in [#4](https://github.com/beettlle/stet/issues/4) |
| 2026-10-18 (SP-051) | Third JS/TS extension list in `hunkid` | `hunkid.langFromPath` (`cli/internal/hunkid/hunkid.go` ~line 84) still hardcodes `.js .mjs .cjs .ts .tsx` (no `.jsx`) for comment stripping, so `.jsx` hunks get no JS comment normalization in finding IDs. Candidate XS task: use `jsts.IsExt` there (outside SP-051 File Scope) |
| 2026-10-11 (SP-039) | Wave review shows findings outside the diff | `scripts/stet-wave-review.sh` on the SP-039 branch (empty `cli/**` diff) listed 21 `cli/` findings matching the SP-033/SP-034 real refs the harness had just reviewed. Likely stet session state shared across the harness clones and the review worktree. Investigate before trusting wave-review counts after a model-eval run. Tracked in [#5](https://github.com/beettlle/stet/issues/5) |

Phase 4 tasks all touch `cli/**`; run them as the separate named batches above, never as one serial lane. Slices sharing files (SP-015/SP-021 `systemone/client.go`; SP-017/SP-022 `run.go`; SP-019/SP-024 `criticeval/`; SP-023/SP-018/SP-025 `main.go`) are ordered by dependency edges and land in different batches.

**Avoid:** `spine batch start pending --wave 0` — auto-plan serializes SP-009 + SP-010 (both `cli/**`) in one lane, repeating wave-0 contract contamination ([#62](https://github.com/beettlle/pi-spine/issues/62)).

---

## Operator recovery

When implementation is **Done** in a lane worktree (`.DONE` present, tests pass) but the batch failed (contract `review_exhausted`, integrate error, or DirtyWorktree):

1. **Diagnose** — `spine batch status`; read lane worker output and `.reviews/` for the failing task.
2. **Retry** — If the failure is transient (stall, gate timeout): `spine batch retry <task-id>` or restart the single task batch.
3. **Skip** — If the task is obsolete or superseded: mark skipped in STATUS and remove from the active batch (operator decision only).
4. **Manual merge** — If code is correct and already merged to `main` outside spine integrate:
   - Confirm `main` contains the lane commits.
   - From the repo root: `spine batch complete --detect-manual-merge`
   - Spine matches lane SHAs to `main` and closes the batch without re-integrating ([#73](https://github.com/beettlle/pi-spine/issues/73)).
5. **Force-merge** — Last resort when SHAs diverged but operator accepts risk: land manually, then `complete --detect-manual-merge`; document in task STATUS Execution Log.

**DirtyWorktree** (untracked `extension/coverage/`): run `scripts/check-untracked-coverage.sh`; ensure coverage dirs are gitignored ([#74](https://github.com/beettlle/pi-spine/issues/74)). SP-013 adds the guard.

---
