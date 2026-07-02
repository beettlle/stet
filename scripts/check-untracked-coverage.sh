#!/usr/bin/env bash
# check-untracked-coverage.sh: fail if extension/coverage/ paths are tracked in git.
# Vitest writes coverage here during npm test; those files must stay untracked
# (see .gitignore and commit 175c1a4).
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || {
  echo "FAIL: not inside a git repository"
  exit 1
}
cd "$REPO_ROOT"

COVERAGE_PATH="extension/coverage"

tracked="$(git ls-files "$COVERAGE_PATH" 2>/dev/null || true)"
if [ -n "$tracked" ]; then
  echo "FAIL: extension/coverage/ must not be tracked in git."
  echo "Tracked paths:"
  echo "$tracked"
  echo "Fix: git rm -r --cached extension/coverage/"
  exit 1
fi

untracked="$(git ls-files --others --exclude-standard "$COVERAGE_PATH" 2>/dev/null || true)"
if [ -n "$untracked" ]; then
  echo "WARN: untracked files under ${COVERAGE_PATH}/ (expected after extension npm test; do not git add):"
  echo "$untracked"
elif [ -d "$COVERAGE_PATH" ] && [ -n "$(find "$COVERAGE_PATH" -mindepth 1 -print -quit 2>/dev/null)" ]; then
  echo "WARN: coverage artifacts present under ${COVERAGE_PATH}/ (gitignored; expected after extension npm test; do not git add)"
fi

echo "PASS: no tracked paths under ${COVERAGE_PATH}/"
