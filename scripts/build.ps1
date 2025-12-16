Param(
    [string] $Targets = "windows-amd64,windows-arm64",
    [string] $BinDir = "bin",
    [string] $Version = $env:VERSION,
    [string] $Commit = $env:COMMIT,
    [string] $BuildDate = $env:BUILD_DATE
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# Compose ldflags similar to Makefile
$ldParts = @("-s", "-w")
if ($Version) { $ldParts += "-X github-hub/internal/version.Version=$Version" }
if ($Commit) { $ldParts += "-X github-hub/internal/version.Commit=$Commit" }
if ($BuildDate) { $ldParts += "-X github-hub/internal/version.BuildDate=$BuildDate" }
$ldArgs = @()
if ($ldParts.Count -gt 0) {
    $ldArgs = @("-ldflags", ($ldParts -join " "))
}

function Build-Pair {
    param(
        [Parameter(Mandatory)] [string] $OS,
        [Parameter(Mandatory)] [string] $Arch
    )
    $suffix = ""
    if ($OS -eq "windows") { $suffix = ".exe" }
    $outDir = Join-Path $BinDir "$OS-$Arch"
    New-Item -ItemType Directory -Force -Path $outDir | Out-Null

    Write-Host "Building $OS-$Arch ..."
    $env:GOOS = $OS
    $env:GOARCH = $Arch
    $env:CGO_ENABLED = "0"

    go build -trimpath @ldArgs -o (Join-Path $outDir "ghh$suffix") ./cmd/ghh
    go build -trimpath @ldArgs -o (Join-Path $outDir "ghh-server$suffix") ./cmd/ghh-server
}

$list = $Targets -split "," | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne "" }
if ($list.Count -eq 0) {
    Write-Error "No targets specified."
}

foreach ($t in $list) {
    $parts = $t -split "-"
    if ($parts.Count -ne 2) {
        Write-Error "Invalid target format: $t (expected os-arch, e.g., windows-amd64)"
    }
    Build-Pair -OS $parts[0] -Arch $parts[1]
}

Write-Host "Done."

