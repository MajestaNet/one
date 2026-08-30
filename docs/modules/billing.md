# Module: `billing`

Optional managed module: **Order from accepted Quote**. Ships in the product image; customer admins must enable it (after `catalog` and `sales`) before Client describe/CRUD expose billing objects.

QuoteLine remains the commercial source of truth. OrderLine is an **immutable snapshot** after the Order leaves `Draft`. See [ADR-031](../adr/031-billing-managed-module.md), [ADR-011](../adr/011-sales-service-managed-modules.md), and [billing-module-build-plan.md](../architecture/billing-module-build-plan.md).

## Dependency

- `core` (must be installed)
- `catalog` (must be enabled)
- `sales` (must be enabled)

## Version

`1.0.0` (`BillingPackageVersion` in `internal/seed`).

## Objects

| Object | Fields | Notes |
|---|---|---|
| Order | Name, OrderNumber (autonumber), Status (required); AccountId, ContactId; OpportunityId; QuoteId; PriceListId; CurrencyCode; Subtotal, TaxAmount, ShippingAmount, TotalAmount; Billing\* / Shipping\* scalars; EffectiveDate, ActivatedAt, Description | Accepted commitment / fulfillment. Party ≥1. |
| OrderLine | OrderId (master_detail), ProductId (required); QuoteLineId; PriceListEntryId; UnitId; LineNumber; Quantity; ListPrice; UnitPrice; DiscountPercent; Amount; Description; PriceSource | Snapshot of QuoteLine. Mutable only while parent Status=`Draft`. |

Flexible `records` storage; `ownership=managed`, `package_name=billing`.

### Field extensions

| Object | Field | Type | Notes |
|---|---|---|---|
| Quote | OrderId | lookup → Order | Set by `quote.accept` when `createOrder` succeeds. |

### Omitted (by design)

| Omitted | Reason |
|---|---|
| Invoice / Payment / CreditMemo | Later ADR (billing v2) |
| Tax engine / revenue recognition | Not a kernel calculator |
| Subscription schedules | Future; Product.ProductType is informational only |
| CDM `customerId` / `OrderProduct` apiName | Majesta One party model 2A + ADR-011 `OrderLine` |

## Platform actions

`quote.accept` is registered on **`sales`** (not this pack). Creating an Order requires this pack:

| apiName | Requires | Optional | Sync | Notes |
|---|---|---|---|---|
| `quote.accept` | `sales`, `catalog` | `billing` (`createOrder`) | yes | Client `POST /client/v1/actions/quote.accept`; guest `ctx.invokeAction`. |

Do not add `POST /acceptQuote`.

## Relationships

```text
Account/Contact ← Order ← OrderLine → Product / PriceListEntry / Unit
Quote.OrderId → Order
Order.QuoteId → Quote
OrderLine.QuoteLineId → QuoteLine
```

## Enable

```http
POST /metadata/v1/packages/billing/enable
Authorization: Bearer <admin metadata token>
```

Idempotent. Requires `catalog` and `sales` installed. Grants Admin object CRUD for Order and OrderLine. Adds `Quote.OrderId`.

**No Postgres migration.**

## Soft-disable

```http
POST /metadata/v1/packages/billing/disable
```

Stops future additive upgrades for `billing`. Does **not** delete billing metadata or records. `quote.accept` with `createOrder: true` then returns `409 PACKAGE_NOT_ENABLED`.

## AuthZ

After enable, Admin has full CRUD. Assign permission sets for non-admin users (object + optional field permissions). `quote.accept` uses the caller / run-as object+FLS+sharing on every touched record.

## Related

- [sales.md](./sales.md) — Quote / QuoteLine; owns `quote.accept`
- [catalog.md](./catalog.md) — Product / PriceList
- [cdm-mapping.md](../architecture/cdm-mapping.md)
