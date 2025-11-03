#!/usr/bin/env bash
set -euo pipefail

# Run end-to-end local sanity: reset DB, scaffold schema, and create
# a few sample rows for each table, then list them.

alias rbc='go run main.go'

command -v rbc >/dev/null 2>&1 || { echo "error: rbc not found (go run main.go)" >&2; exit 1; }

# Helper to extract UUID id from JSON output without relying on jq
json_get_id() {
  if command -v jq >/dev/null 2>&1; then
    printf "%s" "$1" | jq -r '.id'
  else
    # Fallback: grep a UUID-looking value from the id field
    printf "%s" "$1" | grep -oE '"id"\s*:\s*"[0-9a-fA-F-]{36}"' | sed -E 's/.*"([0-9a-fA-F-]{36})".*/\1/'
  fi
}

echo "[1/9] Resetting database (destructive)" >&2
rbc admin db reset --force

echo "[2/9] Scaffolding roles, database, privileges, schema, and content index" >&2
rbc admin db scaffold --all --yes

echo "[3/9] Creating sample workflows and tasks" >&2
rbc admin workflow set --name ci-test --title "Continuous Integration: Test Suite" --description "Runs unit and integration tests." --notes "CI test workflow"
rbc admin workflow set --name ci-lint --title "Continuous Integration: Lint & Format" --description "Lints and vets the codebase." --notes "CI lint workflow"

rbc admin task set --workflow ci-test --command unit --variant go --version 1.0.0 \
  --title "Run Unit Tests" --description "Executes unit tests." --shell bash --run "go test ./..." --timeout "10 minutes" --tags unit,fast --level h2
rbc admin task set --workflow ci-test --command integration --variant "" --version 1.0.0 \
  --title "Run Integration Tests" --description "Runs integration tests." --shell bash --run "docker compose up -d && go test -tags=integration ./..." --timeout "30 minutes" --tags integration,slow --level h2
rbc admin task set --workflow ci-lint --command lint --variant go --version 1.0.0 \
  --title "Lint & Vet" --description "Runs vet and lints." --shell bash --run "go vet ./... && golangci-lint run" --timeout "5 minutes" --tags lint,style --level h2

echo "[4/9] Creating sample conversations and experiments" >&2
cjson=$(rbc admin conversation set --title "Build System Refresh" --project "github.com/acme/build-system" --tags pipeline,build,ci --description "Modernize build tooling." --notes "Goals: faster CI, better DX")
cid=$(json_get_id "$cjson")

cjson2=$(rbc admin conversation set --title "Onboarding Improvement" --project "github.com/acme/product" --tags onboarding,docs,dx --description "Improve onboarding artifacts." --notes "Scope: docs, templates, scripts")
cid2=$(json_get_id "$cjson2")

ejson1=$(rbc admin experiment create --conversation "$cid")
eid1=$(json_get_id "$ejson1")

ejson2=$(rbc admin experiment create --conversation "$cid2")
eid2=$(json_get_id "$ejson2")

echo "[5/9] Creating roles" >&2
rbc admin role set --name user --title "User" --description "Regular end-user role" --tags default
rbc admin role set --name qa   --title "QA"   --description "Quality assurance role" --tags testing

echo "[6/9] Creating tags" >&2
rbc admin tag set --name status  --title "Status"  --description "Common values: draft, active, archived"
rbc admin tag set --name type    --title "Type"    --description "Common values: unit, integration"
rbc admin tag set --name project --title "Project" --description "Example values: ci, website, product"

echo "[7/9] Creating packages (role-bound tasks)" >&2
rbc admin package set --role user --variant unit/go --version 1.0.0
rbc admin package set --role qa   --variant integration --version 1.0.0
rbc admin package set --role user --variant lint/go --version 1.0.0

echo "[8/9] Creating sample messages" >&2
echo "Hello from user12" | rbc admin message set --executor user12 --experiment "$eid1" --title "Greeting" --tags hello
echo "Build started" | rbc admin message set --executor build-bot --experiment "$eid1" --title "BuildStart" --tags build
echo "Onboarding checklist updated" | rbc admin message set --executor docs-bot --experiment "$eid2" --title "DocsUpdate" --tags docs,update

echo "[9/9] Listing all entities and counts" >&2
echo "-- Workflows --" >&2
rbc admin workflow list --role user --limit 50
echo "-- Tasks --" >&2
rbc admin task list --role user --limit 50
echo "-- Conversations --" >&2
rbc admin conversation list --role user --limit 50
echo "-- Experiments --" >&2
rbc admin experiment list --limit 50
echo "-- Messages --" >&2
rbc admin message list --role user --limit 50
echo "-- Tags --" >&2
rbc admin tag list --role user --limit 50
echo "-- Table counts --" >&2
rbc admin db count --json

echo "Done." >&2
