# SP-069: Critic sees the reviewer's expanded context and RAG definitions — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-24
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 1 (REVISE feedback addressed — proposed approach documented below)
**Size:** M

---

## Proposed Approach (Plan)

**Goal:** give both critic backends the same code context the reviewer saw (expanded hunk + RAG definitions) instead of the raw hunk, with a larger context cap; annotation behavior untouched.

### Step 1 approach — critic context cap in the review package

- Add a named constant `defaultCriticContextCap = 16384` in `cli/internal/review/critic.go`, next to the existing `defaultCriticMaxHunkLen = 4096` (which stays, now documented as the rules-annotation cap).
- `BuildCriticPrompt` and `VerifyFinding` default to `defaultCriticContextCap` when `CriticOptions.MaxContextLen <= 0`; keep the `\n[truncated]` marker on overflow.
- For System One, `buildSystemOneState` (used by `rules_annotation.go`) keeps the 4096 cap unchanged; extract the truncation into `buildSystemOneStateCapped(content, maxLen)` and have the critic state call it with `defaultCriticContextCap`, so the annotation path is behaviorally identical (shared code, separate cap parameter — no change to `rules_annotation.go` itself).
- Rename `CriticOptions.MaxHunkContentLen` → `MaxContextLen` since the cap now applies to the prepared prompt, not just hunks.

### Step 2 approach — pass the prepared user prompt to both critics

- Add helper `criticInputFor(p *preparedPrompt) string` in `run.go`: returns `p.User` when non-empty, else falls back to `p.Hunk.RawContent` (defensive).
- `applyCritic` (LLM) passes `criticInputFor(p)` to `review.VerifyFinding`; `applyCriticSystemOne` passes it to `review.VerifyFindingsSystemOne`.
- When tracing is enabled, print `Critic input: N chars` once per hunk next to the existing `Critic:` line in both backends.

### Step 3 approach — testing & verification

- Run-package tests (fakes for LLM critic + System One decider) assert each critic input/state contains the expanded-context section and a RAG definition from the prepared user prompt.
- Truncation tests: over-16384-char critic input truncated with `[truncated]` (LLM prompt and System One state); annotation 4096 truncation test passes unchanged.
- Verify with the Contract `testCommand` (gofmt, vet, tests, stet contract review).

---

### Step 1: Critic context cap in the review package
**Status:** ✅ Complete (approach above)

- [x] Named 16384-character cap for both critics; annotation cap unchanged

---

### Step 2: Pass the prepared user prompt to both critics
**Status:** ✅ Complete

- [x] `applyCritic` / `applyCriticSystemOne` use `p.User`; trace prints critic input length

---

### Step 3: Testing & Verification
**Status:** ✅ Complete

- [x] Tests for every Contract `completionCriteria` case
- [x] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|
| 1 | Plan | — | REVISE | `.reviews/1-20261024T223204.md` — feedback: STATUS.md lacked a documented proposed approach for Step 1 and claimed completion before Plan Review. **Disposition:** addressed — Proposed Approach section added above for all three steps; implementation re-verified against it (all files match the documented plan); Contract `testCommand` re-run and passing (stet review exit 0). |

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| stet contract review (default) reported 11 findings; triaged: 1 fixed, 10 dismissed. Three ERROR findings claimed missing symbols / compile errors (`criticInput`, `fakeCriticLLM.Generate`, unused `preparedUserPromptFor`) that exist in the full files — the reviewer only sees per-hunk diff context, so definitions split from uses read as missing. | stet reviewer limitation noted; findings dismissed as false_positive after `go build`/`go vet`/full suite verified | stet session findings 38a914b, db96354, 15d50d5 |
| One ERROR misread a negative assertion: `strings.Contains` returning false means the guard branch is *not* taken (test passes), and the check is meaningful — it fails exactly when truncation breaks. | stet reviewer bug (inverted logic); dismissed false_positive | stet session finding 82eeea0 |
| `CriticOptions.MaxHunkContentLen` renamed to `MaxContextLen`: the cap now applies to the reviewer's prepared user prompt, not just hunk content; no callers outside critic.go used the field. | fixed (stet finding 6de5302) | cli/internal/review/critic.go |
| `docs/cli-extension-contract.md` checked per PROMPT: it documents critic flags/config (`--verify`, `critic_backend`, `systemone_*`) but does not state what content the critic receives — not affected, no edit. | checked, not affected | docs/cli-extension-contract.md |
| System One client imposes no request-state size limit (only a `_maxResponseBytes` response cap), so the 16384-char critic state is safe. | verified during triage of stet finding a48d832 | cli/internal/systemone/client.go |
