Store Overview — Deprecated

Stores have been removed to simplify the model. Blackboards are now role‑scoped and can carry an optional `lifecycle` field directly.

Key points
- Blackboards no longer require or reference a `store_id`.
- Create blackboards with: `rbc blackboard set --role <ROLE> [--project <NAME>] [--background ...] [--guidelines ...] [--lifecycle weekly|monthly|...]`.
- Sync and stickie flows are unchanged, except `blackboard.yaml` contains `lifecycle` instead of `store_id`.

See BLACKBOARD_USAGE.md and BLACKBOARD_ARCHITECTURE.md for up‑to‑date guidance.
