## stet v1.4.0

### Fixes
- JS/TS reviews include the file's imports for top-level and test-callback hunks, so imported names are no longer reported as missing ([#36](https://github.com/beettlle/stet/issues/36))
- JS/TS symbol lookup resolves imported names in the module they are imported from ([#19](https://github.com/beettlle/stet/issues/19))
- The critic (both backends) now judges findings with the same expanded context, imports, and symbol definitions the reviewer saw, up to 16384 characters ([#18](https://github.com/beettlle/stet/issues/18))
- Dismissed findings stay dismissed when edits above them shift their lines, in both `stet start` (history restore) and `stet run` (dismissed-hunk skip); a dismissal is dropped only when its own lines change ([#38](https://github.com/beettlle/stet/issues/38))

### Features
- Findings that claim a symbol is unused, undefined, or not imported are dropped before the critic when the file shows otherwise ([#33](https://github.com/beettlle/stet/issues/33))
- `stet list --min-severity <error|warning|info|nitpick>` and `--min-confidence <0..1>` hide findings below a threshold; hidden findings stay active ([#37](https://github.com/beettlle/stet/issues/37))
- The spine contract script fails only on findings at `warning` or above with confidence ≥ 0.8 (override with `SPINE_STET_MIN_SEVERITY` / `SPINE_STET_MIN_CONFIDENCE`); with an older `stet` it falls back to gating on every active finding ([#37](https://github.com/beettlle/stet/issues/37), [#25](https://github.com/beettlle/stet/issues/25))

### Upgrade note
- With the critic enabled, each critic request carries more context, so reviews with the critic on take longer.

### Documentation
- `docs/cli-extension-contract.md`: dismissal re-mapping and how to keep review history across `.review` cleanups ([#38](https://github.com/beettlle/stet/issues/38))
- README: `stet list` thresholds and contract gating
- `docs/review-process-internals.md` updated for v1.4.0 (JS/TS imports, existence-claim filter, critic context, dismissal re-mapping)

### Install
```bash
curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh | STET_RELEASE_TAG=v1.4.0 bash
```
