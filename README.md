# baldrick-rebec
Autonomous build automation tool and task runner

## Quick Start

- Configure global settings (server + DBs):
  - `rbc admin config init --overwrite [flags]`
- Plan and provision databases:
  - `rbc admin db plan`
  - `rbc admin db scaffold --create-roles --create-db --grant-privileges --yes`
  - `rbc admin os bootstrap`  # configure https for localhost and ensure+attach lifecycle (ILM/ISM)
  - `rbc admin db init`
  - `rbc admin db status`
- Start the server (optional):
  - `rbc admin server start --detach`

See DATABASES.md for full workflow, credentials guidance, and a setup checklist.

## OpenSearch Lifecycle

- The CLI supports both ILM (Elasticsearch-style) and ISM (OpenSearch plugin):
  - `rbc admin os ilm ensure|show|list|delete`
  - `rbc admin os ism ensure|show|list|delete`
- `rbc admin os bootstrap` detects which is available and configures lifecycle automatically for `messages_content`.

Note on admin passwords (temporary)
- For bootstrap, set the OpenSearch admin temporary password via `--admin-password-temp` or the `OPENSEARCH_INITIAL_ADMIN_PASSWORD` env.
- After bootstrap, clear it to avoid lingering admin creds in your config:
  - `rbc admin config init --os-admin-password-temp ''`
- Optionally add an app user for day-to-day operations and stop relying on admin:
  - `rbc admin config init --os-app-username rbc_app --os-app-password '<app-pass>'`

Note on Postgres admin passwords (temporary)
- For initial provisioning (roles/db/privileges/schema), set the Postgres admin temporary password via `--pg-admin-password-temp`.
- After scaffolding/initialization, clear it to avoid lingering admin creds in your config:
  - `rbc admin config init --pg-admin-password-temp ''`
- Ensure app credentials are configured for runtime use:
  - `rbc admin config init --pg-app-user rbc_app --pg-app-password '<app-pass>'`
