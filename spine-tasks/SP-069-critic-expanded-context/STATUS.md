# SP-069: Critic sees the reviewer's expanded context and RAG definitions — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-24
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

### Step 1: Critic context cap in the review package
**Status:** ✅ Complete

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

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| stet contract review (default) reported 11 findings; triaged: 1 fixed, 10 dismissed. Three ERROR findings claimed missing symbols / compile errors (`criticInput`, `fakeCriticLLM.Generate`, unused `preparedUserPromptFor`) that exist in the full files — the reviewer only sees per-hunk diff context, so definitions split from uses read as missing. | stet reviewer limitation noted; findings dismissed as false_positive after `go build`/`go vet`/full suite verified | stet session findings 38a914b, db96354, 15d50d5 |
| One ERROR misread a negative assertion: `strings.Contains` returning false means the guard branch is *not* taken (test passes), and the check is meaningful — it fails exactly when truncation breaks. | stet reviewer bug (inverted logic); dismissed false_positive | stet session finding 82eeea0 |
| `CriticOptions.MaxHunkContentLen` renamed to `MaxContextLen`: the cap now applies to the reviewer's prepared user prompt, not just hunk content; no callers outside critic.go used the field. | fixed (stet finding 6de5302) | cli/internal/review/critic.go |
| `docs/cli-extension-contract.md` checked per PROMPT: it documents critic flags/config (`--verify`, `critic_backend`, `systemone_*`) but does not state what content the critic receives — not affected, no edit. | checked, not affected | docs/cli-extension-contract.md |
| System One client imposes no request-state size limit (only a `_maxResponseBytes` response cap), so the 16384-char critic state is safe. | verified during triage of stet finding a48d832 | cli/internal/systemone/client.go |
