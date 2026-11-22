## stet v1.7.0

### Fixes
- Pin CI/release Go toolchain to **1.27.4** (GO-2026-6859 / `net/http` MaxBytesReader header race)

### Enhancements
- Protocol A rule-section selection when `rules_selector` is on: `rules_selector_candidates` / `STET_RULES_SELECTOR_CANDIDATES` default **`digest_fair`** (digest sections, file-specific-first, round-robin into ≤64 options, Detect option text); selector stays **off** by default (#40)
- `STET_RULES_SECTION_LOG=1` stderr markers for included/selected rule sections; model_eval `--rules-selector` / `--rules-selector-candidates` and `section_hit` scoring for research

### Docs
- Contract and review-process docs for Protocol A; research note `docs/research/rules-selector-protocol-a.md` (critic-off H(v)=0.75 for Protocol A vs 0.25 digest round-robin / 0.00 bare `full`)

### Install
curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh | STET_RELEASE_TAG=v1.7.0 bash
