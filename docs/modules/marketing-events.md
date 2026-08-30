# Module: `marketing_events`

Optional managed module: **Marketing events and journeys**.

See [industry packages](../architecture/cdm-industry-packages.md) for the curated object set.

## Dependency

- `core` (must be installed)

## Version

`1.0.0`

## Objects

MarketingEvent, Building, Hotel, EventVendor, AttendeePass, EventRegistration, CustomerJourney

Flexible `records` storage; `ownership=managed`, `package_name=marketing_events`.

Does **not** redefine spine apiNames (`Account`, `Contact`, `Product`, `Case`, `Opportunity`, `Campaign`, `Asset`, …).

## Enable

```http
POST /metadata/v1/packages/marketing_events/enable
```

## Soft-disable

```http
POST /metadata/v1/packages/marketing_events/disable
```
