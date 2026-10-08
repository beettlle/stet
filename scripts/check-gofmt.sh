#!/usr/bin/env bash
# Fail if any Go file under cli/ is not gofmt-formatted.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
UNFORMATTED="$(cd "$ROOT/cli" && gofmt -l .)"
if [ -n "$UNFORMATTED" ]; then
  echo "gofmt: unformatted Go files:"
  echo "$UNFORMATTED"
  exit 1
fi
