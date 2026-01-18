# PowerShell script to run go-data-storage API with CGO enabled
# This is required for SQLite support

Write-Host "Starting go-data-storage API..." -ForegroundColor Cyan
Write-Host "CGO_ENABLED=1 (required for SQLite)" -ForegroundColor Yellow

$env:CGO_ENABLED = "1"

# Check if .env exists
if (-not (Test-Path .env)) {
    Write-Host "Warning: .env file not found. Using default configuration." -ForegroundColor Yellow
}

# Run the API
go run ./cmd/api
