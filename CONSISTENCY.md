# CLI Consistency Guide

This document summarizes shared conventions across the `rbc admin` CLI: common commands, flags, outputs, and core table fields. Use it as a checklist when adding or reviewing commands.

## Command Shapes (typical)
- CRUD-style
  - `set`  – create or update an entity (idempotent where unique keys apply)
  - `get`  – fetch a single entity by key (e.g., `--id` or `--name --role`)
  - `list` – list entities (typically requires `--role`, supports pagination)
  - `delete` – delete an entity (with `--force` to skip confirmation)
- Graph/task helpers
  - `task latest --variant <v>` or `--from-id <id>`
  - `task next --id <id> --level patch|minor|major|latest`
  - `stickie-rel set|get|list|delete`
- DB admin
  - `db scaffold --create-roles|--create-db|--grant-privileges --yes` or `--all`
  - `db init` (content/FTS), `db status [--json]`, `db show --output tables|md|json [--concise] [--schema public]`
  - `db backup --output <file>` / `db restore --input <file> [--delete-existing] [--upsert]`
  - `db count [--json]`
  - `db age-status` (diagnostics), `db age-init --yes [--quiet]`
- Queues
  - `queue add|peek|size|take`

## Common Flags
- Identification
  - `--id <uuid>` – primary identifier for entities with UUID keys
  - `--name <text>` – natural key for named entities (often combined with `--role`)
  - `--role <text>` – role scoping key (required by most list/get of role-scoped entities)
- Presentation
  - `--output table|json|md` – default `table`; JSON is pretty-printed
  - `--limit <int>` – default 100 (lists); `--offset <int>` – default 0
  - `--concise` – when supported, show fewer columns (e.g., `db show`)
- Mutation
  - `--title <text>`, `--description <text>`, `--notes <text>`
  - `--tags k=v[,k2=v2]` (repeatable or comma-separated); plain keys map to boolean true
- Safety
  - `--force` – skip confirmation (delete)
  - `--yes` – confirm destructive/privileged operations (db scaffold/init/age-init)
  - `--quiet` – reduce non-critical logs (age-init)
- Task/graph specific
  - `task latest --variant <v>` or `--from-id <uuid>`
  - `task next --id <uuid> --level patch|minor|major|latest`
  - `stickie-rel set --from <uuid> --to <uuid> --type includes|causes|uses|represents|contrasts_with [--labels a,b]`
  - `stickie-rel list --id <uuid> --direction out|in|both [--types t1,t2] [--output json]`
  - `stickie-rel get --from <uuid> --to <uuid> --type <t> [--ignore-missing]`

## Output Conventions
- Default human output is table-formatted (using tablewriter) with terse headers.
- JSON output is a pretty-printed array/object with stable keys and ISO8601 (RFC3339Nano) timestamps.
- Most list commands print a brief stderr summary (`<entity>: <count>`). Mutations print a human line on stderr and JSON on stdout.
- Graph commands include enriched error messages with `cmd=... params={...}` when failing, to ease diagnostics.

## Core Tables (fields)
High-level view of common fields (not exhaustive).
- Auditable fields (many tables)
  - `created TIMESTAMPTZ` (and optionally `updated TIMESTAMPTZ`)
  - `notes TEXT`, `tags JSONB` (free-form metadata)
- Roles
  - `roles(name PK, title, description, created, updated, notes, tags)`
- Workflows
  - `workflows(name PK, title, description, role_name, created, updated, notes)`
- Projects
  - `projects(name, role_name) PK, description, created, updated, notes, tags`
- Workspaces
  - `workspaces(id PK UUID, description, role_name, project_name, build_script_id, created, updated, tags)`
- Scripts
  - `scripts_content(id BYTEA PK, script_content TEXT, created_at)`
  - `scripts(id UUID PK, title, description, motivation, notes, script_content_id BYTEA, role_name, tags, created, updated)`
