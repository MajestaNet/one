# Module: `healthcare`

Optional managed module: **Clinical care**.

See [industry packages](../architecture/cdm-industry-packages.md) for the curated object set.

## Dependency

- `core` (must be installed)

## Version

`1.0.0`

## Objects

Patient, Practitioner, CarePlan, Encounter, Condition, AllergyIntolerance, Observation, MedicationRequest

Flexible `records` storage; `ownership=managed`, `package_name=healthcare`.

Does **not** redefine spine apiNames (`Account`, `Contact`, `Product`, `Case`, `Opportunity`, `Campaign`, `Asset`, …).

## Enable

```http
POST /metadata/v1/packages/healthcare/enable
```

## Soft-disable

```http
POST /metadata/v1/packages/healthcare/disable
```
