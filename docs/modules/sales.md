# Module: `sales`

Optional managed module: **pipeline + quoting**. Ships in the product image; customer admins must enable it (after `catalog`) before Client describe/CRUD expose sales objects.

Quote-centric: **QuoteLine** is the commercial source of truth. No Lead inside this module (Lead lives in optional [`lead_marketing`](./lead-marketing.md)), no OpportunityLineItem, no Order (Order lives in optional [`billing`](./billing.md)), no sales Contract. See [ADR-011](../adr/011-sales-service-managed-modules.md), [ADR-020](../adr/020-cdm-managed-packages.md), [ADR-031](../adr/031-billing-managed-module.md), and [sales-service-data-model.md](../architecture/sales-service-data-model.md).

## Dependency

- `core` (must be installed)
- `catalog` (must be enabled)

## Version

`2.2.0` (`SalesPackageVersion` in `internal/seed`). Adds Quote address/amount snapshot fields, QuoteLine.UnitId, and platform action `quote.accept`.

## Objects

| Object | Fields | Notes |
|---|---|---|
| Opportunity | Name, StageName, CloseDate (required); Amount, Probability, IsClosed, IsWon, Type, LeadSource, NextStep, Description; PrimaryQuoteId; AccountId; ContactId | Pipeline hypothesis. **AccountId and/or ContactId** (≥1). |
| OpportunityContactRole | OpportunityId, ContactId (required); Role; IsPrimary | M:N Contact↔Opportunity. |
| Quote | Name, Status (required); ExpirationDate, BillingName, Subtotal, TaxAmount, ShippingAmount, TotalAmount, CurrencyCode, AcceptedAt, Description; Billing/Shipping address scalars; PriceListId; OpportunityId (optional); AccountId; ContactId | First-class commercial offer. Party ≥1. |
| QuoteLine | QuoteId (master_detail), ProductId (required); PriceListEntryId; UnitId; LineNumber; Quantity; ListPrice; UnitPrice; DiscountPercent; Amount; Description; PriceSource | Commercial SoT for offered lines. |
| Competitor | Name (required); Website; Strengths; Weaknesses; AccountId | Competitor firm |

Flexible `records` storage; `ownership=managed`, `package_name=sales`.

### Omitted (by design)

| Omitted | Reason |
|---|---|
| Lead | Lives in optional `lead_marketing` ([ADR-020](../adr/020-cdm-managed-packages.md)); convert is `lead.convert` on that pack ([ADR-029](../adr/029-platform-actions.md)), not a `sales` automation |
| OpportunityLineItem | Avoid second editable line master; use QuoteLine |
| Order / OrderItem | Lives in optional [`billing`](./billing.md) |
| Sales Contract | Deferred to future `agreements` |
| Forecast extras | Undesigned analytics surfaces |

## Platform actions

`sales` is an **optional package** for `lead.convert` (`createOpportunity`). Enabling this pack unlocks Opportunity creation on convert; disabling it fails that option with `409 PACKAGE_NOT_ENABLED`.

| apiName | Requires | Optional | Sync | Notes |
|---|---|---|---|---|
| `quote.accept` | `sales`, `catalog` | `billing` (`createOrder`) | yes | Client `POST /client/v1/actions/quote.accept`; guest `ctx.invokeAction`. Do not add `POST /acceptQuote`. |

## Relationships

```text
Account/Contact ← Opportunity ← OpportunityContactRole → Contact
Opportunity.PrimaryQuoteId → Quote
Quote (optional OpportunityId) → QuoteLine → Product / PriceListEntry
Quote.PriceListId → PriceList
```

## Enable

```http
POST /metadata/v1/packages/sales/enable
Authorization: Bearer <admin metadata token>
```

Idempotent. Requires `catalog` installed. Grants Admin object CRUD for Opportunity, OpportunityContactRole, Quote, QuoteLine.

**No Postgres migration.**

## Soft-disable

```http
POST /metadata/v1/packages/sales/disable
```

Stops future additive upgrades for `sales`. Does **not** delete sales metadata or records.

## AuthZ

After enable, Admin has full CRUD. Assign permission sets for non-admin users (object + optional field permissions).

## Related

- [catalog.md](./catalog.md) — Product / PriceList
- [crm-bridge.md](./crm-bridge.md) — Case.OpportunityId (**auto-enabled** when Service is also on)
- Future `cpq`, `agreements` attach per ADR-011
- [billing.md](./billing.md) — Order / OrderLine; `quote.accept` `createOrder`
