#!/usr/bin/env pwsh
# Build aom for Linux/amd64 on Windows and deploy it into the WSL user's PATH.

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path $PSScriptRoot -Parent

Write-Host "Building aom for Linux..."

$env:GOOS        = "linux"
$env:GOARCH      = "amd64"
$env:GOTOOLCHAIN = "local"
$env:GOCACHE     = "$ProjectRoot\.gocache"
$env:GOMODCACHE  = "$ProjectRoot\.gomodcache"
$env:GOTELEMETRY = "off"

$OutputPath = "$ProjectRoot\aom-linux"
go build -o $OutputPath "$ProjectRoot\cmd\aom\main.go"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "Copying to WSL ~/.local/bin/aom..."
$WslPath = wsl wslpath -u ($OutputPath -replace "\\", "/")
wsl bash -lc "mkdir -p ~/.local/bin"
wsl cp $WslPath ~/.local/bin/aom
wsl chmod +x ~/.local/bin/aom

Write-Host "Done."
Write-Host "Test with: wsl bash -lc 'command -v aom && aom help'"
