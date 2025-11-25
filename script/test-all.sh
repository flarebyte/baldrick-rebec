#!/usr/bin/env bash
set -euo pipefail

# End-to-end local sanity: reset DB, scaffold schema, create sample rows, then list them.

alias rbc='go run main.go'

command -v rbc >/dev/null 2>&1 || { echo "error: rbc not found (go run main.go)" >&2; exit 1; }

# Use only rbctest* roles throughout this script
TEST_ROLE_USER="rbctest-user"
TEST_ROLE_QA="rbctest-qa"

# Helper to extract UUID id from JSON output without relying on jq
json_get_id() {
  if command -v jq >/dev/null 2>&1; then
    printf "%s" "$1" | jq -r '.id'
  else
    printf "%s" "$1" | grep -oE '"id"\s*:\s*"[0-9a-fA-F-]{36}"' | sed -E 's/.*"([0-9a-fA-F-]{36})".*/\1/'
  fi
}

has_jq() { command -v jq >/dev/null 2>&1; }

# Minimal assertion for relations (SQL-backed)
assert_relations_out() {
  local sid="$1"
  echo "[TEST] Checking relations for stickie $sid (out)" >&2
  local rel_json
  rel_json=$(rbc admin stickie-rel list --id "$sid" --direction out --output json 2>/dev/null || true)
  if has_jq; then
    local n
    n=$(printf "%s" "$rel_json" | jq 'length')
    if [ "${n:-0}" -lt 1 ]; then
      echo "[TEST][FAIL] Expected at least 1 relation from $sid; got $n" >&2
      echo "[TEST] Debug: relations JSON:" >&2
      echo "$rel_json" >&2
      echo "[TEST] Debug: counts:" >&2
      rbc admin db count --json >&2 || true
      exit 1
    fi
  else
    if printf "%s" "$rel_json" | grep -q '^[[:space:]]*\[\][[:space:]]*$'; then
      echo "[TEST][WARN] Could not verify relations (jq not installed); observed empty list" >&2
    fi
  fi
}

# Record a testcase
tc() {
  local title="$1"; shift
  local line="$1"; shift || true
  rbc admin testcase create --role "$TEST_ROLE_USER" --title "$title" --status OK --file "$0" --line "${line:-0}" --tags example,script=test-all >/dev/null
}

echo "[1/11] Resetting database (destructive)" >&2
rbc admin db reset --force --drop-app-role=false

echo "[2/11] Scaffolding roles, database, privileges, schema, and content index" >&2
rbc admin db scaffold --all --yes
tc "db scaffold --all --yes" "$LINENO"

echo "[3/11] Creating sample workflows and tasks" >&2
rbc admin workflow set --name ci-test --title "Continuous Integration: Test Suite" --description "Runs unit and integration tests." --notes "CI test workflow"
rbc admin workflow set --name ci-lint --title "Continuous Integration: Lint & Format" --description "Lints and vets the codebase." --notes "CI lint workflow"
tc "workflow set ci-test" "$LINENO"; tc "workflow set ci-lint" "$LINENO"

# Create scripts and capture their ids
sid_unit_json=$(printf "go test ./...\n" | rbc admin script set --role "$TEST_ROLE_USER" --title "Unit: go test" --description "Run unit tests")
sid_unit=$(json_get_id "$sid_unit_json")
sid_integ_json=$(printf "docker compose up -d && go test -tags=integration ./...\n" | rbc admin script set --role "$TEST_ROLE_USER" --title "Integration: compose+test" --description "Run integration tests")
sid_integ=$(json_get_id "$sid_integ_json")
sid_lint_json=$(printf "go vet ./... && golangci-lint run\n" | rbc admin script set --role "$TEST_ROLE_USER" --title "Lint & Vet" --description "Runs vet and lints")
sid_lint=$(json_get_id "$sid_lint_json")
tc "script set Unit: go test" "$LINENO"; tc "script set Integration: compose+test" "$LINENO"; tc "script set Lint & Vet" "$LINENO"

t_unit_json=$(rbc admin task set --workflow ci-test --command unit --variant go \
  --title "Run Unit Tests" --description "Executes unit tests." --shell bash --run-script "$sid_unit" --timeout "10 minutes" --tags unit,fast --level h2)
t_unit_id=$(json_get_id "$t_unit_json")
tc "task set ci-test unit/go" "$LINENO"

t_integ_json=$(rbc admin task set --workflow ci-test --command integration --variant "" \
  --title "Run Integration Tests" --description "Runs integration tests." --shell bash --run-script "$sid_integ" --timeout "30 minutes" --tags integration,slow --level h2)
