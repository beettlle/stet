# SP-043: `stet status` reports active findings — Status

**Current Step:** Step 2 (Testing & Verification) — contract triage in progress
**Status:** 🟡 Tests passing; contract re-run pending after commit
**Last Updated:** 2026-10-15
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Active count in status
**Status:** ✅ Implemented (build verified; plan review engine-owned)

- [x] `findings:` uses the active count from the loaded session
- [x] `docs/cli-extension-contract.md` `stet status` bullet updated

---

### Step 2: Testing & Verification
**Status:** 🟡 In Progress

- [x] Status-after-dismiss tests (all dismissed; partial)
- [x] Other `findings: ` assertions checked (only line 1209 — no-dismissal case, still valid)
- [ ] Contract `testCommand` passing (stet review exits 0)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| Contract WARNING f191f5f: duplicated active-filter logic between status count and `--ids` list could drift | **Fixed, not dismissed**: extracted `activeFindingsInSession` in main.go; both `runStatus` count and `activeFindings` (used by `--ids`/`list`) share it | `cli/cmd/stet/main.go:123` |
| Contract re-run re-reports a finding verbatim after the fix lands because the reviewed diff is `baseline..HEAD` (committed only) — uncommitted fixes are invisible to `stet run` | Workflow gotcha, not a code defect: commit before re-running the contract; consistent with SP-040 stale-hunk hazard in CONTEXT.md | `scripts/spine-stet-contract-run.sh` |

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-10-14 | Task staged | PROMPT.md and STATUS.md created (release v1.1.0) |
| 2026-10-15 | Step 1 plan review requested | Skipped in-worker (SP-195); engine runs after worker success |
| 2026-10-15 | Step 1 implemented | `runStatus` computes active count inline from loaded session; contract doc bullet updated; `go build ./cli/...` OK |
| 2026-10-15 | Step 2 tests written | Extended `TestRunCLI_dismissPersistence` (findings: 0 + dismissed: 1); added `TestRunCLI_statusActiveCountAfterPartialDismiss` (2 findings, dismiss 1 → findings: 1 + dismissed: 1); `go test ./cli/cmd/stet/... -count=1` ok (64s) |
| 2026-10-15 | Contract run 1 (lenient) | 1 WARNING (duplicated filter logic). Triage: fixed via shared `activeFindingsInSession` helper; re-test ok |
| 2026-10-15 | Contract run 2 (lenient) | Same finding re-reported verbatim — reviewed diff is committed baseline..HEAD; helper fix was uncommitted. Commit Step 2, then re-run |

---

## Blockers

*None*
