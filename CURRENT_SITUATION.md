# Current Situation: Graph Migration to SQL

This note bootstraps a new contributor to investigate the remaining graph issues quickly. It summarizes what is working, what is broken, and the instrumentation already added.

## Context
- We have migrated to a Postgres-only stack; graph features use SQL tables.
- Work is on branch `working-branch-2025-11` (created from main to avoid further direct pushes to main).
- The CLI end-to-end exerciser is `doc/examples/test-all.sh`.

## Environment & Setup
- Docker: Postgres service only. No AGE required.
- Session bootstrap: `SET search_path = "$user", public`.
- Graph init: not needed. Relations live in SQL tables (`task_replaces`, `stickie_relations`).
- Scaffold: `rbc admin db scaffold --all --yes` creates schema and re-grants runtime privileges post-schema.

## Current Symptoms
- test-all.sh still fails at the stickie relationship assertion:
  - "[TEST][FAIL] Expected at least 1 relation from <stickie-id>; got 0"
- Task graph queries (latest/next) previously failed with parser errors; now they include full cypher in error messages for diagnosis.
- AGE diagnostics removed; use `db count` and entity lists.

## What Works
- SQL graph tables power relationships: `task_replaces` (Task REPLACES) and `stickie_relations` (Stickie edges).
- CLI improvements:
  - All cypher calls are parameterized.
  - Rich error context for task graph commands and stickie-rel get (params echoed).
  - `db show` now includes a relationships summary (FKs vs graph) and supports `--output json`.
  - `age-status` includes search_path, operator checks, label/edge probes.

## What Fails / Unknowns
- With `graph.allow_fallback=false` (default), stickie-rel list shows 0 edges in the graph (even after set is called in test-all.sh).
- A previous error: "a name constant is expected" (fixed by emitting literal relationship types in Cypher), but test still shows 0 edges, hinting the write didn’t persist.
- Task latest/next previously failed with parser errors; code now prints the exact cypher if it still happens.

## Instrumentation Added
- Rich error messages remain for DAOs; graph errors are gone (SQL-only).

## Hypotheses
1) Previous AGE-specific issues are moot; graph is SQL-backed.
2) Ensure scripts remove any `age-init`/`age-status` steps.

## Next Actions (ordered)
1) Run test-all.sh with fallback disabled (default) and capture the first failing stickie-rel set call:
   - If it errors, the CLI now returns `cmd=stickie-rel set params={...}`. Use this to replicate minimal failing case.
2) Attempt a minimal manual graph write (no mirror):
   - `rbc admin stickie-rel set --from <st1> --to <st2> --type uses --labels ref`
   - Then `rbc admin stickie-rel list --id <st1> --direction out --output json`
3) If still 0 rows, add a temporary write-probe in age-status (optional):
   - Create a temp Stickie pair and INCLUDES edge; check it appears; then delete.
4) Inspect session role/privs at write time:
   - Confirm `current_user`, `search_path` during `stickie-rel set`.
5) If AGE continues to reject writes on this env, consider fallback-on for CI/regression while keeping a focused ticket to isolate the graph write behavior.

## Repro/Runbook
- Fresh run:
  - `docker compose up -d`
  - `rbc admin db scaffold --all --yes`
  - `sh doc/examples/test-all.sh`
- If failure:
  - Copy the failing command and error (now includes `cmd=... params=...`) into the issue.
- To enable mirror fallback:
  - `~/.baldrick-rebec/config.yaml`:
    ```yaml
    graph:
      allow_fallback: true
    ```

## Useful CLI
- Diagnostics:
  - `rbc admin db show --output json`
  - `rbc admin db count --json`
- Task graph:
  - `rbc admin task latest --variant unit/go`
  - `rbc admin task next --id <task-id> --level patch`
- Stickie relations (SQL):
  - `rbc admin stickie-rel set --from <a> --to <b> --type uses --labels x`
  - `rbc admin stickie-rel list --id <a> --direction out --output json`

## Risks / Alternatives
- We removed AGE. Graph features are implemented with SQL tables.

## Branch & Commit State
- Current work lives in `working-branch-2025-11`.
- Major recent changes: migrate graph to SQL (`task_replaces`, `stickie_relations`), remove AGE.

---
This document should equip a new agent to reproduce, observe, and triage graph failures quickly, with concrete commands and expected signals.