t_integ_id=$(json_get_id "$t_integ_json")
tc "task set ci-test integration" "$LINENO"

t_lint_json=$(rbc admin task set --workflow ci-lint --command lint --variant go \
  --title "Lint & Vet" --description "Runs vet and lints." --shell bash --run-script "$sid_lint" --timeout "5 minutes" --tags lint,style --level h2)
t_lint_id=$(json_get_id "$t_lint_json")
tc "task set ci-lint lint/go" "$LINENO"

# Create replacement tasks with --replaces and levels
rbc admin task set --workflow ci-test --command unit --variant go-patch1 \
  --title "Run Unit Tests (Quick)" --description "Patch: run quick subset" --shell bash --run-script "$sid_unit" \
  --replaces "$t_unit_id" --replace-level patch --replace-comment "Flaky test workaround"

rbc admin task set --workflow ci-test --command unit --variant go-minor1 \
  --title "Run Unit Tests (Race)" --description "Minor: enable race detector" --shell bash --run-script "$sid_integ" \
  --replaces "$t_unit_id" --replace-level minor --replace-comment "Add -race"

rbc admin task set --workflow ci-lint --command lint --variant go-major1 \
  --title "Lint & Vet (Strict)" --description "Major: stricter lint rules" --shell bash --run-script "$sid_lint" \
  --replaces "$t_lint_id" --replace-level major --replace-comment "Enable all linters"

echo "-- Task relations (latest/next) --" >&2
echo "Latest by variant (unit/go):" >&2
rbc admin task latest --variant unit/go || true
echo "Latest from id (unit base):" >&2
rbc admin task latest --from-id "$t_unit_id" || true
echo "Next patch after unit base:" >&2
rbc admin task next --id "$t_unit_id" --level patch || true
echo "Next minor after unit base:" >&2
rbc admin task next --id "$t_unit_id" --level minor || true
echo "Next major after lint base:" >&2
rbc admin task next --id "$t_lint_id" --level major || true

echo "[4/11] Creating sample conversations and experiments" >&2
cjson=$(rbc admin conversation set --title "Build System Refresh" --project "github.com/acme/build-system" --tags pipeline,build,ci --description "Modernize build tooling." --notes "Goals: faster CI, better DX")
cid=$(json_get_id "$cjson")
tc "conversation set Build System Refresh" "$LINENO"

cjson2=$(rbc admin conversation set --title "Onboarding Improvement" --project "github.com/acme/product" --tags onboarding,docs,dx --description "Improve onboarding artifacts." --notes "Scope: docs, templates, scripts")
cid2=$(json_get_id "$cjson2")
tc "conversation set Onboarding Improvement" "$LINENO"

ejson1=$(rbc admin experiment create --conversation "$cid")
eid1=$(json_get_id "$ejson1")
tc "experiment create for conversation $cid" "$LINENO"

ejson2=$(rbc admin experiment create --conversation "$cid2")
eid2=$(json_get_id "$ejson2")
tc "experiment create for conversation $cid2" "$LINENO"

echo "[5/11] Creating roles" >&2
rbc admin role set --name "$TEST_ROLE_USER" --title "User" --description "Regular end-user role" --tags default
rbc admin role set --name "$TEST_ROLE_QA"   --title "QA"   --description "Quality assurance role" --tags testing
tc "role set $TEST_ROLE_USER" "$LINENO"; tc "role set $TEST_ROLE_QA" "$LINENO"

echo "[6/11] Creating tags" >&2
rbc admin tag set --name status  --title "Status"  --description "Common values: draft, active, archived"
rbc admin tag set --name type    --title "Type"    --description "Common values: unit, integration"
rbc admin tag set --name project --title "Project" --description "Example values: ci, website, product"
tc "tag set status" "$LINENO"; tc "tag set type" "$LINENO"; tc "tag set project" "$LINENO"

echo "[6.5/11] Creating topics" >&2
rbc admin topic set --name onboarding --role "$TEST_ROLE_USER" --title "Onboarding" --description "Docs and environment setup" --tags area=docs,priority=med
rbc admin topic set --name devops     --role "$TEST_ROLE_USER" --title "DevOps"    --description "Build, deploy, CI/CD"     --tags area=platform,priority=high
tc "topic set onboarding" "$LINENO"; tc "topic set devops" "$LINENO"

