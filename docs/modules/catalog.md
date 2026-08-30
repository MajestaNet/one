# Module: `catalog`

Optional managed module: thin shared **product catalog** (sellable/supportable identity + list prices). Ships in the product image; customer admins must enable it before Client describe/CRUD expose catalog objects.

Designed to be **CPQ-ready but not CPQ** — configuration, bundles, and advanced pricing belong in a future `cpq` module. See [ADR-011](../adr/011-sales-service-managed-modules.md) and [sales-service-data-model.md](../architecture/sales-service-data-model.md).

## Dependency

- `core` (must be installed)

## Version

`2.2.0` (`CatalogPackageVersion` in `internal/seed`). Adds Product.ProductType.

## Objects

| Object | Fields | Notes |
|---|---|---|
| UnitGroup | Name (required), Description | Unit of measure group |
| Unit | Name (required), UnitGroupId (master_detail), Quantity, IsBaseUnit | Unit of measure |
| Product | Name (required), ProductCode, StockKeepingUnit, IsActive, Family, ProductType (Good / Service / Subscription), Description, ProductURL, QuantityUnitOfMeasureId (lookup→Unit) | Sellable/supportable SKU identity. No option/bundle fields. |
| PriceList | Name (required), IsActive, IsStandard, CurrencyCode, Description, BeginDate, EndDate | Named list-price container (not “Pricebook”). |
| PriceListEntry | ProductId (required), PriceListId (master_detail, required), UnitId (lookup→Unit), ListPrice (required), IsActive | One list amount per Product × PriceList. **Not** a pricing engine. |

Flexible `records` storage; `ownership=managed`, `package_name=catalog`.

### Explicit non-goals (this module)

ProductOption / Feature / Bundle graphs; config attributes; product/price rules; discount schedules; tier/usage matrices; inventory/BOM; cost accounting; subscription billing schedules.

## Relationships

- Unit → UnitGroup
- Product → Unit (optional UoM)
- PriceListEntry → Product, PriceList, Unit
- Consumed by: QuoteLine (`sales`), Asset / ContractLineItem (`service`), OrderLine (`billing`)

## Enable

```http
POST /metadata/v1/packages/catalog/enable
Authorization: Bearer <admin metadata token>
```

Idempotent. Grants Admin permission-set object access for catalog objects. Apps then see objects via `GET /client/v1/describe/{Object}`.

**No Postgres migration** — enable is additive managed metadata sync only.

## Soft-disable

```http
POST /metadata/v1/packages/catalog/disable
```

Stops future additive upgrades for `catalog` on this install. Does **not** delete catalog metadata or records.

## AuthZ

After enable, Admin has full CRUD. Assign a permission set with `objectPermissions` (and optional `fieldPermissions`) for non-admin users.

## Related

- [sales.md](./sales.md) / [service.md](./service.md) depend on this module
- [Attribute mapping](../architecture/cdm-mapping.md)
- Future `cpq` extends Product / QuoteLine without forking this package
