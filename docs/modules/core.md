# Module: `core`

Always-on managed package. Seeded on every install when `AUTO_SEED=1`. Not listed as optional in the enable catalog for installation (already present), but appears in package status for version tracking.

## Dependency

None (root package).

## Version

Recorded in `package_installs` as `core`. Bump `CorePackageVersion` in `internal/seed` when definitions change (currently `2.1.0`).

## Objects

| Object | Storage | Notes |
|---|---|---|
| User | Kernel `users` (`storage_mode=kernel`) | Identity metadata object; customer fields in `users.data`; not a Client record object CRUD target |
| Account | `records` JSONB | Organization / party |
| Contact | `records` JSONB | Person; optional `AccountId` |

See [data-model.md](../data-model.md) for standard fields and relationship rules. Party rules: Account↔Contact remains optional; no Party base object ([ADR-020](../adr/020-cdm-managed-packages.md)).

## Platform actions

None in v1. `record.merge` is reserved on `core` for [BP-046](../../backlog/BP-046-record-merge-dedupe.md) and must use Client `POST /client/v1/actions/record.merge` when implemented.

## Enablement

Automatic via product seed. Do not call `/packages/core/enable` as a substitute for `AUTO_SEED` on a fresh install.
