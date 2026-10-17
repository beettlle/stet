# SP-048: Document the rule digest and `rules_max_tokens` — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-17
**Review Level:** 0
**Review Counter:** 0
**Iteration:** 1
**Size:** S

---

### Step 1: Contract doc
**Status:** ✅ Complete

- [x] Configuration table row (`rules_max_tokens` / `STET_RULES_MAX_TOKENS`)
- [x] "Cursor rules in the review prompt" section: digest, cache, ordering, budget, dropped-section warning

---

### Step 2: README
**Status:** ✅ Complete

- [x] `--rules-max-tokens` / `rules_max_tokens` bullet under "Useful flags", linking to the contract section

---

### Step 3: Testing & Verification
**Status:** ✅ Complete

- [x] `rg -n 'rules_max_tokens|STET_RULES_MAX_TOKENS|rules-digest-cache.json' docs/cli-extension-contract.md README.md` matches both files
- [x] `rg -l 'cli-extension-contract.md' cli/ scripts/` returned no matches (exit 1), so no tests assert on these docs
- [x] Contract testCommand `true` exits 0

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| `--rules-max-tokens` is registered in `addRunLikeFlags`, so it is also on `stet rerun` (PROMPT listed only start/run) | Documented start, run, and rerun (code wins) | `cli/cmd/stet/main.go:516` |
| Negative `rules_max_tokens` in config/env is ignored; a negative flag value is rejected | Documented | `config.go:630,1007`; `main.go:599` |
| `rules_max_tokens` is not persisted in the session | Documented | `cli/internal/session` (no field) |
| Sections are taken round-robin across rules; a single section larger than the whole budget is cut down to its rule's share and marked `[truncated]` rather than dropped | Documented | `cli/internal/rules/criteria.go` |
| Dropped-section warning is suppressed when `rules_selector` is on; on selector fallback the budget still applies | Documented | `cli/internal/run/rules_load.go`, `run.go:1422` |
| `rules_selector` row described the old "tail-truncated concatenation" | Reworded to reference budgeted digests | `docs/cli-extension-contract.md` |
