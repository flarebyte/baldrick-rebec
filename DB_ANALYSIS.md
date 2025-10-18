# Database Analysis: OpenSearch vs PostgreSQL for Message Content

This note compares storing message content in OpenSearch vs PostgreSQL, assuming vector search capability is required. It also outlines high‑level operational steps to run OpenSearch for this use case.

## Summary

- OpenSearch excels at search (full‑text + vector), relevance, and large‑scale indexing, but has higher operational complexity.
- PostgreSQL is simpler to operate and consolidate, and with pgvector can do basic to moderate vector search; it lags on search features/scale vs OpenSearch.
- A hybrid is often best: store normalized content in OpenSearch for search/retrieval; store events, profiles, and relational joins in PostgreSQL.

## Comparison (Pros/Cons)

OpenSearch (content in OS)
- Advantages
  - Search‑first: mature full‑text (analyzers, scoring) + k‑NN vector search (HNSW/IVF)
  - Horizontal scale (shards/replicas), near‑real‑time indexing
  - Rich query DSL, highlighting, filters, aggregations
  - Lifecycle management (ISM/ILM) for rollover/retention
- Drawbacks
  - Operational complexity: TLS/auth, plugins (ISM), snapshotting, shard planning
  - Write amplification and mapping evolution pitfalls (reindex cycles)
  - Data consistency not transactional; eventual consistency semantics

PostgreSQL (content in PG, pgvector + FTS)
- Advantages
  - Simpler ops: single RDBMS for app state + search
  - Transactional integrity, easy joins with events/profiles
  - pgvector supports cosine/L2/IP with HNSW (extensions) for moderate scale
  - Backup/restore is straightforward (pg_dump/pg_basebackup)
- Drawbacks
  - Full‑text features are less search‑centric than OS (analyzers, ranking, query DSL)
  - Vector search scale/perf lower than dedicated engines for large corpora
  - Concurrency and near‑real‑time indexing less optimized than OS

When to prefer OpenSearch
- Large or growing corpus; relevance matters; heterogeneous queries (text + structure + vectors)
- Need analyzers, synonyms, highlighting, aggregations, and ISM/ILM retention

When to prefer PostgreSQL
- Small/medium corpus; ops simplicity prioritized; transactional coherence is key
- Vector search needs are simple; acceptable to trade top‑end recall/latency for lower complexity

## Hybrid Architecture Rationale

- Store unique message bodies in OpenSearch (dedup by hash) for fast retrieval and semantic search; keep only search‑relevant fields and embeddings.
- Store ingest events, profiles, joins, and audit history in PostgreSQL (truth of process and relationships).
- Advantages: best‑of‑breed search without overloading the relational model; clear separation of concerns; controlled retention in OS via ISM/ILM.

## High‑Level Steps to Operate OpenSearch (for this app)

1) Provision + Secure
- Run a local/dev cluster (Docker Compose) or managed service; enable security (TLS + auth)
- Choose scheme/host/port; place admin credentials as temporary bootstrap only

2) Configure App Access
- Create a least‑privilege app user and role (read/write on the content index)
- Put app credentials in the CLI config; clear admin temp secrets after bootstrap

3) Index Design
- Define index mappings: text fields, keyword fields, metadata object, and vector fields (e.g., `knn_vector`)
- Choose analyzers (language, stemming) and similarity; plan for mapping evolution via aliases and rollover

4) Lifecycle Policy (Retention/Rollover)
- If ILM is available, create and attach an ILM policy; otherwise use ISM (OpenSearch plugin)
- Policy should roll hot shards by age/size and delete after retention horizon

5) Ingest + Embeddings
- Decide where embeddings are produced (app pipeline or model service)
- Store content + embeddings atomically at ingest; set refresh policy trade‑offs (latency vs throughput)

6) Query Patterns
- Support hybrid search: text queries + vector k‑NN filters/boosting
- Tune recall/latency: HNSW/efSearch, candidates, re‑rankers as needed

7) Observability + Capacity
- Monitor health (green/yellow/red), shard allocation, indexing latency
- Size shards, replicas, and heap; plan reindex operations for mapping changes

8) Backup/Restore
- Configure a snapshot repository (e.g., S3/FS) and schedule snapshots
- Test restores to validate DR workflows

9) Upgrades + Compatibility
- Track plugin/APIs (ISM vs ILM); vet breaking changes before rolling upgrades
- Use index aliases to hide reindex/migration steps from the application

10) Local Dev Automation (this CLI)
- `rbc admin os bootstrap` to set https and attach lifecycle policy (ILM or ISM)
- `rbc admin db init` to create the `messages_content` index (no lifecycle setting inline)
- `rbc admin db status --json` to verify index presence and lifecycle attachment
- `rbc admin os ilm|ism ensure/show/list/delete` to manage policies explicitly

## Notes on Vector Search

- OpenSearch
  - Use `knn_vector` with HNSW for ANN; tune `ef_search`, `m`, and `k`
  - Consider multi‑stage: vector prefilter → text re‑rank for relevance
- PostgreSQL (pgvector)
  - Use approximate HNSW indexes for large sets; ensure memory/parallel settings
  - Expect simpler query composition; consider offloading advanced ranking to app code

## Recommendation

- Default to the hybrid approach for this app: content in OpenSearch, events/profiles in PostgreSQL.
- If operations must be minimal and corpus stays small, PostgreSQL with pgvector and FTS is a viable single‑stack alternative; plan for migration if search scale or features grow.
