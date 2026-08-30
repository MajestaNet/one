# Agent playbook: data architecture

For agents changing Majesta One’s data model, storage, seed packages, or query path. Follow this before writing code.

## Where to look

| Concern | Path |
|---|---|
| Canonical model + relationship rules | [`docs/data-model.md`](../data-model.md) |
| Sales / Service optional modules | [`sales-service-data-model.md`](./sales-service-data-model.md), [`ADR-011`](../adr/011-sales-service-managed-modules.md) |
| Decision record (core) | [`docs/adr/008-core-data-model.md`](../adr/008-core-data-model.md) |
| Hybrid storage decision | [`docs/adr/002-hybrid-metadata-storage.md`](../adr/002-hybrid-metadata-storage.md) |
| Query / indexes | [`docs/adr/003-sql-query-engine.md`](../adr/003-sql-query-engine.md), `internal/dataengine/` |
| Managed seed (`core`) | `internal/seed/packages.go`, `internal/seed/seed.go` |
| Managed sync / ownership | `internal/metadata/write.go` (`Sync*Managed`, `AssertCustomerMutable`) |
| Deploy reject managed | `internal/packages.IsManagedPackageName` (used by deploy + metadata) |
| Kernel DDL + package remap/cleanup | `migrations/` (`0011_core_package`, `0012_drop_legacy_managed_objects`; `0058_refresh_tokens` — [BP-063](../../backlog/BP-063-refresh-token-sessions.md)) |
| Package versions | `package_installs` table; `metadata.RecordPackageInstall` |
| Scale / high-volume storage | [`ADR-013`](../adr/013-high-volume-flexible-storage.md), [`BP-001`](../../backlog/BP-001-jsonb-query-scale.md), [`BP-035`](../../backlog/BP-035-records-list-partition-covering.md) |
| Field types (custom fields) | [`ADR-017`](../adr/017-canonical-field-types.md), [`BP-036`](../../backlog/BP-036-canonical-field-types.md), `internal/metadata/fieldtypes.go` |
| External ID / upsert / Bulk ingest | [`external-id-upsert-bulk-build-plan.md`](./external-id-upsert-bulk-build-plan.md), [`BP-041`](../../backlog/BP-041-record-external-id-upsert-bulk.md) |
| Cross-object search | [`cross-object-search-build-plan.md`](./cross-object-search-build-plan.md), [`BP-043`](../../backlog/BP-043-cross-object-search-api.md) + [`BP-020`](../../backlog/BP-043-cross-object-search-api.md) |
| Package versioning | [`BP-007`](../adr/020-cdm-managed-packages.md) |
| Platform actions / Lead convert / Quote accept | [`platform-actions-build-plan.md`](./platform-actions-build-plan.md), [`ADR-029`](../adr/029-platform-actions.md), [`BP-061`](../../backlog/BP-061-platform-actions.md); billing: [`billing-module-build-plan.md`](./billing-module-build-plan.md) |

## What ships today

```text
core (managed, AUTO_SEED)
├── User          → kernel table users (managed metadata object, storage_mode=kernel; customer fields in users.data; not records)
├── Account       → records + metadata_objects/fields
└── Contact       → records; AccountId lookup OPTIONAL

agents_starter (managed, AUTO_SEED) — clones AdminSetup + MetadataBuilder AgentSpecs (customer-owned)

Optional managed (admin enable):
  address, notes, activities, lead_marketing,
  catalog (Product, PriceList, PriceListEntry, Unit, UnitGroup),
  sales (Opportunity, OpportunityContactRole, Quote, QuoteLine, Competitor; quote.accept),
  service (Case, CaseComment, Asset, Entitlement, ServiceContract, ContractLineItem, WorkOrder),
  crm_bridge (Case.OpportunityId field extension),
  billing (Order, OrderLine; Quote.OrderId field extension),
  activities (flexible Task/Appointment/PhoneCall/Email; Activity Feed),
  industry: healthcare, financial_services, retail, sustainability, education,
    automotive, nonprofit, marketing_events, portals, project_service
    (see cdm-industry-packages.md)

Record system columns (all flexible objects):
  CreatedById, LastModifiedById (required, auto)
  OwnerId (optional)
  CreatedAt, UpdatedAt
```

Legacy objects **Lead, Opportunity, Activity, Product, PriceBook, Order, Invoice, Payment** were removed from always-on product seed and purged by migration `0012`. Reintroduced shapes ship only as **optional** managed modules per [ADR-011](../adr/011-sales-service-managed-modules.md) / [ADR-020](../adr/020-cdm-managed-packages.md) — do not put them back into `core`. Lead returns only via `lead_marketing`.

AuthZ (ADR-009): every principal needs ≥1 Role (scopes). Object/field access via permission sets assigned to users only.

## What to do (change types)

### A. Add/change a **managed core** field (Account / Contact)

1. Confirm the change is additive if possible (new field > rename/type change).
2. Edit `internal/seed/packages.go` (`InstallCore`); bump `CorePackageVersion` when definitions change.
3. Ensure hot fields set `Indexed` / `Filterable` / `Sortable` appropriately.
4. Update [`docs/data-model.md`](../data-model.md) field tables + ADR-008 if the contract changes.
5. Add/adjust seed or HTTP tests; run `go test -p 1 ./internal/seed/... ./internal/metadata/...`.
6. **Do not** add a SQL migration for a flexible field — metadata sync is enough.

