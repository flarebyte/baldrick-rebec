#!/usr/bin/env bash

set -e

RED="\033[0;31m"
GREEN="\033[0;32m"
NC="\033[0m" # No Color

echo "Checking running services..."

############################################
# Check PostgreSQL
############################################
echo -n "PostgreSQL: "

if podman exec postgres pg_isready > /dev/null 2>&1; then
    # If pg_isready succeeded, get the actual server version
    VERSION=$(podman exec postgres psql -U "${POSTGRES_USER:-postgres}" -tAc "SELECT version();" 2>/dev/null | tr -d ' ')
    echo -e "${GREEN}OK${NC} - ${VERSION}"
else
    echo -e "${RED}FAILED${NC}"
    exit 1
fi


############################################
# Check InfluxDB
############################################
echo -n "InfluxDB: "

# InfluxDB readiness endpoint (returns 204 when OK)
if podman exec influxdb curl -sf "http://localhost:${INFLUXDB_INTERNAL_PORT:-8086}/ready" > /dev/null 2>&1; then
    echo -e "${GREEN}OK${NC}"
else
    echo -e "${RED}FAILED${NC}"
    exit 1
fi


############################################
# Check Grafana
############################################
echo -n "Grafana: "

if podman exec grafana curl -sf "http://localhost:${GRAFANA_INTERNAL_PORT:-3000}/api/health" > /dev/null 2>&1; then
    echo -e "${GREEN}OK${NC}"
else
    echo -e "${RED}FAILED${NC}"
    exit 1
fi


############################################
# All good!
############################################
echo -e "\n${GREEN}All services are healthy!${NC}"
exit 0
