# ADR-017: Canonical field types

## Status

Accepted

## Context

Metadata `field_type` was unconstrained text. Seed and DataEngine used a conventional set (`text`, `lookup`, …) while Control IDE Object Manager posted legacy alias names (`string`, `reference`). Customers creating custom fields had no discoverable allowlist or type-specific validation.

## Decision

1. **Canonical Majesta One field types** (snake_case) are an allowlist enforced on Metadata create (`POST /metadata/v1/fields`). Discovery: `GET /metadata/v1/field-types`.
2. **Industry-standard** labels/semantics with Majesta One names; **no** `multipicklist`, formula, rollup, encrypted, or file types in v1.
3. **Core:** `text`, `textarea`, `email`, `phone`, `url`, `boolean`, `integer`, `number`, `currency`, `percent`, `date`, `datetime`, `time`, `picklist`, `lookup`, `master_detail`.
4. **Enhancements:** `json`, `autonumber`, `richtext`, `address`, `geolocation`.
5. **`polymorphic_lookup` is retired** ([ADR-032](./032-retire-messages-polymorphic-lookup.md)) — Metadata create rejects it; use typed `lookup` / `master_detail`.
6. **Aliases rejected on create** (`string`→use `text`, `reference`→use `lookup`, …). Migration remaps existing alias rows.
7. **Field type is immutable** after create (customer PATCH and managed sync do not change `field_type`).
8. Compound types (`address`, `geolocation`) store a fixed-key JSON object inside `records.data` — no physical columns.
9. Autonumber uses `autonumber_sequences` + `field_options` (`autonumberFormat`, `autonumberStart`).

## Consequences

- DataEngine validate/coerce/query cast and projection DDL follow the registry in `internal/metadata/fieldtypes.go`.
- Control IDE must consume `GET /metadata/v1/field-types` (not a divergent legacy dropdown).
- Follow-ons (formula, multipicklist, encrypted) need a new ADR.

## Related

- [BP-036](../../backlog/BP-036-canonical-field-types.md)
- [ADR-002](./002-hybrid-metadata-storage.md) · [ADR-003](./003-sql-query-engine.md) · [ADR-013](./013-high-volume-flexible-storage.md)
