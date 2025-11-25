.PHONY: test test-coverage lint fmt vet build run clean

# Run all tests (unit tests locally)
test:
	go test -v ./...

# Run tests with Docker (unit tests)
test-docker:
	./scripts/test.ps1

# Run integration tests with Docker (requires database)
test-integration:
	./scripts/test.ps1 --integration

# Run tests with coverage using Docker
test-coverage-docker:
	./scripts/test.ps1 --coverage

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with coverage percentage
test-coverage-html:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Run linter
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...
	goimports -w .

# Run go vet
vet:
	go vet ./...

# Build the application
build:
	go build -o bin/main ./cmd/api

# Run the application
run:
	go run ./cmd/api

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Install dependencies
deps:
	go mod download
	go mod tidy

# Install development tools
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

# Run all quality checks
check: fmt vet lint test

# Seed database with test data
seed:
	go run scripts/seed.go

