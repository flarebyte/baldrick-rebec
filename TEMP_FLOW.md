# Temporary Flow: CLI → Server → DB

This document summarizes the intended data flow from the user CLI to the database, and outlines best‑practice steps to implement the flow in a modular, safe, and consistent way. We will use this file to drive each flow implementation.

## High‑Level Flow

- Schema lifecycle
  - Schema creation/updates are done via admin DB commands (direct DB access). Example: `rbc admin db scaffold --all --yes` or `rbc admin db init`.
  - Day‑to‑day data population and reads go through the server (not direct DB from the CLI).

- Runtime request flow (example: `rbc admin message send`)
  1) CLI parses flags/args (e.g., `--conversation`, `--attempt`, `--to`, etc.).
  2) CLI sends a gRPC request to the server with a typed payload representing the operation (e.g., `MessageSendRequest`).
  3) Server receives the request and performs:
     - Validation (required fields present, types/constraints OK).
     - Enrichment (timestamps, canonicalization, hashes like content SHA-256, defaults, correlation IDs, actor info).
  4) Server calls the appropriate DAO (payload + action, e.g., `MessageDAO.Send`) to persist to the DB.
  5) DAO writes the data; a log DAO (or event DAO) records the audit/event record.
  6) Server returns a typed response to the CLI (`MessageSendResponse`) with success, generated IDs (if any), and key fields; or an error with a stable error code and message.

## Recommended Structure

- CLI layer (`cmd/...`)
  - Parse flags/args; minimal logic.
  - Construct gRPC request and print well‑formed output (JSON/lines) based on server response.

- Protobuf/gRPC (`pb/...`)
  - Define service methods and messages (request/response) with clear, stable fields.
  - Keep field names close to DB or domain names, but not DB‑leaky.

- Server (`internal/server`)
  - Service handlers that:
    - Validate & normalize requests.
    - Enrich payloads (timestamps, hashes, actor/session info).
    - Invoke domain services or DAOs.
    - Map errors to typed gRPC status codes.
  - Add interceptors for logging, tracing, auth, timeouts and panic recovery.

- Domain/Services (`internal/service`)
  - Optional layer for orchestration and cross‑DAO logic.
  - Encapsulate business rules (idempotency, dedup, workflows).

- Data Access (`internal/dao/postgres`)
  - DAOs contain SQL only; parameterized queries via pgx, no string concatenation with untrusted input.
  - Small, focused methods: EnsureSchema, Insert/Update/Get, and transactional helpers.

- Config (`internal/config`)
  - Centralized config loading and validation.

## Naming & Conventions

- Commands
  - CLI verbs match server RPCs: `message send`, `task create`, `workflow create`, etc.
  - Prefer `kebab-case` flags and snake_case DB columns.

- Protobuf
  - Messages: `MessageSendRequest`, `MessageSendResponse`, `TaskCreateRequest`, ...
  - Services: `AdminService`, `IngestService`, or `MessageService` as appropriate.
  - Use consistent field names (e.g., `conversation_id`, `attempt_id`, `task_id`).

- DAOs
  - Files per entity/action: `events.go`, `content.go`, `workflows.go`, `tasks.go`.
  - Functions: `InsertMessageEvent`, `GetMessageEventByID`, `UpsertTask`, etc.
  - Return concrete structs for rows; use `sql.Null*` for nullable fields.

## Safety & Injection Mitigation

- SQL parameterization
  - Always use `$1, $2, ...` placeholders with pgx; never interpolate user input.
  - If dynamic identifiers are required (rare), whitelist and validate against a strict regex; avoid passing raw identifiers from clients.

- Canonicalization & hashing
  - Normalize message content (line endings, trim) before hashing for dedup.

- Validation
  - Validate inputs at server boundary; reject early with meaningful error codes.
  - Enforce business constraints (e.g., semver pattern) before DB calls.

- Transport/auth
  - Use TLS for gRPC beyond localhost; add auth/interceptors when multi‑tenant.
  - Apply timeouts and deadlines for every RPC and DB call.

## Implementation Steps (Incremental)

1) Protobuf
   - Define `MessageService.Send` with request/response messages.
   - Include content, conversation_id, attempt_id, recipients, tags, and optional task reference.

2) Server handler
   - Validate required fields; enrich with timestamps and content hash.
   - Call DAOs within a context with timeout.

3) DAOs
   - Content DAO: `PutMessageContent` (dedupe by hash) and `GetMessageContent`.
   - Events DAO: `InsertMessageEvent` with `task_id` nullable FK.
   - Log/Audit: record the operation (could reuse events or a separate log table later).

4) CLI command
   - Map flags to the gRPC request; send stdin as content when applicable.
   - Print the response (IDs, counts) or a clear error.

5) Observability
   - Add request IDs, structured logs, basic metrics (latency, counts, errors).

6) Tests
   - Unit tests for handler validation/enrichment logic.
   - Integration tests for DAOs (transaction-per-test with rollback).

## Error Handling Guidelines

- Use gRPC status codes (InvalidArgument, NotFound, AlreadyExists, Internal, Unavailable).
- Return a stable error code string in the response body when helpful for CLI/UI.
- Log server‑side errors with context (request ID, user/actor).

## Idempotency & Retries

- Prefer idempotent server methods where feasible (e.g., dedupe on content hash + unique keys).
- When retried by clients, handlers should avoid double‑writes (unique constraints help).

## Example Mapping (Message Send)

- CLI: `echo "Hello" | rbc admin message send --conversation c1 --attempt a1 --to bob --tags t1,t2`
- Request: `{content, conversation_id, attempt_id, recipients:[bob], tags:[t1,t2]}`
- Server enrichment: `{received_at=now, content_hash=sha256(canon(content))}`
- DAOs: `PutMessageContent(content_hash, content)`, then `InsertMessageEvent(task_id?, ... , tags, meta)`
- Response: `{ok:true, content_id, event_id}` or `{ok:false, error_code, message}`

---

This TEMP_FLOW is intended as a pragmatic contract to guide implementation. As flows mature, we can move the content into the main README and service design docs.

