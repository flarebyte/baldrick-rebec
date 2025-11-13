#!/usr/bin/env bash

set -e

RED="\033[0;31m"
GREEN="\033[0;32m"
YELLOW="\033[1;33m"
NC="\033[0m"

echo -e "${YELLOW}=== Resetting containers and upgrading Postgres if needed ===${NC}"

############################################
# 1. Stop everything
############################################
echo -e "${YELLOW}Stopping containers...${NC}"
podman-compose down || true


############################################
# 2. Identify Postgres volume
############################################
echo -e "${YELLOW}Finding Postgres volume...${NC}"
PG_VOLUME=$(podman volume ls --format "{{.Name}}" | grep postgres-data || true)

if [ -z "$PG_VOLUME" ]; then
    echo -e "${GREEN}No old Postgres volume found — nothing to delete.${NC}"
else
    echo -e "${YELLOW}Deleting Postgres volume: ${PG_VOLUME}${NC}"
    podman volume rm "$PG_VOLUME"
fi


############################################
# 3. Rebuild all images without cache
############################################
echo -e "${YELLOW}Rebuilding all images (no cache)...${NC}"
podman-compose build --no-cache


############################################
# 4. Start everything again
############################################
echo -e "${YELLOW}Bringing services up...${NC}"
podman-compose up -d


############################################
# 5. Wait for Postgres to start
############################################
echo -e "${YELLOW}Waiting for Postgres to become ready...${NC}"

ATTEMPTS=0
MAX_ATTEMPTS=20

until podman exec postgres pg_isready > /dev/null 2>&1; do
    ATTEMPTS=$((ATTEMPTS + 1))
    if [ "$ATTEMPTS" -ge "$MAX_ATTEMPTS" ]; then
        echo -e "${RED}Postgres did not become ready in time.${NC}"
        exit 1
    fi
    sleep 1
done


############################################
# 6. Confirm upgraded Postgres version
############################################
echo -e "${YELLOW}Checking Postgres version...${NC}"
PG_VERSION=$(podman exec postgres psql -U "${POSTGRES_USER:-postgres}" -tAc "SHOW server_version;" 2>/dev/null || echo "unknown")

echo -e "${GREEN}Postgres is now running version: ${PG_VERSION}${NC}"

echo -e "${GREEN}=== Reset complete! ===${NC}"
