# baldrick-rebec
Autonomous build automation tool and task runner

![baldrick-rebec hero image](./baldrick-rebec-hero-image.jpg)

## First-Time Setup

Use these steps to configure the CLI and initialize PostgreSQL for the first time.

- Create or update your config
  - Minimal example using local Postgres:
    - `rbc admin config init --overwrite --pg-host 127.0.0.1 --pg-port 5432 --pg-dbname rbc --pg-app-user rbc_app --pg-app-password 'app_password' --pg-admin-user postgres --pg-admin-password 'admin_password'`
  - Check secrets and verify admin role visibility from the app connection:
    - `rbc admin config check --passwords --verify`

- Bootstrap roles, database, privileges, and schema
  - One-shot scaffold (roles + DB + privileges + base schema + content table + FTS):
    - `rbc admin db scaffold --all --yes`

- Confirm database status
  - `rbc admin db status`

- Send a test message (reads stdin; logs metadata to stderr)
  - `echo "Hello world" | rbc admin message send --conversation c1 --attempt a1 --profile default`

If you need to wipe and start fresh in development:
- `rbc admin db reset --force`
Then run the scaffold step again.

See DATABASES.md for full workflow and a setup checklist. For ops-focused learning prompts, see LEARNING.md.

## Notes on Postgres admin passwords
- For initial provisioning (roles/db/privileges/schema), set the Postgres admin password via `--pg-admin-password`.
- Ensure app credentials are configured for runtime use:
  - `rbc admin config init --pg-app-user rbc_app --pg-app-password '<app-pass>'`
