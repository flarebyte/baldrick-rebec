### PostgreSQL Access & Admin in Go — Guidelines Checklist

Here is the **PostgreSQL Access & Admin Guidelines Checklist**, rewritten with the assumption that **only `pgx`** is used (no other third-party dependencies).

---

## PostgreSQL Access & Admin in Go — Guidelines Checklist (Using Only `pgx`)

### Architecture and Design

- [ ] Use the DAO (Data Access Object) pattern to isolate SQL and DB-specific logic from core domain logic.
- [ ] Keep domain-level interfaces abstracted from the database implementation to improve testability and separation of concerns.
- [ ] Avoid introducing the repository pattern unless the abstraction is clearly justified; prefer simplicity.
- [ ] Manually write SQL queries with `pgx` rather than using a query builder or ORM to retain control and clarity.
- [ ] Maintain a strict package boundary between DB access code and business logic (e.g., `internal/db`, `internal/service`).
- [ ] Use migrations through external tooling (not implemented in Go code) to manage schema changes, or write a minimal internal SQL runner if absolutely needed.

### Security and Safety

- [ ] Always use parameterized queries (`$1`, `$2`, etc.) to prevent SQL injection.
- [ ] Never construct SQL statements with string interpolation or concatenation, even for internal tools or admin operations.
- [ ] If dynamic SQL is required (e.g., column names), validate against a whitelist and use `pgx.Identifier.Sanitize()` or equivalent to escape.
- [ ] Restrict DB user permissions: use separate roles for the application and for schema migration or admin access.
- [ ] Avoid embedding database credentials in code; retrieve them from environment variables or secure injection mechanisms.
- [ ] If using Docker for Postgres, bind the container to localhost only and avoid exposing the DB port to external interfaces.
- [ ] Use non-root users in Docker images where applicable and enforce minimal privileges.
- [ ] Log failed connection attempts and other abnormal events for audit and operational security.

### Connection Management

- [ ] Centralize DB configuration in a struct populated from environment variables (`host`, `port`, `user`, `password`, `dbname`, `sslmode`, etc.).
- [ ] Use `pgxpool.Pool` to manage connections efficiently and safely.
- [ ] Set pool configuration explicitly (`MaxConns`, `MinConns`, `MaxConnLifetime`, etc.) based on workload and deployment environment.
- [ ] Support connection retries on startup with exponential backoff to handle container boot delays or transient failures.
- [ ] Close the pool cleanly on shutdown (`defer pool.Close()`).

### Querying and Execution

- [ ] Use `context.Context` with all `pgx` operations to enable timeouts, cancellation, and tracing.
- [ ] Ensure that `Rows` are closed properly with `defer rows.Close()` to prevent resource leaks.
- [ ] Handle NULLs in query results using Go’s `pgtype` types or explicit NULL wrappers where needed.
- [ ] Prefer scanning into known struct types with clear mapping between columns and fields, rather than using generic maps.
- [ ] Validate and sanitize all user input before using in application logic, even if parameterized queries are used.

### Testing and Environments

- [ ] Use different databases (or schemas) for development, testing, and production environments to ensure isolation and avoid data leaks.
- [ ] For integration tests, either use Docker Compose or a manually managed local Postgres instance; avoid test-specific external tools.
- [ ] Wrap each integration test in a transaction and roll back at the end to isolate state changes.
- [ ] Keep test data loading and teardown logic separate from production logic for clarity and safety.

### Admin Operations

- [ ] Keep admin operations (such as migrations, schema resets, or data fixes) in a separate CLI or tool.
- [ ] Ensure admin tools run with elevated credentials and never expose those credentials to the main application runtime.
- [ ] Log all admin operations with context: who triggered them, when, and what changes were made.
- [ ] Never allow the application to perform automatic destructive changes (like dropping or recreating tables) in production.

### Observability and Fail-Fast Behavior

- [ ] Detect and fail fast on misconfigurations: missing env vars, bad connection strings, or invalid schemas.
- [ ] Log connection pool stats and failed query attempts for visibility during debugging and monitoring.
- [ ] Include basic metrics or counters (query timing, connection retries, etc.) to aid in production observability if needed.
