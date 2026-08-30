# Module: `sustainability`

Optional managed module: **Sustainability and emissions**.

See [industry packages](../architecture/cdm-industry-packages.md) for the curated object set.

## Dependency

- `core` (must be installed)

## Version

`1.0.0`

## Objects

Facility, EmissionsSource, EmissionFactor, Emission, Material, FuelType, BusinessTravel, EmployeeCommuting

Flexible `records` storage; `ownership=managed`, `package_name=sustainability`.

Does **not** redefine spine apiNames (`Account`, `Contact`, `Product`, `Case`, `Opportunity`, `Campaign`, `Asset`, …).

## Enable

```http
POST /metadata/v1/packages/sustainability/enable
```

## Soft-disable

```http
POST /metadata/v1/packages/sustainability/disable
```
