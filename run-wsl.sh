#!/bin/bash
# WSL script to run Go API with SQLite (CGO enabled)

# Navigate to project directory (Windows path mounted in WSL)
cd "/mnt/c/Users/Klaus/Documents/Mestrado CA/go-data-storage"

# Clean PATH - only include essential paths, avoid Windows PATH pollution
export PATH="/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export GOROOT="/usr/local/go"

# Enable CGO for SQLite support
export CGO_ENABLED=1

echo "=== Go Data Storage API (WSL) ==="
echo "CGO_ENABLED: $CGO_ENABLED"
echo "Go version: $(go version)"
echo "GCC version: $(gcc --version | head -1)"
echo "Working directory: $(pwd)"
echo ""

# Run the API
go run ./cmd/api
