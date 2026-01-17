# Multi-Agent Coordination Demo Runner
# This script runs the full multi-agent demo

param(
    [switch]$Live,
    [switch]$Verbose,
    [switch]$Sequential,
    [int]$MaxTurns = 10,
    [int]$Timeout = 5,
    [string]$Model = ""
)

$ErrorActionPreference = "Stop"
$baseDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Split-Path -Parent (Split-Path -Parent $baseDir)

Write-Host @"

 __  __       _ _   _          _                    _
|  \/  |_   _| | |_(_)        / \   __ _  ___ _ __ | |_
| |\/| | | | | | __| |_____  / _ \ / _` |/ _ \ '_ \| __|
| |  | | |_| | | |_| |_____/ ___ \ (_| |  __/ | | | |_
|_|  |_|\__,_|_|\__|_|    /_/   \_\__, |\___|_| |_|\__|
                                  |___/
         BRUTUS Multi-Agent Coordination Demo

"@ -ForegroundColor Cyan

# Step 1: Reset demo files
Write-Host "[1/3] Resetting demo files..." -ForegroundColor Yellow
& "$baseDir\reset.ps1"
Write-Host ""

# Step 2: Build brutus-test if needed
Write-Host "[2/3] Checking for brutus-test..." -ForegroundColor Yellow
$brutusTest = Join-Path $projectRoot "brutus-test.exe"
if (-not (Test-Path $brutusTest)) {
    Write-Host "Building brutus-test.exe..." -ForegroundColor Gray
    Push-Location $projectRoot
    go build -o brutus-test.exe ./cmd/brutus-test
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Failed to build brutus-test.exe" -ForegroundColor Red
        exit 1
    }
    Pop-Location
}
Write-Host "brutus-test.exe is ready" -ForegroundColor Green
Write-Host ""

# Step 3: Run the demo
Write-Host "[3/3] Running multi-agent demo..." -ForegroundColor Yellow
Write-Host ""

$args = @()
if ($Verbose) { $args += "-v" }
if ($Sequential) { $args += "-concurrent=false" }

if ($Live) {
    Write-Host "Mode: LIVE (using real Saturn LLM)" -ForegroundColor Magenta
    Write-Host "NOTE: Requires a Saturn beacon on the network!" -ForegroundColor Gray
    Write-Host ""

    $args += "-max-turns=$MaxTurns"
    $args += "-timeout=$Timeout"
    if ($Model) { $args += "-model=$Model" }

    & $brutusTest live-multi-agent @args "$baseDir\live-scenario.json"
} else {
    Write-Host "Mode: MOCKED (no network required)" -ForegroundColor Blue
    Write-Host ""

    & $brutusTest multi-agent @args "$baseDir\multi-scenario.json"
}

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "Demo completed successfully!" -ForegroundColor Green
} else {
    Write-Host ""
    Write-Host "Demo failed with exit code: $LASTEXITCODE" -ForegroundColor Red
    exit $LASTEXITCODE
}
