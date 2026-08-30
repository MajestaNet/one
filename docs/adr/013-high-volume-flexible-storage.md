# ADR-013: High-volume flexible storage (Postgres)

## Status

Accepted

## Context

All flexible business objects share one Postgres `records` table (`object_api_name` + `data` JSONB), per [ADR-002](./002-hybrid-metadata-storage.md) and [ADR-008](./008-core-data-model.md). That preserves “add fields/objects without customer DDL,” and the SQL planner + expression indexes in [ADR-003](./003-sql-query-engine.md) make CRM-scale objects viable.

At ~100M rows for a single append-heavy object (Message, Transaction, event/ledger lines), a shared heap creates real risks: vacuum/autovacuum pressure, global GIN write amplification, planner/stats skew against cold objects, and soft-delete accumulation. LIST-partitioning by `object_api_name` alone isolates Account from Message but does not make a 100M Message slice operable by itself.

`metadata_objects.storage_mode` already defaults to `flexible` and is unused for physical routing — a natural extension point without inventing a second storage paradigm.

## Decision

Stay on **one Postgres database per install** (ADR-001). Keep metadata + JSONB (no per-customer field DDL, no multi-DB sharding). Escalate storage and query discipline in this ladder:

### Tier A — Access-pattern discipline (no schema change)

For Message / Transaction-class objects:

- Treat as append-heavy and time-keyed: default list paths filter `CreatedAt` range and paginate with keyset `(created_at, id)`.
- Mark foreign keys and list filters `indexed=true` so the worker builds partial expression indexes via `field_projections`.
- Reject or divert API patterns that force full-object JSONB scans (unindexed `LIKE`, sort on non-indexed custom fields) once cardinality crosses product thresholds.
- Prefer child loads by indexed reverse lookup over deep self-joins across the hot object.
- Keep `data` payloads lean (large bodies may move to object storage later; not the first lever).

### Tier B — Near-term BP-001 follow-ups

- Push record visibility (owner / creator / `view_all`) into SQL so HTTP does not over-fetch then discard.
- Surface EXPLAIN / missing-index signals when filters lack projections.
- Revisit the global GIN on `records.data` once expression indexes cover hot paths (GIN is often the first write/space casualty at 100M).

### Tier C — LIST-partition `records` by `object_api_name`

Product kernel migration (not customer Metadata):

- Convert `records` to a declarative LIST-partitioned parent on `object_api_name`.
- Dedicated partitions for core / known modules; a DEFAULT partition for obscure customer objects.
- DataEngine keeps `FROM records` when every query already predicates `object_api_name` (true today).
- Benefit: vacuum, stats, and cache isolation across object types; archive/drop of one object becomes tractable.
- Cost: one-time rewrite; attach partition on object create remains **product/worker DDL**, never “create column” customer DDL.

### Tier D — `storage_mode=high_volume`

| `storage_mode` | Physical layout | When |
|---|---|---|
| `flexible` (today) | Shared / LIST-partitioned `records` | CRM-scale objects (Account, Contact, Case, …) |
| `high_volume` (new) | Same JSONB row shape; LIST + **RANGE(`created_at`)** (monthly/weekly); BRIN on `created_at`; stricter query guardrails | Messages, events, ledger lines (~10M+ growing toward 100M) |

Rules for `high_volume`:

1. Still metadata + JSONB — **adding fields still needs no migration**.
2. Creating / enabling a high-volume object may attach time partitions via product/worker DDL (kernel-controlled).
3. Queries without a selective indexed predicate or time bound are rejected or forced onto async/export paths.
4. Old partitions **DETACH** to an archive schema / cheaper tablespace; OLTP defaults to recent hot partitions.
5. Deletes are **hard deletes** (no eternal `deleted_at` tombstones). Archive old HV RANGE partitions via DETACH rather than row-level soft-delete purge.
6. Client / Metadata APIs stay logical (same record object shapes); physical routing is an implementation detail of DataEngine + migrations.

CRM objects stay `flexible` indefinitely. Declare `high_volume` only when expected cardinality and append rate justify partition lifecycle ops.

### Explicit non-goals

- Multi-tenant SaaS row mixing or cross-customer sharding
- Per-customer physical tables for every custom object / custom field
- Non-Postgres primary store as the first scale lever
- Promoting Account/Contact to typed kernel tables (ADR-008 stands unless a future ADR reverses it)

## Consequences

- Shared `records` remains correct for CRM-scale flexible objects; high-volume is an explicit opt-in `storage_mode`, not a fork of the product model.
- Install topology stays simple: one connection string, one customer DB, same API families.
- **Implemented (v1):** physical table `records_hv` (migration `0019_high_volume_records`), DataEngine routing by `storage_mode`, query guardrails.
- **Product HV example retired ([ADR-032](./032-retire-messages-polymorphic-lookup.md)):** optional `messages` / `Message` and `polymorphic_lookup` are not product types. HV storage remains for a later append-heavy object (planned `ExecutionLogEntry`, BP-033).
- **Tier B (BP-001 close-out):** baseline AuthZ visibility SQL (owner/creator when sharing off); query advisor (`indexHints`); HV locator requires time bound + `HighVolumeLocatorMaxRows`; worker `hv.partition.roll` for future Message yearly partitions; global `records_data_gin_idx` dropped (`0031_drop_records_data_gin`).
- **Tier C (BP-035):** shared `records` is LIST-partitioned by `object_api_name` (`0037_records_list_partition`); PK `(id, object_api_name)`; dedicated leaves for core/known modules; **no DEFAULT** (`0038`); `EnsureFlexiblePartition` on object create.
- **Hard-delete (`0037`):** dropped `deleted_at` on `records` / `records_hv`; Message RANGE has no DEFAULT sink; `record_access_grants` PK includes `object_api_name`.
- **Still deferred ([BP-035](../../backlog/BP-035-records-list-partition-covering.md) remainder):** covering projection tables / matviews; HV DETACH/archive of aged RANGE partitions; optional monthly RANGE granularity.
- Locator / query ceilings: HV locator is time-bounded; prefer that over raising unbounded scans toward `LocatorMaxRows` (50M).

## Related

- [ADR-001](./001-dedicated-install.md) · [ADR-002](./002-hybrid-metadata-storage.md) · [ADR-003](./003-sql-query-engine.md) · [ADR-008](./008-core-data-model.md)
- [data-model.md](../data-model.md) · [agent-data-architecture.md](../architecture/agent-data-architecture.md)
- [BP-001](../../backlog/BP-001-jsonb-query-scale.md)
