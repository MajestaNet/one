# Module: `project_service`

Optional managed module: **Project service**.

See [industry packages](../architecture/cdm-industry-packages.md) for the curated object set.

## Dependency

- `core` (must be installed)

## Version

`1.0.0`

## Objects

Project, ProjectTask, Characteristic, BookableResource, TimeEntry, Expense, Estimate

Flexible `records` storage; `ownership=managed`, `package_name=project_service`.

Does **not** redefine spine apiNames (`Account`, `Contact`, `Product`, `Case`, `Opportunity`, `Campaign`, `Asset`, …).

## Enable

```http
POST /metadata/v1/packages/project_service/enable
```

## Soft-disable

```http
POST /metadata/v1/packages/project_service/disable
```
