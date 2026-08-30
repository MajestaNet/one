# Module: `service`

Optional managed module: **customer service** (cases, assets, entitlements, support agreements, work orders). Ships in the product image; customer admins must enable it (after `catalog`) before Client describe/CRUD expose service objects.

See [ADR-011](../adr/011-sales-service-managed-modules.md), [ADR-020](../adr/020-cdm-managed-packages.md), and [sales-service-data-model.md](../architecture/sales-service-data-model.md).

## Dependency

- `core` (must be installed)
- `catalog` (must be enabled) — Asset and ContractLineItem reference Product

## Version

`2.0.0` (`ServicePackageVersion` in `internal/seed`).

## Objects

| Object | Fields | Notes |
|---|---|---|
| Case | Subject, Status (required); Origin, Priority, Type, Reason, IsEscalated, Description; AccountId, ContactId, AssetId, EntitlementId, ParentId | Support ticket. |
| CaseComment | ParentId (master_detail), Body (required), IsPublished | Parent-controlled thread note. |
| Asset | Name (required); Status, SerialNumber, InstallDate, PurchaseDate, Description; AccountId, ProductId (required); ContactId | Installed/owned product instance. |
| Entitlement | Name, AccountId (required); StartDate, EndDate, Status; AssetId; ServiceContractId | Support right / SLA instance. **No** EntitlementProcess. |
| ServiceContract | Name, AccountId (required); Status, StartDate, EndDate | Support entitlement agreement — not sales CLM. |
| ContractLineItem | ServiceContractId (master_detail, required); ProductId; AssetId | |
| WorkOrder | Subject, Status (required); Priority, Description; AccountId, ContactId, CaseId, AssetId, EntitlementId, ServiceContractId | Field/service work. |

Flexible `records` storage; `ownership=managed`, `package_name=service`.

### Omitted (by design)

Knowledge articles; EntitlementProcess / milestones; ServiceAppointment scheduling graph; email / files / campaigns / actions.

## Relationships

```text
Account/Contact/Asset/Entitlement ← Case → CaseComment
Account + Product ← Asset
Account (+ Asset / ServiceContract) ← Entitlement
Account ← ServiceContract → ContractLineItem → Product / Asset
Case / Asset / Account ← WorkOrder
```

Cross-cloud: [crm-bridge.md](./crm-bridge.md) auto-enables `Case.OpportunityId` when Sales is also enabled.

## Enable

```http
POST /metadata/v1/packages/service/enable
Authorization: Bearer <admin metadata token>
```

Idempotent. Requires `catalog` installed. Grants Admin object CRUD for all service objects above.

**No Postgres migration.**

## Soft-disable

```http
POST /metadata/v1/packages/service/disable
```

Stops future additive upgrades for `service`. Does **not** delete service metadata or records.

## AuthZ

After enable, Admin has full CRUD. Assign permission sets for non-admin users (object + optional field permissions).

## Related

- [catalog.md](./catalog.md) — Product
- [crm-bridge.md](./crm-bridge.md) — Case ↔ Opportunity
- [sales.md](./sales.md) — optional companion module
