#!/usr/bin/env bash
# Run Go CLI tests with coverage and the extension suite (spine testing.testWithCoverage).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
(cd "$ROOT/cli" && go test ./... -cover)
(cd "$ROOT/extension" && npm test)
