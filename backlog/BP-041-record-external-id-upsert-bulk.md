# BP-041: Record external ID, upsert, and richer Bulk jobs

- **Severity:** High
- **Status:** Mitigated
- **Area:** `internal/dataengine`, `internal/metadata`, `internal/httpapi`, `migrations/`
- **Design:** [external-id-upsert-bulk-build-plan.md](../docs/architecture/external-id-upsert-bulk-build-plan.md) · remainder: [11-bp-041-046-061-headless-client.md](../docs/architecture/agentic-remainders/11-bp-041-046-061-headless-client.md) · [api-families.md](../docs/api-families.md) (Client family) · [ADR-003](../docs/adr/003-sql-query-engine.md)
- **Identified:** Headless 360 backlog review (2026-07) — integration sync primitives missing from backlog

## Problem

Headless CRM integrations (ETL, iPaaS, data warehouses, migration tools) expect **idempotent record sync** keyed by an external system identifier. Majesta One today has:

- `externalId` on **principals** (SCIM / identity) — not on flexible business records
- `POST /bulk/{object}` — **synchronous create-only** batch insert (despite docs historically saying async jobs)
- No upsert-by-external-key, no bulk update/delete/query job API
- No portable multi-object **data pack** for ordered seed/refresh between installs (metadata Deploy does not move business rows)

Industry bulk ingest APIs and REST upsert (`externalId` + `externalIdField`) are table stakes for headless 360 sync. Without them, every integration must maintain a Majesta One `id` mapping table or risk duplicate rows on retry.

## Why it matters

- Blocks “drop-in CRM sync adapter” narratives for headless implementations
- Forces brittle two-phase create-then-patch patterns and race-prone dedupe in customer code
- Limits agent and automation reliability when replaying outbound/inbound sync (pairs [BP-014](./BP-014-agent-outbound-integrations.md))
- Multi-env “box refresh” of reference data still needs UUID remapping spreadsheets

## Scope (target)

Implement via [external-id-upsert-bulk-build-plan.md](../docs/architecture/external-id-upsert-bulk-build-plan.md):

1. **Metadata:** `externalId` flag on eligible fields — implies unique + indexed; unique expression projections
2. **DataEngine:** `GetByExternalID` + `Upsert` under AuthZ + validation + sharing/FLS
3. **Client API:**
   - REST get/patch/delete by `{externalIdField}/{externalId}` (+ composite upsert)
   - Bulk 2.0–inspired async jobs: `insert` | `update` | `upsert` | `delete` with status + NDJSON results
   - Retain small sync `POST /bulk/{object}` as a helper only
4. **Data packs (`one-datapack/v1`):** ordered multi-object manifests keyed by external IDs; apply to a **connected** install (repo→org style — no peer data push)
5. **AuthZ:** upsert respects create vs update object permissions and FLS ([BP-003](./BP-003-enterprise-auth.md))

## Depends on / pairs with

- [BP-001](./BP-001-jsonb-query-scale.md) / [BP-035](./BP-035-records-list-partition-covering.md) — external-id indexes must stay selective
- [BP-003](./BP-003-enterprise-auth.md) — FLS on upsert field sets
- [BP-005](../docs/architecture/agent-worker.md) — worker claim for ingest jobs
- [BP-033](./BP-033-customer-runtime-isolation.md) — ingest job-class budgets when available
- [BP-046](./BP-046-record-merge-dedupe.md) — merge is separate; upsert prevents dupes at ingest
- [BP-048](./BP-048-one-cli.md) — optional `datapack` CLI commands

## Remainder closed (Mitigated)

Phases 1–3 plus remainder production-hardening are in tree:

- HTTP+DB contract tests (`internal/httpapi/upsert_ingest_integration_test.go`, `internal/datapack/apply_ingest_test.go`)
- Max 2 `InProgress` ingest jobs per install; `IngestChunkSize` (500) transactions when `allOrNone=false`
- Datapack apply uses Client ingest jobs (upsert) when a step has **more than 500 rows**

## Follow-ons (do not reopen Mitigated)

| Item | Why deferred |
|---|---|
| CSV ingest (`text/csv`, delimiters) | Plan Phase 4b; NDJSON-only ingest remains the v1 contract |
| `processingMode: Serial\|Parallel` | Pack-level object order already exists; within-object Parallel is default |
| BP-033 job class `ingest` | Couple when BP-033 admission lands; until then the hard InProgress cap (2) and chunk size apply |
| Drop leftover non-unique `proj_*` when a unique index is built | Operator can rebuild projections; not ingest-blocking |
| Kernel `users.external_id` | Non-goal (SCIM `externalId`) |

## Explicit non-goals

- Third-party bulk API wire-compat or SQL bulk query jobs
- Cross-object upsert in one HTTP call (use composite or data-pack steps)
- Install→install business-data peer push (Deploy stays metadata/tests)
- Bi-directional CRM CDC (see [BP-042](./BP-042-change-feed-cdc-consumer.md))
- External ID on kernel `users` (already covered by SCIM `externalId`)
- Merge / survivorship (BP-046)

## Related

- Remainder (slot 11): [11-bp-041-046-061-headless-client.md](../docs/architecture/agentic-remainders/11-bp-041-046-061-headless-client.md)
- [external-id-upsert-bulk-build-plan.md](../docs/architecture/external-id-upsert-bulk-build-plan.md) · [api-families.md](../docs/api-families.md) · [scim-provisioning.md](../docs/architecture/scim-provisioning.md) · [multi-env-deploy.md](../docs/multi-env-deploy.md)
