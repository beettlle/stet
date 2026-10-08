#!/usr/bin/env bash
# Build the Go CLI and the TypeScript extension (spine testing.build).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
"$ROOT/scripts/check-gofmt.sh"
(cd "$ROOT/cli" && go build ./...)
(cd "$ROOT/extension" && npm run compile)
