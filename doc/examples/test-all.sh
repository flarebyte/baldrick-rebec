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

echo "[1/11] Resetting database (destructive)" >&2
rbc admin db reset --force

echo "[2/11] Scaffolding roles, database, privileges, schema, and content index" >&2
rbc admin db scaffold --all --yes

echo "[3/11] Creating sample workflows and tasks" >&2
rbc admin workflow set --name ci-test --title "Continuous Integration: Test Suite" --description "Runs unit and integration tests." --notes "CI test workflow"
rbc admin workflow set --name ci-lint --title "Continuous Integration: Lint & Format" --description "Lints and vets the codebase." --notes "CI lint workflow"

# Create scripts for tasks and capture their ids
sid_unit_json=$(printf "go test ./...\n" | rbc admin script set --role user --title "Unit: go test" --description "Run unit tests")
sid_unit=$(json_get_id "$sid_unit_json")
sid_integ_json=$(printf "docker compose up -d && go test -tags=integration ./...\n" | rbc admin script set --role user --title "Integration: compose+test" --description "Run integration tests")
sid_integ=$(json_get_id "$sid_integ_json")
sid_lint_json=$(printf "go vet ./... && golangci-lint run\n" | rbc admin script set --role user --title "Lint & Vet" --description "Runs vet and lints")
sid_lint=$(json_get_id "$sid_lint_json")

t_unit_json=$(rbc admin task set --workflow ci-test --command unit --variant go \
  --title "Run Unit Tests" --description "Executes unit tests." --shell bash --run-script "$sid_unit" --timeout "10 minutes" --tags unit,fast --level h2)
t_unit_id=$(json_get_id "$t_unit_json")

t_integ_json=$(rbc admin task set --workflow ci-test --command integration --variant "" \
  --title "Run Integration Tests" --description "Runs integration tests." --shell bash --run-script "$sid_integ" --timeout "30 minutes" --tags integration,slow --level h2)
t_integ_id=$(json_get_id "$t_integ_json")

t_lint_json=$(rbc admin task set --workflow ci-lint --command lint --variant go \
  --title "Lint & Vet" --description "Runs vet and lints." --shell bash --run-script "$sid_lint" --timeout "5 minutes" --tags lint,style --level h2)
t_lint_id=$(json_get_id "$t_lint_json")

# Add examples of patch/minor/major replacements using the graph
# Create updated scripts for replacements
sid_unit_patch_json=$(printf "go test ./... -run Quick\n" | rbc admin script set --role user --title "Unit: quick subset" --description "Patch: quick tests")
sid_unit_patch=$(json_get_id "$sid_unit_patch_json")
sid_unit_minor_json=$(printf "go test ./... -race\n" | rbc admin script set --role user --title "Unit: race" --description "Minor: enable -race")
sid_unit_minor=$(json_get_id "$sid_unit_minor_json")
sid_lint_major_json=$(printf "golangci-lint run --enable-all\n" | rbc admin script set --role user --title "Lint: strict" --description "Major: stricter lint")
sid_lint_major=$(json_get_id "$sid_lint_major_json")

# Create replacement tasks with --replaces and levels
rbc admin task set --workflow ci-test --command unit --variant go-patch1 \
  --title "Run Unit Tests (Quick)" --description "Patch: run quick subset" --shell bash --run-script "$sid_unit_patch" \
  --replaces "$t_unit_id" --replace-level patch --replace-comment "Flaky test workaround"

# Minor replaces the original unit test task (or could replace patch)
rbc admin task set --workflow ci-test --command unit --variant go-minor1 \
  --title "Run Unit Tests (Race)" --description "Minor: enable race detector" --shell bash --run-script "$sid_unit_minor" \
  --replaces "$t_unit_id" --replace-level minor --replace-comment "Add -race"

# Major replacement for lint task
rbc admin task set --workflow ci-lint --command lint --variant go-major1 \
  --title "Lint & Vet (Strict)" --description "Major: stricter lint rules" --shell bash --run-script "$sid_lint_major" \
  --replaces "$t_lint_id" --replace-level major --replace-comment "Enable all linters"

echo "[4/11] Creating sample conversations and experiments" >&2
cjson=$(rbc admin conversation set --title "Build System Refresh" --project "github.com/acme/build-system" --tags pipeline,build,ci --description "Modernize build tooling." --notes "Goals: faster CI, better DX")
cid=$(json_get_id "$cjson")

cjson2=$(rbc admin conversation set --title "Onboarding Improvement" --project "github.com/acme/product" --tags onboarding,docs,dx --description "Improve onboarding artifacts." --notes "Scope: docs, templates, scripts")
cid2=$(json_get_id "$cjson2")

ejson1=$(rbc admin experiment create --conversation "$cid")
eid1=$(json_get_id "$ejson1")

ejson2=$(rbc admin experiment create --conversation "$cid2")
eid2=$(json_get_id "$ejson2")

echo "[5/11] Creating roles" >&2
rbc admin role set --name user --title "User" --description "Regular end-user role" --tags default
rbc admin role set --name qa   --title "QA"   --description "Quality assurance role" --tags testing

echo "[6/11] Creating tags" >&2
rbc admin tag set --name status  --title "Status"  --description "Common values: draft, active, archived"
rbc admin tag set --name type    --title "Type"    --description "Common values: unit, integration"
rbc admin tag set --name project --title "Project" --description "Example values: ci, website, product"

echo "[7/11] Creating projects" >&2
rbc admin project set --name acme/build-system --role user --description "Build system and CI pipeline" --tags status=active,type=ci
rbc admin project set --name acme/product      --role user --description "Main product" --tags status=active,type=app

echo "[8/11] Creating workspaces" >&2
rbc admin workspace set --role user --project acme/build-system \
  --description "Local build-system workspace" --tags status=active
rbc admin workspace set --role user --project acme/product \
  --description "Local product workspace" --tags status=active

echo "[9/12] Creating packages (role-bound tasks)" >&2
rbc admin package set --role user --variant unit/go
rbc admin package set --role qa   --variant integration
rbc admin package set --role user --variant lint/go

echo "[10/12] Creating scripts" >&2
printf "#!/usr/bin/env bash\nset -euo pipefail\necho Deploying service...\n" | \
  rbc admin script set --role user --title "Deploy Service" --description "Simple deploy script" --tags status=active,type=deploy
printf "#!/usr/bin/env bash\nset -euo pipefail\necho Cleaning build artifacts...\n" | \
  rbc admin script set --role user --title "Cleanup Artifacts" --description "Cleanup build artifacts" --tags status=active,type=maintenance

echo "[11/12] Creating sample messages" >&2
echo "Hello from user12" | rbc admin message set --executor user12 --experiment "$eid1" --title "Greeting" --tags hello
echo "Build started" | rbc admin message set --executor build-bot --experiment "$eid1" --title "BuildStart" --tags build
echo "Onboarding checklist updated" | rbc admin message set --executor docs-bot --experiment "$eid2" --title "DocsUpdate" --tags docs,update

echo "[12/12] Listing all entities and counts" >&2
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
echo "-- Projects --" >&2
rbc admin project list --role user --limit 50
echo "-- Workspaces --" >&2
rbc admin workspace list --role user --limit 50
echo "-- Scripts --" >&2
rbc admin script list --role user --limit 50
echo "-- Tags --" >&2
rbc admin tag list --role user --limit 50
echo "-- Table counts --" >&2
rbc admin db count --json

echo "Done." >&2
