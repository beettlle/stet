# Install stet on Windows: download from GitHub Releases by architecture, or show fallback instructions.
#
# Usage (PowerShell; may require -ExecutionPolicy Bypass):
#   irm https://raw.githubusercontent.com/OWNER/REPO/main/install.ps1 | iex
#
# Env: STET_REPO (e.g. owner/repo), STET_RELEASE_TAG (e.g. v1.0.0 or "latest"), STET_INSTALL_DIR

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$STET_REPO = if ($env:STET_REPO) { $env:STET_REPO } else { "stet/stet" }
$STET_RELEASE_TAG = if ($env:STET_RELEASE_TAG) { $env:STET_RELEASE_TAG } else { "latest" }
$INSTALL_DIR = if ($env:STET_INSTALL_DIR) { $env:STET_INSTALL_DIR } else { Join-Path $env:USERPROFILE ".local\bin" }

function Write-Err {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Red
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

# Resolve latest tag from GitHub API
if ($STET_RELEASE_TAG -eq 'latest') {
    try {
        $api = "https://api.github.com/repos/$STET_REPO/releases/latest"
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        $r = Invoke-RestMethod -Uri $api -UseBasicParsing -ErrorAction Stop
        if ($r.tag_name) { $STET_RELEASE_TAG = $r.tag_name }
    } catch {
        $STET_RELEASE_TAG = $null
    }
}

$arch = Get-StetArch
if (-not $arch) {
    Write-Err "Unsupported architecture. Supported: amd64 (x64), arm64."
    exit 1
}

$BINARY_NAME = "stet-windows-$arch.exe"
$DOWNLOAD_URL = "https://github.com/$STET_REPO/releases/download/$STET_RELEASE_TAG/$BINARY_NAME"

if (-not $STET_RELEASE_TAG) {
    Write-Err "Could not resolve latest release (no releases or API error)."
    Write-Host "Install from source: install Go, then: go install github.com/$STET_REPO/cli/cmd/stet@latest"
    Write-Host "Or clone the repo, run 'make release', and copy dist/$BINARY_NAME to a directory in your PATH."
    exit 1
}

New-Item -ItemType Directory -Force -Path $INSTALL_DIR | Out-Null
$dest = Join-Path $INSTALL_DIR "stet.exe"
$tmpFile = Join-Path $INSTALL_DIR "stet.tmp.$PID.exe"

try {
    Write-Host "Downloading stet (release: $STET_RELEASE_TAG)..."
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri $DOWNLOAD_URL -OutFile $tmpFile -UseBasicParsing -ErrorAction Stop
} catch {
    Write-Err "Download failed: $($_.Exception.Message)"
    Write-Host "Install from source: install Go, then: go install github.com/$STET_REPO/cli/cmd/stet@latest"
    Write-Host "Or clone the repo, run 'make release', and copy dist/$BINARY_NAME to a directory in your PATH."
    exit 1
}

if ((Get-Item $tmpFile).Length -le 0) {
    Remove-Item -Force -ErrorAction SilentlyContinue $tmpFile
    Write-Err "Downloaded file is empty."
    exit 1
}

# Optional: verify checksum
$CHECKSUM_URL = "https://github.com/$STET_REPO/releases/download/$STET_RELEASE_TAG/checksums.txt"
try {
    $resp = Invoke-WebRequest -Uri $CHECKSUM_URL -UseBasicParsing -ErrorAction Stop
    $lines = $resp.Content -split "`n"
    $line = $lines | Where-Object { $_ -match "^\s*([0-9a-fA-F]+)\s+$([regex]::Escape($BINARY_NAME))" } | Select-Object -First 1
    if ($line -match "^\s*([0-9a-fA-F]+)\s+") {
        $expected = $Matches[1].ToLower()
        $actual = (Get-FileHash -Path $tmpFile -Algorithm SHA256).Hash.ToLower()
        if ($expected -ne $actual) {
            Write-Host "Warning: Checksum verification failed; continuing anyway." -ForegroundColor Yellow
        }
    }
} catch {
    # No checksums or network error; skip verification
}

Move-Item -Force -Path $tmpFile -Destination $dest
try { Unblock-File -Path $dest -ErrorAction SilentlyContinue } catch { }

Write-Success "Installed stet to $INSTALL_DIR"
Write-Success "You can run 'stet' from the terminal if $INSTALL_DIR is in your PATH."
Write-Host "To add to PATH for this user: [Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + ';$INSTALL_DIR', 'User')"
