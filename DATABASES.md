# Databases

This document explains how the Baldrick‑Rebec CLI uses its databases (PostgreSQL and OpenSearch), what is managed by the CLI, and what typically requires manual setup. It also gives a practical setup guide for local development and production‑like environments.

## Components

1) PostgreSQL (relational)

- Purpose: Stores message ingest “events” and workflow/task definitions.
- Core tables created/managed by CLI:
  - `messages_events`: one row per ingest/processing event referencing stored content.
  - `workflows` and `tasks`: organize versioned execution units.
- Access patterns: inserts/reads, filtering by conversation/attempt; workflows/tasks for orchestration.

2) OpenSearch (search/vector)

- Purpose: Stores unique message content optimized for search and deduplication.
- Index name: `messages_content`
- Document ID: `SHA256(<canonicalized_message_body>)`
- Mapping/settings are ensured by the CLI; lifecycle (ILM) is typically managed by operators.

## What the CLI Manages

- `rbc config init` — Creates `~/.baldrick-rebec/config.yaml` with server/DB settings.
- `rbc db init` — Initializes data stores:
  - PostgreSQL: creates or updates core tables and a trigger to maintain `updated_at` on `message_profiles`.
  - OpenSearch: ensures the `messages_content` index with simplified mappings/settings exists.
- `rbc db status` — Connectivity + basic health checks for PostgreSQL and OpenSearch and presence of `messages_content`.

Notes:
- PostgreSQL schema changes are currently applied directly by `db init` (no versioned migrations yet). For production, consider adopting a migration tool later (e.g., `golang-migrate`).
- OpenSearch ILM policy is referenced (`messages-content-ilm`) but not installed automatically by the CLI. See “Operator Tasks” below.

## Provisioning Workflow (CLI-first)

PostgreSQL and OpenSearch provisioning is designed to be driven by the CLI.

- Configure global settings (server + DBs):
  - `rbc config init --overwrite [flags]`
  - Use `--dry-run` to preview changes without writing.

- Preview planned DB changes (no writes):
  - `rbc db plan`

- Create roles, database, grants, and schema (admin required):
  - `rbc db scaffold --create-roles --create-db --grant-privileges --yes`
  - Then ensure tables/triggers: `rbc db scaffold` (schema-only re-run is safe)

- Configure OpenSearch security + lifecycle for localhost:
  - `rbc os bootstrap` (removed in PG-only; previous guidance for OpenSearch)
  - This command tries ILM first; if ILM is not available, it falls back to ISM automatically.

- Initialize OpenSearch index and verify:
  - `rbc db init` (ensures `messages_content` index and Postgres schema)
  - `rbc db status` (reports index presence and lifecycle policy via ILM or ISM)

OpenSearch Lifecycle (ILM/ISM)

- OpenSearch often uses the Index State Management (ISM) plugin rather than Elasticsearch ILM. The CLI supports both:
  - ILM commands: `rbc os ilm ensure|show|list|delete`
  - ISM commands: `rbc os ism ensure|show|list|delete`
  - For secured local images, `os bootstrap` will configure https and ensure+attach ILM or ISM automatically.

- Example ILM policy (if ILM is available):

```json
PUT _ilm/policy/messages-content-ilm
{
  "policy": {
    "phases": {
      "hot":   {"actions": {"rollover": {"max_primary_shard_size": "50gb", "max_age": "30d"}}},
      "warm":  {"min_age": "60d", "actions": {"forcemerge": {"max_num_segments": 1}}},
      "delete":{"min_age": "180d", "actions": {"delete": {}}}
    }
  }
}
```

If ILM is not available, an equivalent ISM policy is created and attached by the CLI. The status command detects and reports either ILM or ISM.

## Configuration

- Location: `~/.baldrick-rebec/config.yaml` (override with `BALDRICK_REBEC_HOME_DIR`).
- Example minimal config for local Docker:

```yaml
server:
  port: 53051
postgres:
  host: 127.0.0.1
  port: 5432
  dbname: rbc
  sslmode: disable
  app:
    user: rbc_app
    password: rbcpass
  admin:
    user: rbc_admin
    password: ""        # Admin password used for admin operations
opensearch:
  scheme: http
  host: 127.0.0.1
  port: 9200
  app:
    username: rbc_app
    password: ""
  admin:
    username: admin
    password: ""        # Admin password used for admin operations
```

