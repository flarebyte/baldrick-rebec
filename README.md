# baldrick-rebec
Autonomous build automation tool and task runner

## Quick Start

- Configure global settings (server + DBs):
  - `rbc admin config init --overwrite [flags]`
- Plan and provision databases:
  - `rbc admin db plan`
  - `rbc admin db scaffold --create-roles --create-db --grant-privileges --yes`
  - `rbc admin db init`
  - `rbc admin db status`
- Start the server (optional):
  - `rbc admin server start --detach`

See DATABASES.md for full workflow, credentials guidance, and a setup checklist.
