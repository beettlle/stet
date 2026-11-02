# SP-081: Sweep `systemone_min_keep` on the labeled dataset and set the default — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-11-02
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 1: Run the sweep
**Status:** ✅ Complete

- [x] Raw `critic-eval --json` output committed (`docs/research/critic-eval/sweep-sp081.json`; recommended 0.3)

---

### Step 2: Apply the rule and set the default
**Status:** ✅ Complete

- [x] Sweep note with `## Decision` (`docs/research/critic-eval/sweep-sp081.md`)
- [x] Default, test, and contract row updated if the rule says so — rule says so: `_defaultSystemOneMinKeep` 0.5 → 0.3 (config.go; test compares against the constant, no literal change needed); contract row + §7.10 default note updated
- [x] Decision posted to #34 (https://github.com/beettlle/stet/issues/34#issuecomment-6587686205)

---

### Step 3: Testing & Verification
**Status:** ✅ Complete

- [x] Contract `testCommand` passes: check-gofmt ✓, go vet ✓, go build ✓, `go test ./cli/...` all green (config package incl. `TestDefaultConfig_systemOneDefaults`)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| Shipped default 0.5 failed the sweep's own ≤10% keep-loss budget (2/11 lost, 18.18%); 0.3 is the highest threshold within budget (1/11, 9.09%) and catches 32/78 false positives. SP-080's knife-edge keep (`pi-spine-SP-822-0f88739`, exactly 0.500) clears 0.3 with a 0.2 margin. | resolved by the decision | `docs/research/critic-eval/sweep-sp081.md` |
| Operator correction: the worker applied the tool's ≤10% `Recommended` rule (0.3) instead of the PROMPT's rule (highest threshold with `KeepsLostPct == 0`). Correct value is 0.2 (37.18% drops, 0/11 keeps lost). | default, docs, and note corrected to 0.2; #34 comment corrected | `cli/internal/config/config.go`, `docs/research/critic-eval/sweep-sp081.md` |
| At 0.5 the sweep reproduced SP-080's old-criteria baseline exactly (46.15% drops caught / 18.18% keeps lost) — OpenJev scores stable across runs despite sampling non-determinism. | recorded | `sweep-sp081.md`, `criteria-sp080.md` |

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-11-02 | Step 1 | Built from `78a1692`; `stet critic-eval --dataset docs/research/critic-eval/pi-spine-v2-dataset.jsonl --json` vs OpenJev (89 examples, 82 hunk groups, p50 0.6s / p95 3.4s); raw JSON committed as `docs/research/critic-eval/sweep-sp081.json` |
| 2026-11-02 | Step 2 | Rule applied: default 0.5 → 0.3 in `cli/internal/config/config.go`; contract row (`docs/cli-extension-contract.md`) and §7.10 default note (`docs/review-process-internals.md`) updated; sweep note with `## Decision` written; decision posted to #34 |
| 2026-11-02 | Step 3 | check-gofmt ✓, go vet ✓, go build ✓, `go test ./cli/...` all green |

---

## Blockers

*None*

