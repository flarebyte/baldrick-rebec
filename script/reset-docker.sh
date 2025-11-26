#!/usr/bin/env bash

set -e

RED="\033[0;31m"
GREEN="\033[0;32m"
YELLOW="\033[1;33m"
NC="\033[0m"

echo -e "${YELLOW}=== Resetting Podman Compose environment (Postgres + InfluxDB + Grafana) ===${NC}"

############################################
# 1. Stop containers
############################################
echo -e "${YELLOW}Stopping containers...${NC}"
podman-compose down || true


############################################
# 2. Find and remove relevant volumes
############################################
echo -e "${YELLOW}Searching for volumes to delete...${NC}"

PG_VOLUME=$(podman volume ls --format "{{.Name}}" | grep postgres-data || true)
INFLUX_VOLUME=$(podman volume ls --format "{{.Name}}" | grep influxdb-data || true)
GRAFANA_VOLUME=$(podman volume ls --format "{{.Name}}" | grep grafana-data || true)

delete_volume() {
    local V="$1"
    if [ -z "$V" ]; then
        echo -e "${GREEN}No volume found for: $2 (skipping)${NC}"
    else
        echo -e "${YELLOW}Deleting $2 volume: $V${NC}"
        podman volume rm "$V"
    fi
}

delete_volume "$PG_VOLUME" "PostgreSQL"
delete_volume "$INFLUX_VOLUME" "InfluxDB"
delete_volume "$GRAFANA_VOLUME" "Grafana"


############################################
# 3. Rebuild all images without cache
############################################
echo -e "${YELLOW}Rebuilding images (no cache)...${NC}"
podman-compose build --no-cache


############################################
# 4. Bring services back up
############################################
echo -e "${YELLOW}Starting services...${NC}"
podman-compose up -d


############################################
# 5. Wait for PostgreSQL
############################################
echo -e "${YELLOW}Waiting for PostgreSQL...${NC}"
for i in {1..20}; do
    if podman exec postgres pg_isready >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

if ! podman exec postgres pg_isready >/dev/null 2>&1; then
    echo -e "${RED}Postgres did not become ready.${NC}"
    exit 1
fi

PG_VERSION=$(podman exec postgres psql -U "${POSTGRES_USER:-postgres}" -tAc "SHOW server_version;" 2>/dev/null | tr -d ' ')
echo -e "${GREEN}PostgreSQL OK - version: ${PG_VERSION}${NC}"


############################################
# 6. Check InfluxDB readiness
############################################
echo -n "InfluxDB: "
if podman exec influxdb curl -sf "http://localhost:${INFLUXDB_INTERNAL_PORT:-8086}/ready" >/dev/null 2>&1; then
    echo -e "${GREEN}OK${NC}"
else
    echo -e "${RED}FAILED${NC}"
    exit 1
fi


############################################
# 7. Check Grafana health
############################################
echo -n "Grafana: "
if podman exec grafana curl -sf "http://localhost:${GRAFANA_INTERNAL_PORT:-3000}/api/health" >/dev/null 2>&1; then
    echo -e "${GREEN}OK${NC}"
else
    echo -e "${RED}FAILED${NC}"
    exit 1
fi


############################################
# 8. Done
############################################
echo -e "${GREEN}\nAll services fully reset and healthy!${NC}"
exit 0
