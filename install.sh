#!/usr/bin/env bash
#
# Install stet: downloads the binary for this OS/arch from GitHub Releases and verifies it
# against the release's checksums.txt. Falls back to building from source when no release
# binary can be downloaded.
#
# Usage:
#   One-line (Mac/Linux): curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh | bash
#   Pinned version:       curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh | STET_RELEASE_TAG=v1.0.0 bash
#   From repo:            ./install.sh
#   Via clone:            STET_REPO_URL=https://github.com/beettlle/stet.git bash -c "$(curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh)"
#
# Env:
#   STET_REPO               GitHub owner/repo (default beettlle/stet).
#   STET_RELEASE_TAG        Release tag such as v1.0.0, or "latest" (default).
#   STET_INSTALL_DIR        Install directory (default ~/.local/bin).
#   STET_DOWNLOAD_BASE_URL  Base URL of release assets; overrides STET_REPO/STET_RELEASE_TAG (mirrors, testing).
#   STET_REPO_URL           Git clone URL for the build-from-source fallback.
#
set -euo pipefail

STET_REPO="${STET_REPO:-beettlle/stet}"
RELEASE_TAG="${STET_RELEASE_TAG:-latest}"
INSTALL_DIR="${STET_INSTALL_DIR:-$HOME/.local/bin}"
REPO_URL="${STET_REPO_URL:-}"

# releases/latest/download redirects to the newest non-prerelease asset, so no GitHub API
# call (and no unauthenticated rate limit) is needed to resolve "latest".
if [ -n "${STET_DOWNLOAD_BASE_URL:-}" ]; then
	BASE_URL="${STET_DOWNLOAD_BASE_URL%/}"
elif [ "$RELEASE_TAG" = "latest" ]; then
	BASE_URL="https://github.com/${STET_REPO}/releases/latest/download"
else
	BASE_URL="https://github.com/${STET_REPO}/releases/download/${RELEASE_TAG}"
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

TMP_BIN=""
TMP_SUMS=""
TMP_DIR=""
cleanup() {
	[ -z "$TMP_BIN" ] || rm -f "$TMP_BIN"
	[ -z "$TMP_SUMS" ] || rm -f "$TMP_SUMS"
	[ -z "$TMP_DIR" ] || rm -rf "$TMP_DIR"
}
trap cleanup EXIT

sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{ print $1 }'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{ print $1 }'
	else
		return 1
	fi
}

# Stages the binary inside INSTALL_DIR so the final mv is an atomic rename on the same
# filesystem; this also avoids "text file busy" when replacing a running stet.
stage_binary() {
	[ -z "$TMP_BIN" ] || rm -f "$TMP_BIN"
	mkdir -p "$INSTALL_DIR"
	TMP_BIN=$(mktemp "$INSTALL_DIR/.stet.XXXXXX")
}

place_binary() {
	chmod 755 "$TMP_BIN"
	mv -f "$TMP_BIN" "$INSTALL_DIR/stet"
	TMP_BIN=""
}

# Returns 1 when the binary cannot be downloaded (so the caller can fall back to source).
# Exits on checksum problems: a binary that fails verification must never be installed.
install_via_download() {
	stage_binary
	echo "Downloading $BINARY_NAME ($RELEASE_TAG) from $BASE_URL"
	if ! curl -fsSL --retry 3 -o "$TMP_BIN" "$BASE_URL/$BINARY_NAME"; then
		echo "Warning: Could not download $BASE_URL/$BINARY_NAME" >&2
		return 1
	fi
	if [ ! -s "$TMP_BIN" ]; then
		echo "Warning: Downloaded $BINARY_NAME is empty" >&2
		return 1
	fi

	TMP_SUMS=$(mktemp)
	if ! curl -fsSL --retry 3 -o "$TMP_SUMS" "$BASE_URL/checksums.txt"; then
		echo "Error: Could not download $BASE_URL/checksums.txt; refusing to install an unverified binary." >&2
		exit 1
	fi
	EXPECTED=$(awk -v f="$BINARY_NAME" '$2 == f { print $1 }' "$TMP_SUMS")
	if [ -z "$EXPECTED" ]; then
		echo "Error: checksums.txt has no entry for $BINARY_NAME." >&2
		exit 1
	fi
	if ! ACTUAL=$(sha256_of "$TMP_BIN"); then
		echo "Error: Neither sha256sum nor shasum is available to verify the download." >&2
		exit 1
	fi
	if [ "$EXPECTED" != "$ACTUAL" ]; then
		echo "Error: Checksum mismatch for $BINARY_NAME (expected $EXPECTED, got $ACTUAL)." >&2
		exit 1
	fi

	place_binary
}

build_and_place() {
	make build
	stage_binary
	cp -f bin/stet "$TMP_BIN"
	place_binary
}

# Fallback: build from source (in repo or clone via STET_REPO_URL).
install_via_source() {
	if [ -f "go.mod" ] && [ -d "cli" ] && grep -q 'module stet' go.mod 2>/dev/null; then
		build_and_place
		return 0
	fi
	if [ -n "$REPO_URL" ]; then
		TMP_DIR=$(mktemp -d)
		git clone --depth 1 "$REPO_URL" "$TMP_DIR"
		cd "$TMP_DIR"
		build_and_place
		return 0
	fi
	return 1
}

report_installed() {
	if ! VERSION_OUT=$("$INSTALL_DIR/stet" --version 2>&1); then
		echo "Error: Installed $INSTALL_DIR/stet but it failed to run: $VERSION_OUT" >&2
		exit 1
	fi
	echo "Installed $VERSION_OUT to $INSTALL_DIR/stet${1:-}"
	case ":$PATH:" in
		*":$INSTALL_DIR:"*) ;;
		*)
			echo "$INSTALL_DIR is not in your PATH. Add it to ~/.zshrc or ~/.bashrc:"
			echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
			;;
	esac
}

if install_via_download; then
	report_installed
	exit 0
fi

if install_via_source; then
	report_installed " (built from source)"
	exit 0
fi

echo "Error: Could not install stet." >&2
echo "  - Download from GitHub Releases failed (no release for $RELEASE_TAG or unsupported platform)." >&2
echo "  - Not in a stet repo and STET_REPO_URL is not set." >&2
echo "  Run this script from the stet repo root, set STET_REPO_URL to the git clone URL, or install from a GitHub Release." >&2
exit 1
