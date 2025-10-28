#!/usr/bin/env bash
set -euo pipefail

# Run end-to-end local sanity: reset DB, scaffold schema, and create
# a few sample rows for each table, then list them.

alias rbc='go run main.go'

command -v rbc >/dev/null 2>&1 || { echo "error: rbc not found (go run main.go)" >&2; exit 1; }

echo "[1/7] Resetting database (destructive)" >&2
rbc admin db reset --force

echo "[2/7] Scaffolding roles, database, privileges, schema, and content index" >&2
rbc admin db scaffold --all --yes

echo "[3/7] Creating sample workflows and tasks" >&2
rbc admin workflow set --name ci-test --title "Continuous Integration: Test Suite" --description "Runs unit and integration tests." --notes "CI test workflow"
rbc admin workflow set --name ci-lint --title "Continuous Integration: Lint & Format" --description "Lints and vets the codebase." --notes "CI lint workflow"

rbc admin task set --workflow ci-test --command unit --variant go --version 1.0.0 \
  --title "Run Unit Tests" --description "Executes unit tests." --shell bash --run "go test ./..." --timeout "10 minutes" --tags unit,fast --level h2
rbc admin task set --workflow ci-test --command integration --variant "" --version 1.0.0 \
  --title "Run Integration Tests" --description "Runs integration tests." --shell bash --run "docker compose up -d && go test -tags=integration ./..." --timeout "30 minutes" --tags integration,slow --level h2
rbc admin task set --workflow ci-lint --command lint --variant go --version 1.0.0 \
  --title "Lint & Vet" --description "Runs vet and lints." --shell bash --run "go vet ./... && golangci-lint run" --timeout "5 minutes" --tags lint,style --level h2

echo "[4/7] Creating sample conversations and experiments" >&2
cjson=$(rbc admin conversation set --title "Build System Refresh" --project "github.com/acme/build-system" --tags pipeline,build,ci --description "Modernize build tooling." --notes "Goals: faster CI, better DX")
cid=$(printf "%s" "$cjson" | grep -m1 '"id"' | sed -E 's/[^0-9]*([0-9]+).*/\1/')

cjson2=$(rbc admin conversation set --title "Onboarding Improvement" --project "github.com/acme/product" --tags onboarding,docs,dx --description "Improve onboarding artifacts." --notes "Scope: docs, templates, scripts")
cid2=$(printf "%s" "$cjson2" | grep -m1 '"id"' | sed -E 's/[^0-9]*([0-9]+).*/\1/')

ejson1=$(rbc admin experiment create --conversation "$cid")
eid1=$(printf "%s" "$ejson1" | grep -m1 '"id"' | sed -E 's/[^0-9]*([0-9]+).*/\1/')

ejson2=$(rbc admin experiment create --conversation "$cid2")
eid2=$(printf "%s" "$ejson2" | grep -m1 '"id"' | sed -E 's/[^0-9]*([0-9]+).*/\1/')

echo "[5/8] Creating roles" >&2
rbc admin role set --name user --title "User" --description "Regular end-user role" --tags default
rbc admin role set --name qa   --title "QA"   --description "Quality assurance role" --tags testing

echo "[6/8] Starring tasks per role" >&2
rbc admin star set --role user --variant unit/go --version 1.0.0
rbc admin star set --role qa   --variant integration --version 1.0.0
rbc admin star set --role user --variant lint/go --version 1.0.0

echo "[7/8] Creating sample messages" >&2
echo "Hello from user12" | rbc admin message set --executor user12 --experiment "$eid1" --title "Greeting" --tags hello
echo "Build started" | rbc admin message set --executor build-bot --experiment "$eid1" --title "BuildStart" --tags build
echo "Onboarding checklist updated" | rbc admin message set --executor docs-bot --experiment "$eid2" --title "DocsUpdate" --tags docs,update

echo "[8/8] Listing all entities and counts" >&2
echo "-- Workflows --" >&2
rbc admin workflow list --limit 50
echo "-- Tasks --" >&2
rbc admin task list --limit 50
echo "-- Conversations --" >&2
rbc admin conversation list --limit 50
echo "-- Experiments --" >&2
rbc admin experiment list --limit 50
echo "-- Messages --" >&2
rbc admin message list --limit 50
echo "-- Table counts --" >&2
rbc admin db count --json

echo "Done." >&2
