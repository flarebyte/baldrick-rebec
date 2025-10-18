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
