# SP-070: Drop "unused / undefined / not imported" findings the file disproves — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-25
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

## Implementation Plan

### Step 1 approach — `FilterExistenceClaims` library (`cli/internal/findings/existence.go`)

- **Shape:** follow the `FilterFPKillList` / `FilterPraiseOnly` pattern — pure function `FilterExistenceClaims(batch []Finding, fileContent string) []Finding`, returns a new slice, preserves order, never mutates input findings; empty batch → nil, empty/whitespace-only `fileContent` → return batch unchanged (conservative keep).
- **Claim detection** (case-insensitive, whole-word regexes): (a) *missing* claims — `undefined`, `not/never defined|imported|declared`, `missing import(s)`, `without importing`, `ReferenceError`; (b) *unused* claims — `unused`, `not used`, `never used`; (c) *not-exported* claims — `not exported`. A value-use stripper removes spans like `returns undefined` / `stays undefined` first, so value semantics are not mistaken for existence claims.
- **Identifier extraction:** backticked spans from the message; a span counts as an identifier only if it matches `^[A-Za-z_][A-Za-z0-9_]*$`. A non-identifier span that looks like a module ref (contains `/`, `@`, or `.` — e.g. `` `./utils` ``) forces a keep: the filter never reads other modules.
- **Per-claim proof rules** (against `fileContent` only): *missing* → identifier is imported (JS/TS `import`/named-import/`require` binding, Go quoted import specifier, Python `import`/`from … import`), declared (JS/TS `function/const/let/var/class/interface/enum/type`, Go `func`/`var`/`const`/`type` incl. methods, Python `def`/`class`), or a parameter/receiver in a single-line declaration signature; *unused* → ≥2 whole-word occurrences on lines that are not declaration/import lines; *not-exported* → JS/TS export statement forms, or (Go) declared with an uppercase initial. Multi-line parameter lists keep the finding.
- **Conservative drop rule:** a finding is dropped only when **every** claim type present is disproven for **every** extracted identifier; anything inconclusive (no claim, no identifiers, unproven identifier, mixed claims with one unproven leg) is kept.
- **Tests:** table-driven in `existence_test.go` — per claim type a dropped case (symbol provably present) and a kept case (symbol genuinely absent), plus the conservative-keep edge cases.

### Step 2 approach — wiring (`cli/internal/run/run.go`, both post-filter blocks)

- In `runReviewPipeline`, insert the filter in **both** post-filter blocks (pipelined branch and drain branch) immediately after `FilterByAddedLines` and before `applyCritic`, so the critic never spends a request on a disproven claim.
- Read the post-change file once per hunk via `os.ReadFile(filepath.Join(opts.RepoRoot, p.Hunk.FilePath))`; on read error leave the content empty (`""`), which makes the filter keep everything.
- Add the trace line `Existence claims: N -> M` next to the neighboring filter traces (gated on `TraceOut.Enabled()`). The call sits outside the strictness conditionals, so it runs in every strictness mode.
- Blast radius checked (Level 2): touches the hot shared module but adds a leaf call between existing filters; no signature or behavior change for existing filters.

### Step 3 approach — fixtures & verification

- Table cases reproducing the #33 examples: SP-829 (`fs` used while `import fs from "node:fs"` present) dropped; SP-831 (`compareTaskIds` claimed missing while declared) dropped; used import flagged unused dropped; declared parameter flagged undefined dropped.
- Kept cases: no backticked identifiers, cross-module `not exported`, empty/whitespace content, genuinely absent symbols.
- Run-package ordering test `TestRunReviewPipeline_existenceFilterBeforeCritic`: fake backend counts requests by `model == "critic-model"`; asserts the dropped finding never reaches the critic (0 critic requests), the trace shows `Existence claims: 1 -> 0`, and the finding is absent from collected output — in both FP-on and FP-off strictness modes.
- Verification: full Contract `testCommand` (gofmt, vet, package tests, stet contract review).

---

### Step 1: `FilterExistenceClaims` library and table tests
**Status:** ✅ Complete

- [x] Claim detection, identifier extraction, proof rules, conservative keeps

---

