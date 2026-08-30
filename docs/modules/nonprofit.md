# Module: `nonprofit`

Optional managed module: **Nonprofit fundraising and delivery**.

See [industry packages](../architecture/cdm-industry-packages.md) for the curated object set.

## Dependency

- `core` (must be installed)

## Version

`1.0.0`

## Objects

Designation, DonorCommitment, Award, Disbursement, BenefitRecipient, DeliveryFramework, Indicator, Budget

Flexible `records` storage; `ownership=managed`, `package_name=nonprofit`.

Does **not** redefine spine apiNames (`Account`, `Contact`, `Product`, `Case`, `Opportunity`, `Campaign`, `Asset`, …).

## Enable

```http
POST /metadata/v1/packages/nonprofit/enable
```

## Soft-disable

```http
POST /metadata/v1/packages/nonprofit/disable
```
