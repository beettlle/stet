# SP-089: protocol-a-docs — Status

**Current Step:** Complete
**Status:** Complete (operator land — spine workers SIGTERM under host load)
**Last Updated:** 2026-11-21
**Review Level:** 0
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 0: Restore and finalize research note
**Status:** Complete

- [x] Research note/summaries already on main (ab3d6d2); results trees intentionally not restored

### Step 1: Contract + internals docs
**Status:** Complete

- [x] Document rules_selector_candidates (default digest_fair) and STET_RULES_SECTION_LOG
- [x] review-process-internals §7.3 selector paragraph

### Step 2: Testing & Verification
**Status:** Complete

- [x] rg hits for rules_selector_candidates|digest_fair|STET_RULES_SECTION_LOG in contract + internals

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

- Spine batches ae11/12c9/3f40: workers died mid-prompt (empty dirtyPaths); landed docs on main as release operator.
