#!/usr/bin/env bash
set -euo pipefail

set -a
source .env
set +a

echo "======================================="
echo " Checking services (strict mode)"
echo "======================================="


##############################################
# Required environment variables (ALL of them)
##############################################

REQUIRED_ENV=(
  POSTGRES_USER
  POSTGRES_PASSWORD
  POSTGRES_DB
  INFLUXDB_INTERNAL_PORT
  GRAFANA_INTERNAL_PORT
)

echo ""
echo "Validating required environment variables..."

MISSING=()

for VAR in "${REQUIRED_ENV[@]}"; do
    if [ -z "${!VAR:-}" ]; then
        MISSING+=("$VAR")
    fi
done

if [ "${#MISSING[@]}" -ne 0 ]; then
    echo "ERROR: The following environment variables are NOT defined:"
    for VAR in "${MISSING[@]}"; do
        echo "  - $VAR"
    done
    exit 1
fi

echo "All required environment variables are defined."


##############################################
# PostgreSQL
##############################################
echo ""
echo "---------------------------------------"
echo " PostgreSQL"
echo "---------------------------------------"

echo "Command: podman exec postgres pg_isready"
podman exec postgres pg_isready || {
    echo "PostgreSQL readiness check FAILED"
    exit 1
}

echo ""
echo "Command: podman exec postgres psql -U \"$POSTGRES_USER\" -c \"SHOW server_version;\""
podman exec postgres psql -U "$POSTGRES_USER" -c "SHOW server_version;" || {
    echo "PostgreSQL version query FAILED"
    exit 1
}


##############################################
# InfluxDB
##############################################
echo ""
echo "---------------------------------------"
echo " InfluxDB"
echo "---------------------------------------"

echo "Command: podman exec influxdb curl http://localhost:${INFLUXDB_INTERNAL_PORT}/ready"
podman exec influxdb curl "http://localhost:${INFLUXDB_INTERNAL_PORT}/ready" || {
    echo "InfluxDB readiness check FAILED"
    exit 1
}

echo ""
echo "Command: podman exec influxdb curl http://localhost:${INFLUXDB_INTERNAL_PORT}/health"
podman exec influxdb curl "http://localhost:${INFLUXDB_INTERNAL_PORT}/health" || {
    echo "InfluxDB health check FAILED"
    exit 1
}


##############################################
# Grafana
##############################################
echo ""
echo "---------------------------------------"
echo " Grafana"
echo "---------------------------------------"

echo "Command: podman exec grafana curl http://localhost:${GRAFANA_INTERNAL_PORT}/api/health"
podman exec grafana curl "http://localhost:${GRAFANA_INTERNAL_PORT}/api/health" || {
    echo "Grafana health check FAILED"
    exit 1
}


##############################################
# Success
##############################################
echo ""
echo "======================================="
echo " All services healthy"
echo "======================================="
exit 0
