# ADR-003: SQL-native query engine and field projections

## Status

Accepted

## Context

v1 queried flexible `records` by loading a capped row set and filtering in memory. That cannot support enterprise shapes such as Contact with hundreds of custom fields joined to multiple related objects, and it falls short of (or only barely meets) incumbent CRM query ceilings.

## Decision

1. **Push all filters, sorts, and pagination into PostgreSQL** using parameterized JSONB operators (`->>`, typed casts) on `records.data`.
2. **Keyset pagination** on `(created_at, id)` — no offset scans for large result sets.
3. **Relationship joins** as self-joins on `records` (child-to-parent via lookup UUID in JSONB; parent-to-child via reverse lookup), up to configured depth/join count that **exceeds typical incumbent CRM relationship limits**.
4. **Field projections / expression indexes**: metadata `indexed` flag drives worker-created partial expression indexes per `(object_api_name, field)` without customer DDL tables.
5. **Platform query limits** deliberately exceed incumbent CRM query cheatsheet ceilings (see `QUERY_LIMITS` in `@one/shared`).

## Consequences

- Query correctness and scale move to the planner/SQL layer; in-memory matchFilter remains only for unit tests of operators.
- Index creation is async and idempotent; missing indexes degrade to sequential JSONB scans, not failure.
- Still no schema-per-object tables for custom fields (ADR-002 preserved).
- Multi-TB / ~100M-row object isolation (LIST/RANGE partitions, `storage_mode=high_volume`) is specified in [ADR-013](./013-high-volume-flexible-storage.md); implementation tracked under [BP-001](../../backlog/BP-001-jsonb-query-scale.md).
