# RBC Admin Command Cheatsheet

Quick reference of common `rbc` admin commands with concise examples. Commands assume a configured environment (DB, config, etc.). Run with `go run main.go …` during development, or `rbc …` in a built/installed setup.

## Prompt

| Command                   | Purpose                                                                                                                          | Key Options                                                                                                                                            | Example                                                                |
| ------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------- |
| `rbc admin prompt active` | Interactive prompt designer (TUI) for building Markdown prompt blocks; supports text, testcase, stickie, preview, quick UUID add | Keys: `1` add text, `u` quick add UUIDs, `enter/e` edit value, `i` edit id, `[`/`]` move, `x` disable, `c` convert to text, `p` preview, `s` save JSON | `rbc admin prompt active`                                              |
| `rbc admin prompt run`    | Run a single prompt against a configured tool (local or remote)                                                                  | `--tool-name`, `--input` or `--input-file`, `--tools <json-file>`, `--temperature`, `--max-output-tokens`, `--json`, `--remote`, `--addr`              | `rbc admin prompt run --tool-name openai:gpt4o --input 'hello' --json` |

## Blackboards

| Command                       | Purpose                                                                   | Key Options                                                                                                      | Example                                                                 |
| ----------------------------- | ------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| `rbc admin blackboard active` | Browse blackboards and stickies (TUI) with search/filter and multi-select | In-board keys: `/` search note, `t` topic filter, `m` multi-select, space toggle, `a` all, `n` none, `r` refresh | `rbc admin blackboard active --search acme`                             |
| `rbc admin blackboard set`    | Create/update a blackboard                                                | `--id`, `--store-id`, `--project`, `--conversation`, `--task`, `--background`, `--guidelines`                    | `rbc admin blackboard set --store-id <store-uuid> --project acme/build` |
| `rbc admin blackboard get`    | Get a blackboard by id                                                    | `--id`                                                                                                           | `rbc admin blackboard get --id <uuid>`                                  |
| `rbc admin blackboard list`   | List blackboards for a role                                               | `--role`, `--limit`, `--offset`, `--output`                                                                      | `rbc admin blackboard list --role user --output json`                   |
| `rbc admin blackboard delete` | Delete a blackboard                                                       | `--id`                                                                                                           | `rbc admin blackboard delete --id <uuid>`                               |

## Stickies

| Command                  | Purpose                                      | Key Options                                                                                                         | Example                                                                 |
| ------------------------ | -------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- | -------------------------------------------------------- | ----------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `rbc admin stickie set`  | Create/update a stickie (note/code/metadata) | `--id`, `--blackboard <uuid>`, `--topic-name`, `--topic-role`, `--note`, `--code`, `--labels a,b`, `--priority must | should                                                                  | could                                                    | wont`, `--name`, `--variant`, `--archived`, `--score` | `rbc admin stickie set --blackboard <bb> --topic-name devops --topic-role user --note 'cache idea' --code $'name: CI\n…'` |
| `rbc admin stickie get`  | Get a stickie by id                          | `--id`                                                                                                              | `rbc admin stickie get --id <uuid>`                                     |
| `rbc admin stickie list` | List stickies (by board/topic)               | `--blackboard`, `--topic-name`, `--topic-role`, `--limit`, `--offset`, `--output json                               | table`                                                                  | `rbc admin stickie list --blackboard <bb> --output json` |
| `rbc admin stickie find` | Find by complex name (name/variant)          | `--name`, `--variant`, `--archived`, `--blackboard`                                                                 | `rbc admin stickie find --name FeatureX --variant v1 --blackboard <bb>` |

### Stickie Relations

| Command                      | Purpose                                   | Key Options                                    | Example                                                         |
| ---------------------------- | ----------------------------------------- | ---------------------------------------------- | --------------------------------------------------------------- | ----- | -------------------------------------------------------- | --------------------------- | --------------------------------------------------------------------------------------- |
| `rbc admin stickie-rel set`  | Create/update a relation between stickies | `--from`, `--to`, `--type INCLUDES             | CAUSES                                                          | USES  | REPRESENTS                                               | CONTRASTS_WITH`, `--labels` | `rbc admin stickie-rel set --from <id1> --to <id2> --type uses --labels ref,dependency` |
| `rbc admin stickie-rel list` | List relations for a stickie              | `--id`, `--direction out                       | in                                                              | both` | `rbc admin stickie-rel list --id <uuid> --direction out` |
| `rbc admin stickie-rel get`  | Get a specific relation                   | `--from`, `--to`, `--type`, `--ignore-missing` | `rbc admin stickie-rel get --from <id1> --to <id2> --type uses` |

## Conversations, Experiments, Messages