echo "[7/11] Creating projects" >&2
rbc admin project set --name acme/build-system --role "$TEST_ROLE_USER" --description "Build system and CI pipeline" --tags status=active,type=ci
rbc admin project set --name acme/product      --role "$TEST_ROLE_USER" --description "Main product" --tags status=active,type=app
tc "project set acme/build-system" "$LINENO"; tc "project set acme/product" "$LINENO"

echo "[8/11] Creating stores" >&2
rbc admin store set --name ideas-acme-build --role "$TEST_ROLE_USER" --title "Ideas for acme/build-system" --description "Idea backlog" --type journal --scope project --lifecycle monthly --tags topic=ideas,project=acme/build-system
rbc admin store set --name blackboard-global --role "$TEST_ROLE_USER" --title "Shared Blackboard" --description "Scratch space for team" --type blackboard --scope shared --lifecycle weekly --tags visibility=team
tc "store set ideas-acme-build" "$LINENO"; tc "store set blackboard-global" "$LINENO"

echo "[8.1/11] Creating blackboards" >&2
s1_json=$(rbc admin store get --name ideas-acme-build --role "$TEST_ROLE_USER")
s1=$(json_get_id "$s1_json")
s2_json=$(rbc admin store get --name blackboard-global --role "$TEST_ROLE_USER")
s2=$(json_get_id "$s2_json")
bb1_json=$(rbc admin blackboard set --role "$TEST_ROLE_USER" --store-id "$s1" --project acme/build-system --conversation "$cid" \
  --background "Ideas board for build system" --guidelines "Keep concise; tag items with priority")
bb1=$(json_get_id "$bb1_json")
bb2_json=$(rbc admin blackboard set --role "$TEST_ROLE_USER" --store-id "$s2" \
  --background "Team-wide blackboard" --guidelines "Wipe weekly on Mondays")
bb2=$(json_get_id "$bb2_json")
tc "blackboard set for ideas-acme-build ($bb1)" "$LINENO"; tc "blackboard set for blackboard-global ($bb2)" "$LINENO"

echo "[8.2/11] Creating stickies" >&2
st1_json=$(rbc admin stickie set --blackboard "$bb1" \
  --topic-name "onboarding" --topic-role "$TEST_ROLE_USER" \
  --note "Refresh onboarding guide for new hires" --labels onboarding,docs,priority:med --priority should)
st1=$(json_get_id "$st1_json")
st2_json=$(rbc admin stickie set --blackboard "$bb1" \
  --topic-name "devops" --topic-role "$TEST_ROLE_USER" \
  --note "Evaluate GitHub Actions caching for go build" \
  --labels idea,devops --priority could)
st2=$(json_get_id "$st2_json")
st3_json=$(rbc admin stickie set --blackboard "$bb2" \
  --note "Team retro every Friday" --labels team,ritual --priority must)
st3=$(json_get_id "$st3_json")
tc "stickie set onboarding ($st1)" "$LINENO"; tc "stickie set devops ($st2)" "$LINENO"; tc "stickie set team ritual ($st3)" "$LINENO"

echo "[8.3/11] Creating stickie relationships" >&2
rbc admin stickie-rel set --from "$st1" --to "$st2" --type uses --labels ref,dependency
tc "stickie-rel set uses (st1 -> st2)" "$LINENO"
rbc admin stickie-rel set --from "$st2" --to "$st3" --type includes --labels backlog
tc "stickie-rel set includes (st2 -> st3)" "$LINENO"
rbc admin stickie-rel set --from "$st1" --to "$st3" --type contrasts_with --labels tradeoff
tc "stickie-rel set contrasts_with (st1 -> st3)" "$LINENO"

assert_relations_out "$st1"

echo "[8/11] Creating workspaces" >&2
rbc admin workspace set --role "$TEST_ROLE_USER" --project acme/build-system \
  --description "Local build-system workspace" --tags status=active
rbc admin workspace set --role "$TEST_ROLE_USER" --project acme/product \
  --description "Local product workspace" --tags status=active
tc "workspace set for acme/build-system" "$LINENO"; tc "workspace set for acme/product" "$LINENO"

echo "[9/12] Creating packages (role-bound tasks)" >&2
rbc admin package set --role "$TEST_ROLE_USER" --variant unit/go
rbc admin package set --role "$TEST_ROLE_QA"   --variant integration
rbc admin package set --role "$TEST_ROLE_USER" --variant lint/go
tc "package set $TEST_ROLE_USER unit/go" "$LINENO"; tc "package set $TEST_ROLE_QA integration" "$LINENO"; tc "package set $TEST_ROLE_USER lint/go" "$LINENO"

