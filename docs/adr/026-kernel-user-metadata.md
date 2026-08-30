# ADR-026: Kernel User as a metadata object (`storage_mode=kernel`)

## Status

Accepted

## Context

[ADR-008](./008-core-data-model.md) keeps User on the kernel `users` table (AuthN integrity). [ADR-002](./002-hybrid-metadata-storage.md) stores customer custom fields as metadata + JSONB on flexible objects. Customers could extend Account/Contact that way but not User: there was no `metadata_objects` row, so `User.CostCenter__c` required a product migration and could not appear on Client principals.

B2B identity needs the Account pattern on User **without** moving principals into `records`.

## Decision

1. **User stays kernel DDL** (`users`). Never `records` / `records_hv`.
2. Seed a managed `core` object `User` with `storage_mode=kernel`.
3. Standard profile columns are managed `metadata_fields` with `kernel_column` mapping to `users.*`.
4. Customer fields are Metadata writes (`ownership=custom`); values live in `users.data JSONB`.
5. DataEngine refuses kernel CRUD/query (`User is not a flexible object`). Identity writes stay on `/client/v1/principals` (`identity.users`).
6. Client describe (`GET /client/v1/sobjects/User/describe`) and Metadata GET object are allowed. `/sobjects/User` CRUD is not.
7. Customer objects may not use `storage_mode=kernel`. Only managed seed may insert kernel objects.
8. FLS on principal GET/PATCH applies to **customer** User fields in v1. Standard kernel columns stay gated by `identity.users` (so directory admin is not blocked by deny stubs). Describe still FLS-filters all User fields.
9. Deploy promotes customer User **field metadata**, never `users` rows.
10. No records partition, sharing settings, or field projections on kernel objects.

## Consequences

- `users.data` and `metadata_fields.kernel_column` are kernel migrations (fleet-wide).
- SCIM custom attributes (dynamic schema), JIT claim maps, and install `provisioning` shipped with [BP-058](../../backlog/BP-058-user-identity-extension.md) (mitigated).
- IdP groups remain forbidden as AuthZ (ADR-006 / ADR-015).

## Related

- [user-identity-extension-build-plan.md](../architecture/user-identity-extension-build-plan.md)
- [ADR-008](./008-core-data-model.md) · [ADR-002](./002-hybrid-metadata-storage.md) · [ADR-017](./017-canonical-field-types.md)
