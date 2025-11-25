#!/bin/bash

# IoT System Stop Script
# Usage: ./stop.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=================================================${NC}"
echo -e "${BLUE}  Stopping IoT System${NC}"
echo -e "${BLUE}=================================================${NC}"
echo ""

# Get script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# Stop all IoT containers
echo -e "${YELLOW}Stopping all containers...${NC}"

# Stop containers by name pattern
docker ps -q --filter "name=iot-" | xargs -r docker stop 2>/dev/null || true

# Remove containers
docker ps -aq --filter "name=iot-" | xargs -r docker rm 2>/dev/null || true

# Stop docker-compose services
echo -e "${YELLOW}Stopping docker-compose services...${NC}"

cd infra

# Stop full stack
if [ -f "docker-compose.full.yml" ]; then
    docker-compose -f docker-compose.full.yml down -v 2>/dev/null || true
fi

# Stop test database
if [ -f "docker-compose.test.yml" ]; then
    docker-compose -f docker-compose.test.yml down 2>/dev/null || true
fi

cd ..

echo ""
echo -e "${GREEN}✓ All services stopped${NC}"
echo ""

# Show remaining containers (if any)
REMAINING=$(docker ps -q --filter "name=iot-" 2>/dev/null || true)
if [ -n "$REMAINING" ]; then
    echo -e "${YELLOW}Note: Some containers may still be running:${NC}"
    docker ps --filter "name=iot-"
else
    echo -e "${GREEN}All IoT containers stopped successfully${NC}"
fi

echo ""

