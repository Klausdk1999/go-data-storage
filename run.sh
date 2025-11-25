#!/bin/bash

# IoT System Startup Script
# Usage: ./run.sh [--seed|--no-seed] [--rebuild|--no-rebuild]
#   --no-seed: Start all services without seeding (default)
#   --seed: Start all services and seed the database
#   --rebuild: Force rebuild of Docker images (default)
#   --no-rebuild: Use existing images without rebuilding

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default to no seed and rebuild
SEED_DB=false
REBUILD=true

# Parse arguments
for arg in "$@"; do
    case $arg in
        --seed)
            SEED_DB=true
            shift
            ;;
        --no-seed)
            SEED_DB=false
            shift
            ;;
        --rebuild)
            REBUILD=true
            shift
            ;;
        --no-rebuild)
            REBUILD=false
            shift
            ;;
        *)
            if [[ "$arg" != "" ]]; then
                echo -e "${RED}Error: Invalid argument '$arg'${NC}"
                echo "Usage: $0 [--seed|--no-seed] [--rebuild|--no-rebuild]"
                exit 1
            fi
            ;;
    esac
done

echo -e "${BLUE}=================================================${NC}"
echo -e "${BLUE}  IoT System Startup Script${NC}"
echo -e "${BLUE}=================================================${NC}"
echo ""

# Get script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
COMPOSE_FILE="$SCRIPT_DIR/infra/docker-compose.full.yml"

# Check if docker-compose file exists
if [ ! -f "$COMPOSE_FILE" ]; then
    echo -e "${RED}Error: docker-compose.full.yml not found at $COMPOSE_FILE${NC}"
    exit 1
fi

# Function to stop all services
stop_services() {
    echo -e "${YELLOW}Stopping all services...${NC}"
    
    # Stop containers
    docker ps -q --filter "name=iot-" | xargs -r docker stop 2>/dev/null || true
    
    # Remove containers
    docker ps -aq --filter "name=iot-" | xargs -r docker rm 2>/dev/null || true
    
    # Stop docker-compose services
    cd "$SCRIPT_DIR/infra"
    docker-compose -f docker-compose.full.yml down -v 2>/dev/null || true
    
    echo -e "${GREEN}✓ All services stopped${NC}"
    echo ""
}

# Function to start services
start_services() {
    echo -e "${YELLOW}Starting all services...${NC}"
    
    cd "$SCRIPT_DIR/infra"
    
    # Start postgres first
    echo -e "${BLUE}Starting PostgreSQL...${NC}"
    docker-compose -f docker-compose.full.yml up -d postgres
    
    # Wait for postgres to be ready
    echo -e "${BLUE}Waiting for database to be ready...${NC}"
    max_attempts=30
    attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if docker exec iot-postgres pg_isready -U ${DB_USER:-iotuser} > /dev/null 2>&1; then
            echo -e "${GREEN}✓ Database is ready${NC}"
            break
        fi
        attempt=$((attempt + 1))
        sleep 1
    done
    
    if [ $attempt -eq $max_attempts ]; then
        echo -e "${RED}✗ Database failed to start${NC}"
        exit 1
    fi
    
    # Build and start API
    if [ "$REBUILD" = true ]; then
        echo -e "${BLUE}Building API...${NC}"
        docker-compose -f docker-compose.full.yml build api
    fi
    echo -e "${BLUE}Starting API...${NC}"
    docker-compose -f docker-compose.full.yml up -d api
    
    # Wait for API to be ready
    echo -e "${BLUE}Waiting for API to be ready...${NC}"
    max_attempts=30
    attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if curl -s http://localhost:8080/auth/login > /dev/null 2>&1 || [ $? -eq 0 ]; then
            sleep 2  # Give API a moment to fully start
            echo -e "${GREEN}✓ API is ready${NC}"
            break
        fi
        attempt=$((attempt + 1))
        sleep 1
    done
    
    # Build and start Frontend
    if [ "$REBUILD" = true ]; then
        echo -e "${BLUE}Building Frontend...${NC}"
        docker-compose -f docker-compose.full.yml build frontend
    fi
    echo -e "${BLUE}Starting Frontend...${NC}"
    docker-compose -f docker-compose.full.yml up -d frontend
    
    echo -e "${GREEN}✓ All services started${NC}"
    echo ""
}

