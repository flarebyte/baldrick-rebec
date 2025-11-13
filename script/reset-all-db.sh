#!/usr/bin/env bash
#
# Fully resets InfluxDB + Grafana + (optional) PostgreSQL
# Works with Podman Compose and prefixed volumes.
#

set -e

RED="\033[0;31m"
GREEN="\033[0;32m"
YELLOW="\033[1;33m"
NC="\033[0m"

RESET_POSTGRES=false

##############################################
# Parse optional flags
##############################################
for arg in "$@"; do
    case "$arg" in
        --reset-postgres)
            RESET_POSTGRES=true
            ;;
    esac
done

echo -e "${YELLOW}=== Resetting InfluxDB + Grafana (Postgres optional) ===${NC}"
echo -e "Using Podman Compose syntax (podman compose)"


##############################################
# 1. Stop containers
##############################################
echo -e "\n${YELLOW}Stopping Podman Compose stack...${NC}"
podman compose down || true


##############################################
# 2. Detect volumes with prefix
##############################################
echo -e "\n${YELLOW}Detecting real volume names...${NC}"

PG_VOLUME=$(podman volume ls --format "{{.Name}}" | grep "baldrick-rebec_postgres-data" || true)
INFLUX_VOLUME=$(podman volume ls --format "{{.Name}}" | grep "baldrick-rebec_influxdb-data" || true)
GRAFANA_VOLUME=$(podman volume ls --format "{{.Name}}" | grep "baldrick-rebec_grafana-data" || true)

echo -e "Postgres volume: ${PG_VOLUME:-<not found>}"
echo -e "InfluxDB volume: ${INFLUX_VOLUME:-<not found>}"
echo -e "Grafana volume: ${GRAFANA_VOLUME:-<not found>}"


##############################################
# volume deletion helper
##############################################
delete_volume() {
    VOL="$1"
    LABEL="$2"

    if [ -z "$VOL" ]; then
        echo -e "${GREEN}$LABEL: no volume found (skipping)${NC}"
        return
    fi

    echo -e "${YELLOW}Deleting $LABEL volume: $VOL${NC}"
    podman volume rm "$VOL"
}

##############################################
# 3. Delete InfluxDB + Grafana volumes
##############################################
delete_volume "$INFLUX_VOLUME"  "InfluxDB"
delete_volume "$GRAFANA_VOLUME" "Grafana"


##############################################
# 4. Optionally delete Postgres volume
##############################################
if [ "$RESET_POSTGRES" = true ]; then
    delete_volume "$PG_VOLUME" "PostgreSQL"
else
    echo -e "${YELLOW}PostgreSQL reset disabled (use --reset-postgres to enable)${NC}"
fi


##############################################
# 5. Rebuild images without cache
##############################################
echo -e "\n${YELLOW}Rebuilding images (no cache)...${NC}"
podman compose build --no-cache


##############################################
# 6. Start containers
##############################################
echo -e "\n${YELLOW}Starting services...${NC}"
podman compose up -d


##############################################
# 7. Wait for InfluxDB readiness
##############################################
echo -e "\n${YELLOW}Waiting for InfluxDB to become ready...${NC}"

ATT=0
MAX=30

until podman exec influxdb curl -sf "http://localhost:${INFLUXDB_INTERNAL_PORT:-8086}/ready" >/dev/null 2>&1; do
    ATT=$((ATT+1))
    if [ "$ATT" -ge "$MAX" ]; then
        echo -e "${RED}InfluxDB did not become ready.${NC}"
        exit 1
    fi
    sleep 1
done

echo -e "${GREEN}InfluxDB ready.${NC}"


##############################################
# 8. Validate InfluxDB token
##############################################
echo -e "${YELLOW}Validating InfluxDB token...${NC}"

if podman exec influxdb curl -sf \
  -H "Authorization: Token ${INFLUXDB_TOKEN}" \
  "http://localhost:${INFLUXDB_INTERNAL_PORT:-8086}/api/v2/me" >/dev/null 2>&1; then
    echo -e "${GREEN}InfluxDB token OK.${NC}"
else
    echo -e "${RED}InfluxDB token FAILED. Check INFLUXDB_TOKEN in .env.${NC}"
    exit 1
fi


##############################################
# 9. Check Grafana health
##############################################
echo -e "\n${YELLOW}Checking Grafana health...${NC}"

if podman exec grafana curl -sf \
  "http://localhost:${GRAFANA_INTERNAL_PORT:-3000}/api/health" >/dev/null 2>&1; then
    echo -e "${GREEN}Grafana is healthy.${NC}"
else
    echo -e "${RED}Grafana FAILED health check.${NC}"
    exit 1
fi


##############################################
# 10. Optional: check Postgres
##############################################
if [ "$RESET_POSTGRES" = true ]; then
    echo -e "\n${YELLOW}Checking PostgreSQL...${NC}"
    if podman exec postgres pg_isready >/dev/null 2>&1; then
        echo -e "${GREEN}PostgreSQL is ready.${NC}"
    else
        echo -e "${RED}PostgreSQL is not ready.${NC}"
        exit 1
    fi
fi


##############################################
# DONE
##############################################
echo -e "\n${GREEN}=== All services fully reset and healthy ===${NC}"
exit 0