- Tasks
  - `tasks(id UUID PK, command, variant UNIQUE, title, description, motivation, role_name, created, notes, shell, run_script_id, timeout INTERVAL, tool_workspace_id, tags, level)`
  - Variants: `task_variants(variant PK, workflow_id)`
- Messages
  - `messages_content(id UUID PK, text_content, json_content, created_at)`
  - `messages(id UUID PK, content_id, from_task_id, experiment_id, role_name, status, error_message, tags, created)`
- Queues
  - `queues(id UUID PK, description, inQueueSince, status, why, tags, task_id, inbound_message, target_workspace_id)`
- Testcases
  - `testcases(id UUID PK, name, package, classname, title, experiment_id, role_name, status, error_message, tags, level, created, file, line, execution_time)`
- Conversations/Experiments
  - `conversations(id UUID PK, title, description, project, role_name, tags, created, updated, notes)`
  - `experiments(id UUID PK, conversation_id, created)`
- Tags/Topics/Stores
  - `tags(name PK, title, description, role_name, created, updated, notes)`
  - `topics(name, role_name) PK, title, description, created, updated, notes, tags`
  - `stores(id UUID PK, name, role_name UNIQUE per name, title, description, motivation, security, privacy, created, updated, notes, tags, store_type, scope, lifecycle)`
- Blackboards & Stickies
  - `blackboards(id UUID PK, store_id, role_name, conversation_id, project_name, task_id, created, updated, background, guidelines)`
  - `stickies(id UUID PK, blackboard_id, topic_name, topic_role_name, note, labels TEXT[], created, updated, created_by_task_id, edit_count, priority_level, structured JSONB)`
  - SQL mirror for graph edges: `stickie_relations(from_id, to_id, rel_type, labels TEXT[], created)`

## Relationships (FK vs. Graph)
- Relational (FKs)
  - experiments.conversation_id → conversations.id
  - messages.content_id → messages_content.id, messages.from_task_id → tasks.id
  - packages.role_name → roles.name, packages.task_id → tasks.id
  - queues.task_id → tasks.id, queues.inbound_message → messages.id, queues.target_workspace_id → workspaces.id
  - tasks.run_script_id → scripts.id, tasks.tool_workspace_id → workspaces.id
  - testcases.experiment_id → experiments.id
  - workspaces.build_script_id → scripts.id, workspaces.(project_name,role_name) → projects
  - blackboards.store_id → stores.id, blackboards.conversation_id → conversations.id, blackboards.task_id → tasks.id
  - stickies.blackboard_id → blackboards.id, stickies.created_by_task_id → tasks.id
  - stickies.(topic_name,topic_role_name) → topics
- Graph (AGE)
  - Task —[REPLACES {level,comment,created}]→ Task
  - Stickie —[INCLUDES|CAUSES|USES|REPRESENTS|CONTRASTS_WITH {labels}]→ Stickie
  - Mirror (SQL) can be toggled via config (see below).

## Config Conventions
- File: `~/.baldrick-rebec/config.yaml` (loaded/merged with defaults)
- Postgres roles
  ```yaml
  postgres:
    host: 127.0.0.1
    port: 5432
    dbname: rbc
    sslmode: disable
    admin:
      user: rbc
      password: rbcpass
    app:
      user: rbc_app
      password: rbc_app_pass
  ```
- Graph behavior
  ```yaml
  graph:
    allow_fallback: false   # default; true enables SQL mirror fallback for stickie-rel
  ```

## Error Reporting
- Enriched errors for graph commands include the command and parameter context, e.g.:
  - `cmd=task latest params={variant:unit/go,from-id:}`
  - `cmd=stickie-rel set params={from:<id>,to:<id>,type:uses,labels:[...]}`
- Finder functions append the failing `cypher=...` snippet to ease triage.

## Pagination & IDs
- IDs are UUIDs (lowercase, hyphenated).
- Lists use `--limit/--offset` with sane defaults; `--output json` returns arrays of objects with stable keys.

This guide should be kept up to date as new commands are added or flags evolve.
