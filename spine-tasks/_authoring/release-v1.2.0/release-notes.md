## stet v1.2.0

### Features
- Cursor rules reach the review prompt as compact review digests (heading, `❌ Bad:` and `**Detect:**` lines per section), cached in `<state_dir>/rules-digest-cache.json`. Rules matching the file come before `alwaysApply` rules, and the budget scales with the context limit (15%, clamped to 1000–8192 tokens) or is set with `rules_max_tokens` / `STET_RULES_MAX_TOKENS` / `--rules-max-tokens`. Dropped sections produce one warning per run ([#11](https://github.com/beettlle/stet/issues/11), partial)
- Findings must sit on or within 3 lines of a line the diff added; issues in unchanged context lines are no longer reported ([#21](https://github.com/beettlle/stet/issues/21))
- `info`/`nitpick` findings that only praise the change, with no suggestion, are dropped, and the prompt tells the reviewer not to emit them ([#25](https://github.com/beettlle/stet/issues/25), partial)
- `stet run` prints a warning to stderr (even with `--quiet`) when review flags such as `--strictness` or `--verify` had no effect because there were no new hunks ([#23](https://github.com/beettlle/stet/issues/23))

### Fixes
- `stet benchmark` and the run summary show throughput for OpenAI-compatible providers such as LM Studio, labelled end-to-end ([#4](https://github.com/beettlle/stet/issues/4))
- One JS/TS extension list (`.js .jsx .mjs .cjs .ts .tsx`) drives both function-context expansion and symbol lookup: `.jsx` files now get symbol definitions, and `.mjs`/`.cjs` files get enclosing-function context ([#16](https://github.com/beettlle/stet/issues/16))
- A JS/TS function whose body contains an arrow callback keeps its full enclosing scope ([#24](https://github.com/beettlle/stet/issues/24))
- JS symbol lookup picks up functions passed as arguments and names bound by `import`/`require`, and skips built-in method names ([#19](https://github.com/beettlle/stet/issues/19), partial)
- The spine contract script's recovery `stet start` uses the requested strictness ([#22](https://github.com/beettlle/stet/issues/22))

### Documentation
- The configuration contract and README document `rules_max_tokens` and rule digests
- `docs/review-process-internals.md` describes the current rule ordering, JS/TS extension list, praise and added-line filters, and truncated-response handling

### Install
```bash
curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh | STET_RELEASE_TAG=v1.2.0 bash
```
