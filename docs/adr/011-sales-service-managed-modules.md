# ADR-011: Sales & Service managed modules

## Status

Accepted

## Context

Majesta One ships a thin always-on `core` package (User / Account / Contact) per [ADR-008](./008-core-data-model.md). Always-on `agents_starter` seeds day-one AgentSpec templates. Optional managed modules (`notes`, catalog/sales/service, …) enable via Metadata API with additive `Sync*Managed` — no customer Postgres DDL ([ADR-002](./002-hybrid-metadata-storage.md), [BP-007](./020-cdm-managed-packages.md)).

Product strategy needs optional managed **Sales** and **Service** capabilities as optional managed modules that:

1. Customers can enable in any combination (including all together) without PG migration.
2. Leave room for future enterprise **CPQ**, billing/order, and sales agreements without forking Product or inventing a second commercial line master.
3. Stay interoperable with each other and with future modules via a shared party + catalog spine.

Classic CRM shapes create known traps: a fat product catalog that embeds CPQ; Lead convert pipelines duplicating Account/Contact; and three overlapping commercial documents (Quote / Order / Contract) with three editable line masters (OpportunityLine ≈ QuoteLine ≈ OrderItem).

## Decision

### 1. Package DAG

| Package | Depends on | Role | Status |
|---|---|---|---|
| `core` | — | User, Account, Contact | Always-on (ADR-008) |
| `address` | `core` | Multi-address rows | Optional (ADR-020) |
| `activities` | `core` | Task, Appointment, PhoneCall, Email | Optional (ADR-020) |
| `lead_marketing` | `core` | Lead, Campaign, MarketingList | Optional (ADR-020) |
| `catalog` | `core` | Product, PriceList, PriceListEntry, Unit, UnitGroup | Optional (this ADR + ADR-020) |
| `sales` | `core`, `catalog` | Opportunity, OpportunityContactRole, Quote, QuoteLine (+ Competitor) | Optional (this ADR + ADR-020) |
| `service` | `core`, `catalog` | Case, CaseComment, Asset, Entitlement, ServiceContract, ContractLineItem, WorkOrder | Optional (this ADR + ADR-020) |
| `crm_bridge` | `sales`, `service` | Cross-cloud lookup fields only | Optional **AutoEnable** (this ADR) |
| `cpq` | `catalog`, `sales` | Options/Features/Rules; extends QuoteLine | Future |
| `billing` | `catalog`, `sales` | Order, OrderLine (+ later Invoice) | Optional ([ADR-031](./031-billing-managed-module.md)) |
| `agreements` | `core` (+ optional sales) | Sales Contract / Agreement | Future |

Canonical narrative: [sales-service-data-model.md](../architecture/sales-service-data-model.md). Module contracts: [docs/modules/](../modules/README.md).

### 2. Thin catalog (CPQ-ready, not CPQ)

`catalog` holds **sellable/supportable identity + list prices only**:

- **Product** — SKU identity (Name, ProductCode, IsActive, Family)
- **PriceList** — named list-price container (Majesta One naming; not legacy “Pricebook” naming)
- **PriceListEntry** — one list amount for Product × PriceList

**Must not** land in `catalog`: ProductOption / Feature / Bundle graphs, config attributes, product/price rules, discount schedules, tier/usage matrices, inventory/BOM, subscription billing schedules. Those belong in a future `cpq` module that depends on `catalog` + `sales` and attaches via additive managed objects/fields only.

List price is **not** the pricing engine. Commercial lines (`QuoteLine`) store frozen offer amounts and `PriceSource` ∈ `PriceList | Manual | Contract | CpqRule | External` so CPQ can override without forking Product.

### 3. No Lead in `sales` / always-on `core`

Managed **`sales` omits Lead**. Pipeline entry is Opportunity with Account and/or Contact. Pre-qualification may use customer-owned metadata, early Opportunity stages, or the optional **`lead_marketing`** package ([ADR-020](./020-cdm-managed-packages.md)). Legacy always-on Lead remains purged (`0012`). Lead→Account/Contact/Opportunity convert is the platform action `lead.convert` ([ADR-029](./029-platform-actions.md)) owned by `lead_marketing`, with Opportunity creation gated on `sales` — not a `sales` package automation.

### 4. Quote-centric sales; Order and sales Contract deferred

| Object | Role | In `sales` v1? |
|---|---|---|
| Opportunity | Pipeline hypothesis | Yes |
| OpportunityContactRole | Contact ↔ Opportunity M:N | Yes |
| Quote | First-class commercial offer | Yes |
| QuoteLine | Commercial source of truth for lines/prices | Yes |
| OpportunityLineItem | Second line master | **No** |
| Order / OrderItem | Accepted commitment / fulfillment | **No** in `sales` → optional `billing` ([ADR-031](./031-billing-managed-module.md)) |
| Sales Contract | Termed legal/commercial agreement | **No** → future `agreements` |

