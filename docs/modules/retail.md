# Module: `retail`

Optional managed module: **Retail loyalty and merchandising**.

See [industry packages](../architecture/cdm-industry-packages.md) for the curated object set.

## Dependency

- `core` (must be installed)

## Version

`1.0.0`

## Objects

LoyaltyProgram, LoyaltyAccount, LoyaltyCard, CustomerAsset, ProductBrand, ProductCategory, RetailAppointment, SurveyDefinition, SurveyResponse

Flexible `records` storage; `ownership=managed`, `package_name=retail`.

Does **not** redefine spine apiNames (`Account`, `Contact`, `Product`, `Case`, `Opportunity`, `Campaign`, `Asset`, …).

## Enable

```http
POST /metadata/v1/packages/retail/enable
```

## Soft-disable

```http
POST /metadata/v1/packages/retail/disable
```
