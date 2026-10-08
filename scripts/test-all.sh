#!/usr/bin/env bash
# Run Go CLI and extension test suites (spine testing.test).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
(cd "$ROOT/cli" && go test ./... -count=1)
(cd "$ROOT/extension" && npm test)
