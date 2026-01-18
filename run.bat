@echo off
REM Batch script to run go-data-storage API with CGO enabled
REM This is required for SQLite support

echo Starting go-data-storage API...
echo CGO_ENABLED=1 (required for SQLite)

set CGO_ENABLED=1

REM Check if .env exists
if not exist .env (
    echo Warning: .env file not found. Using default configuration.
)

REM Run the API
go run ./cmd/api
