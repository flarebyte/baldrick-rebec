#!/usr/bin/env bash
set -euo pipefail

alias rbc='go run main.go'

# Example: populate example tasks under CI workflows created by workflow.sh
#
# Prereqs:
# - Run doc/examples/workflow.sh first to create 'ci-test' and 'ci-lint' workflows
# - 'rbc' binary on PATH

command -v rbc >/dev/null 2>&1 || { echo "error: rbc not found" >&2; exit 1; }

echo "Creating sample tasks under ci-test and ci-lint..." >&2

# ci-test: unit tests task
rbc admin task set \
  --workflow ci-test \
  --name unit \
  --version 1.0.0 \
  --title "Run Unit Tests" \
  --description "Executes unit tests across all packages." \
  --shell bash \
  --run "go test ./..." \
  --timeout "10 minutes" \
  --tags unit,fast \
  --level h2

# ci-test: integration tests task
rbc admin task set \
  --workflow ci-test \
  --name integration \
  --version 1.0.0 \
  --title "Run Integration Tests" \
  --description "Brings up services and runs integration tests." \
  --shell bash \
  --run "docker compose up -d && go test -tags=integration ./..." \
  --timeout "30 minutes" \
  --tags integration,slow \
  --level h2

# ci-lint: formatting / linting
rbc admin task set \
  --workflow ci-lint \
  --name lint \
  --version 1.0.0 \
  --title "Lint & Vet" \
  --description "Runs vet and lints the codebase." \
  --shell bash \
  --run "go vet ./... && golangci-lint run" \
  --timeout "5 minutes" \
  --tags lint,style \
  --level h2

echo "Done." >&2

