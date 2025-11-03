#!/usr/bin/env bash
set -euo pipefail

alias rbc='go run main.go'

# Helper to extract UUID id from JSON output without relying on jq
json_get_id() {
  if command -v jq >/dev/null 2>&1; then
    printf "%s" "$1" | jq -r '.id'
  else
    printf "%s" "$1" | grep -oE '"id"\s*:\s*"[0-9a-fA-F-]{36}"' | sed -E 's/.*"([0-9a-fA-F-]{36})".*/\1/'
  fi
}

# Example: populate example tasks under CI workflows created by workflow.sh
#
# Prereqs:
# - Run doc/examples/workflow.sh first to create 'ci-test' and 'ci-lint' workflows
# - 'rbc' binary on PATH

command -v rbc >/dev/null 2>&1 || { echo "error: rbc not found" >&2; exit 1; }

echo "Creating sample tasks under ci-test and ci-lint..." >&2

# Create scripts for each task and capture their ids
sid_unit_json=$(printf "go test ./...\n" | rbc admin script set --role user --title "Unit: go test" --description "Run unit tests")
sid_unit=$(json_get_id "$sid_unit_json")
sid_integ_json=$(printf "docker compose up -d && go test -tags=integration ./...\n" | rbc admin script set --role user --title "Integration: compose+test" --description "Run integration tests")
sid_integ=$(json_get_id "$sid_integ_json")
sid_lint_json=$(printf "go vet ./... && golangci-lint run\n" | rbc admin script set --role user --title "Lint & Vet" --description "Runs vet and lints")
sid_lint=$(json_get_id "$sid_lint_json")

# ci-test: unit tests task
rbc admin task set \
  --workflow ci-test \
  --command unit \
  --variant go \
  --version 1.0.0 \
  --title "Run Unit Tests" \
  --description "Executes unit tests across all packages." \
  --shell bash \
  --run-script "$sid_unit" \
  --timeout "10 minutes" \
  --tags unit,fast \
  --level h2

# ci-test: integration tests task
rbc admin task set \
  --workflow ci-test \
  --command integration \
  --variant "" \
  --version 1.0.0 \
  --title "Run Integration Tests" \
  --description "Brings up services and runs integration tests." \
  --shell bash \
  --run-script "$sid_integ" \
  --timeout "30 minutes" \
  --tags integration,slow \
  --level h2

# ci-lint: formatting / linting
rbc admin task set \
  --workflow ci-lint \
  --command lint \
  --variant go \
  --version 1.0.0 \
  --title "Lint & Vet" \
  --description "Runs vet and lints the codebase." \
  --shell bash \
  --run-script "$sid_lint" \
  --timeout "5 minutes" \
  --tags lint,style \
  --level h2

echo "Done." >&2