| Command                         | Purpose                                    | Key Options                                                                          | Example                                                                       |
| ------------------------------- | ------------------------------------------ | ------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------- | ---------------------------------------------------------- |
| `rbc admin conversation active` | Browse recent conversations per role (TUI) | Keys: `/` search, `n` next role, `r` refresh                                         | `rbc admin conversation active`                                               |
| `rbc admin conversation set`    | Create/update a conversation               | `--id`, `--title`, `--description`, `--project`, `--role`, `--tags k=v,…`, `--notes` | `rbc admin conversation set --title 'Roadmap' --role user --project acme/app` |
| `rbc admin conversation get`    | Get a conversation by id                   | `--id`                                                                               | `rbc admin conversation get --id <uuid>`                                      |
| `rbc admin conversation list`   | List conversations                         | `--role`, `--project`, `--limit`, `--offset`, `--output`                             | `rbc admin conversation list --role user --output json`                       |
| `rbc admin experiment create`   | Create an experiment under a conversation  | `--conversation`                                                                     | `rbc admin experiment create --conversation <conv-uuid>`                      |
| `rbc admin experiment list`     | List experiments                           | `--conversation`, `--limit`, `--offset`                                              | `rbc admin experiment list --conversation <conv-uuid>`                        |
| `rbc admin message set`         | Create a message (stdin body)              | `--experiment`, `--title`, `--tags`, `--role`                                        | `echo 'hello'                                                                 | rbc admin message set --experiment <exp> --title Greeting` |
| `rbc admin message list`        | List messages                              | `--role`, `--experiment`, `--task`, `--status`, `--limit`, `--offset`, `--output`    | `rbc admin message list --role user --output json`                            |

## Testcases

| Command                     | Purpose                                        | Key Options                                                             | Example                                                                |
| --------------------------- | ---------------------------------------------- | ----------------------------------------------------------------------- | ---------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| `rbc admin testcase active` | View active testcases for a conversation (TUI) | Keys: `n` cycle experiments, `e` errors-only, `r` refresh               | `rbc admin testcase active --conversation <conv-uuid>`                 |
| `rbc admin testcase create` | Create a testcase row                          | `--title`, `--role`, `--experiment`, `--status OK                       | KO                                                                     | TODO`, `--level h1..h6`, `--name`, `--pkg`, `--classname`, `--file`, `--line`, `--execution-time` | `rbc admin testcase create --title 'go vet' --role user --experiment <exp> --status OK` |
| `rbc admin testcase list`   | List testcases                                 | `--role`, `--experiment`, `--status`, `--limit`, `--offset`, `--output` | `rbc admin testcase list --role user --experiment <exp> --output json` |

## Tasks & Workflows

| Command                      | Purpose                                          | Key Options                                                                                                                                   | Example                                                                                                  |
| ---------------------------- | ------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- | ----- | ------- | -------------------------------------------------- |
| `rbc admin workflow set`     | Create/update a workflow                         | `--name`, `--title`, `--description`, `--role`, `--notes`                                                                                     | `rbc admin workflow set --name ci-test --title 'CI Test' --role user`                                    |
| `rbc admin task set`         | Create/update a task (optionally as replacement) | `--workflow`, `--command`, `--variant`, `--role`, `--title`, `--description`, `--shell`, `--replaces`, `--replace-level`, `--replace-comment` | `rbc admin task set --workflow ci-test --command unit --variant go --role user --title 'Run Unit Tests'` |
| `rbc admin task list`        | List tasks                                       | `--role`, `--workflow`, `--limit`, `--offset`, `--output`                                                                                     | `rbc admin task list --role user --output json`                                                          |
| `rbc admin task latest`      | Get latest task variant                          | `--variant`, `--from-id`                                                                                                                      | `rbc admin task latest --variant unit/go`                                                                |
| `rbc admin task next`        | Compute next version from an id                  | `--id`, `--level patch                                                                                                                        | minor                                                                                                    | major | latest` | `rbc admin task next --id <task-id> --level minor` |
| `rbc admin task run`         | Run a task against a queue/tooling (if wired)    | task-specific                                                                                                                                 | `rbc admin task run --workflow ci-test --command unit`                                                   |
| `rbc admin task script add`  | Attach a script to a task                        | `--task`, `--script`, `--name`, `--alias`                                                                                                     | `rbc admin task script add --task <task-id> --script <script-id> --name build`                           |
| `rbc admin task script list` | List task scripts                                | `--task`                                                                                                                                      | `rbc admin task script list --task <task-id>`                                                            |

## Projects, Stores, Topics, Roles, Tags, Tools, Workspaces

| Command                   | Purpose                            | Key Options                                                                                                                                        | Example                                                                                         |
| ------------------------- | ---------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| `rbc admin project set`   | Create/update a project            | `--name`, `--role`, `--description`, `--notes`, `--tags`                                                                                           | `rbc admin project set --name acme/build-system --role user --description 'Build & CI'`         |
| `rbc admin store set`     | Create/update a store              | `--name`, `--role`, `--title`, `--description`, `--motivation`, `--security`, `--privacy`, `--notes`, `--type`, `--scope`, `--lifecycle`, `--tags` | `rbc admin store set --name complete-store --role user --title 'Complete Store' --type journal` |
| `rbc admin topic set`     | Create/update a topic              | `--name`, `--role`, `--title`, `--description`, `--tags`                                                                                           | `rbc admin topic set --name devops --role user --title DevOps`                                  |
| `rbc admin role set`      | Create/update a role               | `--name`, `--title`, `--description`, `--notes`                                                                                                    | `rbc admin role set --name rbctest-user --title 'RBCTest User'`                                 |
| `rbc admin tag set`       | Create/update a tag                | `--name`, `--title`, `--role`                                                                                                                      | `rbc admin tag set --name priority-high --title 'High Priority' --role user`                    |
| `rbc admin tool set`      | Create/update tool config for LLMs | `--name`, `--provider`, `--model`, `--api-key-secret`, `--temperature`, `--max-output-tokens`, `--top-p`, `--settings`                             | `rbc admin tool set --name openai:gpt4o --provider openai --model gpt-4o`                       |
| `rbc admin workspace set` | Create/update a workspace          | `--role`, `--project`, `--description`, `--tags`, `--build-script-id`                                                                              | `rbc admin workspace set --role user --project acme/build-system --description 'Local build'`   |