### Step 2: Wire into both pipeline branches
**Status:** ✅ Complete

- [x] Runs after the evidence filter and before the critic in both branches; trace line

---

### Step 3: Testing & Verification
**Status:** ✅ Complete

- [x] #33 fixture tests and run-package ordering test
- [x] Contract `testCommand` passing (stet review exits 0)

---

## Completion Criteria

- [x] All steps complete
- [x] All tests passing — `check-gofmt` ✓, `go vet ./cli/...` ✓, `go test ./cli/internal/findings/... ./cli/internal/run/... -count=1` ✓, stet contract review exits 0 (0 active findings)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|
| 1 | Engine review (plan/code, post-.DONE) | all | REVISE → addressed 2026-10-25 | `.reviews/1-20261025T001544.md` |

**Revise feedback addressed:** (1) implementation plan added above (per-step approach sections); (2) `.DONE` had been created before review gates completed — all gates now satisfied: implementation committed at step boundaries, Contract `testCommand` re-run green on 2026-10-25 (gofmt ✓, vet ✓, `go test ./cli/internal/findings/... ./cli/internal/run/... -count=1` ✓, stet contract review exit 0 with 0 active findings).

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| Contract finding 8596293: "Unused import 'path/filepath' will cause a compilation error" — demonstrably false: `filepath.Join` is called in both pipeline branches and the build/vet pass (unused imports are compile errors in Go). Stet reviewer bug. | Dismissed `false_positive` | `cli/internal/run/run.go:19` |
| Contract findings beb1e0d + 9584059: "path traversal" via `filepath.Join(opts.RepoRoot, p.Hunk.FilePath)` — false positive in this codebase's trust model: `Hunk.FilePath` originates from the repo's own `git diff` (git cannot track `..`/absolute paths), and the identical join already exists at `expand.go:134` plus every RAG resolver — the same file content already reaches the prompt context, so the new read adds no exposure. | Dismissed `false_positive` | `cli/internal/run/run.go:513`, `cli/internal/run.go:611` |
| Contract finding 7264a37: claims the immutability test's `reflect.DeepEqual` is "brittle" over the `Range *LineRange` pointer — false positive: fixtures leave `Range` nil, and `FilterExistenceClaims` never dereferences or writes any finding field (it appends value copies to a new slice), so no in-place mutation path exists. | Dismissed `false_positive` | `cli/internal/findings/existence_test.go:393` |
| Contract finding b5c7846 (O(N·M) regex scans in `identUsedTwiceOutsideDecl`) was a legitimate performance warning — fixed by `combinedPattern`, which folds the per-pattern scans into one alternation regex (flag scopes preserved via passive groups) for all existence proofs. | Fixed | `cli/internal/findings/existence.go` |
| Doc check per PROMPT: `docs/cli-extension-contract.md` does not list individual trace post-filter lines (only high-level strictness/FP behavior), so no doc edit required for the new `Existence claims: N -> M` trace. SP-076 owns doc updates. | No action | `docs/cli-extension-contract.md` |
| Mixed-claim messages (e.g. "`fs` is unused and `x` is not defined") require every extracted identifier to satisfy every present claim's proof before dropping — the strict reading of "every identifier is proven", biased to conservative keeps. | By design | `cli/internal/findings/existence.go` (`existenceClaimDisproven`) |
| Contract finding 5d805ab: "Unused import 'sync/atomic' will cause a compilation error" — demonstrably false: `atomic.Int32` is used in `TestRunReviewPipeline_existenceFilterBeforeCritic` (`Add`/`Load`), and `go vet` + the package tests pass (an unused import is a compile error in Go, so the package could not build). Same existence-claim FP class this task filters. | Dismissed `false_positive` | `cli/internal/run/run_test.go:17` |
| Contract finding 3a3de32: claims the ordering test would fail on the `len(collected)` assertion before `criticRequests`, making it "harder to debug" — speculative style critique: both assertions use `t.Errorf` (non-fatal) so both always run with distinct debuggable messages, and `criticRequests` is counted independently via `body.Model == "critic-model"`. Test passes and matches the completion criteria. | Dismissed `false_positive` | `cli/internal/run/run_test.go:2366` |
