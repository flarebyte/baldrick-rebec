# Current Situation: AGE Graph + CLI Regression Debug

This note bootstraps a new contributor to investigate the remaining graph issues quickly. It summarizes what is working, what is broken, and the instrumentation already added.

## Context
- We are migrating to a Postgres-only stack with optional Apache AGE for graph features (task replacements; stickie relationships).
- Work is on branch `working-branch-2025-11` (created from main to avoid further direct pushes to main).
- The CLI end-to-end exerciser is `doc/examples/test-all.sh`.

## Environment & Setup
- Docker: Postgres service, typically with AGE available. We now avoid custom images; AGE is handled at runtime (CREATE EXTENSION, age-init).
- Session bootstrap (on every DB connect):
  - `LOAD 'age'`
  - `SET search_path = "$user", public, ag_catalog`
- Graph init: `rbc admin db age-init --yes [--quiet]` creates graph, labels (Task, Stickie, REPLACES/INCLUDES/CAUSES/USES/REPRESENTS/CONTRASTS_WITH), and grants DML on rbc_graph objects to the app role.
- Scaffold: `rbc admin db scaffold --all --yes` creates schema and re-grants runtime privileges post-schema.

## Current Symptoms
- test-all.sh still fails at the stickie relationship assertion:
  - "[TEST][FAIL] Expected at least 1 relation from <stickie-id>; got 0"
- Task graph queries (latest/next) previously failed with parser errors; now they include full cypher in error messages for diagnosis.
- The graph probes in `age-status` currently report no rows (not an error) for MATCH patterns, implying the graph has no vertices/edges in the test run.

## What Works
- AGE extension loads; graph `rbc_graph` exists.
- Operators exist (agtype @>, graphid =); `cypher_usable=true`.
- AGE privileges on rbc_graph label tables are granted to the app role.
- SQL mirror for stickie relations (table `stickie_relations`) is available and integrated (behind a config flag).
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
- `rbc admin db age-status` (JSON):
  - postgres_version, age_version
  - extension_installed, graph_exists, cypher_usable
  - search_path, agtype_contains_operator, graphid_eq_operator
  - probe_match_any, probe_match_stickie, edge_probe per type (parameterized)
- Rich error messages in:
  - task latest/next: includes `cmd=... params={...}` and cypher
  - stickie-rel get: includes parameters on error
- Config flag: `graph.allow_fallback` (default false). When true, stickie-rel falls back to SQL mirror.

## Hypotheses
1) Graph writes are not persisting during test-all.sh (timing, role, or transaction scope?).
2) A subtle AGE planner quirk is still triggered in stickie set paths on this environment (despite literal type fix).
3) test-all.sh may be creating stickies but not calling stickie-rel set under the same conditions we expect (IDs, sequence, env), or fallback was implicitly used previously.

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
  - `rbc admin db age-init --yes --quiet`
  - `rbc admin db age-status` (confirm cypher_usable=true, edge probes ok or no rows)
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
  - `rbc admin db age-status`
  - `rbc admin db show --output json`
  - `rbc admin db count --json` (includes graph edge counts when cypher can run)
- Task graph:
  - `rbc admin task latest --variant unit/go`
  - `rbc admin task next --id <task-id> --level patch`
- Stickie graph (pure):
  - `rbc admin stickie-rel set --from <a> --to <b> --type uses --labels x`
  - `rbc admin stickie-rel list --id <a> --direction out --output json`

## Risks / Alternatives
- AGEs can differ slightly across environments; session bootstrap (LOAD, search_path) and explicit grants are essential.
- Mirror fallback ensures UX and data continuity even if AGE misbehaves; we keep it behind a config flag to surface issues by default.
- If AGE instability persists, consider swapping to SQL-only relationships and keeping the graph as optional enhancement.

## Branch & Commit State
- Current work lives in `working-branch-2025-11`.
- Major recent changes: parameterized Cypher, literal relationship types, improved error messages, age-status probes, age-init grants sequence, db show relationships.

---
This document should equip a new agent to reproduce, observe, and triage graph failures quickly, with concrete commands and expected signals.
