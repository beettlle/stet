## stet v1.1.0

### Fixes
- `stet status` now reports active findings only, matching the docs ([#7](https://github.com/beettlle/stet/issues/7))
- Dismissals survive `stet finish`; a new `stet start` re-applies them to matching findings in unchanged files ([#8](https://github.com/beettlle/stet/issues/8))
- stet writes a `.gitignore` into `.review/worktrees/` so the review worktree is never committed as a gitlink ([#9](https://github.com/beettlle/stet/issues/9))
- JS/TS hunks include the file's imports, and the prompt tells the reviewer not to report missing imports or `await` it cannot see ([#10](https://github.com/beettlle/stet/issues/10))
- Release binaries are built with the latest Go 1.27 patch (1.27.2+), which fixes 13 standard library vulnerabilities

### Documentation
- Docs describe the current "Previously dismissed false positives" prompt section ([#6](https://github.com/beettlle/stet/issues/6))

### Install
```bash
curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh | STET_RELEASE_TAG=v1.1.0 bash
```
