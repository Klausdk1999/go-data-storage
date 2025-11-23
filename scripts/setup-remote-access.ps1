# PowerShell script to set up remote access for IoT project

Write-Host "Setting up Remote Access for IoT Project..." -ForegroundColor Green
Write-Host ""

# Check if services are running
Write-Host "Checking if services are running..." -ForegroundColor Yellow
$servicesRunning = docker-compose ps 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Host "Services are not running. Starting them..." -ForegroundColor Yellow
    .\start.ps1
}

Write-Host ""
Write-Host "=== Remote Access Options ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "1. Tailscale (Recommended - Easiest)" -ForegroundColor Green
Write-Host "   - No port forwarding needed"
Write-Host "   - Works behind firewalls"
Write-Host "   - Free for personal use"
Write-Host "   - Setup: 5 minutes"
Write-Host ""
Write-Host "2. Port Forwarding + Dynamic DNS" -ForegroundColor Yellow
Write-Host "   - Requires router access"
Write-Host "   - Free dynamic DNS available"
Write-Host "   - Setup: 15-30 minutes"
Write-Host ""
Write-Host "3. Cloudflare Tunnel" -ForegroundColor Yellow
Write-Host "   - Free and secure"
Write-Host "   - No port forwarding"
Write-Host "   - Setup: 15 minutes"
Write-Host ""

$choice = Read-Host "Choose option (1, 2, or 3)"

switch ($choice) {
    "1" {
        Write-Host ""
        Write-Host "=== Tailscale Setup ===" -ForegroundColor Cyan
        Write-Host ""
        Write-Host "1. Download Tailscale from: https://tailscale.com/download" -ForegroundColor White
        Write-Host "2. Install and sign in with Google/Microsoft/GitHub" -ForegroundColor White
        Write-Host "3. Note your Tailscale IP (e.g., 100.x.x.x)" -ForegroundColor White
        Write-Host ""
        
        # Check if Tailscale is installed
        $tailscaleInstalled = Get-Command tailscale -ErrorAction SilentlyContinue
        if ($tailscaleInstalled) {
            Write-Host "Tailscale is installed! Checking status..." -ForegroundColor Green
            $status = tailscale status 2>$null
            if ($LASTEXITCODE -eq 0) {
                Write-Host $status
                Write-Host ""
                Write-Host "Your Tailscale IP addresses:" -ForegroundColor Green
                tailscale ip -4
            } else {
                Write-Host "Tailscale is installed but not running. Please start it." -ForegroundColor Yellow
            }
        } else {
            Write-Host "Tailscale is not installed." -ForegroundColor Yellow
            Write-Host "Would you like to install it now? (Y/N)" -ForegroundColor White
            $install = Read-Host
            if ($install -eq "Y" -or $install -eq "y") {
                Write-Host "Installing Tailscale..." -ForegroundColor Yellow
                winget install Tailscale.Tailscale
            }
        }
        
        Write-Host ""
        Write-Host "=== Your Fixed Endpoints ===" -ForegroundColor Cyan
        Write-Host "Once Tailscale is set up, your endpoints will be:" -ForegroundColor White
        Write-Host "  Frontend: http://[YOUR_TAILSCALE_IP]" -ForegroundColor Green
        Write-Host "  API:      http://[YOUR_TAILSCALE_IP]/api" -ForegroundColor Green
        Write-Host "  IoT Data: POST http://[YOUR_TAILSCALE_IP]/api/readings" -ForegroundColor Green
    }
    
    "2" {
        Write-Host ""
        Write-Host "=== Port Forwarding Setup ===" -ForegroundColor Cyan
        Write-Host ""
        Write-Host "1. Get a free Dynamic DNS:" -ForegroundColor White
        Write-Host "   - DuckDNS: https://www.duckdns.org/" -ForegroundColor Yellow
        Write-Host "   - No-IP: https://www.noip.com/" -ForegroundColor Yellow
        Write-Host ""
        Write-Host "2. Configure your router:" -ForegroundColor White
        Write-Host "   - Forward port 80 → Your PC IP:80" -ForegroundColor Yellow
        Write-Host "   - Forward port 8080 → Your PC IP:8080 (optional)" -ForegroundColor Yellow
        Write-Host ""
        Write-Host "3. Install Dynamic DNS updater on this PC" -ForegroundColor White
        Write-Host ""
        
        # Get local IP
        $localIP = (Get-NetIPAddress -AddressFamily IPv4 | Where-Object {$_.InterfaceAlias -notlike "*Loopback*" -and $_.IPAddress -notlike "169.254.*"}).IPAddress | Select-Object -First 1
        Write-Host "Your local IP address: $localIP" -ForegroundColor Green
        Write-Host ""
        Write-Host "Configure router to forward ports to: $localIP" -ForegroundColor Yellow
    }
    
    "3" {
        Write-Host ""
        Write-Host "=== Cloudflare Tunnel Setup ===" -ForegroundColor Cyan
        Write-Host ""
        Write-Host "1. Install cloudflared:" -ForegroundColor White
        Write-Host "   Download from: https://github.com/cloudflare/cloudflared/releases" -ForegroundColor Yellow
        Write-Host "   Or: choco install cloudflared" -ForegroundColor Yellow
        Write-Host ""
        Write-Host "2. Create tunnel:" -ForegroundColor White
        Write-Host "   cloudflared tunnel create iot-project" -ForegroundColor Yellow
        Write-Host ""
        Write-Host "3. Configure and run:" -ForegroundColor White
        Write-Host "   See REMOTE_ACCESS_SETUP.md for detailed instructions" -ForegroundColor Yellow
    }
}

Write-Host ""
Write-Host "=== Firewall Configuration ===" -ForegroundColor Cyan
Write-Host "Configuring Windows Firewall rules..." -ForegroundColor Yellow

# Check if rules exist, create if not
$rules = @(
    @{Name="IoT API"; Port=8080},
    @{Name="IoT Frontend"; Port=3000},
    @{Name="IoT Nginx"; Port=80}
)

foreach ($rule in $rules) {
    $existing = Get-NetFirewallRule -DisplayName $rule.Name -ErrorAction SilentlyContinue
    if (-not $existing) {
        New-NetFirewallRule -DisplayName $rule.Name -Direction Inbound -LocalPort $rule.Port -Protocol TCP -Action Allow | Out-Null
        Write-Host "  ✓ Created firewall rule for port $($rule.Port)" -ForegroundColor Green
    } else {
        Write-Host "  ✓ Firewall rule for port $($rule.Port) already exists" -ForegroundColor Green
    }
}

Write-Host ""
Write-Host "=== Next Steps ===" -ForegroundColor Cyan
Write-Host "1. Complete the remote access setup above" -ForegroundColor White
Write-Host "2. Test your endpoints from another device" -ForegroundColor White
Write-Host "3. Update your IoT device code with the fixed endpoint" -ForegroundColor White
Write-Host ""
Write-Host "For detailed instructions, see: REMOTE_ACCESS_SETUP.md" -ForegroundColor Yellow

