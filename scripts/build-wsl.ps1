#!/usr/bin/env pwsh
# Build aom-linux (Linux/amd64) and deploy to WSL home directory.

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

Write-Host "Copying to WSL ~/aom-linux..."
$WslPath = wsl wslpath -u ($OutputPath -replace "\\", "/")
wsl cp $WslPath ~/aom-linux
wsl chmod +x ~/aom-linux

Write-Host "Done. Test with: wsl ~/aom-linux --help"
