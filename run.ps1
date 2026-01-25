# Single entrypoint to run the API on Windows (PowerShell).
# Detects environment prerequisites and runs with pure-Go SQLite.

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

function Get-GoVersion {
    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $output = & go version 2>$null
    $exitCode = $LASTEXITCODE
    $ErrorActionPreference = $oldPreference

    if ($exitCode -ne 0) { return $null }
    if (-not $output) { return $null }
    if ($output -match "go version go(\d+)\.(\d+)(\.\d+)?") {
        return @{ Major = [int]$Matches[1]; Minor = [int]$Matches[2] }
    }
    return $null
}

$isWindows = $env:OS -eq "Windows_NT"
$isWsl = [bool]$env:WSL_DISTRO_NAME

Write-Host "Starting go-data-storage API..." -ForegroundColor Cyan
Write-Host "Using pure-Go SQLite (no C compiler required)" -ForegroundColor Yellow
if ($isWsl) {
    Write-Host "WSL detected. Running with Linux toolchain." -ForegroundColor Yellow
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go is not installed or not on PATH. Install Go 1.25+ from https://go.dev/dl/."
}

$env:GOTOOLCHAIN = "local"

$goVersion = Get-GoVersion
if (-not $goVersion -or $goVersion.Major -lt 1 -or ($goVersion.Major -eq 1 -and $goVersion.Minor -lt 25)) {
    throw "Go 1.25+ is required. Found: $(& go version). Install via https://go.dev/dl/ and reopen PowerShell."
}

$env:CGO_ENABLED = "0"

if (-not (Test-Path .env)) {
    Write-Host "Warning: .env file not found. Using default configuration." -ForegroundColor Yellow
}

go run ./cmd/api