### B. Change **kernel** identity / system tables (User, auth, metadata catalog)

1. Add a numbered SQL file under `migrations/` and a journal entry in `migrations/meta/_journal.json`.
2. Update Go stores under `internal/db/` as needed.
3. Remember: kernel changes apply on every install at upgrade — keep forward-compatible when possible ([ops.md](../ops.md)).

### C. Customer custom fields / objects

1. Implement via Metadata API only (`ownership=custom`); never `internal/seed`.
2. Promote with Deploy between same-`CUSTOMER_ID` installs.
3. See [customer-customizations.md](../customer-customizations.md).

### D. New **managed module** (optional package)

1. Read [ADR-011](../adr/011-sales-service-managed-modules.md) and [sales-service-data-model.md](./sales-service-data-model.md) before adding Sales/Service/catalog/CPQ-related modules.
2. Add a module in `internal/seed` (defs) and register it in `internal/packages` registry (`depends_on`, version, object/field defs).
3. Use `ownership=managed` and a distinct `package_name`; never Deploy.
4. `packages.IsManagedPackageName` / Deploy reject lists pick up registry names automatically.
5. Document under `docs/modules/<name>.md` and link from `docs/modules/README.md` + `docs/data-model.md`.
6. Customer admins enable via `POST /metadata/v1/packages/{name}/enable`. Boot re-migrates **enabled** packages only.
7. Opportunity (`sales`): require ContactId **or** AccountId (at least one). Lead belongs in optional `lead_marketing` only ([ADR-020](../adr/020-cdm-managed-packages.md)); do not put Lead in `sales` or `core`.
8. Domain attribute mapping: [cdm-mapping.md](./cdm-mapping.md). Industry packs: [cdm-industry-packages.md](./cdm-industry-packages.md). Hand-author seed defs; do not vendor external schema trees into the product image. Industry packs must not collide on spine apiNames.
8. Keep `catalog` thin (Product / PriceList / PriceListEntry only). CPQ objects go in a future `cpq` package, not `catalog`.
9. Cross-package optional lookups (e.g. Case.OpportunityId) belong in an **AutoEnable** bridge (`crm_bridge`), not inside a single cloud package — and not as a customer-facing enable step.
10. Enabling a module must never add a customer Postgres migration — metadata `Sync*Managed` only.
11. Integrity verbs (convert, merge, accept) are **platform actions** on the module (`ActionDef`), not guest TypeScript in seed — [platform-actions-build-plan.md](./platform-actions-build-plan.md) / [ADR-029](../adr/029-platform-actions.md). Do not add `POST /convertLead`.

### E. Performance work on flexible objects

Prefer ADR-003 patterns: expression indexes via `indexed` metadata + `field_projections`, planner limits. For shared-`records` scale and ~100M-row objects, follow [ADR-013](../adr/013-high-volume-flexible-storage.md):

- CRM-scale: stay `storage_mode=flexible` on LIST-partitioned `records` (Tier C / migration `0036`)
- High-volume: `storage_mode=high_volume` → physical `records_hv` (migration `0019`); DataEngine routes + query guardrails
- First product HV consumer: planned `ExecutionLogEntry` ([BP-033](../../backlog/BP-033-customer-runtime-isolation.md)). Optional `messages` / `polymorphic_lookup` were retired ([ADR-032](../adr/032-retire-messages-polymorphic-lookup.md)).
- Covering/matview projections remain under [BP-035](../../backlog/BP-035-records-list-partition-covering.md)
- Cross-object find (`POST /client/v1/search`, `searchable` + `pg_trgm` document) is [cross-object-search-build-plan.md](./cross-object-search-build-plan.md) — do not revive global JSONB GIN

Do **not** create per-customer DDL tables for custom fields. Do **not** promote Account/Contact to typed kernel tables without a new ADR reversing ADR-008. Do **not** introduce a second database product for high-volume objects.

## Explicit non-goals (until docs say otherwise)

- CRM / ERP product naming or Marketplace SKUs for those packs as always-on seed
- Always-on seed beyond User / Account / Contact (optional modules must be admin-enabled)
- Lead in managed `sales` / always-on `core` (Lead only via optional `lead_marketing`); OpportunityLineItem; Order/sales Contract inside `sales` v1 (Order ships in optional `billing` — [ADR-031](../adr/031-billing-managed-module.md))
- Party / Customer base object or polymorphic `parentCustomer` (ADR-020)
- `polymorphic_lookup` field type and managed `messages` / `Message` ([ADR-032](../adr/032-retire-messages-polymorphic-lookup.md))
- Wholesale external schema import / industry+ops packs beyond curated modules
- CPQ configuration objects inside `catalog`
- Shipping managed package internals through Deploy
- Typed SQL tables for Account / Contact
- Client `/sobjects/User` **CRUD** (describe-only after BP-058 Phase 1; not a flexible record object)
- Per-verb Client routes for convert/merge (use `/client/v1/actions/{apiName}`)

## Checklist before merging a data-model PR

- [ ] Read `docs/data-model.md` + this playbook
- [ ] Ownership correct (`managed` vs `custom`)
- [ ] Docs/ADR updated if the fleet contract changed
- [ ] Seed version bumped when managed defs change
- [ ] Tests cover optional Account↔Contact if touched
- [ ] No customer fixtures committed under `internal/seed` or `migrations/`
