# BP-035: LIST-partition shared `records` (Tier C) + covering projections

- **Severity:** High
- **Status:** Partially mitigated
- **Area:** `internal/dataengine`, `migrations/`, `internal/db`
- **Design:** [ADR-013](../docs/adr/013-high-volume-flexible-storage.md)
- **Spun out from:** [BP-001](./BP-001-jsonb-query-scale.md) (medium Tier B remainders closed separately)

## Problem

CRM-scale objects still share one unpartitioned `records` heap. At large cardinality this stresses vacuum, planner stats, and cache isolation across object types. Analytics/export paths also lack covering indexes or materialized projection tables beyond per-field expression indexes.

## Scope

1. **Tier C — LIST-partition `records` by `object_api_name`** — **done** (`0036` + `0037`)
   - Kernel migration to declarative LIST parent; dedicated partitions for core/known modules
   - **No DEFAULT** — `EnsureFlexiblePartition` required before first write (`0037`)
   - DataEngine keeps `FROM records` (queries already predicate `object_api_name`)
   - PK `(id, object_api_name)`; hard-delete (no `deleted_at`); grants PK includes `object_api_name`

2. **Covering indexes / materialized projection tables** — **still open**
   - Extend `field_projections` toward covering INCLUDE / matview-style export paths
   - Refresh/consistency story for dual `records` / `records_hv` routing
   - Optional: monthly HV RANGE + DETACH/archive lifecycle

## Explicit non-goals

- Reopening BP-001 Tier B work (AuthZ SQL, advisor, HV locator, RANGE roll, GIN drop)
- Per-customer physical tables for custom fields (ADR-002 stands)

## Related

- [BP-001](./BP-001-jsonb-query-scale.md)
- [ADR-013](../docs/adr/013-high-volume-flexible-storage.md)
- [ADR-002](../docs/adr/002-hybrid-metadata-storage.md)
- [ADR-003](../docs/adr/003-sql-query-engine.md)
