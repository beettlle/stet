# SP-090: protocol-a-selector — Status

**Current Step:** Complete
**Status:** Complete (operator land — spine workers SIGTERM under host load)
**Last Updated:** 2026-11-21
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

### Step 0: Protocol A candidate building + SelectSections
**Status:** Complete

- [x] digest_fair candidates + SelectSections + unit tests (from protocol-a-wip stash)

### Step 1: Config + pipeline wiring
**Status:** Complete

- [x] Config/env, run/review/main wiring, section log; default candidates digest_fair

### Step 2: Testing & Verification
**Status:** Complete

- [x] go test rules/config/review/run -count=1; gofmt; go vet ./cli/...

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

- Default RulesSelectorCandidates changed from stash "full" to digest_fair per release plan A/B.
- Operator land after repeated mid-prompt worker deaths; GitNexus impact on SelectSections was LOW earlier in session.
