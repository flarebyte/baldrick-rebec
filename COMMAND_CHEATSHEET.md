# RBC Command Cheatsheet

Quick reference of common `rbc` commands with concise examples. Use `go run main.go …` during development, or `rbc …` if installed.

## Prompt

| Command                   | Purpose                                                                                                     | Keys / Options                                                                                                                                         | Example                                                                |
| ------------------------- | ----------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------- |
| `rbc prompt active` | Interactive prompt designer (TUI) for Markdown blocks (text, testcase, stickie); preview and quick UUID add | Keys: `1` add text, `u` quick-add UUIDs, `enter/e` edit value, `i` edit id, `[`/`]` move, `x` disable, `c` convert to text, `p` preview, `s` save JSON | `rbc prompt active`                                              |
| `rbc prompt run`    | Run a single prompt against a tool (local/remote)                                                           | `--tool-name`, `--input` OR `--input-file`, `--tools <json-file>`, `--temperature`, `--max-output-tokens`, `--json`, `--remote`, `--addr`              | `rbc prompt run --tool-name openai:gpt4o --input 'hello' --json` |

## Blackboards

| Command                       | Purpose                                                                   | Keys / Options                                                                                                   | Example                                                                 |
| ----------------------------- | ------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| `rbc blackboard active` | Browse blackboards and stickies (TUI) with search/filter and multi-select | In-board keys: `/` search note, `t` topic filter, `m` multi-select, space toggle, `a` all, `n` none, `r` refresh | `rbc blackboard active --search acme`                             |
| `rbc blackboard set`    | Create/update a blackboard                                                | `--id`, `--project`, `--conversation`, `--task`, `--background`, `--guidelines`, `--lifecycle`                   | `rbc blackboard set --role user --project acme/build --lifecycle weekly` |
| `rbc blackboard get`    | Get a blackboard by id                                                    | `--id`                                                                                                           | `rbc blackboard get --id <uuid>`                                  |
| `rbc blackboard list`   | List blackboards for a role                                               | `--role`, `--limit`, `--offset`, `--output`                                                                      | `rbc blackboard list --role user --output json`                   |
| `rbc blackboard delete` | Delete a blackboard                                                       | `--id`                                                                                                           | `rbc blackboard delete --id <uuid>`                               |
| `rbc blackboard sync`   | Sync blackboard and stickies id ↔ folder                                   | `id:<uuid> folder:<rel>`, `--dry-run`, `--delete`, `--clear-ids`, `--force-write`, `--include-archived`, id:`_` shortcut | `rbc blackboard sync id:_ folder:features`                         |
| `rbc blackboard diff`   | Show differences between id and folder                                     | `id:<uuid> folder:<rel>`, `--detailed`, `--include-archived`, id:`_` shortcut                                      | `rbc blackboard diff id:_ folder:features --detailed`             |

## Stickies

| Command                  | Purpose                                      | Keys / Options                                                                                                                                                                         | Example                                                                                                                   |
| ------------------------ | -------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `rbc stickie set`  | Create/update a stickie (note/code/metadata) | `--id`, `--blackboard <uuid>`, `--note`, `--code`, `--labels a,b`, `--priority must/should/could/wont`, `--name`, `--archived`, `--score` | `rbc stickie set --blackboard <bb> --note 'cache idea' --code $'name: CI\n…'` |
| `rbc stickie get`  | Get a stickie by id                          | `--id`                                                                                                                                                                                 | `rbc stickie get --id <uuid>`                                                                                       |
| `rbc stickie list` | List stickies (by board)                     | `--blackboard`, `--limit`, `--offset`, `--output json/table`                                                                                            | `rbc stickie list --blackboard <bb> --output json`                                                                  |
| `rbc stickie find` | Find by complex name (name/variant)          | `--name`, `--variant`, `--archived`, `--blackboard`                                                                                                                                    | `rbc stickie find --name FeatureX --variant v1 --blackboard <bb>`                                                   |

### Stickie Relations

| Command                      | Purpose                                   | Keys / Options                                                                        | Example                                                                                 |
| ---------------------------- | ----------------------------------------- | ------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| `rbc stickie-rel set`  | Create/update a relation between stickies | `--from`, `--to`, `--type INCLUDES/CAUSES/USES/REPRESENTS/CONTRASTS_WITH`, `--labels` | `rbc stickie-rel set --from <id1> --to <id2> --type uses --labels ref,dependency` |
| `rbc stickie-rel list` | List relations for a stickie              | `--id`, `--direction out/in/both`                                                     | `rbc stickie-rel list --id <uuid> --direction out`                                |
| `rbc stickie-rel get`  | Get a specific relation                   | `--from`, `--to`, `--type`, `--ignore-missing`                                        | `rbc stickie-rel get --from <id1> --to <id2> --type uses`                         |

## Conversations, Experiments, Messages

