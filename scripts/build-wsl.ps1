#!/usr/bin/env pwsh
# build-wsl.ps1 — build aom inside WSL and install to /usr/local/bin/aom
#
# Usage (from Windows terminal / PowerShell):
#   .\scripts\build-wsl.ps1           # build + install
#   .\scripts\build-wsl.ps1 --test    # build + quick tests + install
#   .\scripts\build-wsl.ps1 --dry     # build only, don't install
#
# Requires WSL with Go installed. The binary is installed system-wide via
# "wsl -u root" so no sudo password prompt is needed.

param(
    [switch]$Test,
    [switch]$Dry
)

$ErrorActionPreference = "Stop"

# ── Resolve project root (WSL path) ──────────────────────────────────────────

$WinRoot = Split-Path $PSScriptRoot -Parent
# Convert Windows path to WSL path:  C:\Users\... → /mnt/c/Users/...
$WslRoot = wsl wslpath -u ($WinRoot -replace "\\", "/")

Write-Host "[aom-install] Project root (WSL): $WslRoot"

# ── Build inside WSL ─────────────────────────────────────────────────────────

Write-Host "[aom-install] Building..."

$buildScript = @"
set -euo pipefail
cd '$WslRoot'
GOTOOLCHAIN=local GOCACHE=/tmp/aom-gocache GOMODCACHE=/tmp/aom-gomodcache GOTELEMETRY=off \
    go build -o /tmp/aom-new ./cmd/aom/main.go
echo "Build OK: \$(du -h /tmp/aom-new | cut -f1)"
"@

wsl bash -lc $buildScript
if ($LASTEXITCODE -ne 0) {
    Write-Error "Build failed."
    exit 1
}

# ── Tests (optional) ─────────────────────────────────────────────────────────

if ($Test) {
    Write-Host "[aom-install] Running unit tests..."

    $testScript = @"
set -euo pipefail
cd '$WslRoot'
GOTOOLCHAIN=local GOCACHE=/tmp/aom-gocache GOMODCACHE=/tmp/aom-gomodcache GOTELEMETRY=off \
    go test -timeout 5m \
        -run 'TestExecuteProjectInit|TestExecuteStatus|TestCaptureAll|TestHelp|TestTask|TestStep|TestChannel|TestMessage|TestSession\$|TestSessionList|TestSessionStop|TestSessionArchive|TestDB' \
        ./internal/...
"@

    wsl bash -lc $testScript
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Tests failed — fix before installing (or run without --test to skip)."
        exit 1
    }
    Write-Host "[aom-install] Tests passed."
}

# ── Dry run exit ─────────────────────────────────────────────────────────────

if ($Dry) {
    Write-Host "[aom-install] Dry run — skipping installation."
    Write-Host "[aom-install] Binary is at /tmp/aom-new inside WSL."
    exit 0
}

# ── Install via wsl -u root ───────────────────────────────────────────────────

Write-Host "[aom-install] Installing to /usr/local/bin/aom..."
wsl -u root cp /tmp/aom-new /usr/local/bin/aom
if ($LASTEXITCODE -ne 0) {
    Write-Error "Install failed."
    exit 1
}

# ── Verify ────────────────────────────────────────────────────────────────────

$verify = wsl bash -lc "ls -lh /usr/local/bin/aom"
Write-Host "[aom-install] Installed: $verify"
Write-Host "[aom-install] Ready.  Run: wsl aom --help"
