#!/usr/bin/env bash
set -euo pipefail

alias rbc='go run main.go'

# Example: populate example workflows for CI test and lint.
#
# Requirements:
# - 'rbc' binary on PATH
# - Config initialized (see README First-Time Setup)
# - Database scaffolded (rbc admin db scaffold --all --yes)
#
# This script is idempotent: 'rbc admin workflow set' performs an upsert.

command -v rbc >/dev/null 2>&1 || {
  echo "error: rbc not found on PATH" >&2
  exit 1
}

echo "Creating CI workflows (test, lint)..." >&2

# CI: Test workflow — runs unit/integration tests
rbc admin workflow set \
  --name ci-test \
  --title "Continuous Integration: Test Suite" \
  --description "Runs unit and integration tests with caching and parallelism." \
  --notes "Primary CI test workflow: executes 'go test ./...' with race detector in key packages; emits JUnit and coverage artifacts."

# CI: Lint workflow — formatting, vetting, and linting
rbc admin workflow set \
  --name ci-lint \
  --title "Continuous Integration: Lint & Format" \
  --description "Runs static analysis, formatting checks, and vetting." \
  --notes "Runs 'go vet', formatting checks, and golangci-lint (if configured); blocks on style violations."

echo "Done." >&2

