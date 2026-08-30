# Module: `crm_bridge`

Managed **bridge** package: cross-cloud lookup fields between Sales and Service. Ships in the product image.

Adds managed fields only — **no new objects**. Required because Metadata lookup sync calls `requireObject` on the referenced type; `Case.OpportunityId` cannot live inside `service` alone when Opportunity is absent. See [ADR-011](../adr/011-sales-service-managed-modules.md).

## Auto-enable

`crm_bridge` has `AutoEnable=true`. Majesta One **enables it automatically** once both `sales` and `service` are enabled (on the enable that completes the pair, and again on boot migrate). Customers do **not** need to call `/packages/crm_bridge/enable`.

Manual soft-disable is rejected while the bridge is automatic — disable `sales` or `service` instead if you want to stop upgrades of a parent cloud.

## Dependency

- `sales` (must be enabled)
- `service` (must be enabled)

(Transitively requires `core` and `catalog`.)

## Version

`1.0.0` (`CrmBridgePackageVersion` in `internal/seed`).

## Fields

| Object | Field | Type | Notes |
|---|---|---|---|
| Case | OpportunityId | lookup → Opportunity | Links a support case to a related deal. Indexed / filterable. |

`ownership=managed`, `package_name=crm_bridge` on the field definition.

### Deferred (optional later bridge fields)

- `WorkOrder.OpportunityId` → Opportunity

Do not add reverse `Opportunity.PrimaryCaseId` in v1 (one-directional Case→Opportunity is enough).

## Enable

Prefer enabling `sales` and `service`; the bridge follows automatically.

Manual enable remains idempotent if needed:

```http
POST /metadata/v1/packages/crm_bridge/enable
Authorization: Bearer <admin metadata token>
```

**No Postgres migration.**

## Soft-disable

Not supported for auto-enable bridges (`DisablePackage` returns an error). Soft-disable `sales` and/or `service` instead.

## AuthZ

Field-level security follows Case object permissions. Admin already has Case access from `service` enable.

## Related

- [sales.md](./sales.md) / [service.md](./service.md)
- [sales-service-data-model.md](../architecture/sales-service-data-model.md)
