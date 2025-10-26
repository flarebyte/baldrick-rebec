#!/usr/bin/env bash
set -euo pipefail

alias rbc='go run main.go'

# Example: run a previously created task by variant+version
# Prereqs: run 01-workflow.sh and 02-task.sh first

command -v rbc >/dev/null 2>&1 || { echo "error: rbc not found" >&2; exit 1; }

echo "Running task unit/go@1.0.0..." >&2
rbc admin task run \
  --variant unit/go \
  --version 1.0.0 \
  --experiment 1 \
  --timeout 2m \
  --env FOO=bar

echo "Done." >&2

