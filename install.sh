#!/usr/bin/env bash
#
# Install stet: prefers download from GitHub Releases (by OS/arch), falls back to build from source.
#
# Usage:
#   One-line (Mac/Linux): curl -sSL https://raw.githubusercontent.com/OWNER/REPO/main/install.sh | bash
#   From repo:           ./install.sh
#   Via clone:           STET_REPO_URL=https://github.com/OWNER/REPO.git bash -c "$(curl -sSL .../install.sh)"
#
# Env: STET_REPO (e.g. owner/repo), STET_RELEASE_TAG (e.g. v1.0.0 or "latest"), STET_INSTALL_DIR, STET_REPO_URL (for source fallback).
#
set -euo pipefail

# GitHub repo (owner/repo). Set STET_REPO to override.
STET_REPO="${STET_REPO:-}"
if [ -z "$STET_REPO" ]; then
	# Default when not set: use placeholder; replace with real org/repo when publishing.
	STET_REPO="stet/stet"
fi

INSTALL_DIR="${STET_INSTALL_DIR:-$HOME/.local/bin}"
REPO_URL="${STET_REPO_URL:-}"

# Resolve release tag: STET_RELEASE_TAG or "latest".
RELEASE_TAG="${STET_RELEASE_TAG:-latest}"
if [ "$RELEASE_TAG" = "latest" ]; then
	# Resolve latest tag from GitHub API (no auth required for public repos).
	RELEASE_TAG=$(curl -sSf "https://api.github.com/repos/${STET_REPO}/releases/latest" | grep -o '"tag_name": *"[^"]*"' | head -1 | sed 's/"tag_name": *"\(.*\)"/\1/') || true
	if [ -z "$RELEASE_TAG" ]; then
		RELEASE_TAG=""
	fi
fi

# Detect OS and arch (GOOS/GOARCH style for binary name).
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
	x86_64) ARCH="amd64" ;;
	aarch64|arm64) ARCH="arm64" ;;
	*)
		echo "Error: Unsupported architecture: $(uname -m)" >&2
		exit 1
		;;
esac
case "$OS" in
	linux) ;;
	darwin) ;;
	*)
		echo "Error: Unsupported operating system: $OS" >&2
		exit 1
		;;
esac

BINARY_NAME="stet-${OS}-${ARCH}"
TMP_FILE=""
cleanup() {
	[ -n "$TMP_FILE" ] && [ -f "$TMP_FILE" ] && rm -f "$TMP_FILE"
}
trap cleanup EXIT

# Try download from GitHub Releases.
install_via_download() {
	[ -n "$RELEASE_TAG" ] || return 1
	DOWNLOAD_URL="https://github.com/${STET_REPO}/releases/download/${RELEASE_TAG}/${BINARY_NAME}"
	TMP_FILE=$(mktemp)
	if ! curl -fSL -o "$TMP_FILE" "$DOWNLOAD_URL" 2>/dev/null; then
		rm -f "$TMP_FILE"
		TMP_FILE=""
		return 1
	fi
	if [ ! -s "$TMP_FILE" ]; then
		rm -f "$TMP_FILE"
		TMP_FILE=""
		return 1
	fi
	# Optional: verify checksum if checksums.txt exists for this release.
	CHECKSUM_URL="https://github.com/${STET_REPO}/releases/download/${RELEASE_TAG}/checksums.txt"
	if curl -sSfL -o /dev/null "$CHECKSUM_URL" 2>/dev/null; then
		EXPECTED=$(curl -sSfL "$CHECKSUM_URL" | awk -v f="$BINARY_NAME" '$2 == f { print $1 }')
		if [ -n "$EXPECTED" ]; then
			if command -v sha256sum >/dev/null 2>&1; then
				ACTUAL=$(sha256sum "$TMP_FILE" | awk '{ print $1 }')
			elif command -v shasum >/dev/null 2>&1; then
				ACTUAL=$(shasum -a 256 "$TMP_FILE" | awk '{ print $1 }')
			else
				ACTUAL=""
			fi
			if [ -n "$ACTUAL" ] && [ "$EXPECTED" != "$ACTUAL" ]; then
				echo "Warning: Checksum verification failed for $BINARY_NAME; continuing anyway." >&2
			fi
		fi
	fi
	mkdir -p "$INSTALL_DIR"
	cp -f "$TMP_FILE" "$INSTALL_DIR/stet"
	chmod +x "$INSTALL_DIR/stet"
	return 0
}

# Fallback: build from source (in repo or clone via STET_REPO_URL).
install_via_source() {
	if [ -f "go.mod" ] && [ -d "cli" ] && grep -q 'module stet' go.mod 2>/dev/null; then
		make build
		mkdir -p "$INSTALL_DIR"
		cp -f bin/stet "$INSTALL_DIR/stet"
		chmod +x "$INSTALL_DIR/stet"
		return 0
	fi
	if [ -n "$REPO_URL" ]; then
		TMP_DIR=$(mktemp -d)
		trap 'rm -rf "$TMP_DIR"' EXIT
		git clone --depth 1 "$REPO_URL" "$TMP_DIR"
		cd "$TMP_DIR"
		make build
		mkdir -p "$INSTALL_DIR"
		cp -f bin/stet "$INSTALL_DIR/stet"
		chmod +x "$INSTALL_DIR/stet"
		return 0
	fi
	return 1
}

if install_via_download; then
	echo "Installed stet to $INSTALL_DIR"
	echo "Ensure $INSTALL_DIR is in your PATH."
	exit 0
fi

if install_via_source; then
	echo "Installed stet to $INSTALL_DIR (built from source)"
	echo "Ensure $INSTALL_DIR is in your PATH."
	exit 0
fi

echo "Error: Could not install stet." >&2
echo "  - Download from GitHub Releases failed (no release or unsupported platform)." >&2
echo "  - Not in a stet repo and STET_REPO_URL is not set." >&2
echo "  Run this script from the stet repo root, set STET_REPO_URL to the git clone URL, or install from a GitHub Release." >&2
exit 1
