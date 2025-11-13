#!/usr/bin/env bash
#
# Fully resets InfluxDB + Grafana (and optionally PostgreSQL)
# for Podman Compose environments.
#
# - Stops containers
# - Removes volumes (InfluxDB, Grafana, Postgres optional)
# - Rebuilds all images with --no-cache
# - Starts services
# - Waits for readiness
# - Validates InfluxDB admin token
# - Validates Grafana health

set -e

RED="\033[0;31m"
GREEN="\033[0;32m"
YELLOW="\033[1;33m"
NC="\033[0m"

RESET_POSTGRES=false

########################################
# Optional flag: --reset-postgres
########################################
for arg in "$@"; do
    case "$arg" in
        --reset-postgres)
            RESET_POSTGRES=true
            ;;
    esac
done

echo -e "${YELLOW}=== Resetting Podman Compose Environment ===${NC}"
echo -e "${YELLOW}InfluxDB + Grafana will be reset${NC}"
if [ "$RESET_POSTGRES" = true ]; then
    echo -e "${YELLOW}PostgreSQL reset ENABLED${NC}"
else
    echo -e "${YELLOW}PostgreSQL reset SKIPPED (use --reset-postgres to enable)${NC}"
fi


########################################
# 1. Stop stack
########################################
echo -e "\n${YELLOW}Stopping containers...${NC}"
podman compose down || true


########################################
# 2. Locate volumes
########################################
echo -e "\n${YELLOW}Locating volumes...${NC}"

PG_VOLUME=$(podman volume ls --format "{{.Name}}" | grep postgres-data || true)
INFLUX_VOLUME=$(podman volume ls --format "{{.Name}}" | grep influxdb-data || true)
GRAFANA_VOLUME=$(podman volume ls --format "{{.Name}}" | grep grafana-data || true)


delete_volume() {
    NAME=$1
    LABEL=$2

    if [ -z "$NAME" ]; then
        echo -e "${GREEN}No volume found for: $2 (skipping)${NC}"
        return
    fi

    echo -e "${YELLOW}Found $2 volume: $NAME${NC}"
    read -p "Delete this $2 volume? (y/N): " ANSWER
    case "$ANSWER" in
        y|Y|yes|YES)
            echo -e "${YELLOW}Deleting $2 volume...${NC}"
            podman volume rm "$NAME"
            ;;
        *)
            echo -e "${RED}Skipping deletion of $2 volume. Reset cannot apply fully.${NC}"
            ;;
    esac
}


########################################
# 3. Delete volumes (InfluxDB, Grafana, optional Postgres)
########################################
delete_volume "$INFLUX_VOLUME"  "InfluxDB"
delete_volume "$GRAFANA_VOLUME" "Grafana"

if [ "$RESET_POSTGRES" = true ]; then
    delete_volume "$PG_VOLUME" "PostgreSQL"
fi


########################################
# 4. Rebuild images without cache
########################################
echo -e "\n${YELLOW}Rebuilding images (no cache)...${NC}"
podman compose build --no-cache


########################################
# 5. Start the stack
########################################
echo -e "\n${YELLOW}Starting services...${NC}"
podman compose up -d


########################################
# 6. Wait for InfluxDB readiness
########################################
echo -e "\n${YELLOW}Waiting for InfluxDB...${NC}"

ATTEMPTS=0
MAX_ATTEMPTS=30
until podman exec influxdb curl -sf "http://localhost:${INFLUXDB_INTERNAL_PORT:-8086}/ready" >/dev/null 2>&1; do
    ATTEMPTS=$((ATTEMPTS+1))
    if [ "$ATTEMPTS" -ge "$MAX_ATTEMPTS" ]; then
        echo -e "${RED}InfluxDB did not become ready.${NC}"
        exit 1
    fi
    sleep 1
done

echo -e "${GREEN}InfluxDB is ready.${NC}"


########################################
# 7. Validate InfluxDB Admin Token
########################################
echo -e "${YELLOW}Validating InfluxDB token...${NC}"

if podman exec influxdb curl -sf -H "Authorization: Token ${INFLUXDB_TOKEN}" \
        http://localhost:${INFLUXDB_INTERNAL_PORT:-8086}/api/v2/me >/dev/null 2>&1; then
    echo -e "${GREEN}Admin token is valid!${NC}"
else
    echo -e "${RED}Admin token FAILED – check INFLUXDB_TOKEN in .env${NC}"
    exit 1
fi


########################################
# 8. Check Grafana health
########################################
echo -e "\n${YELLOW}Checking Grafana health...${NC}"

if podman exec grafana curl -sf \
        "http://localhost:${GRAFANA_INTERNAL_PORT:-3000}/api/health" >/dev/null 2>&1; then
    echo -e "${GREEN}Grafana is healthy.${NC}"
else
    echo -e "${RED}Grafana FAILED healthcheck.${NC}"
    exit 1
fi


########################################
# 9. (Optional) Check Postgres
########################################
if [ "$RESET_POSTGRES" = true ]; then
    echo -e "\n${YELLOW}Checking PostgreSQL...${NC}"
    if podman exec postgres pg_isready >/dev/null 2>&1; then
        echo -e "${GREEN}PostgreSQL is ready.${NC}"
    else
        echo -e "${RED}PostgreSQL is NOT ready.${NC}"
        exit 1
    fi
fi


########################################
# 10. Success
########################################
echo -e "\n${GREEN}=== Environment fully reset and healthy! ===${NC}"