Improvements vs classic CRM platforms:

- Quote is first-class: party required (AccountId and/or ContactId); **optional** OpportunityId; optional PriceListId.
- `Opportunity.PrimaryQuoteId` documents which Quote drives the deal.
- QuoteLine is the only managed sellable line shape in v1; Opportunity.Amount is manual or later primary-Quote rollup — not a second line table.
- OrderLine (`billing`) is an immutable snapshot from accepted QuoteLine, not a third live editor.
- **ServiceContract** (support entitlement agreement) stays in `service` and is not sales CLM.

### 5. Opportunity party rule

When `sales` is enabled, Opportunity **must** reference Contact **or** Account (at least one) — the contract documented in [data-model.md](../data-model.md) / ADR-008. Seed ships AccountId/ContactId as optional lookups; enforce ≥1 via validation rule or write path as a follow-up.

### 6. Service module

Optional `service` depends on `core` + `catalog`. Objects: Case, CaseComment, Asset, Entitlement, ServiceContract, ContractLineItem, WorkOrder. Asset and ContractLineItem reference Product. Omit EntitlementProcess, Knowledge, email/files/campaigns.

### 7. Cross-cloud bridge

Lookup field sync requires the referenced object to exist (`requireObject`). Optional FKs such as `Case.OpportunityId` live in `crm_bridge` (depends on `sales` + `service`), not inside either cloud alone. v1 bridge field: `Case.OpportunityId` only.

`crm_bridge` is **`AutoEnable`**: Majesta One enables it automatically when both `sales` and `service` are enabled (and on boot migrate). Customers do not manually enable/disable bridges; soft-disable of an AutoEnable package is rejected.

### 8. Enablement and storage

- All business objects remain flexible `records` + metadata ([ADR-002](./002-hybrid-metadata-storage.md)).
- Enabling any module is Metadata `POST /packages/{name}/enable` → additive `Sync*Managed` + `package_installs` — **never** a customer Postgres migration.
- Soft-disable stops upgrades; does not delete metadata/records.
- No Person-Account dual-write; keep Account (org) + Contact (person).
- Seed Go registration lives in `internal/seed/module_{catalog,sales,service,crm_bridge,billing,address,activities,lead_marketing}.go`. Bridge modules may declare `FieldExtensions` (managed fields on dependency-owned objects) without `SyncObjectManaged` of the host object. `billing` extends Quote with `OrderId`.

### 9. Explicit omit list

Lead **inside `sales` / `core`** (Lead/Campaign/MarketingList may ship in optional `lead_marketing` per ADR-020); OpportunityLineItem; Order/OrderItem inside `sales` (Order lives in optional `billing` — [ADR-031](./031-billing-managed-module.md)); sales Contract (until `agreements`); package **Actions** as customer-uneditable TS inside `sales` (integrity verbs are [ADR-029](./029-platform-actions.md) platform actions on the **owning** pack, e.g. `lead.convert` on `lead_marketing`, `quote.accept` on `sales`); Files; Knowledge; EntitlementProcess / milestones; CPQ configuration objects inside `catalog`. Activity email/phone/task shapes ship in optional `activities` (record shapes only — not a product mailer).

**Messages** as a CRM channel object are **retired** ([ADR-032](./032-retire-messages-polymorphic-lookup.md)). Do not reintroduce `messages` / `polymorphic_lookup` as part of Sales/Service clouds.

### 9b. Standard attribute targets

`catalog` / `sales` / `service` evolve toward curated standard attribute sets in place ([ADR-020](./020-cdm-managed-packages.md)). Do not register a second Product/Opportunity/Case under alternate package names. Mapping: [cdm-mapping.md](../architecture/cdm-mapping.md).

### 10. No in-kernel email / SMTP (system or CRM)

Majesta One **does not** store SMTP credentials or ship SES/SendGrid/Mailgun clients for product-owned send. Email is an identity attribute and a field type only. Admin/system contact uses outbox → customer webhooks / connectors ([BP-038](../../backlog/BP-038-no-product-mailer-byo-alerts.md)); CRM email is an `activities` Email record shape, not a kernel mailer. Do not introduce a kernel mailer without a new ADR.

## Consequences

- Sales-only, Service-only, and Sales+Service+catalog+bridge installs are all valid.
- Future `cpq` / `billing` / `agreements` attach without schema forks or PG migrations for business objects.
- Product naming stays `PriceList` / `PriceListEntry` (not Pricebook).
- Agents must follow [sales-service-data-model.md](../architecture/sales-service-data-model.md) and module contracts; do not invent parallel CRM/ERP package names as always-on `core`.
- Remaining follow-ups: ADRs for `cpq` / `agreements` and Invoice/Payment (billing v2); `record.merge` ([ADR-029](./029-platform-actions.md)). Opportunity/Quote/Order party ≥1 and `quote.accept` ship with [ADR-031](./031-billing-managed-module.md).
