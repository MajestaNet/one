# Core data model

Canonical product data architecture for Majesta One: what ships managed with every install, how it is stored, why it is hard to change fleet-wide, and how customers extend it.

**Public object catalog:** [objects.md](./objects.md) (core tables + module index). This file is contributor architecture (storage, performance, migrate rules). Do not publish `GET /describe` as a docs catalog.

See also [ADR-008](./adr/008-core-data-model.md), [ADR-002](./adr/002-hybrid-metadata-storage.md), [ADR-003](./adr/003-sql-query-engine.md).

## Principles

1. **Dedicated install (one customer per database)** — one customer database per install; no SaaS `tenant_id` isolation column on business rows.
2. **Hybrid storage** — identity and kernel tables use real Postgres DDL; business objects use `records.data` JSONB interpreted by metadata.
3. **Managed vs customer ownership** — product-owned definitions are `ownership=managed`; customer customizations are `ownership=custom` and promote via Deploy only.
4. **Core is a fleet contract** — the managed `core` package is the same on every install. Changing it requires a product release that every customer must eventually take. Treat core field/object changes as nearly irreversible.

## What ships in `core`

| Object | Storage | Role |
|---|---|---|
| **User** | Kernel table `users` (+ auth migrations) | Identity / principal (`principal_type`: user \| service \| agent). Linked to records via `CreatedById` / `LastModifiedById`; optional `OwnerId` |
| **Account** | Flexible `records` (`object_api_name=Account`) | Organization / party |
| **Contact** | Flexible `records` (`object_api_name=Contact`) | Person |

Package: `package_name=core`, `ownership=managed`. Seeded by `internal/seed.InstallCore` on boot when `AUTO_SEED=1`. Version recorded in `package_installs`.

User is the AuthZ identity table (`users`). It is **not** stored in `records`. A managed `core` object `User` with `storage_mode=kernel` describes standard columns (`metadata_fields.kernel_column`) and customer fields (`users.data` JSONB) — [ADR-026](./adr/026-kernel-user-metadata.md), [BP-058](../backlog/BP-058-user-identity-extension.md). There is no `/client/v1/sobjects/User` CRUD in v1 (describe only; writes stay on `/client/v1/principals`).

### Record system fields (all flexible objects)

| Field | Column | Rules |
|---|---|---|
| `CreatedById` | `created_by_id` | Auto on create; client cannot set |
| `LastModifiedById` | `last_modified_by_id` | Auto on create/update; client cannot set |
| `OwnerId` | `owner_id` | Optional; omit/`null` stores NULL (not defaulted to actor) |
| `CreatedAt` / `UpdatedAt` | timestamps | Auto |

**Visibility (sharing disabled):** admin, or object `view_all`/`modify_all`, or `OwnerId` matches actor (when set), or `CreatedById` matches actor.

**Visibility (sharing enabled):** see [ADR-016](./adr/016-record-sharing.md) — OWD per object, data-role hierarchy, criteria rules; permission sets still gate object CRUD.

### AuthZ packaging

- Every principal must have ≥1 **Role** → API family scopes (`client` / `metadata` / `deploy` / `ops` / `admin`).
- **Permission sets** assign only to users (`user_permission_sets`) and grant object/field access (future: system permissions on the PS definition).
- Seed roles: `SystemAdmin`, `StandardUser`. See [ADR-009](./adr/009-record-audit-authz-packaging.md).

### Account (standard fields)

| Field | Type | Notes |
|---|---|---|
| Name | text, required, indexed, **searchable** | |
| AccountNumber | text, indexed, **searchable** | |
| Website | url, **searchable** | |
| Industry | text | |
| Phone | phone, **searchable** (not btree indexed) | |
| Fax | phone | |
| TickerSymbol | text | |
| Type | picklist | Prospect / Customer / Partner |
| Ownership | picklist | Public / Private / Subsidiary / Other |
| Description | textarea | |
| ParentAccountId | lookup → Account | Hierarchy |
| PrimaryContactId | lookup → Contact | Optional |
| BillingStreet / City / State / PostalCode / Country | text | Primary billing scalars |
| ShippingStreet / City / State / PostalCode / Country | text | Primary shipping scalars |

### Contact (standard fields)

