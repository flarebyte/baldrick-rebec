# Proposal: Simplify Stack — Replace OpenSearch with PostgreSQL

Goal: Reduce operational complexity by storing message content in PostgreSQL (using built‑in full‑text search only) and removing OpenSearch as a runtime dependency.

## Rationale
- Ops simplicity: one datastore to operate, backup, upgrade, and secure.
- Transactional coherence: content and events/profiles in a single transactional boundary.
- Acceptable tradeoff: for small/medium corpora, PostgreSQL FTS is sufficient; we can revisit specialized search later if needs grow.

## Scope (What changes)
- Content storage
  - Move unique message content from OpenSearch to PostgreSQL (new table).
  - Keep existing events/profiles tables; relate events to content rows via FK.
- CLI
  - Remove or deprecate `rbc admin os ...` commands (bootstrap, ilm/ism) once parity is reached.
  - `rbc admin db init` creates/updates the content table and FTS index.
  - `rbc admin db status` reports FTS readiness and content table presence.
- DAOs
  - Delete OpenSearch DAOs (client, messages_content) and replace with PostgreSQL content DAO (CRUD, dedup by hash).
- Config
  - Remove OpenSearch section; keep only PostgreSQL + server.
  - Remove feature flags after migration; PG‑only becomes the default.

## Data Model (Content in PG)
- New table `messages_content_pg` (name TBD; or reuse `messages_content` in PG):
  - `id TEXT PRIMARY KEY` — SHA256(canonicalized body)
  - `content TEXT NOT NULL`
  - `content_type TEXT` (keyword)
  - `language TEXT` (keyword)
  - `metadata JSONB` (arbitrary)
  - Indexes: GIN on `to_tsvector('simple', content)` for fast FTS

## Search
- Text search: PostgreSQL FTS with language configs; simple ranking.
- Vector search: out of scope for this phase; keep complexity low and avoid performance surprises.

## Migration Plan
1) Code path switch (feature flag)
   - Add a config flag `features.pg_only = true` to use PG code paths in parallel with OS for a short transition.
2) Schema
   - `rbc admin db init` creates the content table and FTS index if missing (no vector).
3) Ingest
   - `rbc admin message send` writes content to PG (dedupe by hash) and events to PG.
4) Migration & Removal
   - If any content is in OpenSearch, provide a one‑off migration tool/command to read content by `_id` and bulk insert into PG; otherwise skip.
   - Remove OS code, config, and docs; delete `rbc admin os ...` commands.
   - Remove OpenSearch from `docker-compose.yaml` and dependencies from `go.mod`.
   - Remove feature flags; PG becomes the only path.

## Risks / Tradeoffs
- Search quality/features weaker vs OpenSearch (see DB_ANALYSIS.md).
- Large‑scale corpora: FTS may need careful tuning; growth bounds must be monitored.
- Migration effort: re‑wiring DAOs, CLI commands, and docs.

## Work Breakdown (Checklist)
- [ ] Config: add `features.pg_only`, default false; publish deprecation plan for OpenSearch
- [ ] Schema: add `messages_content_pg`, FTS index
- [ ] DAO: PG content DAO (ensure/put/get), dedupe by hash
- [ ] CLI: `db init/status/search` for PG content; guard/remove `os` commands
- [ ] CLI: `message send` writes to PG content/events
- [ ] Optional migration: one‑off import from OpenSearch (if needed)
- [ ] Remove OS: delete code, config keys, Docker services, and deps; flip `pg_only` to default true and then remove flag
- [ ] Docs: update README, DATABASES.md, DB_ANALYSIS.md; add migration guide
- [ ] Tests: minimal sanity for DAO + CLI flows

## Rollback
- Keep OS paths behind a feature flag until confidence is built; if issues arise, flip flag back without data loss.

## Decision Gate
- Approve PG‑only direction for initial scope (small/medium corpora). If later search needs exceed PG capabilities, re‑introduce OpenSearch with a migration plan.
