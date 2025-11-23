# PowerShell script to start the IoT project

Write-Host "Starting IoT Data Storage and Visualization Project..." -ForegroundColor Green

# Check if Docker is running
try {
    docker ps | Out-Null
    Write-Host "Docker is running" -ForegroundColor Green
} catch {
    Write-Host "Docker is not running. Please start Docker Desktop." -ForegroundColor Red
    exit 1
}

# Create network if it doesn't exist
Write-Host "Creating Docker network..." -ForegroundColor Yellow
docker network create iot-network 2>$null

# Change to script directory (go-data-storage root)
$scriptDir = Split-Path -Parent $PSScriptRoot
Set-Location $scriptDir

# Create .env file if it doesn't exist
if (-not (Test-Path ".env")) {
    Write-Host "Creating .env file from example..." -ForegroundColor Yellow
    if (Test-Path ".env.example") {
        Copy-Item ".env.example" ".env"
        Write-Host "Please edit .env with your database credentials" -ForegroundColor Yellow
    }
}

# Start main services
Write-Host "Starting main services (PostgreSQL, API)..." -ForegroundColor Yellow
docker-compose -f infra/docker-compose.yml up -d

Write-Host "Waiting for services to be ready..." -ForegroundColor Yellow
Start-Sleep -Seconds 10

# Check service status
Write-Host "`nService Status:" -ForegroundColor Cyan
docker-compose -f infra/docker-compose.yml ps

Write-Host "`nServices are starting up!" -ForegroundColor Green
Write-Host "API: http://localhost:8080" -ForegroundColor Cyan
Write-Host "PostgreSQL: localhost:5432" -ForegroundColor Cyan
Write-Host "`nNote: Frontend should be run separately from the data-visualizer repository" -ForegroundColor Yellow

Write-Host "`nTo view logs: docker-compose -f infra/docker-compose.yml logs -f" -ForegroundColor Yellow
Write-Host "To stop services: docker-compose -f infra/docker-compose.yml down" -ForegroundColor Yellow

