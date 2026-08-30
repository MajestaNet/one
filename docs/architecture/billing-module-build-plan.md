# Billing module — build plan

Executable plan for optional managed **`billing`**: Order + OrderLine as an immutable snapshot from accepted Quote, invoked via platform action `quote.accept`.

**ADR:** [ADR-031](../adr/031-billing-managed-module.md)  
**Backlog:** [BP-044](../../backlog/BP-044-billing-module-order-from-quote.md)  
**Playbooks:** [agent-data-architecture.md](./agent-data-architecture.md) · [platform-actions-build-plan.md](./platform-actions-build-plan.md) · [agent-api-families.md](./agent-api-families.md)  
**Domain agents:** `db-backend-perf` then `api-families`  
**Related:** [ADR-011](../adr/011-sales-service-managed-modules.md) · [ADR-020](../adr/020-cdm-managed-packages.md) · [ADR-029](../adr/029-platform-actions.md) · [BP-061](../../backlog/BP-061-platform-actions.md)

---

## Thesis

> Quote-to-cash is a **commercial close-loop**, not an ERP. `billing` ships Order / OrderLine only. `quote.accept` is product Go on the existing Client catalog. QuoteLine stays the commercial SoT; OrderLine is a frozen snapshot after the Order leaves Draft.

```text
Client / guest invokeAction
        │
        ▼
POST /client/v1/actions/quote.accept
        │
        ▼
internal/actions.acceptQuote
        ├─ sales + catalog enabled?
        ├─ createOrder → billing enabled?
        ├─ AuthZ on Quote / QuoteLine / Order / OrderLine
        └─ DataEngine tx: Draft Order + lines → Activated; Quote.Status=Accepted
```

---

## Locked product decisions

| Topic | Choice |
|---|---|
| Package | Optional `billing`; `POST /metadata/v1/packages/billing/enable` |
| Objects | Order (ownable peer), OrderLine (master_detail → Order) |
| Line SoT | QuoteLine; OrderLine immutable after Order leaves Draft |
| Action | `quote.accept` on `sales`; Requires `sales`+`catalog`; Optional `billing` for `createOrder` |
| HTTP | `POST /client/v1/actions/quote.accept` only |
| Storage | Flexible `records` + metadata sync; no customer PG migration |
| Party | Opportunity, Quote, Order: AccountId **and/or** ContactId (≥1) |
| Direct Orders | Client may create Draft Orders; lines editable only while parent Status=`Draft` |
| Invoice | Later ADR |
| IDE | No `tools/control-ide` edits |
| CDM | Curated mapping; not full `crmCommon/sales` Order dump |

### Order / OrderLine field contract (v1)

**Order:** `Name` (required), `OrderNumber` (autonumber `ORD-{00000}`, searchable), `Status` (`Draft \| Activated \| Fulfilled \| Cancelled`, required), `AccountId` / `ContactId` (≥1), optional `OpportunityId`, `QuoteId` (required when created by accept), `PriceListId`, `CurrencyCode`, `Subtotal`, `TaxAmount`, `ShippingAmount`, `TotalAmount`, Billing/Shipping address scalars, `EffectiveDate`, `ActivatedAt`, `Description`. `OwnerId` is the system column.

**OrderLine:** `OrderId` (master_detail, required), `QuoteLineId`, `ProductId` (required), optional `PriceListEntryId` / `UnitId`, `LineNumber`, `Quantity`, `ListPrice`, `UnitPrice`, `DiscountPercent`, `Amount`, `Description`, `PriceSource`.

### Sales / catalog additive fields

**Quote:** Billing/Shipping address scalars, `CurrencyCode`, `TaxAmount`, `ShippingAmount`, `AcceptedAt`. `Quote.OrderId` is a `billing` FieldExtension.

**QuoteLine:** optional `UnitId` → Unit.

**Product:** `ProductType` picklist `Good \| Service \| Subscription` (informational).

### `quote.accept` contract

**Input:** `{ quoteId` (required), `createOrder?` boolean `}`

- `createOrder` default: `true` when `billing` is enabled, else `false`.
- `createOrder: true` without `billing` → `409 PACKAGE_NOT_ENABLED` `{ packageName: "billing", option: "createOrder" }`.

**Rules (sync, syncSafe):** Quote readable; Status in `Draft \| Presented`; ≥1 QuoteLine; party ≥1. Copy managed standard fields only. Set Quote.Status=`Accepted`, `AcceptedAt=now`. If Order created: Draft Order + lines then Status=`Activated`. Idempotent already-accepted path.

**Output:** `{ quoteId, orderId?, alreadyAccepted }`.

**Immutability:** DataEngine rejects OrderLine create/update/delete when parent Order.Status ≠ `Draft`. Accept creates Draft + lines then patches Activated in one tx.

---

## Phases

### Phase 0 — Contracts

ADR-031, this file, [billing.md](../modules/billing.md), DAG/spine/CDM/playbook/backlog amendments. **Done.**

### Phase 1 — Seed

`internal/seed/module_billing.go`; `FieldDef` autonumber options; sales/catalog additive fields; enable tests. **Done.**

### Phase 2 — Write path

Party ≥1 for Opportunity/Quote/Order; OrderLine freeze unless parent Draft. **Done.**

### Phase 3 — `quote.accept`

`ActionDef` on `sales`; Go handler; HTTP + guest tests (same catalog as `lead.convert`). **Done.**

### Phase 4 — Close

BP-044 mitigated; BP-061 Phase 5 `quote.accept` done; targeted `go test`. **Done.**

---

## Non-goals (this delivery)

Invoice/Payment/CreditMemo; tax engine; subscriptions; CPQ; `agreements`; `operationsCommon`; Control IDE chrome; per-verb HTTP routes; AutoEnable of `billing`.

## Related

- [sales-service-data-model.md](./sales-service-data-model.md)
- [platform-actions-build-plan.md](./platform-actions-build-plan.md)
- [cdm-mapping.md](./cdm-mapping.md)
- [modules/billing.md](../modules/billing.md)
