# Module: `notes`

Optional managed module that adds a simple **Note** object. Ships in every product image; customer admins must enable it before Client describe/CRUD expose Note.

## Dependency

- `core` (must be installed)

## Version

`1.0.0` (see `internal/seed` / packages registry).

## Objects

| Object | Fields | Notes |
|---|---|---|
| Note | Title (text, required, indexed), Body (textarea), AccountId (optional lookup → Account) | Flexible `records` storage; `ownership=managed`, `package_name=notes` |

## Enable

```http
POST /metadata/v1/packages/notes/enable
Authorization: Bearer <admin metadata token>
```

Idempotent. Grants Admin permission-set object access for `Note`. Apps then see Note via `GET /client/v1/describe/Note`.

## Soft-disable

```http
POST /metadata/v1/packages/notes/disable
```

Stops future additive upgrades for `notes` on this install. Does **not** delete Note metadata or records.

## AuthZ

After enable, Admin has full CRUD. Assign a permission set with `objectPermissions` (and optional `fieldPermissions`) for non-admin users.