echo "[10/12] Creating scripts" >&2
printf "#!/usr/bin/env bash\nset -euo pipefail\necho Deploying service...\n" | \
  rbc admin script set --role "$TEST_ROLE_USER" --title "Deploy Service" --description "Simple deploy script" --tags status=active,type=deploy
printf "#!/usr/bin/env bash\nset -euo pipefail\necho Cleaning build artifacts...\n" | \
  rbc admin script set --role "$TEST_ROLE_USER" --title "Cleanup Artifacts" --description "Cleanup build artifacts" --tags status=active,type=maintenance
tc "script set Deploy Service" "$LINENO"; tc "script set Cleanup Artifacts" "$LINENO"

echo "[11/13] Creating sample messages" >&2
echo "Hello from user12" | rbc admin message set --experiment "$eid1" --title "Greeting" --tags hello
echo "Build started" | rbc admin message set --experiment "$eid1" --title "BuildStart" --tags build
echo "Onboarding checklist updated" | rbc admin message set --experiment "$eid2" --title "DocsUpdate" --tags docs,update
tc "message set Greeting" "$LINENO"; tc "message set BuildStart" "$LINENO"; tc "message set DocsUpdate" "$LINENO"

echo "[12/13] Creating queues" >&2
qid1_json=$(rbc admin queue add --description "Run quick unit subset" --status Waiting --why "waiting for CI window" --tags kind=test,priority=low)
qid1=$(json_get_id "$qid1_json")
qid2_json=$(rbc admin queue add --description "Run full integration suite" --status Buildable --tags kind=test,priority=high)
qid2=$(json_get_id "$qid2_json")
qid3_json=$(rbc admin queue add --description "Strict lint pass" --status Blocked --why "env not ready" --tags kind=lint)
qid3=$(json_get_id "$qid3_json")

echo "-- Queue: peek oldest two --" >&2
rbc admin queue peek --limit 2
tc "queue peek --limit 2" "$LINENO"
echo "-- Queue: size (all) --" >&2
rbc admin queue size
tc "queue size" "$LINENO"
echo "-- Queue: take one --" >&2
rbc admin queue take --id "$qid1"
tc "queue take $qid1" "$LINENO"

echo "[13/13] Listing all entities and counts" >&2
echo "-- Workflows --" >&2
rbc admin workflow list --role "$TEST_ROLE_USER" --limit 50 || true
echo "-- Tasks --" >&2
rbc admin task list --role "$TEST_ROLE_USER" --limit 50 || true
echo "-- Conversations --" >&2
rbc admin conversation list --role "$TEST_ROLE_USER" --limit 50 || true
echo "-- Experiments --" >&2
rbc admin experiment list --limit 50
echo "-- Messages --" >&2
rbc admin message list --role "$TEST_ROLE_USER" --limit 50 || true
echo "-- Projects --" >&2
rbc admin project list --role "$TEST_ROLE_USER" --limit 50 || true
echo "-- Workspaces --" >&2
rbc admin workspace list --role "$TEST_ROLE_USER" --limit 50 || true
echo "-- Scripts --" >&2
rbc admin script list --role "$TEST_ROLE_USER" --limit 50 || true
echo "-- Stores --" >&2
rbc admin store list --role "$TEST_ROLE_USER" --limit 50 || true
echo "-- Topics --" >&2
rbc admin topic list --role "$TEST_ROLE_USER" --limit 50 || true
echo "-- Blackboards --" >&2
rbc admin blackboard list --role "$TEST_ROLE_USER" --limit 50 || true
echo "-- Stickies (all) --" >&2
rbc admin stickie list --limit 50
echo "-- Stickies for ideas-acme-build board --" >&2
rbc admin stickie list --blackboard "$bb1" --limit 50
echo "-- Stickies with topic=devops --" >&2
rbc admin stickie list --topic-name devops --topic-role "$TEST_ROLE_USER" --limit 50 || true
echo "-- Stickie relations (out from st1) --" >&2
rbc admin stickie-rel list --id "$st1" --direction out
echo "-- Stickie relation get (st1 uses st2) --" >&2
rbc admin stickie-rel get --from "$st1" --to "$st2" --type uses --ignore-missing
echo "-- Tags --" >&2
rbc admin tag list --role "$TEST_ROLE_USER" --limit 50 || true
echo "-- Table counts --" >&2
rbc admin db count --json

echo "Done." >&2