## Scripts

| Command                 | Purpose                             | Key Options                                                               | Example                                                     |
| ----------------------- | ----------------------------------- | ------------------------------------------------------------------------- | ----------------------------------------------------------- | --------------------------------------------------------- |
| `rbc admin script set`  | Create/update a script (stdin body) | `--role`, `--title`, `--description`, `--name`, `--variant`, `--archived` | `echo '#!/usr/bin/env bash'                                 | rbc admin script set --role user --title 'Unit: go test'` |
| `rbc admin script list` | List scripts                        | `--role`, `--limit`, `--offset`, `--output`                               | `rbc admin script list --role user --output json`           |
| `rbc admin script find` | Find by complex name                | `--name`, `--variant`, `--archived`                                       | `rbc admin script find --name 'Unit: go test' --variant ''` |

## Queue & Messages

| Command                | Purpose                | Key Options                                    | Example                                                            |
| ---------------------- | ---------------------- | ---------------------------------------------- | ------------------------------------------------------------------ |
| `rbc admin queue add`  | Add a queue item       | `--description`, `--status`, `--why`, `--tags` | `rbc admin queue add --description 'build image' --status PENDING` |
| `rbc admin queue peek` | Peek next item         | —                                              | `rbc admin queue peek`                                             |
| `rbc admin queue take` | Take/process next item | —                                              | `rbc admin queue take`                                             |
| `rbc admin queue size` | Queue size             | —                                              | `rbc admin queue size`                                             |

## Server & DB

| Command                          | Purpose                      | Key Options                      | Example                             |
| -------------------------------- | ---------------------------- | -------------------------------- | ----------------------------------- | -------------------------------------------------- |
| `rbc admin server start`         | Start HTTP/GRPC server       | `--config`, environment specific | `rbc admin server start`            |
| `rbc admin server status`        | Show server status           | —                                | `rbc admin server status`           |
| `rbc admin server reload_config` | Reload config                | —                                | `rbc admin server reload_config`    |
| `rbc admin server stop`          | Stop server                  | —                                | `rbc admin server stop`             |
| `rbc admin db scaffold`          | Create roles, schema, grants | `--all`, `--yes`                 | `rbc admin db scaffold --all --yes` |
| `rbc admin db reset`             | Reset database (destructive) | `--force`, `--drop-app-role=true | false`                              | `rbc admin db reset --force --drop-app-role=false` |
| `rbc admin db backup`            | Backup database              | `--schema`, destination config   | `rbc admin db backup --schema app`  |
| `rbc admin db restore`           | Restore from backup          | `--schema`, source config        | `rbc admin db restore --schema app` |
| `rbc admin db status`            | DB status                    | —                                | `rbc admin db status`               |
| `rbc admin db search`            | Search DB (utility)          | implementation-defined           | `rbc admin db search 'SELECT 1'`    |
| `rbc admin db count`             | Row counts by table          | —                                | `rbc admin db count`                |

## Snapshots

| Command                      | Purpose                              | Key Options                                   | Example                                                               |
| ---------------------------- | ------------------------------------ | --------------------------------------------- | --------------------------------------------------------------------- | ------------------------------------------------------ |
| `rbc admin snapshot backup`  | Create a snapshot                    | `--description`, `--who`, `--json`            | `rbc admin snapshot backup --description 'nightly' --who user --json` |
| `rbc admin snapshot list`    | List snapshots                       | `--limit`, `--offset`                         | `rbc admin snapshot list --limit 5`                                   |
| `rbc admin snapshot show`    | Show a snapshot                      | `--id`                                        | `rbc admin snapshot show --id <uuid>`                                 |
| `rbc admin snapshot verify`  | Verify snapshot                      | `--id`                                        | `rbc admin snapshot verify --id <uuid>`                               |
| `rbc admin snapshot restore` | Restore snapshot (dry-run or append) | `--id`, `--mode dry                           | append`                                                               | `rbc admin snapshot restore --id <uuid> --mode append` |
| `rbc admin snapshot prune`   | Prune snapshots                      | `--older-than`, `--schema`, `--yes`, `--json` | `rbc admin snapshot prune --older-than 30d --schema app --yes --json` |

---

Tips:

- Add `--output json` where available to script with JSON.
- Most setters accept optional fields; omitted flags leave values unchanged on update.
- TUIs list key bindings at the top; use `q` to quit.