- If your OpenSearch is secured via TLS and auth:

```yaml
opensearch:
  scheme: https
  host: 127.0.0.1
  port: 9200
  insecure_skip_verify: true # dev only
  app:
    username: rbc_app
    password: ""
  admin:
    username: admin
    password: ${OPENSEARCH_ADMIN_PASSWORD}
```

## Local Development (Docker/Podman)

- The repository includes `docker-compose.yaml` for OpenSearch (2 nodes + dashboards) and PostgreSQL. Typical flow:
  1. Start services:
     - `docker compose up -d postgres`
     - `docker compose up -d opensearch-node1 opensearch-node2 opensearch-dashboards`
  2. Create config: `rbc config init --overwrite [flags]`
  3. Plan changes: `rbc db plan`
  4. Scaffold DB: `rbc db scaffold --create-roles --create-db --grant-privileges --yes`
  5. Configure OpenSearch secure localhost and lifecycle: `rbc os bootstrap`
  6. Initialize schema: `rbc db init`
  7. Verify: `rbc db status`

- Podman users can use `podman-compose` with the same file (adjust commands accordingly).

## Backups and Maintenance

PostgreSQL

- Backups: `pg_dump -h HOST -U USER -d DB -Fc > backup.dump`
- Restores: `pg_restore -h HOST -U USER -d DB --clean --create backup.dump`
- Routine maintenance: analyze/vacuum as per your platform defaults.

OpenSearch

- For production, use snapshot repositories (S3, filesystem snapshots). For local dev, preserving the compose volume is often sufficient.
- ILM handles index rollover/retention; monitor cluster health and disk watermarks.

## Environments and Naming

- For multi‑environment deployments, consider prefixing index and database names or using separate clusters/instances. The current implementation uses a single index name `messages_content`; environment separation can be managed by distinct clusters or credentials.

## Security Considerations

- Do not commit credentials. Use environment variables or secret stores to supply passwords.
- Restrict database users to least privilege (e.g., `rbc_app` with DML only; `rbc_admin` for migrations).
- Prefer TLS for OpenSearch and PostgreSQL in non‑local environments.

## Current Limitations and Roadmap

- PostgreSQL schema changes are applied ad‑hoc via `rbc db init`. Introduce versioned migrations before widening usage.
- The CLI creates the OpenSearch index but does not create ILM policies. Add `rbc os ilm ensure` if you prefer full automation.
- Add health endpoints and richer diagnostics in `rbc db status` (doc counts, table existence, index settings) as needed.

## Credentials Map (Roles → Password Location)

This table maps each principal (role/user) to where its password should live in different environments. Avoid committing any credentials to version control.

| System | Role/User | Purpose | Local dev (single-user) | Docker Compose init | CI/Staging/Prod |
| --- | --- | --- | --- | --- | --- |
| PostgreSQL | `rbc_admin` | Schema owner; migrations | Not typically needed at runtime; if used, store in `~/.baldrick-rebec/config.yaml` temporarily | Use `.env` for compose bootstrap only; do NOT commit | Secret manager (e.g., AWS SM, Vault) or Kubernetes Secret; inject to migration job |
| PostgreSQL | `rbc_app` | Runtime DML by CLI/server | `~/.baldrick-rebec/config.yaml` under `postgres.app.password` | `.env` passed to app container env; override via secrets when possible | Secret manager / Kubernetes Secret; injected as env/secret volume to the service |
| OpenSearch | `admin` | Operator tasks (ILM, bootstrap) | Use only when needed; supply via `~/.baldrick-rebec/config.yaml` or env var at run time; remove after | `.env` with `OPENSEARCH_INITIAL_ADMIN_PASSWORD` for demo images | Secret manager / Kubernetes Secret; use sparingly by ops tooling, not app |
| OpenSearch | `rbc_app` | Index read/write for `messages_content` | `~/.baldrick-rebec/config.yaml` under `opensearch.app.username/password` | `.env` for local containers; prefer non-admin user | Secret manager / Kubernetes Secret; injected to app; least-privilege role |

Guidelines
- Prefer secrets managers in shared environments; avoid long-lived credentials in files.
- For Docker Compose, `.env` must be gitignored and only used for local bootstrap.
- In production, use a non-admin OpenSearch user with index-scoped privileges and a runtime-only Postgres user (`rbc_app`).
