# Install stet on Windows: downloads the binary for this architecture from GitHub Releases and
# verifies it against the release's checksums.txt.
#
# Usage (PowerShell; may require -ExecutionPolicy Bypass):
#   irm https://raw.githubusercontent.com/beettlle/stet/main/install.ps1 | iex
#   Pinned version: $env:STET_RELEASE_TAG = "v1.0.0"; irm https://raw.githubusercontent.com/beettlle/stet/main/install.ps1 | iex
#
# Env:
#   STET_REPO               GitHub owner/repo (default beettlle/stet).
#   STET_RELEASE_TAG        Release tag such as v1.0.0, or "latest" (default).
#   STET_INSTALL_DIR        Install directory (default %USERPROFILE%\.local\bin).
#   STET_DOWNLOAD_BASE_URL  Base URL of release assets; overrides STET_REPO/STET_RELEASE_TAG (mirrors, testing).
#
# Errors are raised with throw rather than exit: under "irm | iex", exit would close the user's shell.

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$STET_REPO = if ($env:STET_REPO) { $env:STET_REPO } else { "beettlle/stet" }
$STET_RELEASE_TAG = if ($env:STET_RELEASE_TAG) { $env:STET_RELEASE_TAG } else { "latest" }
$INSTALL_DIR = if ($env:STET_INSTALL_DIR) { $env:STET_INSTALL_DIR } else { Join-Path $env:USERPROFILE ".local\bin" }

# releases/latest/download redirects to the newest non-prerelease asset, so no GitHub API
# call (and no unauthenticated rate limit) is needed to resolve "latest".
$BASE_URL = if ($env:STET_DOWNLOAD_BASE_URL) {
    $env:STET_DOWNLOAD_BASE_URL.TrimEnd('/')
} elseif ($STET_RELEASE_TAG -eq 'latest') {
    "https://github.com/$STET_REPO/releases/latest/download"
} else {
    "https://github.com/$STET_REPO/releases/download/$STET_RELEASE_TAG"
}

function Write-Success {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Green
}

function Get-StetArch {
    try {
        $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
        switch ($arch) {
            'X64' { return 'amd64' }
            'Arm64' { return 'arm64' }
            default { return $null }
        }
    } catch {
        $pa = $env:PROCESSOR_ARCHITECTURE
        if ($pa -match 'ARM64') { return 'arm64' }
        if ($pa -match '64') { return 'amd64' }
        return $null
    }
}

$arch = Get-StetArch
if (-not $arch) {
    throw "Unsupported architecture. Supported: amd64 (x64), arm64."
}

$BINARY_NAME = "stet-windows-$arch.exe"
$SOURCE_HINT = "To build from source: clone https://github.com/$STET_REPO, run 'make release VERSION=vX.Y.Z', and copy dist\$BINARY_NAME to a directory in your PATH as stet.exe."

New-Item -ItemType Directory -Force -Path $INSTALL_DIR | Out-Null
$dest = Join-Path $INSTALL_DIR "stet.exe"
# Staged in INSTALL_DIR so Move-Item is a same-volume rename.
$tmpFile = Join-Path $INSTALL_DIR "stet.tmp.$PID.exe"
$tmpSums = Join-Path $INSTALL_DIR "stet.checksums.$PID.txt"

[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

try {
    Write-Host "Downloading $BINARY_NAME ($STET_RELEASE_TAG) from $BASE_URL"
    try {
        Invoke-WebRequest -Uri "$BASE_URL/$BINARY_NAME" -OutFile $tmpFile -UseBasicParsing
    } catch {
        throw "Download of $BASE_URL/$BINARY_NAME failed: $($_.Exception.Message)`n$SOURCE_HINT"
    }
    if ((Get-Item $tmpFile).Length -le 0) {
        throw "Downloaded $BINARY_NAME is empty."
    }

    # Saved to a file rather than read from .Content: Windows PowerShell 5.1 returns
    # octet-stream release assets as byte[], not text.
    try {
        Invoke-WebRequest -Uri "$BASE_URL/checksums.txt" -OutFile $tmpSums -UseBasicParsing
    } catch {
        throw "Could not download $BASE_URL/checksums.txt; refusing to install an unverified binary. $($_.Exception.Message)"
    }
    $pattern = "^\s*([0-9a-fA-F]{64})\s+\*?$([regex]::Escape($BINARY_NAME))\s*$"
    $line = Get-Content -Path $tmpSums | Where-Object { $_ -match $pattern } | Select-Object -First 1
    if (-not $line) {
        throw "checksums.txt has no entry for $BINARY_NAME."
    }
    $null = $line -match $pattern
    $expected = $Matches[1].ToLower()
    $actual = (Get-FileHash -Path $tmpFile -Algorithm SHA256).Hash.ToLower()
    if ($expected -ne $actual) {
        throw "Checksum mismatch for $BINARY_NAME (expected $expected, got $actual)."
    }

    Move-Item -Force -Path $tmpFile -Destination $dest
    Unblock-File -Path $dest -ErrorAction SilentlyContinue
} finally {
    Remove-Item -Force -ErrorAction SilentlyContinue $tmpFile, $tmpSums
}

$versionOut = & $dest --version 2>&1
if ($LASTEXITCODE -ne 0) {
    throw "Installed $dest but it failed to run: $versionOut"
}
Write-Success "Installed $versionOut to $dest"

$onPath = $env:Path -split ';' | Where-Object { $_ -and ($_.TrimEnd('\') -ieq $INSTALL_DIR.TrimEnd('\')) }
if (-not $onPath) {
    Write-Host "$INSTALL_DIR is not in your PATH. To add it for this user, run:"
    Write-Host "  [Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + ';$INSTALL_DIR', 'User')"
}