| Command                         | Purpose                                    | Keys / Options                                                                       | Example                                                                       |
| ------------------------------- | ------------------------------------------ | ------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------- |
| `rbc conversation active` | Browse recent conversations per role (TUI) | Keys: `/` search, `n` next role, `r` refresh                                         | `rbc conversation active`                                               |
| `rbc conversation set`    | Create/update a conversation               | `--id`, `--title`, `--description`, `--project`, `--role`, `--tags k=v,…`, `--notes` | `rbc conversation set --title 'Roadmap' --role user --project acme/app` |
| `rbc conversation get`    | Get a conversation by id                   | `--id`                                                                               | `rbc conversation get --id <uuid>`                                      |
| `rbc conversation list`   | List conversations                         | `--role`, `--project`, `--limit`, `--offset`, `--output`                             | `rbc conversation list --role user --output json`                       |
| `rbc experiment create`   | Create an experiment under a conversation  | `--conversation`                                                                     | `rbc experiment create --conversation <conv-uuid>`                      |
| `rbc experiment list`     | List experiments                           | `--conversation`, `--limit`, `--offset`                                              | `rbc experiment list --conversation <conv-uuid>`                        |
| `rbc message set`         | Create a message (stdin body)              | `--experiment`, `--title`, `--tags`, `--role`                                        | `echo 'hello' \| rbc message set --experiment <exp> --title Greeting`   |
| `rbc message list`        | List messages                              | `--role`, `--experiment`, `--task`, `--status`, `--limit`, `--offset`, `--output`    | `rbc message list --role user --output json`                            |

## Testcases

| Command                     | Purpose                                        | Keys / Options                                                                                                                                         | Example                                                                                 |
| --------------------------- | ---------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------- |
| `rbc testcase active` | View active testcases for a conversation (TUI) | Keys: `n` cycle experiments, `e` errors-only, `r` refresh                                                                                              | `rbc testcase active --conversation <conv-uuid>`                                  |
| `rbc testcase create` | Create a testcase row                          | `--title`, `--role`, `--experiment`, `--status OK/KO/TODO`, `--level h1..h6`, `--name`, `--pkg`, `--classname`, `--file`, `--line`, `--execution-time` | `rbc testcase create --title 'go vet' --role user --experiment <exp> --status OK` |
| `rbc testcase list`   | List testcases                                 | `--role`, `--experiment`, `--status`, `--limit`, `--offset`, `--output`                                                                                | `rbc testcase list --role user --experiment <exp> --output json`                  |

## Tasks & Workflows

| Command                      | Purpose                                          | Keys / Options                                                                                                                                | Example                                                                                                  |
| ---------------------------- | ------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| `rbc workflow set`     | Create/update a workflow                         | `--name`, `--title`, `--description`, `--role`, `--notes`                                                                                     | `rbc workflow set --name ci-test --title 'CI Test' --role user`                                    |
| `rbc task set`         | Create/update a task (optionally as replacement) | `--workflow`, `--command`, `--variant`, `--role`, `--title`, `--description`, `--shell`, `--replaces`, `--replace-level`, `--replace-comment` | `rbc task set --workflow ci-test --command unit --variant go --role user --title 'Run Unit Tests'` |
| `rbc task list`        | List tasks                                       | `--role`, `--workflow`, `--limit`, `--offset`, `--output`                                                                                     | `rbc task list --role user --output json`                                                          |
| `rbc task latest`      | Get latest task variant                          | `--variant`, `--from-id`                                                                                                                      | `rbc task latest --variant unit/go`                                                                |
| `rbc task next`        | Compute next version from an id                  | `--id`, `--level patch/minor/major/latest`                                                                                                    | `rbc task next --id <task-id> --level minor`                                                       |
| `rbc task run`         | Run a task against a queue/tooling (if wired)    | task-specific                                                                                                                                 | `rbc task run --workflow ci-test --command unit`                                                   |
| `rbc task script add`  | Attach a script to a task                        | `--task`, `--script`, `--name`, `--alias`                                                                                                     | `rbc task script add --task <task-id> --script <script-id> --name build`                           |
| `rbc task script list` | List task scripts                                | `--task`                                                                                                                                      | `rbc task script list --task <task-id>`                                                            |

## Projects, Stores, Topics, Roles, Tags, Tools, Workspaces

