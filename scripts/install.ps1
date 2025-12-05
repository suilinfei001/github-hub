#requires -version 5.0
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
$BinDir = if ($env:BIN_DIR) { $env:BIN_DIR } else { Join-Path $Root "bin" }
$GoBin = if ($env:GO) { $env:GO } else { "go" }

if (-not (Test-Path $BinDir)) {
    New-Item -ItemType Directory -Path $BinDir | Out-Null
}

Write-Host "Installing ghh-server -> $BinDir/ghh-server"
& $GoBin build -o (Join-Path $BinDir "ghh-server") (Join-Path $Root "cmd/ghh-server") | Out-Null

Write-Host "Installing ghh -> $BinDir/ghh"
& $GoBin build -o (Join-Path $BinDir "ghh") (Join-Path $Root "cmd/ghh") | Out-Null

Write-Host "Done."
