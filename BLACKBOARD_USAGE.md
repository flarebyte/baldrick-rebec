Blackboards — Fast CLI Skill

This is a concise, AI/human‑friendly recipe to create and use a blackboard with the CLI. It covers creating the data model, adding stickies, and syncing to/from a folder.

Prerequisites

- DB configured in `~/.baldrick-rebec/config.yaml`.
- A role exists (examples use `user`).

Tip: Most commands accept `--role`. Replace values to fit your setup.

1. Create a Blackboard

- Create a blackboard (optionally link a project and set lifecycle):
  `rbc blackboard set --role user --project acme/build-system --background "Ideas board" --guidelines "Keep concise; tag priority" --lifecycle weekly`
- List blackboards and capture your blackboard id:
  `rbc blackboard list --role user --output json`

Create via YAML (optional)

- Prepare a `blackboard.yaml` (project must already exist for the role if provided):

  role: user
  project: acme/complete
  background: Created via YAML
  guidelines: From YAML
  lifecycle: weekly

- Pipe it to the CLI (flags override YAML):
  `cat blackboard.yaml | rbc blackboard set --cli-input-yaml`

  Notes: `id` is optional (omit to create). Valid YAML keys: id, role, conversation_id, project, task_id, background, guidelines, lifecycle.

2. Create / Update / Delete Stickies (CLI)

- Create a stickie on a blackboard (by id):
  `rbc stickie set --blackboard <BOARD_ID> --note "Evaluate CI caching for go build" --labels idea,devops --priority could --name "DevOps Caching"`
- Update a stickie by id (change any field):
  `rbc stickie set --id <STICKIE_ID> --note "Refine plan; prototype in a branch"`
- Delete a stickie by id:
  `rbc stickie delete --id <STICKIE_ID> --force`
- List or find stickies:
  `rbc stickie list --blackboard <BOARD_ID> --output json`
  `rbc stickie find --name "DevOps Caching" --blackboard <BOARD_ID>`

3. Sync id ↔ folder
   The sync command moves a blackboard’s content between a DB id and a local relative folder.

General shape:

- `rbc blackboard sync id:<BOARD_ID> folder:<RELATIVE_PATH> [--dry-run] [--delete] [--clear-ids]`
- `rbc blackboard sync folder:<RELATIVE_PATH> id:<BOARD_ID> [--dry-run]`

Not allowed: `id→id` and `folder→folder` (returns an error).

Shortcuts
- When using folder paths that contain a `blackboard.yaml`, you can use `_` as a
  placeholder for the id and it will be resolved from the YAML:
  - `rbc blackboard sync id:_ folder:<RELATIVE_PATH>` reads the id from `<RELATIVE_PATH>/blackboard.yaml`.
  - `rbc blackboard sync folder:<RELATIVE_PATH> id:_` reads the id from `<RELATIVE_PATH>/blackboard.yaml`.
  - Errors if `blackboard.yaml` is missing or does not contain an `id:`.

A) Export: id → folder

- `rbc blackboard sync id:<BOARD_ID> folder:board/ideas`
- Creates/updates:
  - `board/ideas/blackboard.yaml`
  - `board/ideas/<STICKIE_ID>.stickie.yaml`
- Rules and options:
  - Updated decision: uses `updated` timestamps (DB vs destination file) to skip or write.
  - `--delete`: removes extra `*.stickie.yaml` files present in the folder but not in the DB.
  - `--dry-run`: only prints planned actions.
  - `--clear-ids`: omits the `id` field inside stickie YAML files (filenames still include ids).

B) Import: folder → id

- `rbc blackboard sync folder:board/ideas id:<BOARD_ID>`
- Reads `*.stickie.yaml` in the folder. One folder = one blackboard.
- Rules and safety:
  - To update: include `id:` inside the YAML that already exists on that blackboard.
  - The tool compares content hashes (topic/name/note/code/labels/priority/score/archived). If changed, it updates and DB sets `updated=now()` automatically.
  - To create: omit `id:` in YAML; a new UUID is assigned on insert.
  - Security guard: if a YAML has an `id` that does not exist for that blackboard, sync fails.
  - `updated` values in YAML are ignored on import.
  - `blackboard_id` is not used in YAML (one folder per board).
  - `--dry-run`: prints planned creates/updates without writing.

Tip: Keep folder paths relative (e.g., `board/ideas`). The export step will create the folder if missing.

4. Handy Checks

- Show a blackboard: `rbc blackboard get --id <BOARD_ID>`
- Show a stickie: `rbc stickie get --id <STICKIE_ID>`
- Count entities by role: `rbc db count --per-role`

You now have the essentials to create, edit, and sync blackboards and stickies efficiently.
