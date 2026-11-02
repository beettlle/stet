# SP-081: `systemone_min_keep` threshold sweep — decision record

**Task:** SP-081 (#34) · **Date:** 2026-11-02 · **Status:** Complete — default lowered 0.5 → 0.2 per the pre-registered rule in the SP-081 PROMPT (#34: lose no true positive)
**Question:** Which `systemone_min_keep` default catches the most false positives while losing **no** real (`keep`) finding on the labeled pi-spine v2 dataset?

## Run

- Binary: `go build -o $TMPDIR/stet-sp081 ./cli/cmd/stet` from this repo at `78a1692` (criteria unchanged since SP-080; `main` @ `e5f2e8e`).
- Command: `stet critic-eval --dataset docs/research/critic-eval/pi-spine-v2-dataset.jsonl --json` from the repo root.
- Environment: local OpenJev (`/health` → 200) at `systemone_base_url=http://127.0.0.1:8080`, `systemone_model=openjev-latest`, `systemone_timeout=90s`, `systemone_samples` default 1, loaded from `.review/config.toml` (unchanged). Raw output: [sweep-sp081.json](sweep-sp081.json).
- Dataset: 89 examples (78 `drop`, 11 `keep`) in 82 hunk groups; both labels clear `minReliableExamples` (10), so no reliability warning. Score sends `min_keep=0` per group call, so every P(yes) is visible to the sweep.

## Sweep table (from sweep-sp081.json)

| threshold | false positives dropped | real findings lost | kept |
|-----------|------------------------|--------------------|------|
| 0.1 | 25.64% (20/78) | 0% (0/11) | 69 |
| **0.2** | **37.18% (29/78)** | **0% (0/11)** | **60** |
| 0.3 (tool `Recommended`) | 41.03% (32/78) | 9.09% (1/11) | 56 |
| 0.4 | 44.87% (35/78) | 18.18% (2/11) | 52 |
| 0.5 (shipped default) | 46.15% (36/78) | 18.18% (2/11) | 51 |
| 0.6 | 48.72% (38/78) | 27.27% (3/11) | 48 |
| 0.7 | 51.28% (40/78) | 45.45% (5/11) | 44 |
| 0.8 | 56.41% (44/78) | 45.45% (5/11) | 40 |
| 0.9 | 62.82% (49/78) | 54.55% (6/11) | 34 |

Latency: p50 0.6s, p95 3.4s per hunk-group request.

## Reading

- The shipped default 0.5 loses 2/11 real findings: `pi-spine-SP-820-3295372` (P≈0.227) and `pi-spine-SP-838-8653ab9` (P≈0.385) — the same two keeps SP-080's baseline lost, so scores are stable across runs (0.5 row matches SP-080's old-criteria baseline exactly: 46.15% drops caught, 18.18% keeps lost).
- 0.2 is the highest threshold that loses no real finding; it catches 29/78 false positives.
- 0.3 is the tool's built-in `Recommended` value (it tolerates up to 10% keeps lost) and catches 3 more false positives, but loses the P≈0.227 keep. #34's rule is stricter and is the one applied.
- SP-080's knife-edge hand-off is resolved: keep `pi-spine-SP-822-0f88739` scored exactly 0.500 under the old default (kept only by `>=`); at 0.2 it clears the threshold with a 0.3 margin.

## Decision

Applied mechanically from the PROMPT's pre-registered rule:

1. **Reliability:** 78 `drop` and 11 `keep` examples, no `recommendation unreliable` warning → proceed.
2. **Choose** `t*` = highest threshold with `KeepsLostPct == 0` → **0.2** (0.3 loses 9.09%).
3. **Change the default:** `t* != 0.5`, and `t* < 0.5` because 0.5 loses `keep` examples (18.18%) → change.

**`_defaultSystemOneMinKeep` 0.5 → 0.2** (`cli/internal/config/config.go`). The sweep tool's recommendation algorithm is unchanged.

Updated with the default:

- `cli/internal/config/config.go` — `_defaultSystemOneMinKeep = 0.2`; `TestDefaultConfig_systemOneDefaults` compares against the constant, so the test follows without a literal change.
- `docs/cli-extension-contract.md` §Configuration — `systemone_min_keep` default row `0.5` → `0.2`.
- `docs/review-process-internals.md` §7.10 systemone bullet — "(default 0.5)" → "(default 0.2)".

Amendment (release operator, 2026-11-02): the worker first adopted the tool's `Recommended` value 0.3 (≤10% keeps lost), which is not the PROMPT's rule; the operator corrected the default to 0.2 per steps 1–3 above. The correction is posted to #34.
