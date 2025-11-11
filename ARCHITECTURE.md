# Architecture Overview

This codebase provides a CLI-first admin and data-manipulation workflow on top of PostgreSQL with optional Apache AGE for graph features. It consists of a thin command layer, a configuration layer, and a Postgres DAO layer (including graph helpers). A small gRPC server scaffold exists for future remote operation but is not required for current CLI flows.

## High-level Layers
- CLI (Cobra)
  - Commands in `cmd/admin/**` implement subcommands for each entity (roles, workflows, projects, workspaces, scripts, tasks, messages, queues, testcases, tags, topics, stores, blackboards, stickies) and for DB admin (scaffold/init/status/show/count/backup/restore/age-status/age-init) and graph helpers (task latest/next, stickie-rel).
  - Conventions: parse flags, load config, open DB connection, call DAO functions, print stderr summary and JSON to stdout.

- Config
  - `internal/config/config.go` loads `~/.baldrick-rebec/config.yaml`, merges with defaults, and exposes Postgres admin/app credentials and graph behavior flags:
    - `graph.allow_fallback` (default `false`) controls SQL mirror fallback for stickie-rel commands.

- Postgres DAO
  - `internal/dao/postgres/db.go` handles connection pool creation and session bootstrap:
    - `LOAD 'age'` (best-effort)
    - `SET search_path = "$user", public, ag_catalog` (public first, AGE operators visible)
  - `internal/dao/postgres/schema.go` creates/maintains relational schema, triggers, indexes, and best‑effort extension/graph setup.
  - Entity DAOs are in dedicated files (e.g., `projects.go`, `workspaces.go`, `scripts.go`, `tasks.go`, ...).
  - Graph helpers live in `graph.go` with AGE-specific logic, all using parameterized `ag_catalog.cypher($graph,$cypher,$params)`.
  - Stickie relations also have a SQL mirror (`stickie_relations` table) implemented in `stickie_relations.go` (used when fallback is allowed).

- Server (gRPC scaffold)
  - `internal/server/server.go` contains a minimal gRPC server wiring (port, PID, reload signals). There are no service definitions wired yet; the CLI talks directly to Postgres.

## Data Flow (CLI → DB)
1) User runs a command, e.g. `rbc admin project set --name X --role user --description ...`.
2) Cobra command parses flags and loads config (`internal/config`).
3) Command opens a DB connection via `OpenApp` (app role) or `OpenAdmin` (admin role). The AfterConnect hook configures the session (LOAD AGE; search_path).
4) Command calls a DAO function (e.g., `UpsertProject`) with validated inputs.
5) DAO executes parameterized SQL against Postgres, returning rows/ids.
6) Command prints a concise line on stderr (status) and a JSON object/array on stdout.

For graph operations (AGE):
- Commands build a cypher string and a params JSON map. DAOs call `ag_catalog.cypher($1,$2,$3)` with graph name, cypher, params, and parse results.
- All graph queries are parameterized and include enriched error messages on failure (cypher text for task graph finders and command context for stickie-rel).
- stickie-rel can optionally fall back to the SQL mirror (`graph.allow_fallback: true`), but defaults to pure-graph behavior so issues surface during regression.

## DB Admin Flows
- `db scaffold --all --yes`
  - Creates roles (if requested), database (if requested), and grants runtime privileges to the app role.
  - Ensures SQL schema and re-applies `GRANT` after `EnsureSchema` to cover newly created tables.
- `db age-init --yes [--quiet]`
  - Creates AGE extension/graph, creates required labels, and grants DML privileges on `rbc_graph` tables (and default privileges for future labels).
  - `--quiet` suppresses benign "already exists" notes.
- `db age-status`
  - Diagnostics: versions, search_path, operator presence, and match probes for vertices and edges (parameterized), emitting structured JSON.

## Error Reporting
- Graph-related commands provide context-rich errors to speed up triage:
  - Task graph finders return: `AGE cypher failed: <pg-error>` plus `cypher=<text>`.
  - CLI wrappers (task latest/next) add: `cmd=... params={...}` to the error.
  - stickie-rel get wraps errors with `cmd=stickie-rel get params={...}`.
- Most list/mutation commands provide a concise stderr summary and JSON payload on stdout.

## Data Model (selected)
- Relational tables: roles, workflows, projects, workspaces, scripts_content, scripts, task_variants, tasks, messages_content, messages, queues, testcases, conversations, experiments, tags, topics, stores, blackboards, stickies.
- Graph labels: `Task`, `Stickie` with edges `REPLACES` and `INCLUDES|CAUSES|USES|REPRESENTS|CONTRASTS_WITH`.
- SQL mirror: `stickie_relations(from_id,to_id,rel_type,labels)` to persist stickie relations when fallback is enabled.

## Protobuf / gRPC (current and future)
- Current state: the CLI talks directly to Postgres; there are no protobuf messages or service handlers in use today.
- Future direction (optional):
  - Define protobuf messages for each entity/action (e.g., ProjectSetRequest/Response).
  - Implement gRPC services (AdminService) in the server and call DAOs within handlers.
  - Update CLI to offer a `--remote` mode that performs the same actions via gRPC instead of direct DB access.
  - Benefits: remote execution, centralized auth/auditing, consistent validations.

## Test & Examples
- `doc/examples/test-all.sh` exercises a full setup scenario:
  - Resets DB, scaffolds, initializes AGE, creates entities (workflows, scripts, tasks, projects, workspaces, messages, queues, stores, topics, blackboards, stickies), and validates relationships.
  - Records each step as a testcase via the `testcases` table.
  - Contains guardrails and diagnostics (AGE readiness; graph relation assertions; counts).

## Key Design Choices
- Parameterized SQL/Cypher only — no query string interpolation with user data.
- Public schema first in `search_path` — ensures CREATE/INSERT targets public by default.
- AGE optional — robust diagnostics and a controlled mirror fallback for stickie relations.
- CLI emits JSON — suitable for automation and scripting; stderr for human-readable status.

This architecture supports both local CLI workflows and an incremental path toward remote execution via gRPC/protobuf if desired.
