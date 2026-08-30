# Module: `financial_services`

Optional managed module: **Banking and insurance**.

See [industry packages](../architecture/cdm-industry-packages.md) for the curated object set.

## Dependency

- `core` (must be installed)

## Version

`1.0.0`

## Objects

Bank, Branch, FinancialProduct, Collateral, Claim, Coverage, Limit, MortgageApplication, KYC

Flexible `records` storage; `ownership=managed`, `package_name=financial_services`.

Does **not** redefine spine apiNames (`Account`, `Contact`, `Product`, `Case`, `Opportunity`, `Campaign`, `Asset`, …).

## Enable

```http
POST /metadata/v1/packages/financial_services/enable
```

## Soft-disable

```http
POST /metadata/v1/packages/financial_services/disable
```
