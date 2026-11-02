# SP-080: System One critic rejects out-of-view "missing / undefined" claims — Status

**Current Step:** Step 3
**Status:** 🟨 In Progress
**Last Updated:** 2026-11-02
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

### Step 1: Baseline with the current criteria
**Status:** ✅ Complete

- [x] `stet-old` critic-eval results recorded; out-of-view rows identified
  (`docs/research/critic-eval/criteria-sp080.md`: old = 36/78 drops, 2/11 keeps lost,
  7/19 out-of-view rows dropped; 19 OOV ids fixed by the pre-registered classifier)

---

### Step 2: New criteria wording
**Status:** ✅ Complete

- [x] New true/false criteria (wording v1 in `critic_systemone.go`; absence claims
  about code the shown context does not include count as unsupported)
- [x] Unit test (criteria pinned on every chunked request question; short + hunk-free
  guard; rules-annotation request unchanged)

---

### Step 3: Validate and document
**Status:** ⬜ Not Started

- [ ] `stet-new` critic-eval; validation rule applied
- [ ] Contract and internals docs updated

---

### Step 4: Testing & Verification
**Status:** ⬜ Not Started

- [ ] Completion-criteria tests
- [ ] Contract `testCommand` passes

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-11-02 | Step 1 | Built `stet-old` from unchanged main (`a90293d`); `critic-eval --json` + per-example harness run vs OpenJev (82 groups, 89 examples); baseline recorded in `criteria-sp080.md` with pre-registered OOV classifier |
| 2026-11-02 | Step 2 | Reworded critic criteria (wording v1); added criteria-pinning, hunk-free guard, and annotation-unchanged tests; `go test ./cli/...` green |

---

## Blockers

*None*
