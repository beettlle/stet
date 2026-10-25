# SP-074: Docs: dismissals follow line shifts; keep history across `.review` cleanup — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-10-25
**Review Level:** 0
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Dismissal and state-storage text
**Status:** ✅ Complete

- [x] `stet dismiss` entry describes re-mapping and the run-time skip
- [x] Cleanup note with `state_dir` and a verified gitignore exception

---

### Step 2: Testing & Verification
**Status:** ✅ Complete

- [x] Contract `rg` checks pass
- [x] Doc test references noted

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| No test or script references `docs/cli-extension-contract.md` (`rg -l 'cli-extension-contract.md' cli/ scripts/` returns nothing), so no code updates are coupled to this doc change. | Noted | Step 2 |
| Gitignore exceptions verified in a scratch repo: `!.review/history.jsonl` + `!.review/history.jsonl.*.gz` make `git check-ignore -v` report the `!` rules as the deciding match (not ignored) and `git clean -nfdX -- .review` remove only `session.json`/`lock`. | Verified before writing | `docs/cli-extension-contract.md` "Keeping dismissals across cleanups" |
| `git clean -e` semantics: a positive `-e '.review/history.jsonl*'` adds an ignore rule and removes MORE; the negative form `-e '!.review/history.jsonl*'` is what protects history for one-off cleans (verified with `-n`). | Documented | same section |
| `stet rerun` sets `ForceFullReview: true` (`cli/cmd/stet/main.go`), so the dismissed-hunk skip applies to `stet run` only. | Reflected in doc | `stet dismiss` entry |
| Dismissal restoration reads rotated archives too (`history.ReadRecords` reads `history.jsonl.N.gz` + active file), so the exception must cover `.gz`. | Reflected in doc | same section |
