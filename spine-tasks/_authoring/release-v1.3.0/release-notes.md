## stet v1.3.0

### Fixes
- `stet benchmark` reads the repository's `.review/config.toml`, so it benchmarks the configured provider and model ([#32](https://github.com/beettlle/stet/issues/32))
- `stet benchmark`: the typical-hunk estimate for OpenAI-compatible providers accounts for prompt size ([#31](https://github.com/beettlle/stet/issues/31))
- Comment-only edits in `.jsx` files no longer trigger a re-review ([#26](https://github.com/beettlle/stet/issues/26))
- JS/TS symbol lookup finds repo functions named like built-in methods (`get`, `set`, `log`, `find`, ...) when they are called directly or imported; only member calls such as `x.get(` are skipped ([#28](https://github.com/beettlle/stet/issues/28))
- CLI tests drain captured output while commands run and close their pipes, so large output can no longer deadlock a test ([#29](https://github.com/beettlle/stet/issues/29))

### Maintenance
- `test_output.txt` is ignored, and the release check fails if one is tracked ([#30](https://github.com/beettlle/stet/issues/30))
- The JS/TS extension list is read-only for callers: `jsts.Extensions()` returns a copy ([#27](https://github.com/beettlle/stet/issues/27))

### Upgrade note
- Semantic hunk IDs for `.jsx` files change once. Existing sessions re-review `.jsx` hunks one time after upgrading.

### Documentation
- README: `stet benchmark` configuration source and end-to-end estimate
- `docs/review-process-internals.md` updated for v1.3.0 (`.jsx` hunk IDs, JS RAG built-in method handling, `jsts.Extensions()`)

### Install
```bash
curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh | STET_RELEASE_TAG=v1.3.0 bash
```
