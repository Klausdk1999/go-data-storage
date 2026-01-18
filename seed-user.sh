#!/bin/bash
# Quick script to seed the database with test user

cd "/mnt/c/Users/Klaus/Documents/Mestrado CA/go-data-storage"
export PATH="/usr/local/go/bin:$PATH"

echo "=== Running Database Seed Script ==="
echo "This will create test user: test@example.com / password123"
echo ""

go run scripts/seed.go
