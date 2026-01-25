# baldrick-rebec
Autonomous build automation tool and task runner

![baldrick-rebec hero image](./baldrick-rebec-hero-image.jpg)

## First-Time Setup

Use these steps to configure the CLI and initialize PostgreSQL for the first time.

- Create or update your config
  - Minimal example using local Postgres:
    - `rbc config init --overwrite --pg-host 127.0.0.1 --pg-port 5432 --pg-dbname rbc --pg-app-user rbc_app --pg-app-password 'app_password' --pg-admin-user postgres --pg-admin-password 'admin_password'`
  - Check secrets and verify admin role visibility from the app connection:
    - `rbc config check --passwords --verify`

- Bootstrap roles, database, privileges, and schema
  - One-shot scaffold (roles + DB + privileges + base schema + content table + FTS):
    - `rbc db scaffold --all --yes`

- Confirm database status
  - `rbc db status`

- Send a test message (reads stdin; logs metadata to stderr)
  - `echo "Hello world" | rbc message send --conversation c1 --attempt a1 --profile default`

If you need to wipe and start fresh in development:
- `rbc db reset --force`
Then run the scaffold step again.

See DATABASES.md for full workflow and a setup checklist. For ops-focused learning prompts, see LEARNING.md.

## Blackboard CLI

- Sync id ↔ folder: `rbc blackboard sync id:_ folder:features [--dry-run] [--delete] [--clear-ids] [--force-write] [--include-archived]`
- Diff id vs folder: `rbc blackboard diff id:_ folder:features [--detailed] [--include-archived]`
- Import from folder: `rbc blackboard import features [--detailed]` (preserves IDs; requires ids not to exist in DB)

## Snapshot Backups

Use the schema-aware snapshot subsystem to capture and restore long‑lived entities into a dedicated Postgres schema (default: `backup`). It stores full JSONB row snapshots plus tracked entity schemas, and supports append/replace restores.

- Create a backup
  - `rbc snapshot backup --description "before schema cleanup" --who your-user --json`
  - Optional filters:
    - `--include roles,projects` (overrides defaults) and/or `--exclude stickies`
    - `--schema backup_alt` for a custom backup schema
- List backups
  - `rbc snapshot list --limit 20`
  - JSON: `rbc snapshot list --json`
- Show backup summary
  - `rbc snapshot show <backup-id>`
- Restore
  - Append missing rows: `rbc snapshot restore <backup-id> --mode append`
  - Replace table contents: `rbc snapshot restore <backup-id> --mode replace`
  - Limit entities: `--entity roles,projects`
  - Validate without changes: `--dry-run`
- Delete backup
  - `rbc snapshot delete <backup-id> --force`

By default, permanent-ish entities like `roles`, `workflows`, `tags`, `projects`, `scripts`, `tasks`, `topics`, `workspaces`, `blackboards`, `stickies`, `stickie_relations`, `task_replaces`, `packages`, `task_variants`, and `scripts_content` are included. Ephemeral tables such as `conversations`, `experiments`, `messages`, `messages_content`, `queues`, and `testcases` are excluded unless explicitly included.

Snapshot connections require a dedicated backup role configured in `~/.baldrick-rebec/config.yaml` (no admin fallback):

postgres:
  host: 127.0.0.1
  port: 5432
  dbname: rbc
  sslmode: disable
  admin:
    user: rbc_admin
    password: pass
  app:
    user: rbc_app
    password: pass
  backup:
    user: rbc_backup
    password: pass

Grant this role permissions to own or write to the backup schema, e.g. (or run `rbc db scaffold --grant-backup --yes`):
- `CREATE SCHEMA IF NOT EXISTS backup AUTHORIZATION rbc_backup;`
- `GRANT USAGE ON SCHEMA backup TO rbc_backup;`

## Notes on Postgres admin passwords
- For initial provisioning (roles/db/privileges/schema), set the Postgres admin password via `--pg-admin-password`.
- Ensure app credentials are configured for runtime use:
  - `rbc config init --pg-app-user rbc_app --pg-app-password '<app-pass>'`
## ZX CLI Helpers (scripts)

This repo includes helper utilities under `script/cli-helper.mjs` to make end-to-end flows easier to read and maintain. They wrap the `rbc` CLI using Google ZX.

Key exports:

- Core: `runRbc`, `runRbcJSON`, `idFrom`, `logStep`, `assert`
- Roles/Workflows: `runSetRole({name,title,description?,notes?})`, `runSetWorkflow({name,title,description?,role?,notes?})`
- Scripts: `createScript(role,title,description,body,{name?,variant?,archived?})`, `scriptListJSON({role,...})`, `scriptFind({name,variant?,archived?,role?})`
- Tasks: `runSetTask({...})`, `taskSetReplacement({...})`
- Blackboards: `blackboardSet({...})`
- Stickies: `stickieSet({...})`, `stickieListJSON({...})`, `stickieFind({...})`, `stickieList`, `stickieListByBlackboard`, `stickieRelSet`, `stickieRelList`, `stickieRelGet`
- Conversations/Experiments: `conversationSet({title,role?})`, `experimentCreate({conversation})`
- Queue: `queueAdd({...})`, `queuePeek`, `queueSize`, `queueTake`
- Lists and counts: `listWithRole(cmd,role,limit)`, `experimentList(limit)`, `dbCountPerRole`, `dbCountJSON`
- Snapshot: `snapshotBackupJSON({description,who})`, `snapshotList`, `snapshotShow`, `snapshotRestoreDry`, `snapshotDelete`
  - Verify/Prune: `snapshotVerifyJSON({id,schema?})`, `snapshotPrunePreviewJSON({olderThan?,schema?})`, `snapshotPruneYesJSON({olderThan?,schema?})`

Example:

```js
import { runSetRole, runSetWorkflow, createScript, scriptFind } from './cli-helper.mjs';

await runSetRole({ name: 'rbctest-user', title: 'Test User' });
await runSetWorkflow({ name: 'ci-test', title: 'CI Test', role: 'rbctest-user' });
const sid = await createScript('rbctest-user', 'Unit: go test', 'Run unit tests', '#!/usr/bin/env bash\ngo test ./...\n', { name: 'Unit: go test', variant: ''});
const script = await scriptFind({ name: 'Unit: go test', variant: '', role: 'rbctest-user' });
```

The test script `script/test-all.mjs` demonstrates broader usage across entities.