| Field | Type | Notes |
|---|---|---|
| Salutation | picklist | Mr. / Mrs. / Ms. / Dr. / Prof. |
| FirstName | text, **searchable** | |
| MiddleName | text | |
| LastName | text, required, indexed, **searchable** | |
| Email | email, indexed, **searchable** | |
| JobTitle | text | |
| Department | text | |
| MobilePhone / HomePhone | phone, **searchable** | |
| Fax | phone | |
| Description | textarea | |
| AccountId | lookup → Account, **optional**, indexed | May be null |
| MailingStreet / City / State / PostalCode / Country | text | Primary mailing scalars |

Attribute mapping: [architecture/cdm-mapping.md](./architecture/cdm-mapping.md), [ADR-020](./adr/020-cdm-managed-packages.md). Multi-address rows: optional [`address`](./modules/address.md) package.

## Relationship rules

| Rule | Status |
|---|---|
| Customers may use Account only, Contact only, or both | Supported |
| Account may have zero Contacts | Supported |
| Contact may exist without Account (`AccountId` omitted) | Supported |
| Contact may optionally reference an Account | Supported |
| Future **Opportunity** (`sales` module): must reference Contact **or** Account (at least one) | Contract in [ADR-011](./adr/011-sales-service-managed-modules.md); enforced when `sales` seed ships |

Nothing else is enforced between core objects. See [sales-service-data-model.md](./architecture/sales-service-data-model.md) for optional Sales/Service relationship spines.

## Performance (Go + Postgres)

Account and Contact stay on the flexible store so customer custom fields never require production DDL (ADR-002). **All flexible objects share one `records` table** (`object_api_name` + `data` JSONB) — intentional, not an accident. Performance for the Go + Postgres stack:

- SQL-native query planner pushes filters/sorts/joins into Postgres JSONB operators (ADR-003).
- Hot core fields are marked `indexed` / `filterable` / `sortable` so the worker can build partial expression indexes (`field_projections`).
- Cross-object find (`searchable` + maintained `search_document` + `pg_trgm`) is the Client search path — [cross-object-search-build-plan.md](./architecture/cross-object-search-build-plan.md) ([BP-043](../backlog/BP-043-cross-object-search-api.md)); do not revive global GIN on `records.data`.
- Keyset pagination on `(created_at, id)`; object-scoped indexes on LIST-partitioned `records` (ADR-013 Tier C / migrations `0036`–`0037`; no DEFAULT leaf; hard-delete).
- Typed kernel tables for Account/Contact were rejected: they would improve FKs but force a dual DataEngine path and fight custom-field flexibility.

Shared-heap risks at high cardinality and the Postgres-only ladder for ~100M-row objects are locked in [ADR-013](./adr/013-high-volume-flexible-storage.md). **Implemented:** LIST-partitioned `records` (Tier C, no DEFAULT), hard-delete, `records_hv` + DataEngine routing. Optional `messages` / `polymorphic_lookup` were retired ([ADR-032](./adr/032-retire-messages-polymorphic-lookup.md)); HV storage remains. Covering/matview projections remain under [BP-035](../backlog/BP-035-records-list-partition-covering.md).

**Planned debug objects (BP-033):** managed `ExecutionRun` (flexible) + `ExecutionLogEntry` (`high_volume`) for customer-visible automation/agent/deploy logs — see [customer-runtime-isolation-build-plan.md](./architecture/customer-runtime-isolation-build-plan.md). Not a substitute for operator OTEL ([BP-008](../backlog/BP-008-production-packaging.md)). Not a CRM Message object ([ADR-032](./adr/032-retire-messages-polymorphic-lookup.md)).

**Agent / chat audit:** principal threads on `/client/v1/agents/conversations` ([ADR-022](./adr/022-agent-conversations.md)); hosted tool-loop execution on kernel `agent_runs`; Client mutation history on `audit_log`. Do not store those as business records.

## Immutability contract

Once `core` ships to customer installs:

- Metadata API **cannot** mutate managed object/field definitions.
- Deploy API **rejects** managed package artifacts in customer bundles.
- Only product image upgrade + additive seed migrate can change managed defs.
- Additive migrate inserts missing managed fields and syncs product-owned attributes; it refuses to overwrite customer-owned apiNames.
- Breaking changes (rename, type change, removal) need an explicit product policy — prefer additive fields forever on core.

