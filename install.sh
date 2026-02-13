#!/bin/bash
#
# Install stet: builds from source and installs to ~/.local/bin
# Run from the stet repo root, or set STET_REPO_URL to clone first.
#
# Usage:
#   From repo:  ./install.sh
#   Via clone:  STET_REPO_URL=https://github.com/org/stet.git bash -c "$(curl -sSL https://raw.githubusercontent.com/org/stet/main/install.sh)"
#

set -euo pipefail

INSTALL_DIR="${STET_INSTALL_DIR:-$HOME/.local/bin}"
REPO_URL="${STET_REPO_URL:-}"

mkdir -p "$INSTALL_DIR"

if [ -f "go.mod" ] && [ -d "cli" ] && grep -q 'module stet' go.mod 2>/dev/null; then
	# We're in the stet repo, build from here
	make build
	cp -f bin/stet "$INSTALL_DIR/stet"
elif [ -n "$REPO_URL" ]; then
	# Clone and build
	TMP_DIR=$(mktemp -d)
	trap 'rm -rf "$TMP_DIR"' EXIT
	git clone --depth 1 "$REPO_URL" "$TMP_DIR"
	cd "$TMP_DIR"
	make build
	cp -f bin/stet "$INSTALL_DIR/stet"
else
	echo "Error: Not in a stet repo and STET_REPO_URL is not set." >&2
	echo "  Run this script from the stet repo root, or set STET_REPO_URL to the git clone URL." >&2
	exit 1
fi

chmod +x "$INSTALL_DIR/stet"
echo "Installed stet to $INSTALL_DIR"
echo "Ensure $INSTALL_DIR is in your PATH."