| Command                   | Purpose                            | Keys / Options                                                                                                                                     | Example                                                                                         |
| ------------------------- | ---------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| `rbc project set`   | Create/update a project            | `--name`, `--role`, `--description`, `--notes`, `--tags`                                                                                           | `rbc project set --name acme/build-system --role user --description 'Build & CI'`         |
<!-- store commands removed -->
| `rbc topic set`     | Create/update a topic              | `--name`, `--role`, `--title`, `--description`, `--tags`                                                                                           | `rbc topic set --name devops --role user --title DevOps`                                  |
| `rbc role set`      | Create/update a role               | `--name`, `--title`, `--description`, `--notes`                                                                                                    | `rbc role set --name rbctest-user --title 'RBCTest User'`                                 |
| `rbc tag set`       | Create/update a tag                | `--name`, `--title`, `--role`                                                                                                                      | `rbc tag set --name priority-high --title 'High Priority' --role user`                    |
| `rbc tool set`      | Create/update tool config for LLMs | `--name`, `--provider`, `--model`, `--api-key-secret`, `--temperature`, `--max-output-tokens`, `--top-p`, `--settings`                             | `rbc tool set --name openai:gpt4o --provider openai --model gpt-4o`                       |
| `rbc workspace set` | Create/update a workspace          | `--role`, `--project`, `--description`, `--tags`, `--build-script-id`                                                                              | `rbc workspace set --role user --project acme/build-system --description 'Local build'`   |

## Scripts

| Command                 | Purpose                             | Keys / Options                                                            | Example                                                                                  |
| ----------------------- | ----------------------------------- | ------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `rbc script set`  | Create/update a script (stdin body) | `--role`, `--title`, `--description`, `--name`, `--variant`, `--archived` | `echo '#!/usr/bin/env bash' \| rbc script set --role user --title 'Unit: go test'` |
| `rbc script list` | List scripts                        | `--role`, `--limit`, `--offset`, `--output`                               | `rbc script list --role user --output json`                                        |
| `rbc script find` | Find by complex name                | `--name`, `--variant`, `--archived`                                       | `rbc script find --name 'Unit: go test' --variant ''`                              |

## Queue & Messages

| Command                | Purpose                | Keys / Options                                 | Example                                                            |
| ---------------------- | ---------------------- | ---------------------------------------------- | ------------------------------------------------------------------ |
| `rbc queue add`  | Add a queue item       | `--description`, `--status`, `--why`, `--tags` | `rbc queue add --description 'build image' --status PENDING` |
| `rbc queue peek` | Peek next item         | —                                              | `rbc queue peek`                                             |
| `rbc queue take` | Take/process next item | —                                              | `rbc queue take`                                             |
| `rbc queue size` | Queue size             | —                                              | `rbc queue size`                                             |

## Server & DB

| Command                          | Purpose                      | Keys / Options                          | Example                                            |
| -------------------------------- | ---------------------------- | --------------------------------------- | -------------------------------------------------- |
| `rbc server start`         | Start HTTP/GRPC server       | `--config`                              | `rbc server start`                           |
| `rbc server status`        | Show server status           | —                                       | `rbc server status`                          |
| `rbc server reload_config` | Reload config                | —                                       | `rbc server reload_config`                   |
| `rbc server stop`          | Stop server                  | —                                       | `rbc server stop`                            |
| `rbc db scaffold`          | Create roles, schema, grants | `--all`, `--yes`                        | `rbc db scaffold --all --yes`                |
| `rbc db reset`             | Reset database (destructive) | `--force`, `--drop-app-role=true/false` | `rbc db reset --force --drop-app-role=false` |
| `rbc db backup`            | Backup database              | `--schema`                              | `rbc db backup --schema app`                 |
| `rbc db restore`           | Restore from backup          | `--schema`                              | `rbc db restore --schema app`                |
| `rbc db status`            | DB status                    | —                                       | `rbc db status`                              |
| `rbc db search`            | Search DB (utility)          | SQL input                               | `rbc db search 'SELECT 1'`                   |
| `rbc db count`             | Row counts by table          | —                                       | `rbc db count`                               |

## Snapshots

| Command                      | Purpose                              | Keys / Options                                | Example                                                               |
| ---------------------------- | ------------------------------------ | --------------------------------------------- | --------------------------------------------------------------------- |
| `rbc snapshot backup`  | Create a snapshot                    | `--description`, `--who`, `--json`            | `rbc snapshot backup --description 'nightly' --who user --json` |
| `rbc snapshot list`    | List snapshots                       | `--limit`, `--offset`                         | `rbc snapshot list --limit 5`                                   |
| `rbc snapshot show`    | Show a snapshot                      | `--id`                                        | `rbc snapshot show --id <uuid>`                                 |
| `rbc snapshot verify`  | Verify snapshot                      | `--id`                                        | `rbc snapshot verify --id <uuid>`                               |
| `rbc snapshot restore` | Restore snapshot (dry-run or append) | `--id`, `--mode dry/append`                   | `rbc snapshot restore --id <uuid> --mode append`                |
| `rbc snapshot prune`   | Prune snapshots                      | `--older-than`, `--schema`, `--yes`, `--json` | `rbc snapshot prune --older-than 30d --schema app --yes --json` |

---

Tips:

- Add `--output json` where available to script with JSON.
- Most setters accept optional fields; omitted flags leave values unchanged on update.
- TUIs list key bindings at the top; use `q` to quit.
