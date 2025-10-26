#!/usr/bin/env bash
set -euo pipefail

alias rbc='go run main.go'

# Example: create a couple of conversations for demos/tests.
#
# Prereqs:
# - 'rbc' binary on PATH
# - DB scaffolded: rbc admin db scaffold --all --yes

command -v rbc >/dev/null 2>&1 || { echo "error: rbc not found" >&2; exit 1; }

# Ensure schema exists (requires admin creds in config)
echo "Ensuring database schema (rbc admin db init)..." >&2
rbc admin db init >/dev/null

echo "Creating example conversations..." >&2

# Build system refresh initiative (id auto-generated)
rbc admin conversation set \
  --title "Build System Refresh" \
  --project "github.com/acme/build-system" \
  --tags pipeline,build,ci \
  --description "Consolidate and modernize build tooling across repos." \
  --notes "# Goals\n- Reduce CI times\n- Simplify developer onboarding\n- Standardize linters and test runners"

# Onboarding improvements (id auto-generated)
rbc admin conversation set \
  --title "Onboarding Improvement Initiative" \
  --project "github.com/acme/product" \
  --tags onboarding,docs,dx \
  --description "Streamline onboarding artifacts and developer experience." \
  --notes "# Scope\n- Revamp docs\n- Starter templates\n- Better local dev scripts"

echo "Done." >&2
