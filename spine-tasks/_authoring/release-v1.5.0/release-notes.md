## stet v1.5.0

### Fixes
- `stet list`, `stet start`, and `stet status` show only findings inside the session's reviewed diff; findings on lines outside it no longer appear ([#5](https://github.com/beettlle/stet/issues/5))

### Changes to defaults
- `systemone_min_keep` default 0.5 → 0.2. On a labeled dataset harvested from pi-spine release reviews (78 drop, 11 keep), 0.2 is the highest threshold that keeps every real finding; 0.5 dropped 18% of them ([#34](https://github.com/beettlle/stet/issues/34))
- The rules budget in review prompts is now capped at 1000 tokens (was 8192). In a 63-run A/B, 1000 tokens caught as many seeded rule violations as 2000 and more than 8192 (3.33 vs 3.00 of 4 per run), with fewer findings on clean code. `rules_max_tokens` still overrides ([#11](https://github.com/beettlle/stet/issues/11))

### Measurements
- Critic context (v1.4.0 change): on the SP-831 comparison the critic now drops on average 3.0 false findings per run with no real finding lost ([#18](https://github.com/beettlle/stet/issues/18); `docs/research/critic-context-sp831.md`)
- System One criteria wording: three rewordings were tested against the labeled dataset; none passed the pre-registered rule, so the shipped criteria are unchanged ([#20](https://github.com/beettlle/stet/issues/20) stays open; `docs/research/critic-eval/criteria-sp080.md`)

### Tooling
- `scripts/model_eval.py run` accepts `--rules-dir` and `--rules-max-tokens`; new Go and Python seeded-rule eval cases ([#11](https://github.com/beettlle/stet/issues/11))
- Labeled critic-eval dataset `docs/research/critic-eval/pi-spine-v2-dataset.jsonl` with provenance ([#34](https://github.com/beettlle/stet/issues/34))

### Upgrade note
- With `critic_backend = "systemone"`, more findings are kept by default. Set `systemone_min_keep = 0.5` to restore the old behavior.
- Repositories with large rule sets get less rule text per prompt. Set `rules_max_tokens` to raise it.

### Documentation
- `docs/cli-extension-contract.md` and `docs/review-process-internals.md` updated for the new `systemone_min_keep` default and rules budget clamp
- Research write-ups: `docs/research/critic-context-sp831.md`, `docs/research/critic-eval/sweep-sp081.md`, `docs/research/critic-eval/criteria-sp080.md`, `docs/research/rules-budget-ab.md`

### Install
```bash
curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh | STET_RELEASE_TAG=v1.5.0 bash
```
