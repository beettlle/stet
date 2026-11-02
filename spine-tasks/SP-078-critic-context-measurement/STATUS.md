# SP-078: Measure the critic with the reviewer's expanded context (SP-831 off/on re-run) — Status

**Current Step:** Complete
**Status:** ✅ Complete
**Last Updated:** 2026-11-02
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** S

---

### Step 0: Preflight
**Status:** ✅ Complete

- [x] Versions, model, and ref recorded in `preflight.txt`
- [x] LM Studio smoke request OK (HTTP 200, reply `OK`)
- [x] OpenJev reachable (`/health` → 200 `{"status":"ok"}`)

---

### Step 1: Run critic off and critic on
**Status:** ✅ Complete

- [x] 3 critic-off runs (12/13/14 findings; 392.9s/151.8s/163.8s — all `ok` first attempt)
- [x] 3 critic-on runs (13/11/13 findings; 266.5s/229.7s/247.1s — all `ok` first attempt)
- [x] Retries: `--retry-failed` not needed (0 non-`ok` runs); results committed

---

### Step 2: Label, decide, and write the doc
**Status:** ✅ Complete

- [x] `labels.tsv` (76 findings, all labeled) committed before the rule was applied
- [x] Doc written with the decision (rule holds → **#18 resolved**); table posted to #18: https://github.com/beettlle/stet/issues/18#issuecomment-6583249208

---

### Step 3: Testing & Verification
**Status:** ✅ Complete

- [x] Contract `testCommand` passes (exit 0: labels.tsv non-empty, `## Decision` and `## Out-of-view claims still kept` present)
- [x] Nothing outside File Scope changed (`git status` clean after commits; only File Scope files + task STATUS.md touched)

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| stderr logs contain no critic kept/dropped counts (harness does not enable tracing) | Used the pre-registered off-vs-on fallback for the drops condition | doc §Results, §Threats 1 |
| Critic-on reports introduce no new false-finding families (all 37 match off families) | Noted | doc §Results |
| All 6 runs missed a real (minor) defect: stale JSDoc "sorted lexicographically" on `filterPendingTaskIds` after this diff made it numeric-aware | Logged as SP-080 recall-baseline input; no scope expansion | doc §Threats 6 |
| Off run1 wall 392.9s is a cold-start outlier (first run post model load) | Latency reported via mean and median | doc §Results |
| `gh issue view 18 --json body` returns only the run table (issue body is machine-generated) | Used `--comments` for original evidence | execution log |

---

## Execution Log

| Timestamp | Action | Outcome |
|-----------|--------|---------|
| 2026-11-02T16:28Z | Preflight: stet v1.4.0, HEAD f2c09f9, pi-spine ref 37f22c1 present | OK |
| 2026-11-02T16:29Z | `lms load qwen/qwen3.8-27b -c 65536 --parallel 1 --no-speculative-draft-mtp -y` | OK (14.98 GiB, 46.8s) |
| 2026-11-02T16:30Z | LM Studio smoke POST /v1/chat/completions | HTTP 200, content `OK` |
| 2026-11-02T16:30Z | OpenJev GET /health | HTTP 200 `{"status":"ok"}` |
| 2026-11-02T16:31Z | Critic-off: 3 runs `ok` (12/13/14 findings; 392.9/151.8/163.8s), exit 0 first attempt | OK |
| 2026-11-02T16:39Z | Critic-on: 3 runs `ok` (13/11/13 findings; 266.5/229.7/247.1s), exit 0 first attempt | OK |
| 2026-11-02T16:47Z | labels.tsv (76 rows, all false; 44 out-of-view) committed before applying the rule | OK (332f83a) |
| 2026-11-02T16:52Z | Decision rule applied: condition 1 holds (3.0 absent false findings/run avg ≥ 1), condition 2 holds vacuously (0 real) → #18 resolved | OK |
| 2026-11-02T16:53Z | Result table posted to #18 | https://github.com/beettlle/stet/issues/18#issuecomment-6583249208 |
| 2026-11-02T16:55Z | Contract testCommand exit 0; git status clean; doc committed (6806bf3) | OK |

---

## Blockers

*None*