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
