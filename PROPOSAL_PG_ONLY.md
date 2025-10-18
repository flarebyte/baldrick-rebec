# Proposal: Simplify Stack — Replace OpenSearch with PostgreSQL

Goal: Reduce operational complexity by storing message content in PostgreSQL (with pgvector and built‑in FTS) and removing OpenSearch as a runtime dependency.

## Rationale
- Ops simplicity: one datastore to operate, backup, upgrade, and secure.
- Transactional coherence: content and events/profiles in a single transactional boundary.
- Acceptable tradeoff: for small/medium corpora, pgvector + FTS is sufficient; we can revisit OS if search scale/features grow.

## Scope (What changes)
- Content storage
  - Move unique message content from OpenSearch to PostgreSQL (new table + optional embedding column via pgvector).
  - Keep existing events/profiles tables; relate events to content rows via FK.
- CLI
  - Remove or deprecate `rbc admin os ...` commands (bootstrap, ilm/ism).
  - `rbc admin db init` creates/updates the content table and optional pgvector extension.
  - `rbc admin db status` reports pgvector/FTS availability and content table presence.
- DAOs
  - Delete OpenSearch DAOs (client, messages_content) and replace with PostgreSQL content DAO (CRUD, dedup by hash).
- Config
  - Remove OpenSearch section; keep only PostgreSQL + server.

## Data Model (Content in PG)
- New table `messages_content_pg` (name TBD; or reuse `messages_content` in PG):
  - `id TEXT PRIMARY KEY` — SHA256(canonicalized body)
  - `content TEXT NOT NULL`
  - `content_type TEXT` (keyword)
  - `language TEXT` (keyword)
  - `metadata JSONB` (arbitrary)
  - Optional: `embedding VECTOR(n)` (pgvector) if enabled
  - Indexes: GIN on `to_tsvector('simple', content)`, and `ivfflat/hnsw` on `embedding` when present

## Search
- Text search: PostgreSQL FTS with language configs; simple ranking.
- Vector search: pgvector ANN (HNSW if supported by pgvector/PG version); fallback to IVF/flat.
- Hybrid: app‑level combination (e.g., vector prefilter + FTS rerank) to keep DB complexity low.

## Migration Plan
1) Code path switch (feature flag)
   - Add a config flag `features.pg_only = true` to use PG code paths in parallel with OS for a transition period.
2) Schema
   - `rbc admin db init` creates the content table and extensions/indexes if missing.
3) Ingest
   - `rbc admin message send` writes content to PG (dedupe by hash) and events to PG.
4) Cleanup
   - Remove OS code, config, docs; leave a short “Migrated to PG” note and a migration guide.

## Risks / Tradeoffs
- Search quality/features weaker vs OpenSearch (see DB_ANALYSIS.md).
- Large‑scale corpora: PG vector/FTS may need careful tuning; growth bounds must be monitored.
- Migration effort: re‑wiring DAOs, CLI commands, and docs.

## Work Breakdown (Checklist)
- [ ] Config: add `features.pg_only`, remove OS config when enabled
- [ ] Schema: add `messages_content_pg`, extensions/indexes
- [ ] DAO: new PG content DAO (ensure/put/get), dedupe by hash
- [ ] CLI: `db init/status` for PG content; remove `os` commands behind feature flag
- [ ] CLI: `message send` writes to PG content/events
- [ ] Docs: README, DATABASES.md, DB_ANALYSIS.md updates; deprecations & migration guide
- [ ] Tests: minimal sanity for DAO + CLI flows

## Rollback
- Keep OS paths behind a feature flag until confidence is built; if issues arise, flip flag back without data loss.

## Decision Gate
- Approve PG‑only direction for initial scope (small/medium corpora). If later search needs exceed PG capabilities, re‑introduce OpenSearch with a migration plan.
