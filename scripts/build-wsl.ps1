#!/usr/bin/env pwsh
# build-wsl.ps1 — build and install aom from Windows PowerShell via WSL
#
# Usage:
#   .\scripts\build-wsl.ps1           # build + install
#   .\scripts\build-wsl.ps1 -Test     # build + unit tests + install
#   .\scripts\build-wsl.ps1 -Dry      # build only, don't install
#
# Delegates all work to scripts/install.sh running inside WSL so there
# are no PowerShell/bash escaping issues.

param(
    [switch]$Test,
    [switch]$Dry
)

$ErrorActionPreference = "Stop"

# Convert Windows project root to WSL path
$WinRoot  = Split-Path $PSScriptRoot -Parent
$WslRoot  = (wsl wslpath -u ($WinRoot -replace "\\", "/")).Trim()
$WslScript = "$WslRoot/scripts/install.sh"

# Build the flag string to pass through
$Flags = ""
if ($Test) { $Flags += " --test" }
if ($Dry)  { $Flags += " --dry"  }

Write-Host "[aom-install] Delegating to install.sh inside WSL..."
wsl bash -lc "chmod +x '$WslScript' && '$WslScript'$Flags"
exit $LASTEXITCODE
