# Module: `automotive`

Optional managed module: **Automotive devices and deals**.

See [industry packages](../architecture/cdm-industry-packages.md) for the curated object set.

## Dependency

- `core` (must be installed)

## Version

`1.0.0`

## Objects

DeviceBrand, DeviceModel, Device, BusinessFacility, Deal, DealCustomer, DealDevice, DeviceInspection

Flexible `records` storage; `ownership=managed`, `package_name=automotive`.

Does **not** redefine spine apiNames (`Account`, `Contact`, `Product`, `Case`, `Opportunity`, `Campaign`, `Asset`, …).

## Enable

```http
POST /metadata/v1/packages/automotive/enable
```

## Soft-disable

```http
POST /metadata/v1/packages/automotive/disable
```
