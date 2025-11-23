# PowerShell script to start infrastructure services

Write-Host "Starting Infrastructure Services..." -ForegroundColor Green

# Change to script directory (go-data-storage root)
$scriptDir = Split-Path -Parent $PSScriptRoot
Set-Location $scriptDir

# Check if Docker is running
try {
    docker ps | Out-Null
    Write-Host "Docker is running" -ForegroundColor Green
} catch {
    Write-Host "Docker is not running. Please start Docker Desktop." -ForegroundColor Red
    exit 1
}

# Ensure main services network exists
Write-Host "Ensuring Docker network exists..." -ForegroundColor Yellow
docker network create iot-network 2>$null

# Create nginx directories if they don't exist
if (-not (Test-Path "infra\nginx\ssl")) {
    New-Item -ItemType Directory -Path "infra\nginx\ssl" -Force | Out-Null
    Write-Host "Created infra\nginx\ssl directory" -ForegroundColor Yellow
}

# Create nextcloud data directory
if (-not (Test-Path "nextcloud_data")) {
    New-Item -ItemType Directory -Path "nextcloud_data" -Force | Out-Null
    Write-Host "Created nextcloud_data directory" -ForegroundColor Yellow
}

# Start infrastructure services
Write-Host "Starting infrastructure services (Nginx, Portainer, Nextcloud)..." -ForegroundColor Yellow
docker-compose -f infra/docker-compose.infrastructure.yml up -d

Write-Host "Waiting for services to be ready..." -ForegroundColor Yellow
Start-Sleep -Seconds 15

# Check service status
Write-Host "`nInfrastructure Service Status:" -ForegroundColor Cyan
docker-compose -f infra/docker-compose.infrastructure.yml ps

Write-Host "`nInfrastructure services are starting up!" -ForegroundColor Green
Write-Host "Portainer (Container Management): http://localhost:9000" -ForegroundColor Cyan
Write-Host "Nextcloud (File Storage): http://localhost:8081" -ForegroundColor Cyan
Write-Host "Nginx (Reverse Proxy): http://localhost" -ForegroundColor Cyan

Write-Host "`nNext Steps:" -ForegroundColor Yellow
Write-Host "1. Access Portainer and create an admin account" -ForegroundColor White
Write-Host "2. Access Nextcloud and complete the setup wizard" -ForegroundColor White
Write-Host "3. Configure cloud sync in Nextcloud (Apps > External Storage)" -ForegroundColor White
Write-Host "4. Set up SSL certificates for HTTPS (see SETUP.md)" -ForegroundColor White

Write-Host "`nTo view logs: docker-compose -f infra/docker-compose.infrastructure.yml logs -f" -ForegroundColor Yellow
Write-Host "To stop services: docker-compose -f infra/docker-compose.infrastructure.yml down" -ForegroundColor Yellow

