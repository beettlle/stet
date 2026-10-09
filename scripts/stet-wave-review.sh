#!/bin/bash
# Operator-side advisory stet review of a spine wave before gate approve.
# Usage: scripts/stet-wave-review.sh <orch-branch> [out.json]
# Reviews <orch-branch> against its merge-base with main in a throwaway worktree, writes the
# findings JSON to out.json (default /tmp/stet-wave-<branch>.json), and prints active findings.
# Advisory: exits 0 whatever the finding count; non-zero only when stet itself fails.
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
	echo "usage: $0 <orch-branch> [out.json]" >&2
	exit 2
fi
branch="$1"
out="${2:-/tmp/stet-wave-${branch//\//-}.json}"

if ! command -v stet >/dev/null 2>&1; then
	echo '{"ok":true,"skipped":"stet not on PATH"}'
	exit 0
fi

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"
if ! git rev-parse --verify --quiet "${branch}^{commit}" >/dev/null; then
	echo "stet-wave-review: branch '$branch' not found" >&2
	exit 2
fi
base="$(git merge-base main "$branch")"

review_dir="$(mktemp -d "${TMPDIR:-/tmp}/stet-wave-review.XXXXXX")"
cleanup() {
	(cd "$review_dir" && stet finish >/dev/null 2>&1) || true
	git -C "$ROOT" worktree remove --force "$review_dir" >/dev/null 2>&1 || rm -rf "$review_dir"
	git -C "$ROOT" worktree prune
}
trap cleanup EXIT

git worktree add --detach --quiet "$review_dir" "$branch"
cd "$review_dir"

echo "stet-wave-review: reviewing $branch against merge-base ${base:0:12}"
stet start "$base" --allow-dirty --quiet --json --strictness default > "$out"

active_list="$(stet list)"
active_count="$(printf '%s' "$active_list" | grep -c . || true)"
echo "stet-wave-review: ${active_count} active finding(s); JSON at $out"
if [[ "$active_count" -gt 0 ]]; then
	printf '%s\n' "$active_list"
fi
