# ADR-002: Hybrid metadata storage

## Status

Accepted

## Context

Leading metadata-driven platforms separate metadata from data and avoid per-customer DDL for custom fields. Majesta One needs the same flexibility without shared multi-tenant pivot-table complexity.

## Decision

Use a hybrid model:

1. **Kernel/system entities** — real Postgres tables migrated with Drizzle (users, metadata definitions, outbox, audit, jobs).
2. **Flexible business objects** — `records` table with `data JSONB` interpreted by metadata. Field adds/changes are metadata writes only.
3. **Projections (later)** — generated indexes / projection tables for hot query paths, built asynchronously by the worker.

Validation expressions use JSONLogic in v1 for portability.

## Consequences

- Production avoids frequent DDL for customer customizations.
- Query performance on sparse custom fields may need projections as scale grows.
- System objects remain strongly typed and migratable when integrity matters.
- Shared `records` at very high cardinality is addressed by [ADR-013](./013-high-volume-flexible-storage.md) (`storage_mode=high_volume`, partitioning) without abandoning JSONB.
