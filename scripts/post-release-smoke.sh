#!/usr/bin/env bash
# Post-release smoke: confirm a published stet release has exactly the expected assets, then
# install it with the public one-line installer and check the installed version.
#
# Usage: scripts/post-release-smoke.sh vX.Y.Z
# Env:   STET_REPO (default beettlle/stet)
#
# The install is retried with bounded backoff because raw.githubusercontent.com can serve a
# cached install.sh for a few minutes after main moves. Exhausted retries fail closed.
set -euo pipefail

TAG="${1:-}"
if ! printf '%s' "$TAG" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
	echo "usage: $0 vX.Y.Z" >&2
	exit 2
fi

REPO="${STET_REPO:-beettlle/stet}"
INSTALLER_URL="https://raw.githubusercontent.com/${REPO}/main/install.sh"
MAX_ATTEMPTS=6
MAX_DELAY_SECONDS=60
EXPECTED_ASSETS="checksums.txt
install.ps1
install.sh
stet-darwin-amd64
stet-darwin-arm64
stet-linux-amd64
stet-linux-arm64
stet-windows-amd64.exe
stet-windows-arm64.exe"

echo "== release $TAG ($REPO)"
release_json="$(gh release view "$TAG" -R "$REPO" --json isDraft,isPrerelease,url,assets)"
if [ "$(printf '%s' "$release_json" | jq -r '.isDraft')" != "false" ]; then
	echo "FAIL: release $TAG is a draft" >&2
	exit 1
fi
actual_assets="$(printf '%s' "$release_json" | jq -r '.assets[].name' | sort)"
if [ "$actual_assets" != "$EXPECTED_ASSETS" ]; then
	echo "FAIL: release assets differ from the expected set" >&2
	diff <(printf '%s\n' "$EXPECTED_ASSETS") <(printf '%s\n' "$actual_assets") >&2 || true
	exit 1
fi
echo "PASS: $(printf '%s\n' "$actual_assets" | wc -l | tr -d ' ') assets at $(printf '%s' "$release_json" | jq -r '.url')"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
# Run the installer outside any checkout so a failed download cannot fall back to a source build.
cd "$work"

attempt=1
delay=5
while true; do
	echo "== install attempt $attempt/$MAX_ATTEMPTS"
	if install_out="$(curl -fsSL "$INSTALLER_URL" | STET_RELEASE_TAG="$TAG" STET_INSTALL_DIR="$work/bin" bash 2>&1)" &&
		version_out="$("$work/bin/stet" --version 2>&1)"; then
		case "$version_out" in
			*"$TAG"*)
				echo "PASS: installed $version_out"
				exit 0
				;;
		esac
		install_out="installed binary reports '$version_out', expected $TAG"
	fi
	echo "$install_out" >&2
	if [ "$attempt" -ge "$MAX_ATTEMPTS" ]; then
		echo "FAIL: install smoke for $TAG failed after $MAX_ATTEMPTS attempts" >&2
		exit 1
	fi
	echo "retrying in ${delay}s" >&2
	sleep "$delay"
	attempt=$((attempt + 1))
	delay=$((delay * 2))
	if [ "$delay" -gt "$MAX_DELAY_SECONDS" ]; then
		delay="$MAX_DELAY_SECONDS"
	fi
done