# Function to seed database
seed_database() {
    echo -e "${YELLOW}Seeding database...${NC}"
    
    # Wait a bit more for API to be fully ready
    sleep 3
    
    cd "$SCRIPT_DIR"
    
    # Set environment variables for local connection (since DB is exposed on localhost)
    DB_HOST_VAL=localhost
    DB_PORT_VAL=5432
    DB_USER_VAL=${DB_USER:-iotuser}
    DB_PASSWORD_VAL=${DB_PASSWORD:-iotpassword}
    DB_NAME_VAL=${DB_NAME:-iotdb}
    
    # Export environment variables so they're available to the Go process
    export DB_HOST="$DB_HOST_VAL"
    export DB_PORT="$DB_PORT_VAL"
    export DB_USER="$DB_USER_VAL"
    export DB_PASSWORD="$DB_PASSWORD_VAL"
    export DB_NAME="$DB_NAME_VAL"
    
    # Check if Go is available in PATH
    if command -v go &> /dev/null; then
        # Run seed script locally (connects to localhost:5432)
        echo -e "${BLUE}Running seed script with local Go installation...${NC}"
        go run scripts/seed.go
    else
        # Try to find Go in common locations (Windows)
        GO_BIN=""
        if [ -f "/c/Program Files/Go/bin/go.exe" ]; then
            GO_BIN="/c/Program Files/Go/bin/go.exe"
        elif [ -f "/mnt/c/Program Files/Go/bin/go.exe" ]; then
            GO_BIN="/mnt/c/Program Files/Go/bin/go.exe"
        elif [ -f "$HOME/go/bin/go" ]; then
            GO_BIN="$HOME/go/bin/go"
        fi
        
        if [ -n "$GO_BIN" ] && [ -x "$GO_BIN" ]; then
            echo -e "${BLUE}Found Go at: $GO_BIN${NC}"
            # Create temporary .env file for seed script to read
            # This works better than passing env vars to Windows executables from WSL
            TEMP_ENV=$(mktemp)
            cat > "$TEMP_ENV" << EOF
DB_HOST=$DB_HOST_VAL
DB_PORT=$DB_PORT_VAL
DB_USER=$DB_USER_VAL
DB_PASSWORD=$DB_PASSWORD_VAL
DB_NAME=$DB_NAME_VAL
EOF
            # Copy to .env in the project directory temporarily
            if [ -f ".env" ]; then
                cp .env .env.backup 2>/dev/null || true
            fi
            cp "$TEMP_ENV" .env
            rm -f "$TEMP_ENV"
            
            # Run seed script
            "$GO_BIN" run scripts/seed.go
            SEED_EXIT_CODE=$?
            
            # Restore original .env if it existed
            if [ -f ".env.backup" ]; then
                mv .env.backup .env
            else
                rm -f .env
            fi
            
            if [ $SEED_EXIT_CODE -ne 0 ]; then
                echo -e "${RED}Failed to seed database${NC}"
                return 1
            fi
        else
            # Use temporary Go Docker container to run seed script
            echo -e "${YELLOW}Go not found in PATH. Using temporary Go Docker container...${NC}"
            
            # Check if we can connect to the database from host
            if docker exec iot-postgres pg_isready -U ${DB_USER:-iotuser} > /dev/null 2>&1; then
                # Determine the correct host for database connection
                # On Windows/WSL, use host.docker.internal; on Linux, use host network or gateway
                DB_HOST_FOR_DOCKER="host.docker.internal"
                if [ "$(uname -s)" = "Linux" ] && [ -z "$WSL_DISTRO_NAME" ]; then
                    # Native Linux - try host network first, fallback to gateway
                    DB_HOST_FOR_DOCKER="172.17.0.1"
                fi
                
                # Try to get the actual gateway IP if host.docker.internal doesn't work
                GATEWAY_IP=$(docker network inspect iot-network --format '{{range .IPAM.Config}}{{.Gateway}}{{end}}' 2>/dev/null || echo "")
                
                # Run seed script in temporary Go container
                # Connect to postgres container via Docker network
                docker run --rm \
                    --network iot-network \
                    -v "${PWD}:/app" \
                    -w /app \
                    -e DB_HOST=postgres \
                    -e DB_PORT=5432 \
                    -e DB_USER="$DB_USER_VAL" \
                    -e DB_PASSWORD="$DB_PASSWORD_VAL" \
                    -e DB_NAME="$DB_NAME_VAL" \
                    golang:1.23-alpine \
                    sh -c "go mod download && go run scripts/seed.go" || {
                    echo -e "${RED}Failed to seed database using Docker container${NC}"
                    echo -e "${YELLOW}Please install Go locally and run:${NC}"
                    echo -e "${BLUE}  go run scripts/seed.go${NC}"
                    return
                }
            else
                echo -e "${RED}Database is not accessible. Cannot seed database.${NC}"
                return
            fi
        fi
    fi
    
    echo -e "${GREEN}✓ Database seeded${NC}"
    echo ""
}

    # Main execution
    echo -e "${BLUE}Mode: ${SEED_DB:+With seed}${SEED_DB:-Without seed}${NC}"
    echo -e "${BLUE}Rebuild: ${REBUILD:+Yes}${REBUILD:-No}${NC}"
    echo ""

# Stop existing services
stop_services

# Start services
start_services

# Seed database if requested
if [ "$SEED_DB" = true ]; then
    seed_database
fi

# Show status
echo -e "${BLUE}=================================================${NC}"
echo -e "${GREEN}All services are running!${NC}"
echo -e "${BLUE}=================================================${NC}"
echo ""
echo -e "${GREEN}Services:${NC}"
echo -e "  • Database:  ${GREEN}localhost:5432${NC}"
echo -e "  • API:       ${GREEN}http://localhost:8080${NC}"
echo -e "  • Frontend:  ${GREEN}http://localhost:3000${NC}"
echo ""

if [ "$SEED_DB" = true ]; then
    echo -e "${YELLOW}Test Credentials:${NC}"
    echo -e "  • Email:    ${BLUE}test@example.com${NC}"
    echo -e "  • Password: ${BLUE}password123${NC}"
    echo ""
fi

echo -e "${YELLOW}To view logs:${NC}"
echo -e "  ${BLUE}cd infra && docker-compose -f docker-compose.full.yml logs -f${NC}"
echo ""
echo -e "${YELLOW}To stop all services:${NC}"
echo -e "  ${BLUE}cd infra && docker-compose -f docker-compose.full.yml down${NC}"
echo ""

