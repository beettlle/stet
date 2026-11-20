## stet v1.6.0

### Fixes
- Drop unused-import existence claims when the symbol is used elsewhere in the same file, even across hunks (#39)
- Avoid ERROR severity for imported-callee return claims without callee-body evidence (#39)

### Enhancements
- Pass task intent into the reviewer and System One critic via `stet run --intent-file` (#35)
- Reject out-of-view "missing / undefined" existence claims in the System One critic (#20)
- Batch RAG symbol `git grep` lookups into fewer processes per hunk (#15)

### Install
curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh | STET_RELEASE_TAG=v1.6.0 bash
