# SP-082: Rule-targeted model-eval cases and harness rule options — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-11-02
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

### Step 1: Harness flags
**Status:** ✅ Complete

- [x] `--rules-dir` / `--rules-max-tokens`, defaults unchanged
- [x] Unit tests

---

### Step 2: Seeded rule cases
**Status:** ✅ Complete

- [x] Two Go and two Python rule cases

---

### Step 3: Clean controls and case tests
**Status:** ✅ Complete

- [x] Clean controls; case tests extended and passing

---

### Step 4: Testing & Verification
**Status:** ✅ Complete

- [x] Completion-criteria tests
- [x] Contract `testCommand` passes

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|
| 1 | plan | 1 | skipped (engine-owned; SP-195 blocks nested reviewer spawn) | .reviews/1-20261102T073057.md |

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| Stet reviewer claims `args.rules_dir` is a str (6a7af57) though `build_parser` declares `type=Path`; hallucinated runtime failure | Dismissed false_positive (triage verified) | scripts/model_eval.py:651 |
| Stet reviewer claims `pinned_env` raises TypeError for int `rules_max_tokens` (033f307); implementation converts with `str()` and a test asserts the string value | Dismissed false_positive (triage verified) | scripts/model_eval.py:534 |
| Stet reviewer claims `RULE_ID_PATTERN.fullmatch` can KeyError (43bbf21); `bug.get("rule")` short-circuits before `bug["rule"]` is evaluated | Dismissed false_positive (triage verified) | scripts/tests/test_model_eval_cases.py:141 |
| Stet reviewer miscounts fixture totals (3969206 wants 8 clean/16 buggy); 12+4 buggy and 2+2 clean = 20 cases is correct | Dismissed false_positive (triage verified) | scripts/tests/test_model_eval_cases.py:118 |
| Stet reviewer flags claims invisible in the reviewed hunk (f6acd28 "definition not visible" for a variable defined ~15 lines earlier in the same function; e9322c7 "unused import re" used by RULE_ID_PATTERN) | Dismissed false_positive (triage verified) | scripts/model_eval.py:676 |
| Contract review re-reviews prior lane work: the lane session baseline (3d10bd4) predates SP-077's commits on this branch, so 16 findings targeted `cli/cmd/stet/main*.go` changes SP-077 already triaged (see its contract round, 91ffe1b/ac614d7). Dismissed out_of_scope; lane operators may want the worktree hook to set the baseline at the lane branch point | Log; no action in SP-082 (`.spine/` out of scope) | .review/spine-stet-baseline.ref |
| `ee4a719` claimed UnboundLocalError + bad `stet finish` in `perform_review`'s finally; verified impossible: if the try raises, the return is never reached, and `finish_code != 0` is already handled (pre-existing, documented SP-037 behavior) | Dismissed false_positive (triage verified) | scripts/model_eval.py:524 |
| `python3` in this environment lacked pytest; installed pytest 9.2.0 into the user site so the contract's literal `python3 -m pytest` works (environment setup, no repo change) | Log | shell env |
| Fresh stet session (prior `stet finish` reset dismissals) regenerated 36 findings at head 1b49b3b; all re-triaged this session: 17 on `scripts/**` dismissed false_positive after code verification (str() conversion at :311, `>= 1` validation at :664 + test :756, `type=Path` parser, try/finally propagation at :524-561, short-circuit `and` at :141, 16/4 counts, conditional rule-key validation at :140, positive-value tests at :273/:533), 19 on SP-077's `cli/**` dismissed out_of_scope (fileScopeMustNotChange; `go build ./...` OK disproves the bytes-import claim) | Dismissed (gate 0 active) | `stet list` history |
| Contract re-run: `stet run` saw no new hunks (review state covers diff), gating list 0, `stet finish` succeeded, session closed, script exit 0 | Resolved | .review/history.jsonl |

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-11-02 | Step 1 | `feat(SP-082)` fc0ccce: `--rules-dir` copies `DIR/*.mdc` into the before commit only; `--rules-max-tokens` pins `STET_RULES_MAX_TOKENS`; both recorded in meta; defaults unchanged; 89 unit tests green |
| 2026-11-02 | Step 2 | `test(SP-082)` 8d164b8 (part): go-rule-context (§1.3), go-rule-global-prefix (§2.1), py-rule-membership (§2.3), py-rule-naming (§3.4) with `category: "rule"` + `rule` keys |
| 2026-11-02 | Step 3 | `test(SP-082)` 8d164b8 (part): go-rule-clean / py-rule-clean controls; `EXPECTED_IDS` extended, `rule` category + rule-key validation added; 211 tests green |
| 2026-11-02 | Step 4 | Contract testCommand: pytest 211 passed; dry-run pass over 20 cases with `--rules-dir .cursor/rules --rules-max-tokens 2000` exit 0; stet contract exit 0 after fixing 0 and dismissing 31 (16 out_of_scope on SP-077 `cli/**` re-review, 15 false_positive with triage notes) |
| 2026-11-02 | Re-verification (resume) | Re-ran full contract testCommand after session reset: pytest 211 passed / 20 skipped; dry-run exit 0; stet review 36 findings triaged and dismissed, contract re-run gates 0 and exits 0 |

---

## Blockers

*None*
