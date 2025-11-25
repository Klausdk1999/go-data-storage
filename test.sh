#!/bin/bash

# IoT System Test Script
# Usage: ./test.sh [--integration] [--coverage]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default options
TEST_TYPE="unit"
WITH_COVERAGE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --integration)
            TEST_TYPE="integration"
            shift
            ;;
        --coverage)
            WITH_COVERAGE=true
            shift
            ;;
        --help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --integration    Run integration tests (requires database)"
            echo "  --coverage       Generate test coverage report"
            echo "  --help           Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                      # Run unit tests"
            echo "  $0 --integration       # Run integration tests"
            echo "  $0 --coverage          # Run tests with coverage"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

echo -e "${BLUE}=================================================${NC}"
echo -e "${BLUE}  IoT System Test Script${NC}"
echo -e "${BLUE}=================================================${NC}"
echo ""

# Get script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# Check if Docker is running
if ! docker ps > /dev/null 2>&1; then
    echo -e "${RED}Error: Docker is not running. Please start Docker.${NC}"
    exit 1
fi

# Start test database if integration tests
if [ "$TEST_TYPE" = "integration" ]; then
    echo -e "${YELLOW}Starting test database...${NC}"
    
    cd infra
    docker-compose -f docker-compose.test.yml up -d postgres-test
    cd ..
    
    # Wait for database to be ready
    echo -e "${BLUE}Waiting for test database to be ready...${NC}"
    max_attempts=30
    attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if docker exec iot-postgres-test pg_isready -U iotuser > /dev/null 2>&1; then
            echo -e "${GREEN}✓ Test database is ready${NC}"
            break
        fi
        attempt=$((attempt + 1))
        sleep 1
    done
    
    if [ $attempt -eq $max_attempts ]; then
        echo -e "${RED}✗ Test database failed to start${NC}"
        cd infra
        docker-compose -f docker-compose.test.yml down
        cd ..
        exit 1
    fi
fi

# Build test image
echo -e "${YELLOW}Building test image...${NC}"
docker build -f Dockerfile.test -t iot-api-test:latest .

if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Failed to build test image${NC}"
    if [ "$TEST_TYPE" = "integration" ]; then
        cd infra
        docker-compose -f docker-compose.test.yml down
        cd ..
    fi
    exit 1
fi

# Prepare test command
if [ "$WITH_COVERAGE" = true ]; then
    TEST_CMD="go test -v -coverprofile=coverage/coverage.out ./... && go tool cover -html=coverage/coverage.out -o coverage/coverage.html"
else
    TEST_CMD="go test -v ./..."
fi

# Create coverage directory if needed
if [ "$WITH_COVERAGE" = true ]; then
    mkdir -p coverage
fi

# Run tests in container
echo -e "${YELLOW}Running tests...${NC}"
echo ""

TEST_EXIT_CODE=0

if [ "$TEST_TYPE" = "integration" ]; then
    # Run tests with database connection
    if [ "$WITH_COVERAGE" = true ]; then
        docker run --rm \
            --network iot-test-network \
            -e DB_HOST=postgres-test \
            -e DB_PORT=5432 \
            -e DB_USER=iotuser \
            -e DB_PASSWORD=iotpassword \
            -e DB_NAME=iotdb_test \
            -v "${PWD}/coverage:/app/coverage" \
            -w /app \
            iot-api-test:latest sh -c "$TEST_CMD"
        TEST_EXIT_CODE=$?
    else
        docker run --rm \
            --network iot-test-network \
            -e DB_HOST=postgres-test \
            -e DB_PORT=5432 \
            -e DB_USER=iotuser \
            -e DB_PASSWORD=iotpassword \
            -e DB_NAME=iotdb_test \
            -w /app \
            iot-api-test:latest sh -c "$TEST_CMD"
        TEST_EXIT_CODE=$?
    fi
else
    # Run unit tests (no database needed)
    if [ "$WITH_COVERAGE" = true ]; then
        docker run --rm \
            -v "${PWD}/coverage:/app/coverage" \
            -w /app \
            iot-api-test:latest sh -c "$TEST_CMD"
        TEST_EXIT_CODE=$?
    else
        docker run --rm \
            -w /app \
            iot-api-test:latest sh -c "$TEST_CMD"
        TEST_EXIT_CODE=$?
    fi
fi

# Cleanup
if [ "$TEST_TYPE" = "integration" ]; then
    echo ""
    echo -e "${YELLOW}Stopping test database...${NC}"
    cd infra
    docker-compose -f docker-compose.test.yml down
    cd ..
fi

# Report results
echo ""
if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}=================================================${NC}"
    echo -e "${GREEN}All tests passed!${NC}"
    echo -e "${GREEN}=================================================${NC}"
    if [ "$WITH_COVERAGE" = true ]; then
        echo -e "${BLUE}Coverage report: coverage/coverage.html${NC}"
    fi
else
    echo -e "${RED}=================================================${NC}"
    echo -e "${RED}Tests failed with exit code: $TEST_EXIT_CODE${NC}"
    echo -e "${RED}=================================================${NC}"
fi

exit $TEST_EXIT_CODE

