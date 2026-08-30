# ADR-008: Core data model (User / Account / Contact)

## Status

Accepted

## Context

Majesta One needs a standard business model that every install shares. Earlier product language split managed seed into CRM and ERP packages with a wide object set (Lead, Opportunity, Product, Invoice, …). That blurred domain SKUs, made the fleet contract larger than necessary, and mixed optional commerce shapes into the always-on base.

Constraints:

- Hybrid storage (ADR-002): kernel DDL for system entities; flexible JSONB for business objects.
- SQL-native JSONB query path (ADR-003) is the performance strategy for Go + Postgres.
- Managed metadata upgrades with the product image; Deploy promotes only customer-owned artifacts (ADR-004).
- Changing a managed definition after customers depend on it is effectively a fleet-wide migration.

## Decision

1. **Ship managed package `core` only** — Account and Contact as flexible managed objects; User remains the kernel `users` identity table.
2. **Drop CRM/ERP product naming and SKU flags** — core always migrates with `AUTO_SEED`; optional domain packs are deferred.
3. **Account↔Contact is optional** — `Contact.AccountId` is a non-required lookup; customers may use either object alone or both.
4. **Keep flexible storage for Account/Contact** — do not promote them to typed kernel tables; rely on indexed metadata fields + the SQL planner for performance.
5. **Upgrade path** — remap legacy `crm` Account/Contact package rows to `core` (`0011`); **delete** former Lead / Opportunity / Activity / commerce managed objects and their records (`0012`).
6. **Document future Opportunity rule** — when reintroduced as a managed extension, it must link to Contact **or** Account (at least one); no other relationship enforcement in core.
7. **Record audit fields** — optional `OwnerId`; automatic immutable `CreatedById` / `LastModifiedById` ([ADR-009](./009-record-audit-authz-packaging.md)).
8. **Account/Contact field enrichment (additive)** — expand Account/Contact managed fields toward a richer standard attribute set without changing relationship rules, introducing a Party base, or polymorphic `parentCustomer` ([ADR-020](./020-cdm-managed-packages.md), [cdm-mapping.md](../architecture/cdm-mapping.md)). Bump `CorePackageVersion` when defs change.

## Consequences

- Smaller, clearer product contract; easier to keep immutable across customers.
- Extensions/installable managed packages must be designed cleanly later (BP-007 follow-up); they are not part of this decision’s implementation.
- Upgraded installs lose legacy managed object metadata and data for objects no longer in `core` (intentional cleanup).
- Treating User as a flexible record object remains **out of scope**. User is a kernel metadata object (`storage_mode=kernel`, customer values in `users.data`) per [ADR-026](./026-kernel-user-metadata.md). SCIM custom attributes and JIT claim maps shipped with [BP-058](../../backlog/BP-058-user-identity-extension.md).
- Core field lists grow over product releases via additive migrate only; prefer new fields over rename/type change ([ADR-020](./020-cdm-managed-packages.md)).
