#!/usr/bin/env bash
# Release gate: the single check run locally, by .github/workflows/ci.yml, and by
# .github/workflows/release.yml before publishing. Exits non-zero on the first failing step.
#
# Usage: scripts/release-check.sh
# Steps: gofmt, go vet, Go tests with coverage thresholds, extension tests, coverage-artifact
# guard, and a cross-compile of every release target with a --version smoke test.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# Cross-compile check only; real releases pass the tag as VERSION.
CHECK_VERSION="v0.0.0"

current_step=""
on_exit() {
	local code=$?
	if [ "$code" -ne 0 ] && [ -n "$current_step" ]; then
		echo "FAIL: $current_step (exit $code)" >&2
	fi
}
trap on_exit EXIT

step() {
	current_step="$1"
	echo "==> $current_step"
}

pass() {
	echo "PASS: $current_step"
	current_step=""
}

step "gofmt"
bash scripts/check-gofmt.sh
pass

step "go vet"
go vet ./cli/...
pass

step "go test with coverage"
go test ./cli/... -coverprofile=coverage.out -count=1
bash scripts/check-coverage.sh coverage.out
pass

step "extension tests"
(cd extension && npm ci --no-audit --no-fund && npm test)
pass

step "coverage artifact guard"
bash scripts/check-untracked-coverage.sh
pass

step "cross-compile release targets"
make release VERSION="$CHECK_VERSION"
case "$(uname -s)-$(uname -m)" in
	Darwin-arm64) host_bin="dist/stet-darwin-arm64" ;;
	Darwin-x86_64) host_bin="dist/stet-darwin-amd64" ;;
	Linux-x86_64) host_bin="dist/stet-linux-amd64" ;;
	Linux-aarch64 | Linux-arm64) host_bin="dist/stet-linux-arm64" ;;
	*) host_bin="" ;;
esac
if [ -n "$host_bin" ]; then
	version_out="$("$host_bin" --version)"
	case "$version_out" in
		*"$CHECK_VERSION"*) echo "$version_out" ;;
		*)
			echo "unexpected --version output: $version_out" >&2
			exit 1
			;;
	esac
else
	echo "no host binary for $(uname -s)-$(uname -m); skipped --version smoke"
fi
pass

echo "release-check: all steps passed"
