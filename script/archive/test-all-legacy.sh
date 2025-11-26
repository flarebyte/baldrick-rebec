#!/usr/bin/env bash
# Archived legacy version of test-all.sh (Bash)
# This file was moved to preserve historical behavior.
# New implementation uses Google ZX in script/test-all.sh

set -euo pipefail

alias rbc='go run main.go'

command -v rbc >/dev/null 2>&1 || { echo "error: rbc not found (go run main.go)" >&2; exit 1; }

TEST_ROLE_USER="rbctest-user"
TEST_ROLE_QA="rbctest-qa"

json_get_id() {
  if command -v jq >/dev/null 2>&1; then
    printf "%s" "$1" | jq -r '.id'
  else
    printf "%s" "$1" | grep -oE '"id"\s*:\s*"[0-9a-fA-F-]{36}"' | sed -E 's/.*"([0-9a-fA-F-]{36})".*/\1/'
  fi
}

has_jq() { command -v jq >/dev/null 2>&1; }

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

tc() {
  local title="$1"; shift
  local line="$1"; shift || true
  rbc admin testcase create --role "$TEST_ROLE_USER" --title "$title" --status OK --file "$0" --line "${line:-0}" --tags example,script=test-all >/dev/null
}

echo "[1/11] Resetting database (destructive)" >&2
rbc admin db reset --force --drop-app-role=false

echo "[2/11] Scaffolding roles, database, privileges, schema, and content index" >&2
rbc admin db scaffold --all --yes

# The remainder of the legacy flow is preserved in repository history.
# Please use the ZX-based script/test-all.sh going forward.

