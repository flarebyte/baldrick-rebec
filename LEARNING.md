# Ops Learning Guide (Entry Points)

This guide lists concise learning prompts for operating the Baldrick‑Rebec app’s data layer. Each paragraph is a starting prompt you can give to a learning tool or use for self‑study. Keep focus on hands‑on, localhost-first workflows that mirror what the CLI automates.

## PostgreSQL

 Learn to manage Postgres roles and privileges for a small app: create an admin role for migrations and an app role for runtime DML; practice granting/revoking schema/table/sequence privileges and verify with system catalogs; relate this to `rbc db scaffold --create-roles --grant-privileges` and `rbc db status`.

 Learn to provision and secure a dedicated database: create a database owned by the admin role, grant CONNECT to the app role, and understand identifier safety; connect with `psql` and confirm with queries; map these steps to `rbc db scaffold --create-db`.

Learn schema lifecycle basics without overengineering migrations: create the core tables (`messages_events`, `workflows`, `tasks`) and a trigger maintaining `updated` on workflows; practice idempotent DDL and how the CLI does this in `db init`/`db scaffold`.

 Learn connection hygiene and credentials handling: configure localhost connectivity, SSL modes, env and config file usage; understand temporary admin passwords vs app credentials; practice clearing temporary secrets with `rbc config clear-admin-temp`.

Learn backup/restore essentials: perform a compressed dump with `pg_dump -Fc` and restore with `pg_restore --clean --create`; test round‑trip on a local instance and document retention expectations.

Learn performance triage basics: add an index, check query plans with `EXPLAIN (ANALYZE, BUFFERS)`, and observe how `VACUUM`/`ANALYZE` affect plans; stay practical and narrowly scoped to the app’s two tables.

 Learn operational verification with the CLI: run `rbc db status --json` and interpret roles, database existence, schema completeness, and privilege checks; use `db plan` to preview changes, and `db revoke-privileges` for safe rollback scenarios.

 Learn troubleshooting authentication: diagnose “password authentication failed” and role/DB missing errors; practice fixing via `rbc config init` (app/admin creds) and `db scaffold`.

## OpenSearch

 Learn secured localhost connectivity: configure HTTPS with self‑signed certs, use admin temporary password for bootstrap, and understand the difference between admin and app credentials; practice with `rbc os bootstrap` and then clear admin temp.

 Learn index lifecycle management in OpenSearch: compare ILM (Elasticsearch API) and ISM (OpenSearch plugin); ensure a rollover/retention policy and attach it to `messages_content`; practice with `rbc os ilm ensure` or `rbc os ism ensure` plus `list/show/delete`.

 Learn index provisioning and health checks: create the `messages_content` index without lifecycle settings and attach policies separately; use `rbc db init` to provision, then `rbc db status --json` to confirm index presence, policy attachment (ILM/ISM), and doc counts.

Learn security error triage: resolve HTTP 401 and TLS issues by verifying scheme/host/port, app creds, and `--os-insecure` for self‑signed dev certs; validate with `curl -k -u user:pass https://127.0.0.1:9200/_cluster/health` and reflect the same in the CLI config.

Learn snapshot/retention awareness: review OpenSearch snapshot concepts for future backups and how ILM/ISM policy timing interacts with storage; focus on local volumes for dev and snapshot repositories for staged/prod.

## Local Ops with Docker Compose

Learn to start/stop and inspect services: bring up `postgres`, `opensearch-node1`, `opensearch-node2`, and `opensearch-dashboards`; use logs to diagnose startup issues; map service ports and volumes to the CLI config.

 Learn environment-to-config flow: declare POSTGRES_USER/PASSWORD/DB and OPENSEARCH_INITIAL_ADMIN_PASSWORD in a `.env` (gitignored), bootstrap the stack, then generate and merge config with `rbc config init` and verify with `rbc config validate`.

## Security Hygiene

 Learn least‑privilege principles for runtime: prefer app roles/users; restrict admin credentials to bootstrap only; rotate passwords regularly; clear temporary admin secrets via `rbc config clear-admin-temp` after provisioning.

Learn safe SQL and API usage: validate identifiers, avoid interpolation for untrusted input, and rely on driver quoting for secrets; understand how the CLI applies identifier validation and literal quoting for Postgres role management.

## Troubleshooting Scenarios (Practice)

 Practice fixing a broken Postgres setup: app connects but privileges are missing—use `rbc db status`, grant with `db scaffold --grant-privileges`, and validate again.

Practice fixing a secured OpenSearch setup: HTTP 401 on health—switch to HTTPS, add admin temp password for bootstrap, run `os bootstrap`, ensure/attach lifecycle, then move to app creds and clear admin temp; confirm with `db status --json`.

 Practice end‑to‑end bring‑up: start Compose services, run `config init`, `db plan`, `db scaffold`, `os bootstrap`, `db init`, verify with `db status`, and finally start the server `rbc server start --detach`.
