#!/usr/bin/env bash
# Read-only release intake snapshot for stet: version baseline, GitHub issues, spine backlog,
# issue-to-task mapping, and dependency drift. Optional tools that are missing or fail are
# reported inline and never abort the snapshot.
#
# Usage (from anywhere in the repo):
#   .agents/skills/stet-release-operator/scripts/collect-release-intake.sh [TARGET_VERSION]
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
cd "$REPO_ROOT"

GH_REPO="beettlle/stet"
TASKS_ROOT="spine-tasks"
MAX_COMMITS_LISTED=50
TARGET="${1:-unknown}"
TARGET="${TARGET#v}"

has() { command -v "$1" >/dev/null 2>&1; }

LAST_TAG="$(git describe --tags --abbrev=0 --match 'v*' 2>/dev/null || echo "")"

echo "=== stet release intake snapshot ==="
echo "Target: v${TARGET}"
echo "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Repo: $REPO_ROOT"
echo "Branch: $(git branch --show-current 2>/dev/null || echo unknown)"
echo "HEAD: $(git rev-parse --short HEAD)"
echo "Last tag: ${LAST_TAG:-(none)}"
echo "Working tree: $(if [ -z "$(git status --porcelain)" ]; then echo clean; else echo dirty; fi)"
echo ""

echo "=== Commits since ${LAST_TAG:-start} (newest first, max $MAX_COMMITS_LISTED) ==="
if [ -n "$LAST_TAG" ]; then
	echo "Count: $(git rev-list --count "${LAST_TAG}..HEAD")"
	git log --oneline --max-count="$MAX_COMMITS_LISTED" "${LAST_TAG}..HEAD"
else
	git log --oneline --max-count="$MAX_COMMITS_LISTED"
fi
echo ""

echo "=== GitNexus status ==="
if has gitnexus; then
	gitnexus status 2>&1 || echo "(gitnexus status failed)"
else
	echo "(gitnexus not installed)"
fi
echo ""

if has gh; then
	for label in "" documentation bug enhancement; do
		if [ -z "$label" ]; then
			echo "=== GitHub open issues ($GH_REPO) ==="
			gh issue list --repo "$GH_REPO" --state open --limit 100 \
				--json number,title,labels \
				--jq '.[] | "#\(.number) [\(.labels | map(.name) | join(","))] \(.title)"' \
				2>&1 || echo "(gh failed — check auth)"
		else
			echo "=== Open issues: $label ==="
			gh issue list --repo "$GH_REPO" --state open --label "$label" --limit 100 \
				--json number,title --jq '.[] | "#\(.number) \(.title)"' 2>&1 || true
		fi
		echo ""
	done
else
	echo "=== GitHub issues ==="
	echo "(gh not installed)"
	echo ""
fi

echo "=== Spine plan pending ==="
if has spine; then
	spine plan pending 2>&1 || true
else
	echo "(spine not installed)"
fi
echo ""

echo "=== Issue -> task mapping (Closes/Partial in PROMPT.md) ==="
if has rg; then
	rg -n 'Closes:? #[0-9]+|Partial:? #[0-9]+' "$TASKS_ROOT"/*/PROMPT.md 2>/dev/null || echo "(no mappings found)"
else
	grep -nE 'Closes:? #[0-9]+|Partial:? #[0-9]+' "$TASKS_ROOT"/*/PROMPT.md 2>/dev/null || echo "(no mappings found)"
fi
echo ""

echo "=== Pending spine tasks (no .DONE) ==="
pending=0
for dir in "$TASKS_ROOT"/SP-*/; do
	[ -d "$dir" ] || continue
	if [ ! -f "$dir/.DONE" ]; then
		echo "$(basename "$dir")"
		pending=$((pending + 1))
	fi
done
echo "Pending count: $pending"
echo ""

echo "=== Go module updates (go list -u -m) ==="
if has go; then
	go list -u -m -f '{{if .Update}}{{.Path}} {{.Version}} -> {{.Update.Version}}{{if .Indirect}} (indirect){{end}}{{end}}' all 2>&1 |
		grep -v '^$' || echo "(all modules current)"
else
	echo "(go not installed)"
fi
echo ""

echo "=== Go vulnerabilities (govulncheck) ==="
if has govulncheck; then
	govulncheck ./cli/... 2>&1 || echo "(govulncheck reported findings or failed — see above)"
else
	echo "(govulncheck not installed; run: go run golang.org/x/vuln/cmd/govulncheck@latest ./cli/...)"
fi
echo ""

echo "=== Toolchain pins ==="
echo "Local: $(go version 2>/dev/null || echo 'go not installed')"
rg -n --no-heading 'go-version:|node-version:|uses: ' .github/workflows/ 2>/dev/null || echo "(no workflows)"
echo ""

echo "=== Extension npm drift (not shipped; record and defer by default) ==="
if has npm && [ -f extension/package.json ]; then
	(cd extension && npm outdated 2>&1) || true
	(cd extension && npm audit --omit=dev 2>&1 | tail -n 3) || true
else
	echo "(npm or extension/package.json missing)"
fi
echo ""

echo "=== Next Task ID ($TASKS_ROOT/CONTEXT.md) ==="
grep -E '^\*\*Next Task ID:\*\*' "$TASKS_ROOT/CONTEXT.md" 2>/dev/null || echo "(CONTEXT.md not found)"
