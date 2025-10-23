# Architecture decision records

An [architecture
decision](https://cloud.google.com/architecture/architecture-decision-records)
is a software design choice that evaluates:

- a functional requirement (features).
- a non-functional requirement (technologies, methodologies, libraries).

The purpose is to understand the reasons behind the current architecture, so
they can be carried-on or re-visited in the future.

## Initial idea

**Problem Statement**
Design a modular multi-agent chat and execution coordination system written in Go. Agents interact primarily through stdout/stderr channels, exchanging structured messages and executing shell commands. The system coordinates these interactions via a central server that tracks workflow progress, persists message history, and responds to CLI commands or background listeners.

---

**Use Cases**

1. A CLI client sends a shell command message to the server, which is routed to a specific agent (human or machine) in a conversation.
2. Multiple agents exchange messages within a conversation and collaborate through a shared stdout/stderr history.
3. A listener polls or is notified of new messages with specific tags, and conditionally triggers shell commands.
4. A developer runs a task from a YAML-defined workflow stored locally using the CLI.
5. The server records task state transitions and workflow progress using a PostgreSQL database.
6. A vector-based OpenSearch instance is used to store and retrieve message content for similarity or embedding-based analysis.
7. Conversations and attempts are tracked, joined, or left via CLI.
8. Old messages are deleted based on age using a CLI-triggered cleanup command.
9. A GUI frontend (future desktop app) queries the server for workflows and their states for visualization.

---

**Edge Cases**

1. A malformed YAML file causes invalid workflow task definitions.
2. A shell command spawns another workflow leading to recursive triggers.
3. A message with invalid or missing metadata fields (e.g., missing attempt ID) is submitted.
4. A workflow executes multiple branches in parallel, where one path times out.
5. The listener triggers the same shell command repeatedly due to missing loop protection.
6. Message routing fails when `to-recipients` is ambiguous or empty.
7. The vector database is unavailable during a message indexing operation.
8. Multiple CLI instances try to modify the same conversation or attempt concurrently.
9. Messages are sent with the same `attempt-id` but from different workflows or conversations.

---

**Limitations / What Should Not Be Solved**

1. Do not implement the desktop GUI yet; only anticipate its interaction needs (e.g., local server API).
2. Do not support remote server deployments; assume all modules run locally or in simple container setups.
3. No authentication or access control for now; assume trusted local usage.
4. Do not handle dynamic plugin execution; only shell commands and YAML-defined tasks.
5. Avoid attempting complex DAG scheduling; evaluate immediate next tasks only in a linear or conditional path.
6. Do not perform advanced loop detection; basic cycle-prevention mechanisms only.
7. No versioning or rollback of workflows or messages.
8. Do not implement concurrent message sending within the same CLI session; single-threaded usage expected.

---

**Message Fields**

- `stdin`: Text input or command input
- `stdout`: Command output or agent response
- `status`: Execution status (e.g., started, in-progress, finished)
- `title`: Short label for the message
- `level`: Hierarchical depth (e.g., h1, h2, h3)
- `shell_command`: Command to be executed
- `prompt`: Instruction or question for the recipient
- `from`: Sender identifier (agent or user)
- `to_recipients`: Target recipients (list or identifier)
- `conversation_id`: Link to conversation thread
- `attempt_id`: Execution or run instance
- `tags`: Categorization or reaction triggers
- `description`: Longer explanation or context
- `goal`: Intended outcome of the message
- `message_profile`: Reference to profile config (see below)
- `timeout`: Max allowed duration for execution or response

---

**Message Profile (referenced config)**

- `is_vector`: Whether content is indexed in vector DB
- `description`: Profile purpose
- `goal`: Functional intent
- `tags`: Relevant for filtering, routing, or reactions
- `timeout`: Default timeout if not set per message
- `sensitive`: Indicates if message should be excluded from logging/indexing

**Language: Go**

- **Why**: Concurrency support (goroutines, channels) for handling multi-agent messaging and polling; strong ecosystem for CLI tools and gRPC; suitable for building efficient, lightweight servers.

---

**Communication Protocol: Protocol Buffers + gRPC (optional)**

- **Why**: Efficient serialization for structured messages; cross-language compatibility for future integration (e.g., Flutter GUI); gRPC offers bi-directional streaming and strong typing, useful if a push-based listener is implemented.

---

**CLI Tool (Go)**

- **Why**: Local user interface for interacting with the server (start/stop, send messages, trigger workflows); Go's flag and Cobra libraries support robust CLI design; single binary simplifies distribution.

---

**Data Storage**

1. **Relational Database: PostgreSQL**

   - **Why**: Stores structured data like messages, conversations, attempts, and workflow/task statistics; supports complex queries, transactions, and indexing.
   - **Use**: Workflow progress tracking, state transitions, conversation/attempt management.

2. **Search/Vector Database: OpenSearch**

   - **Why**: Stores unstructured or semi-structured content (e.g., stdout/stderr, prompts); supports full-text search and vector similarity queries.
   - **Use**: Future semantic search, message deduplication, embedding comparisons.

---

**Workflow Definition: YAML (GitHub Actions-like syntax)**

- **Why**: Familiar structure for devs; human-readable and easy to define tasks declaratively; supports reusable task configuration and branching logic.

---

**Configuration**

- **Home Config**: Stores global CLI settings (e.g., database credentials, ports).
- **Project Folder Config**: Contains multiple YAML files for workflows.
- **Why**: Separation of concerns—global vs. project-level config; predictable structure for tooling.

---

**Listener**

- **Why**: Background component to monitor conversations or messages, triggering actions on specific tags; essential for automation agents.
- **Tech**: Go routine with polling (or optional gRPC stream if push is feasible and simple).

---

**Future Tech: Flutter Desktop App**

- **Why**: Cross-platform UI to visualize workflows, task states, and message history; connects to the same local server via API.

### Commands

| Command                          | Description                                                   | Options / Notes                        |
| -------------------------------- | ------------------------------------------------------------- | -------------------------------------- |
| `rbc admin server start`         | Start the local server.                                       | `--detach` to run in background.       |
| `rbc admin server stop`          | Stop the running server gracefully.                           | —                                      |
| `rbc admin server status`        | Show current server state and activity.                       | —                                      |
| `rbc admin server reload-config` | Reload config files without restart.                          | —                                      |
| `rbc admin db init`              | Initialize or migrate relational and search/vector databases. | —                                      |
| `rbc admin db dump`              | Export database contents to file.                             | `--output=<path>`                      |
| `rbc admin db clean-old`         | Delete messages older than a set number of days.              | `--days=<n>`                           |
| `rbc admin conversation list`    | List all existing conversations.                              | —                                      |
| `rbc admin conversation join`    | Join a specific conversation.                                 | `--id=<id>`                            |
| `rbc admin conversation leave`   | Leave a specific conversation.                                | `--id=<id>`                            |
| `rbc admin attempt list`         | List all attempts (optionally by conversation).               | `--conversation-id=<id>`               |
| `rbc admin attempt join`         | Join an existing attempt.                                     | `--id=<id>`                            |
| `rbc admin attempt leave`        | Leave a specific attempt.                                     | `--id=<id>`                            |
| `rbc admin message send`         | Send a structured message.                                    | `--file=<path>` or `--data=<json>`     |
| `rbc admin stats show`           | Display usage statistics for workflows and tasks.             | —                                      |
| `rbc admin loops check`          | Detect potential infinite loops or reaction cycles.           | —                                      |
| `rbc admin listener start`       | Start a listener reacting to tags.                            | `--detach`, `--react-tags=<tag1,tag2>` |

## **Data Model Specification**

### **Overview**

The system separates immutable message **content** from mutable **ingest metadata**:

- **`messages_content` (OpenSearch)** → stores each unique message body once.
- **`messages_events` (PostgreSQL)** → records every ingest or processing event referencing that content.

This design:

- Eliminates duplicate storage of identical messages.
- Keeps OpenSearch optimized for search & retrieval.
- Uses PostgreSQL for integrity, history, and joins.
- Enables efficient synchronization and audit trails.

---

### **1. OpenSearch Index — `messages_content`**

**Purpose:**
Store and index unique message content for search, deduplication, and retrieval.

**Index name:** `messages_content`

**Document ID:**
`_id = SHA256(<canonicalized_message_body>)`

**Mappings (simplified):**

```json
{
  "mappings": {
    "properties": {
      "message_id": { "type": "keyword" }, // same as _id
      "content": { "type": "text" }, // full searchable text
      "content_type": { "type": "keyword" }, // e.g. "email", "report"
      "language": { "type": "keyword" },
      "metadata": { "type": "object", "enabled": true }
    }
  },
  "settings": {
    "index.lifecycle.name": "messages-content-ilm", // rollover & retention
    "refresh_interval": "1s"
  }
}
```

**Ingest behavior:**

- Hash computed client-side over normalized `content` (excluding volatile metadata).
- 409 conflict ⇒ content already stored → skip body re-upload.

---

### **2. PostgreSQL Table — `messages_events`**

**Purpose:**
Store ingestion events, metadata, and relational information tied to a message content hash.

**Table definition:**

```sql
CREATE TABLE messages_events (
    id BIGSERIAL PRIMARY KEY,
    content_id TEXT NOT NULL,             -- FK to messages_content._id
    source TEXT NOT NULL,                 -- e.g. pipeline name, API client
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    status TEXT DEFAULT 'ingested',       -- ingest / processed / error
    tags TEXT[] DEFAULT '{}',
    meta JSONB DEFAULT '{}',
    UNIQUE (content_id, source, received_at)
);

CREATE INDEX ON messages_events (content_id);
CREATE INDEX ON messages_events (received_at DESC);
CREATE INDEX messages_events_tags_gin ON messages_events USING GIN (tags);
CREATE INDEX messages_events_meta_gin ON messages_events USING GIN (meta);
```

**Behavior:**

- Each new ingest → one row.
- Enforces unique combination if desired (`content_id`, `source`, `received_at`).
- Supports time-partitioning for retention and performance.

---

### **3. Write Flow**

1. **Client receives new message**

   - Canonicalize body (e.g., remove timestamps).
   - Compute `SHA256` → `content_id`.

2. **Store content in OpenSearch**

   ```http
   POST messages_content/_create/<content_id>
   { "content": "...", "content_type": "email", "language": "en" }
   ```

   - If conflict (409): skip indexing; already present.

3. **Record ingest event in PostgreSQL**

   ```sql
   INSERT INTO messages_events (content_id, source, tags, meta)
   VALUES ('<content_id>', 'ingest-A', ARRAY['foo'], '{"ingest_time":"2025-10-13T12:00Z"}');
   ```

---

### **4. Query Patterns**

| Use Case                      | Query Location     | Example                                                        |
| ----------------------------- | ------------------ | -------------------------------------------------------------- |
| Full-text search, similarity  | OpenSearch         | `content:"urgent update"`                                      |
| Find all events for a message | PostgreSQL         | `SELECT * FROM messages_events WHERE content_id = ...`         |
| Count ingests per source      | PostgreSQL         | `SELECT source, COUNT(*) FROM messages_events GROUP BY source` |
| Re-index by recent sources    | Postgres → OS join | App query Postgres → bulk fetch from OpenSearch                |

---

### **5. Lifecycle & Retention**

- **OpenSearch ILM**: rollover by size/age, retain 180 days (content rarely changes).
- **PostgreSQL partitioning**: monthly partitions on `received_at`, drop/archive old partitions as needed.

## Agent task definition

**Problem Definition:**

A system must define and register _task agents_ in a PostgreSQL database. Each task agent represents a defined executable unit of work that can be uniquely identified, versioned, and described through structured metadata. The system should ensure that agents can be uniquely referenced, queried, and updated in a consistent manner.

**Core Requirements:**

- Each task agent must have a _unique name_ and a _version_. Together, they act as the unique identifier (similar to a package registry like npm).
- The name may follow a convention such as `company/name/language`, where:

  - `company` identifies the organization or namespace.
  - `name` identifies the agent itself.
  - `language` identifies the primary implementation or runtime environment.

- The version follows semantic or incremental versioning rules to distinguish agent revisions.

**Stored Definition (fields to persist):**

- name (string)
- version (string)
- title (string)
- description (text)
- goal (text)
- prompt (text)
- shell_command (text)
- tags (array or text)
- timeout (integer, representing seconds)
- metrics (structured data; includes cost per token, quality score, etc.)
- created_at and updated_at timestamps (auto-generated)

**Behavior and Use Cases:**

1. Register a new task agent with full metadata into the database.
2. Retrieve an existing agent by name and version.
3. List all versions for a given agent name.
4. Update an existing agent’s definition when a new version is released.
5. Validate that names and versions are unique and comply with naming conventions.
6. Allow filtering by tags or language when querying agents.

**Edge Cases:**

- Attempting to register an agent with an existing (name, version) pair should be rejected.
- Missing required fields (name, version, or title) should cause validation failure.
- Invalid name format (not following convention or containing disallowed characters).
- Version rollback attempts (registering an older version number after a newer one).
- Excessively large descriptions or prompts should be rejected to prevent data overflow.

**Example Contexts:**

- A company registers `acme/text-summarizer/python` v1.0.0 describing a summarization task.
- Another registers `openai/code-reviewer/js` v2.1.1 for code analysis.
- Updating `acme/text-summarizer/python` to v1.1.0 with improved prompt and new metrics.

## Workflow configuration

**Problem Definition:**

A system must manage _documentation metadata_ for individual tasks and for _workflows_ composed of tasks. Both types of documentation must be stored and retrievable from a PostgreSQL database. Locally, workflow definitions should be manageable through YAML configuration files, supporting version tracking, environment profiles, and update checks similar to a package manager (e.g., npm).

---

**Documentation Metadata Requirements:**

Each **task documentation** must include:

- title (string)
- description (text)
- goal (text)
- links (list of objects containing `title` and `url`)

Each **workflow documentation** must include:

- title (string)
- description (text)
- tasks (list of task references: name and version)

Both documentation types should be persistable in PostgreSQL and retrievable by ID, name, or related entity.

---

**Workflow Management Requirements:**

A **workflow** represents a sequence or collection of registered tasks.
Each workflow definition (in database or local YAML) must specify:

- name (string)
- profile (string, e.g., dev-ai, dev-basic, qa, ci)
- tasks (key-value mapping of `task_name`: `version`)
- optional metadata (title, description)

Local YAML format example:

```
workflow:
  name: example-workflow
  profile: dev-ai
  tasks:
    summarizer: "1.2.1"
    translator: "1.3.0"
```

---

**Capabilities and Use Cases:**

1. Store documentation metadata for tasks and workflows in PostgreSQL.
2. Retrieve, update, or delete documentation records by name or workflow reference.
3. Load a local YAML workflow file and validate its structure and task version references.
4. Check for outdated task versions based on stored registry data (compare YAML vs DB).
5. Upgrade tasks within a workflow definition to the latest available versions.
6. Support multiple workflow profiles, each maintaining its own task versions and configurations.
7. Allow exporting or syncing local YAML workflow definitions with database-stored documentation.

---

**Edge Cases:**

- Missing or invalid links (no title or malformed URL).
- Inconsistent task versions (referencing a task that does not exist or is unregistered).
- Conflicting task definitions between profiles.
- YAML parsing errors due to indentation or type mismatch.
- Workflow referencing tasks without associated documentation.
- Attempt to upgrade tasks beyond available versions.

---

**Example Contexts:**

- A YAML workflow for `qa` uses task versions optimized for testing (`summarizer: 1.2.0`), while `dev-ai` uses newer experimental ones (`summarizer: 1.3.0`).
- The system checks the registry and flags that `summarizer 1.2.0` is outdated.
- The database stores the full documentation for each task and workflow, allowing browsing or linking to related docs via stored URLs.

This section defines the storage, retrieval, and version-checking context for documentation metadata and workflow definitions, without specifying implementation logic or schema.

## Spec: Workflow and Task Tables in PostgreSQL (Go with pgx)

**Problem Overview**
Define two PostgreSQL tables to represent workflows and their associated tasks. Workflows group related tasks. Tasks belong to workflows and contain structured metadata including semantic versioning and command execution definitions. The system must support uniquely versioned tasks per workflow, markdown content, and future execution interfaces like `workflow task`.

**Workflow Table**
Represents a named collection of tasks, including metadata.

- Fields:

  - `name`: string, unique identifier
  - `title`: string, human-readable label
  - `description`: string, plain text
  - `created`: timestamp with timezone
  - `updated`: timestamp with timezone
  - `notes`: string, markdown-formatted

**Task Table**
Represents a versioned execution unit under a workflow.

- Fields:

  - `name`: string, required
  - `title`: string, human-readable label
  - `description`: string, plain text
  - `motivation`: string, purpose or context
  - `version`: string, must follow semantic versioning (e.g. 1.0.0)
  - `created`: timestamp with timezone
  - `notes`: string, markdown-formatted
  - `shell`: string, shell environment (e.g. "bash", "python")
  - `run`: string, command to execute
  - `workflow_id`: foreign key to Workflow (`name`)

**Use Cases**

- Insert a new workflow with metadata.
- Insert a new task tied to a specific workflow and version.
- Retrieve all tasks under a workflow by name (e.g. list tasks in `test`).
- Retrieve a task by workflow and name (e.g. run `test unit`).
- Display markdown notes for a task or workflow.
- Store command metadata per task for automated runners.

**Edge Cases**

- Attempting to insert a task with the same name and version under a workflow must fail.
- A task with the same name but a different version is valid within the same workflow.
- Workflow names are globally unique; task names are only unique within workflow+version.
- Markdown notes may contain special characters or very large text blocks.
- Tasks may use non-standard shells (e.g. `zsh`, `sh`, `powershell`).
