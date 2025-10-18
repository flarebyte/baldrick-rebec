# Databases

This document explains how the Baldrick‑Rebec CLI uses its databases (PostgreSQL and OpenSearch), what is managed by the CLI, and what typically requires manual setup. It also gives a practical setup guide for local development and production‑like environments.

## Components

1) PostgreSQL (relational)

- Purpose: Stores message ingest “events” and reusable “profiles”.
- Core tables created/managed by CLI:
  - `messages_events`: one row per ingest/processing event referencing content stored in OpenSearch.
  - `message_profiles`: named profiles containing behavioral defaults and metadata.
- Access patterns: inserts/reads, filtering by conversation/attempt/profile, later joins to operational reports.

2) OpenSearch (search/vector)

- Purpose: Stores unique message content optimized for search and deduplication.
- Index name: `messages_content`
- Document ID: `SHA256(<canonicalized_message_body>)`
- Mapping/settings are ensured by the CLI; lifecycle (ILM) is typically managed by operators.

## What the CLI Manages

- `rbc admin db configure` — Creates `~/.baldrick-rebec/config.yaml` with server/DB settings.
- `rbc admin db init` — Initializes data stores:
  - PostgreSQL: creates or updates core tables and a trigger to maintain `updated_at` on `message_profiles`.
  - OpenSearch: ensures the `messages_content` index with simplified mappings/settings exists.
- `rbc admin db status` — Connectivity + basic health checks for PostgreSQL and OpenSearch and presence of `messages_content`.

Notes:
- PostgreSQL schema changes are currently applied directly by `db init` (no versioned migrations yet). For production, consider adopting a migration tool later (e.g., `golang-migrate`).
- OpenSearch ILM policy is referenced (`messages-content-ilm`) but not installed automatically by the CLI. See “Operator Tasks” below.

## Operator Tasks (Manual)

PostgreSQL

- Provision roles and databases for least-privilege operation (recommended for staging/prod):

```sql
-- Admin role (schema owner)
CREATE ROLE rbc_admin LOGIN PASSWORD 'change-me';
-- App role (runtime)
CREATE ROLE rbc_app LOGIN PASSWORD 'change-me';

-- Dedicated database
CREATE DATABASE rbc OWNER rbc_admin;

-- Privileges for runtime
GRANT CONNECT ON DATABASE rbc TO rbc_app;
\c rbc
GRANT USAGE ON SCHEMA public TO rbc_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO rbc_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO rbc_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO rbc_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO rbc_app;
```

- For local development (Docker Compose), the compose environment variables can create the database automatically without the above.

OpenSearch

- Configure security and users (for secured clusters):
  - Create a user (e.g., `rbc_app`) with index-level permissions for `messages_content` (read/write) and restricted cluster privileges.
  - Keep the built‑in `admin` user for bootstrap and operator tasks only.
- Create/attach an ILM policy (recommended):

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

The CLI sets the index’s `index.lifecycle.name` when creating `messages_content`, but does not create the ILM policy itself.

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
    password_temp: ""   # Temporary only; remove after use
opensearch:
  scheme: http
  host: 127.0.0.1
  port: 9200
  app:
    username: rbc_app
    password: ""
  admin:
    username: admin
    password_temp: ""   # Temporary only; remove after use
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
    password_temp: ${OPENSEARCH_INITIAL_ADMIN_PASSWORD} # Use temporarily; remove after use
```

## Local Development (Docker/Podman)

- The repository includes `docker-compose.yaml` for OpenSearch (2 nodes + dashboards) and PostgreSQL. Typical flow:
  1. Start services:
     - `docker compose up -d postgres`
     - `docker compose up -d opensearch-node1 opensearch-node2 opensearch-dashboards`
  2. Create config: `rbc admin db configure --overwrite [flags]`
  3. Initialize stores: `rbc admin db init`
  4. Verify: `rbc admin db status`

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

- PostgreSQL schema changes are applied ad‑hoc via `rbc admin db init`. Introduce versioned migrations before widening usage.
- The CLI creates the OpenSearch index but does not create ILM policies. Add `rbc admin os ilm ensure` if you prefer full automation.
- Add health endpoints and richer diagnostics in `rbc admin db status` (doc counts, table existence, index settings) as needed.

## Credentials Map (Roles → Password Location)

This table maps each principal (role/user) to where its password should live in different environments. Avoid committing any credentials to version control.

| System | Role/User | Purpose | Local dev (single-user) | Docker Compose init | CI/Staging/Prod |
| --- | --- | --- | --- | --- | --- |
| PostgreSQL | `rbc_admin` | Schema owner; migrations | Not typically needed at runtime; if used, store in `~/.baldrick-rebec/config.yaml` temporarily | Use `.env` for compose bootstrap only; do NOT commit | Secret manager (e.g., AWS SM, Vault) or Kubernetes Secret; inject to migration job |
| PostgreSQL | `rbc_app` | Runtime DML by CLI/server | `~/.baldrick-rebec/config.yaml` under `postgres.password` | `.env` passed to app container env; override via secrets when possible | Secret manager / Kubernetes Secret; injected as env/secret volume to the service |
| OpenSearch | `admin` | Operator tasks (ILM, bootstrap) | Use only when needed; supply via `~/.baldrick-rebec/config.yaml` or env var at run time; remove after | `.env` with `OPENSEARCH_INITIAL_ADMIN_PASSWORD` for demo images | Secret manager / Kubernetes Secret; use sparingly by ops tooling, not app |
| OpenSearch | `rbc_app` | Index read/write for `messages_content` | `~/.baldrick-rebec/config.yaml` under `opensearch.username/password` | `.env` for local containers; prefer non-admin user | Secret manager / Kubernetes Secret; injected to app; least-privilege role |

Guidelines
- Prefer secrets managers in shared environments; avoid long-lived credentials in files.
- For Docker Compose, `.env` must be gitignored and only used for local bootstrap.
- In production, use a non-admin OpenSearch user with index-scoped privileges and a runtime-only Postgres user (`rbc_app`).
