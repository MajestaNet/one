# Module: `address`

Optional managed module: multi-address rows for Account and Contact. Primary billing/shipping/mailing scalars remain on `core` Account/Contact; use this package when customers need multiple addresses per party.

See [ADR-020](../adr/020-cdm-managed-packages.md) and [cdm-mapping.md](../architecture/cdm-mapping.md).

## Dependency

- `core` (must be installed)

## Version

`1.0.0` (`AddressPackageVersion` in `internal/seed`).

## Objects

| Object | Fields | Notes |
|---|---|---|
| Address | Name (required), Street, City, State, PostalCode, Country, AddressType (Billing/Shipping/Mailing/Other), IsPrimary, AccountId, ContactId | At least one of AccountId / ContactId expected by convention |

Flexible `records` storage; `ownership=managed`, `package_name=address`.

## Enable

```http
POST /metadata/v1/packages/address/enable
```

## Soft-disable

```http
POST /metadata/v1/packages/address/disable
```
