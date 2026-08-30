# BP-001: JSONB query path will not scale

- **Severity:** High
- **Status:** Mitigated
- **Area:** `internal/dataengine`, `migrations/`

## Outcome

SQL-native query planner, keyset pagination, field projections, and high-volume `records_hv` shipped. Do not reintroduce in-process filter/sort of JSONB rows.

## Do not reopen

Scale follow-on is [BP-035](./BP-035-records-list-partition-covering.md) (LIST covering / materialized projections). Query contracts live in [ADR-003](../docs/adr/003-sql-query-engine.md) and [ADR-013](../docs/adr/013-high-volume-flexible-storage.md).

## Related

- Playbook: [agent-data-architecture.md](../docs/architecture/agent-data-architecture.md)