Upgraded installs run migration `0012_drop_legacy_managed_objects`, which **deletes** formerly seeded Lead, Opportunity, Activity, Product, PriceBook, Order, Invoice, and Payment metadata (and their `records`). Migration `0060` deletes retired `Message` metadata and `records_hv` Message rows ([ADR-032](./adr/032-retire-messages-polymorphic-lookup.md)). New and upgraded installs then have only Account + Contact from `core` (plus kernel User).

## Customer customization

Allowed on this install (then Deploy-promote between same-`CUSTOMER_ID` peers):

- Customer-owned custom fields on Account / Contact (e.g. `Account.Region__c`); **User** custom fields in `users.data` ([BP-058](../backlog/BP-058-user-identity-extension.md) mitigated)
- Customer-owned custom objects related by lookup to Account / Contact
- Validation rules, automations, permission sets (customer-owned)

Not allowed:

- Editing managed field types/labels via Metadata API
- Promoting `core` / `platform` internals via Deploy
- Committing customer metadata into product seed or migrations

## Optional managed modules

Optional product modules extend `core` without being always-on seed. They ship in the product image as managed metadata; a customer admin **enables** them on a running install via Metadata API. Majesta One upgrades enabled modules on product image roll (additive package migrate). See [docs/modules/README.md](./modules/README.md) and [BP-007](./adr/020-cdm-managed-packages.md).

| Rule | Detail |
|---|---|
| Dependency | Modules declare `depends_on` (at least `core`) |
| Ownership | `ownership=managed`, distinct `package_name` (not `core`) |
| Versioning | `package_installs` (version + `enabled`) |
| Enable | `POST /metadata/v1/packages/{name}/enable` (admin + `metadata`) — **not** Deploy |
| Soft-disable | `POST /metadata/v1/packages/{name}/disable` stops future upgrades; does not delete metadata/records |
| Conflict safety | Additive `Sync*Managed`; refuses overwrite of customer-owned apiNames |
| AuthZ | On enable/create, every permission set gets an object data-access stub (Admin = full CRUD; others = deny); FLS via `field_permissions` (deny-by-default; Admin field catalog auto-filled; other PSs get deny stubs) |
| Public docs | Module contracts are documented under `docs/modules/`; runtime schema stays authenticated |

Do **not** reintroduce domain packs as always-on `core` seed.

Optional **Sales / Service / Billing** domain modules (`catalog`, `sales`, `service`, `crm_bridge`, `billing`) are specified in [ADR-011](./adr/011-sales-service-managed-modules.md), [ADR-031](./adr/031-billing-managed-module.md), and [sales-service-data-model.md](./architecture/sales-service-data-model.md). Contracts live under [modules/](./modules/README.md); seed registration in `internal/seed/module_*.go`. Enablement remains Metadata-only (no customer PG migration).

`agents_starter` is an **always-on** package (seeded with `AUTO_SEED`) that **clones** AgentSpec templates to `ownership=custom`. Customers may define additional AgentSpecs anytime. See [modules/agents-starter.md](./modules/agents-starter.md) and [customer-agents.md](./customer-agents.md).

## Agent orientation

Data-architecture work should start at [`docs/architecture/README.md`](./architecture/README.md) and follow [`docs/architecture/agent-data-architecture.md`](./architecture/agent-data-architecture.md). For Sales/Service extensions, also read [sales-service-data-model.md](./architecture/sales-service-data-model.md) and [ADR-011](./adr/011-sales-service-managed-modules.md).

## Related

- [Architecture](./architecture.md)
- [Architecture docs index](./architecture/README.md)
- [Sales & Service data model](./architecture/sales-service-data-model.md)
- [ADR-011](./adr/011-sales-service-managed-modules.md)
- [ADR-031](./adr/031-billing-managed-module.md)
- [ADR-013](./adr/013-high-volume-flexible-storage.md) — high-volume / shared `records` scale
- [Managed modules](./modules/README.md)
- [Customer customizations](./customer-customizations.md)
- [Objects (public catalog)](./objects.md)
- [API families](./api-families.md)
- [ADR-004 ownership](./adr/004-three-api-families.md)
