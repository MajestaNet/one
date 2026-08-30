# ADR-031: Billing managed module (Order from accepted Quote)

## Status

Accepted (implementation phased — see [billing-module-build-plan.md](../architecture/billing-module-build-plan.md))

## Context

[ADR-011](./011-sales-service-managed-modules.md) named a future optional `billing` module (Order, OrderLine; Invoice later) depending on `catalog` + `sales`. QuoteLine is the commercial source of truth; Order was deferred so Majesta One would not ship three editable line masters.

[BP-044](../../backlog/BP-044-billing-module-order-from-quote.md) records the gap: without `billing`, accepted Quote has no in-product path to a fulfillment commitment. Headless quote-to-cash integrations invent customer Order objects. [ADR-029](./029-platform-actions.md) already reserved `quote.accept` on the Client actions catalog.

Microsoft CDM `crmCommon/sales/Order` is “Quote that has been accepted.” Majesta One maps that semantic with the same curated CDM policy as other packs ([ADR-020](./020-cdm-managed-packages.md)): hand-authored seed, PascalCase apiNames, no polymorphic `customerId`, no wholesale sales-folder dump, no `operationsCommon`.

## Decision

### 1. Optional module `billing`

| | |
|---|---|
| Package | `billing` v1.0.0 |
| Depends on | `catalog`, `sales` (transitively `core`) |
| Enable | `POST /metadata/v1/packages/billing/enable` — not AutoEnable, not Deploy |
| Storage | Flexible `records` + metadata sync; no customer Postgres migration |
| Objects | **Order** (ownable peer), **OrderLine** (master_detail → Order) |

Do **not** put Order inside `sales`. Do **not** reintroduce purged always-on Order/Invoice/Payment from migration `0012`.

### 2. QuoteLine remains commercial SoT; OrderLine is a snapshot

- `quote.accept` copies managed standard fields from Quote / QuoteLine onto Order / OrderLine.
- After Order leaves `Draft`, OrderLine create/update/delete is rejected by DataEngine.
- Customer `__c` fields are not copied (wrapping automation, same as `lead.convert`).

### 3. Platform action `quote.accept`

Owned by `sales`. Requires `sales` + `catalog`. Optional pack `billing` gates `createOrder`.

- HTTP: `POST /client/v1/actions/quote.accept` only — no `/acceptQuote`.
- Guest: existing `ctx.invokeAction`. Sync and `syncSafe`.
- `createOrder` defaults to `true` when `billing` is enabled, else `false`. Explicit `true` without `billing` → `409 PACKAGE_NOT_ENABLED`.

### 4. Party model (2A)

Order and Quote require AccountId **and/or** ContactId (≥1). Same rule is enforced on Opportunity. No CDM `customerId` / `customerIdType`.

### 5. Cross-package lookup

`Quote.OrderId` is a `billing` **FieldExtension** on Quote (owned by `sales`), same `requireObject` pattern as `crm_bridge`.

### 6. Direct Draft Orders

Client CRUD may create Orders with Status=`Draft` (iPaaS / historical load). Lines are editable only while the parent is `Draft`. `quote.accept` creates Draft + lines then patches Status=`Activated` in one transaction.

### 7. Curated CDM mapping (not full CDM)

Document field provenance in [cdm-mapping.md](../architecture/cdm-mapping.md). Majesta One names stay PascalCase and match existing Quote/Account scalars (`BillingStreet`, `PriceListId`, `UnitPrice`, `OrderLine` not CDM `OrderProduct`).

### 8. Explicit deferral

Invoice, Payment, CreditMemo, tax engine, revenue recognition, subscription schedules, CPQ, sales Contract (`agreements`), Control IDE Quote/Order chrome.

## Consequences

- Customers enable `billing` when they need quote-to-order. Quoting without Orders remains valid.
- ADR-011 `billing` status is Optional (this ADR), not Future.
- ADR-020 §7: Order ships in `billing`; Invoice still deferred.
- Opportunity/Quote/Order party ≥1 is now a DataEngine write rule (ADR-011 follow-up closed for those objects).

## Related

- Build plan: [billing-module-build-plan.md](../architecture/billing-module-build-plan.md)
- [ADR-011](./011-sales-service-managed-modules.md) · [ADR-020](./020-cdm-managed-packages.md) · [ADR-029](./029-platform-actions.md)
- [BP-044](../../backlog/BP-044-billing-module-order-from-quote.md) · [BP-061](../../backlog/BP-061-platform-actions.md)
- Module: [billing.md](../modules/billing.md)
